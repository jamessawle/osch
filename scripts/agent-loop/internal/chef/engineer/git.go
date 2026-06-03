package engineer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GitRunner is the interface that the engineer uses to drive git
// operations. The production impl shells out to `git`.
type GitRunner interface {
	Fetch(ctx context.Context, repoPath, remote, branch string) error
	WorktreeAdd(ctx context.Context, repoPath, worktreePath, branch, baseRef string) error
	WorktreeRemove(ctx context.Context, repoPath, worktreePath string) error
	Push(ctx context.Context, worktreePath, remote, branch string) error
	CommitCount(ctx context.Context, worktreePath, baseRef string) (int, error)
}

// WorktreeLayout describes where the engineer will check out a Chit's
// working tree and what branch it sits on.
type WorktreeLayout struct {
	WorktreePath string
	BranchName   string
}

// DeriveWorktreeLayout picks a sibling-directory worktree path and
// branch name from the source repo path and task ID.
func DeriveWorktreeLayout(repoPath, taskID string) WorktreeLayout {
	return WorktreeLayout{
		WorktreePath: repoPath + "-agent-issue-" + taskID,
		BranchName:   "agent/issue-" + taskID,
	}
}

// ConformOrFallback runs `conform` against the given title. If conform
// rejects it, returns ("chore: <taskTitle>", true). Otherwise returns
// the title unchanged with replaced=false.
func ConformOrFallback(generated, taskTitle string) (string, bool) {
	if err := runConform(generated); err == nil {
		return generated, false
	}
	return "chore: " + taskTitle, true
}

func runConform(title string) error {
	cmd := exec.Command("conform", "enforce", "--commit-msg", title)
	cmd.Stdin = strings.NewReader(title)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("conform rejected title: %s: %w", string(out), err)
	}
	return nil
}
