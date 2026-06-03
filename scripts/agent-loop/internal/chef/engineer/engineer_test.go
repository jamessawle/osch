package engineer_test

import (
	"testing"

	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef"
	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef/engineer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_UnknownKindReturnsFailedProof(t *testing.T) {
	t.Parallel()
	c := chef.Chit{Kind: "design", Task: chef.ChitTask{Ref: chef.ChitRef{Source: "github", ID: "1"}}}
	p, err := engineer.Run(t.Context(), c, engineer.Deps{})
	require.NoError(t, err)
	assert.Equal(t, chef.StatusFailed, p.Status)
	assert.Contains(t, p.Message, "unsupported kind")
}

func TestRun_ImplementKindCallsRunImplement(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	layout := engineer.DeriveWorktreeLayout(repoDir, "500", "test")
	require.NoError(t, mkdirAll(layout.WorktreePath))
	writeBrigadeYAML(t, layout.WorktreePath)

	deps := engineer.Deps{
		Claude:   &fakeClaude{outputs: []claudeOutput{{Output: "i"}, {Output: "b"}, {Output: "feat: x"}}},
		Shell:    &fakeShell{},
		Git:      &fakeGit{commits: map[string]int{layout.WorktreePath: 1}},
		GH:       &fakeGH{pr: engineer.PRInfo{URL: "u", Number: 7}},
		NewRunID: testRunID,
	}
	c := chef.Chit{
		Kind: "implement",
		Task: chef.ChitTask{
			Ref:   chef.ChitRef{Source: "github", ID: "500"},
			Title: "Test",
		},
		Repo: chef.ChitRepo{Path: repoDir},
	}
	p, err := engineer.Run(t.Context(), c, deps)
	require.NoError(t, err)
	assert.Equal(t, chef.StatusOK, p.Status)
	require.NotNil(t, p.PR)
	assert.Equal(t, 7, p.PR.Number)
}
