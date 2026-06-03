package engineer_test

import (
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
