package engineer_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef/engineer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClaude struct {
	calls   []claudeCall
	outputs []claudeOutput
}
type claudeCall struct{ Workdir, Prompt string }
type claudeOutput struct {
	Output string
	Err    error
}

func (f *fakeClaude) Run(_ context.Context, workdir, prompt string) (string, error) {
	f.calls = append(f.calls, claudeCall{workdir, prompt})
	if len(f.outputs) == 0 {
		return "", nil
	}
	out := f.outputs[0]
	f.outputs = f.outputs[1:]
	return out.Output, out.Err
}

type fakeShell struct {
	calls   []shellCall
	outputs map[string]shellOutput
}
type shellCall struct{ Workdir, Command string }
type shellOutput struct {
	Output string
	Err    error
}

func (f *fakeShell) Run(_ context.Context, workdir, command string) (string, error) {
	f.calls = append(f.calls, shellCall{workdir, command})
	if o, ok := f.outputs[command]; ok {
		return o.Output, o.Err
	}
	return "", nil
}

type fakeGit struct {
	commits map[string]int
	pushed  bool
	added   bool
	removed bool
	fetched bool
}

func (f *fakeGit) Fetch(_ context.Context, _, _, _ string) error {
	f.fetched = true
	return nil
}
func (f *fakeGit) WorktreeAdd(_ context.Context, _, _, _, _ string) error {
	f.added = true
	return nil
}
func (f *fakeGit) WorktreeRemove(_ context.Context, _, _ string) error {
	f.removed = true
	return nil
}
func (f *fakeGit) Push(_ context.Context, _, _, _ string) error {
	f.pushed = true
	return nil
}
func (f *fakeGit) CommitCount(_ context.Context, worktree, base string) (int, error) {
	_ = base
	return f.commits[worktree], nil
}

type fakeGH struct {
	pr       engineer.PRInfo
	prErr    error
	comments []string
}

func (f *fakeGH) CreatePR(_ context.Context, _ string, _ engineer.CreatePROpts) (engineer.PRInfo, error) {
	return f.pr, f.prErr
}
func (f *fakeGH) CommentIssue(_ context.Context, _, _ string, body string) error {
	f.comments = append(f.comments, body)
	return nil
}

func newDeps(claude *fakeClaude, sh *fakeShell, git *fakeGit, gh *fakeGH) engineer.Deps {
	return engineer.Deps{Claude: claude, Shell: sh, Git: git, GH: gh}
}

func writeBrigadeYAML(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, ".brigade.yml")
	require.NoError(t, writeBytes(path, []byte(`setup:
  - go mod download
checks:
  - go build ./...
  - go test ./...
`)))
}

func TestRunImplement_HappyPath(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "123")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{
		{Output: "implemented"},
		{Output: "PR body content"},
		{Output: "feat: add thing"},
	}}
	sh := &fakeShell{}
	git := &fakeGit{commits: map[string]int{layout.WorktreePath: 2}}
	gh := &fakeGH{pr: engineer.PRInfo{URL: "https://example.com/pr/456", Number: 456}}

	in := engineer.ImplementInput{
		TaskRef:       engineer.TaskRef{Source: "github", ID: "123"},
		Title:         "Fix things",
		Description:   "Body",
		Specification: "## Agent Brief\n...",
		RepoPath:      repoDir,
	}
	res, err := engineer.RunImplement(t.Context(), in, newDeps(claude, sh, git, gh))
	require.NoError(t, err)
	assert.Equal(t, 456, res.PR.Number)
	assert.Equal(t, "https://example.com/pr/456", res.PR.URL)
	assert.True(t, git.fetched, "expected fetch")
	assert.True(t, git.added, "expected worktree add")
	assert.True(t, git.pushed, "expected push")
	assert.Len(t, claude.calls, 3, "expected implement + body + title claude calls")
	assert.Empty(t, gh.comments, "no failure comment on happy path")
}
