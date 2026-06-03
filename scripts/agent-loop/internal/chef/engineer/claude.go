package engineer

import (
	"bytes"
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
	settingsPath := filepath.Join(workdir, ".brigade-settings.json")
	if err := os.WriteFile(settingsPath, embeddedSettings, 0o600); err != nil {
		return "", fmt.Errorf("write settings: %w", err)
	}
	defer func() { _ = os.Remove(settingsPath) }()

	cmd := exec.CommandContext(ctx, "claude",
		"-p", prompt,
		"--permission-mode", "acceptEdits",
		"--settings", settingsPath,
	)
	cmd.Dir = workdir
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = io.MultiWriter(&stdoutBuf, r.stderr)
	err := cmd.Run()
	return stdoutBuf.String(), err
}
