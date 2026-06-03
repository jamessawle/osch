package engineer_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef/engineer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errExitNonZero = errors.New("exit status 1")

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
	dynamic func(command string) shellOutput
}
type shellCall struct{ Workdir, Command string }
type shellOutput struct {
	Output string
	Err    error
}

func (f *fakeShell) Run(_ context.Context, workdir, command string) (string, error) {
	f.calls = append(f.calls, shellCall{workdir, command})
	if f.dynamic != nil {
		o := f.dynamic(command)
		return o.Output, o.Err
	}
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

type fakeGitErr struct {
	fetchErr error
	addErr   error
	pushErr  error
	commits  map[string]int
}

func (f *fakeGitErr) Fetch(_ context.Context, _, _, _ string) error { return f.fetchErr }
func (f *fakeGitErr) WorktreeAdd(_ context.Context, _, _, _, _ string) error {
	return f.addErr
}
func (f *fakeGitErr) WorktreeRemove(_ context.Context, _, _ string) error { return nil }
func (f *fakeGitErr) Push(_ context.Context, _, _, _ string) error        { return f.pushErr }
func (f *fakeGitErr) CommitCount(_ context.Context, worktree, _ string) (int, error) {
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

type fakeGHCapture struct {
	pr      engineer.PRInfo
	capture *engineer.CreatePROpts
}

func (f *fakeGHCapture) CreatePR(_ context.Context, _ string, opts engineer.CreatePROpts) (engineer.PRInfo, error) {
	*f.capture = opts
	return f.pr, nil
}
func (f *fakeGHCapture) CommentIssue(_ context.Context, _, _ string, _ string) error { return nil }

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
  - go vet ./...
  - gofmt -l . | (! grep .)
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

func TestRunImplement_RetryThenPass(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "200")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{
		{Output: "attempt 1"},
		{Output: "attempt 2"},
		{Output: "PR body"},
		{Output: "feat: x"},
	}}
	failOnce := map[string]int{"go test ./...": 1}
	sh := &fakeShell{}
	sh.dynamic = func(cmd string) shellOutput {
		if cmd == "go test ./..." && failOnce[cmd] > 0 {
			failOnce[cmd]--
			return shellOutput{Output: "FAIL: TestSomething", Err: errExitNonZero}
		}
		return shellOutput{}
	}

	git := &fakeGit{commits: map[string]int{layout.WorktreePath: 1}}
	gh := &fakeGH{pr: engineer.PRInfo{URL: "u", Number: 1}}

	res, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "200"},
		Title:    "T",
		RepoPath: repoDir,
	}, newDeps(claude, sh, git, gh))
	require.NoError(t, err)
	assert.Equal(t, 1, res.PR.Number)

	require.GreaterOrEqual(t, len(claude.calls), 2)
	assert.Contains(t, claude.calls[1].Prompt, "go test ./...")
	assert.Contains(t, claude.calls[1].Prompt, "FAIL: TestSomething")
}

func TestRunImplement_ExhaustedRetries(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "201")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{
		{Output: "a1"}, {Output: "a2"}, {Output: "a3"},
	}}
	sh := &fakeShell{}
	sh.dynamic = func(cmd string) shellOutput {
		if cmd == "go test ./..." {
			return shellOutput{Output: "always fails", Err: errExitNonZero}
		}
		return shellOutput{}
	}
	git := &fakeGit{commits: map[string]int{layout.WorktreePath: 1}}
	gh := &fakeGH{}

	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "201"},
		Title:    "T",
		RepoPath: repoDir,
	}, newDeps(claude, sh, git, gh))
	require.Error(t, err)
	assert.Len(t, claude.calls, 3, "expected exactly 3 implement attempts")
	require.Len(t, gh.comments, 1)
	assert.Contains(t, gh.comments[0], "always fails")
}

func TestRunImplement_MultipleFailuresAllFedBack(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "202")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{
		{Output: "a1"}, {Output: "a2"}, {Output: "body"}, {Output: "feat: x"},
	}}
	failed := map[string]int{"go vet ./...": 1, "go test ./...": 1}
	sh := &fakeShell{}
	sh.dynamic = func(cmd string) shellOutput {
		if n, ok := failed[cmd]; ok && n > 0 {
			failed[cmd]--
			return shellOutput{Output: cmd + " output", Err: errExitNonZero}
		}
		return shellOutput{}
	}
	git := &fakeGit{commits: map[string]int{layout.WorktreePath: 1}}
	gh := &fakeGH{pr: engineer.PRInfo{URL: "u", Number: 1}}

	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "202"},
		Title:    "T",
		RepoPath: repoDir,
	}, newDeps(claude, sh, git, gh))
	require.NoError(t, err)

	checkRuns := 0
	for _, c := range sh.calls {
		if c.Command == "go vet ./..." || c.Command == "go test ./..." || c.Command == "go build ./..." || c.Command == "gofmt -l . | (! grep .)" {
			checkRuns++
		}
	}
	assert.GreaterOrEqual(t, checkRuns, 8)

	require.GreaterOrEqual(t, len(claude.calls), 2)
	assert.Contains(t, claude.calls[1].Prompt, "go vet ./...")
	assert.Contains(t, claude.calls[1].Prompt, "go test ./...")
}

func TestRunImplement_FetchFailure(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	gh := &fakeGH{}
	git := &fakeGitErr{fetchErr: errors.New("network down")}
	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "300"},
		RepoPath: repoDir,
	}, engineer.Deps{Claude: &fakeClaude{}, Shell: &fakeShell{}, Git: git, GH: gh})
	require.Error(t, err)
	require.Len(t, gh.comments, 1)
	assert.Contains(t, gh.comments[0], "board setup")
}

func TestRunImplement_ConfigMissing(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "301")
	require.NoError(t, mkdirAll(layout.WorktreePath))

	gh := &fakeGH{}
	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "301"},
		RepoPath: repoDir,
	}, engineer.Deps{
		Claude: &fakeClaude{}, Shell: &fakeShell{}, Git: &fakeGit{}, GH: gh,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, engineer.ErrConfigMissing)
	require.Len(t, gh.comments, 1)
}

func TestRunImplement_SetupCommandFails(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "302")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	sh := &fakeShell{outputs: map[string]shellOutput{
		"go mod download": {Output: "permission denied", Err: errExitNonZero},
	}}
	gh := &fakeGH{}
	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "302"},
		RepoPath: repoDir,
	}, engineer.Deps{
		Claude: &fakeClaude{}, Shell: sh, Git: &fakeGit{}, GH: gh,
	})
	require.Error(t, err)
	require.Len(t, gh.comments, 1)
	assert.Contains(t, gh.comments[0], "permission denied")
}

func TestRunImplement_ZeroCommits(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "303")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{{Output: "did nothing"}}}
	git := &fakeGit{commits: map[string]int{layout.WorktreePath: 0}}
	gh := &fakeGH{}
	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "303"},
		RepoPath: repoDir,
	}, engineer.Deps{
		Claude: claude, Shell: &fakeShell{}, Git: git, GH: gh,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zero new commits")
	require.Len(t, gh.comments, 1)
}

func TestRunImplement_ClaudeCrashFatal(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "304")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{{Err: errExitNonZero, Output: "panic"}}}
	gh := &fakeGH{}
	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "304"},
		RepoPath: repoDir,
	}, engineer.Deps{
		Claude: claude, Shell: &fakeShell{}, Git: &fakeGit{}, GH: gh,
	})
	require.Error(t, err)
	assert.Len(t, claude.calls, 1, "no retry on claude crash")
	require.Len(t, gh.comments, 1)
}

func TestRunImplement_PushFailure(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "305")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{
		{Output: "a"}, {Output: "body"}, {Output: "feat: x"},
	}}
	git := &fakeGitErr{
		commits: map[string]int{layout.WorktreePath: 1},
		pushErr: errors.New("rejected"),
	}
	gh := &fakeGH{}
	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "305"},
		RepoPath: repoDir,
	}, engineer.Deps{
		Claude: claude, Shell: &fakeShell{}, Git: git, GH: gh,
	})
	require.Error(t, err)
	require.Len(t, gh.comments, 1)
	assert.Contains(t, gh.comments[0], "rejected")
}

func TestRunImplement_PRCreateFailure(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "306")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{
		{Output: "a"}, {Output: "body"}, {Output: "feat: x"},
	}}
	git := &fakeGit{commits: map[string]int{layout.WorktreePath: 1}}
	gh := &fakeGH{prErr: errors.New("gh: 403")}
	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "306"},
		RepoPath: repoDir,
	}, engineer.Deps{
		Claude: claude, Shell: &fakeShell{}, Git: git, GH: gh,
	})
	require.Error(t, err)
	require.Len(t, gh.comments, 1)
	assert.Contains(t, gh.comments[0], "403")
}

func TestRunImplement_PRBodyFallback(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "400")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{
		{Output: "impl ok"},
		{Output: "", Err: errExitNonZero},
		{Output: "feat: x"},
	}}
	git := &fakeGit{commits: map[string]int{layout.WorktreePath: 1}}
	var captured engineer.CreatePROpts
	gh := &fakeGHCapture{pr: engineer.PRInfo{Number: 1}, capture: &captured}
	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "400"},
		Title:    "T",
		RepoPath: repoDir,
	}, engineer.Deps{
		Claude: claude, Shell: &fakeShell{}, Git: git, GH: gh,
	})
	require.NoError(t, err)
	assert.Contains(t, captured.Body, "Implements #400")
}

func TestRunImplement_PRTitleFallback(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "401")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	claude := &fakeClaude{outputs: []claudeOutput{
		{Output: "impl"},
		{Output: "body"},
		{Output: "", Err: errExitNonZero},
	}}
	git := &fakeGit{commits: map[string]int{layout.WorktreePath: 1}}
	var captured engineer.CreatePROpts
	gh := &fakeGHCapture{pr: engineer.PRInfo{Number: 1}, capture: &captured}
	_, err := engineer.RunImplement(t.Context(), engineer.ImplementInput{
		TaskRef:  engineer.TaskRef{Source: "github", ID: "401"},
		Title:    "Original title",
		RepoPath: repoDir,
	}, engineer.Deps{
		Claude: claude, Shell: &fakeShell{}, Git: git, GH: gh,
	})
	require.NoError(t, err)
	assert.Equal(t, "chore: Original title", captured.Title)
}
