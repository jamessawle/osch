package engineer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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
// branch name from the source repo path, task ID, and a per-run nonce.
// The path stays deterministic (so local cleanup is straightforward); the
// branch carries the nonce so successive attempts against the same issue
// never collide on a stale remote ref.
func DeriveWorktreeLayout(repoPath, taskID, nonce string) WorktreeLayout {
	return WorktreeLayout{
		WorktreePath: repoPath + "-agent-issue-" + taskID,
		BranchName:   "agent/issue-" + taskID + "-" + nonce,
	}
}

// ConformOrFallback runs `go tool conform` against the given title (conform
// is a module tool dep in this repo, not a PATH binary). If conform accepts
// the title it is returned unchanged; otherwise the function returns
// ("chore: <taskTitle>", true).
func ConformOrFallback(generated, taskTitle string) (string, bool) {
	if err := runConform(generated); err == nil {
		return generated, false
	}
	return "chore: " + taskTitle, true
}

func runConform(title string) error {
	// conform reads .conform.yaml from cwd, so run from the repo root.
	rootBytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("find repo root: %w", err)
	}
	repoRoot := string(bytes.TrimSpace(rootBytes))

	f, err := os.CreateTemp("", "brigade-conform-*.txt")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.WriteString(title + "\n"); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	cmd := exec.Command("go", "tool", "conform", "enforce", "--commit-msg-file", f.Name())
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("conform rejected title: %s: %w", string(out), err)
	}
	return nil
}

type realGit struct{ stderr io.Writer }

func (r realGit) gitOut(ctx context.Context, workdir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workdir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = io.MultiWriter(&buf, r.stderr)
	err := cmd.Run()
	return buf.String(), err
}

func (r realGit) Fetch(ctx context.Context, repoPath, remote, branch string) error {
	_, err := r.gitOut(ctx, repoPath, "fetch", remote, branch)
	return err
}

func (r realGit) WorktreeAdd(ctx context.Context, repoPath, worktreePath, branch, baseRef string) error {
	_, err := r.gitOut(ctx, repoPath, "worktree", "add", "-b", branch, worktreePath, baseRef)
	return err
}

func (r realGit) WorktreeRemove(ctx context.Context, repoPath, worktreePath string) error {
	_, err := r.gitOut(ctx, repoPath, "worktree", "remove", "--force", worktreePath)
	return err
}

func (r realGit) Push(ctx context.Context, worktreePath, remote, branch string) error {
	_, err := r.gitOut(ctx, worktreePath, "push", "-u", remote, branch)
	return err
}

func (r realGit) CommitCount(ctx context.Context, worktreePath, baseRef string) (int, error) {
	out, err := r.gitOut(ctx, worktreePath, "rev-list", "--count", baseRef+"..HEAD")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, b := range []byte(out) {
		if b >= '0' && b <= '9' {
			n = n*10 + int(b-'0')
		}
	}
	return n, nil
}
