package engineer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// GitRunner is the interface that the engineer uses to drive git
// operations. The production impl shells out to `git`.
type GitRunner interface {
	Fetch(ctx context.Context, repoPath, remote, branch string) error
	Prune(ctx context.Context, repoPath string) error
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

// defaultTitleValidator runs `go tool conform` from repoPath. conform is a
// module tool dep in this repo, not a PATH binary; the test suite injects
// fakes via Deps.ValidateTitle so this never runs in unit tests.
func defaultTitleValidator(ctx context.Context, repoPath, title string) error {
	if repoPath == "" {
		return fmt.Errorf("repo path required")
	}
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

	cmd := exec.CommandContext(ctx, "go", "tool", "conform", "enforce", "--commit-msg-file", f.Name())
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("conform rejected title: %s: %w", string(out), err)
	}
	return nil
}

type realGit struct{ stderr io.Writer }

// gitOut runs git with separated stdout/stderr. stdout is returned for
// parsing; stderr is forwarded to the engineer's log writer and only used
// for error context if the command fails.
func (r realGit) gitOut(ctx context.Context, workdir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workdir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	errTail := newTailBuffer()
	cmd.Stderr = io.MultiWriter(errTail, r.stderr)
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, errTail.String())
	}
	return stdout.String(), nil
}

func (r realGit) Fetch(ctx context.Context, repoPath, remote, branch string) error {
	_, err := r.gitOut(ctx, repoPath, "fetch", remote, branch)
	return err
}

func (r realGit) Prune(ctx context.Context, repoPath string) error {
	_, err := r.gitOut(ctx, repoPath, "worktree", "prune")
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
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("parse commit count %q: %w", out, err)
	}
	return n, nil
}
