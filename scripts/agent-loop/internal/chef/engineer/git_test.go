package engineer_test

import (
	"os/exec"
	"testing"

	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef/engineer"
	"github.com/stretchr/testify/assert"
)

func TestDeriveWorktreeLayout(t *testing.T) {
	t.Parallel()
	got := engineer.DeriveWorktreeLayout("/home/user/code/osch", "123", "01jx")
	assert.Equal(t, "/home/user/code/osch-agent-issue-123", got.WorktreePath)
	assert.Equal(t, "agent/issue-123-01jx", got.BranchName)
}

func TestConformFallbackTitle(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("conform"); err != nil {
		t.Skip("conform binary not on PATH")
	}
	t.Run("good title returned unchanged", func(t *testing.T) {
		t.Parallel()
		out, replaced := engineer.ConformOrFallback("feat: add thing", "Original task title")
		assert.Equal(t, "feat: add thing", out)
		assert.False(t, replaced)
	})
	t.Run("bad title falls back to chore", func(t *testing.T) {
		t.Parallel()
		out, replaced := engineer.ConformOrFallback("just some text", "Original task title")
		assert.Equal(t, "chore: Original task title", out)
		assert.True(t, replaced)
	})
}
