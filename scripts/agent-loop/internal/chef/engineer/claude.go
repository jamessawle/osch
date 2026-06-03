package engineer

import (
	"context"
	_ "embed"
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
