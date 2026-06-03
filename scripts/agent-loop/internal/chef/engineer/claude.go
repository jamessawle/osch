package engineer

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed settings.json
var embeddedSettings []byte

// SettingsJSON returns the embedded sandbox settings bytes.
// Exported for testing; production callers should use ClaudeRunner.
func SettingsJSON() []byte { return embeddedSettings }

// ClaudeRunner runs a non-interactive `claude -p` call inside a worktree.
// Captures combined stdout/stderr. Returns the captured output and any
// non-zero-exit error from the claude process itself.
type ClaudeRunner interface {
	Run(ctx context.Context, worktreePath, prompt string) (output string, err error)
}

type realClaude struct{ stderr io.Writer }

func (r realClaude) Run(ctx context.Context, workdir, prompt string) (string, error) {
	// Sandbox settings live outside the worktree so the agent can't see, edit,
	// or accidentally commit them; a fresh tempdir per call also avoids any
	// leftover-symlink trust inversion if a previous run crashed.
	settingsDir, err := os.MkdirTemp("", "brigade-settings-*")
	if err != nil {
		return "", fmt.Errorf("settings tempdir: %w", err)
	}
	defer func() { _ = os.RemoveAll(settingsDir) }()
	settingsPath := filepath.Join(settingsDir, ".brigade-settings.json")
	if err := os.WriteFile(settingsPath, embeddedSettings, 0o600); err != nil {
		return "", fmt.Errorf("write settings: %w", err)
	}

	cmd := exec.CommandContext(ctx, "claude",
		"-p", prompt,
		"--permission-mode", "dontAsk",
		"--settings", settingsPath,
	)
	cmd.Dir = workdir
	out := newTailBuffer()
	cmd.Stdout = out
	cmd.Stderr = io.MultiWriter(out, r.stderr)
	runErr := cmd.Run()
	return out.String(), runErr
}
