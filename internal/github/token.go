package github

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TokenSource yields a GitHub token if it has one. ok=false means "nothing
// here, try the next source" — it is never an error, so a source that fails to
// produce a token (a missing gh binary, an unauthenticated gh, a gh that
// errors) degrades silently to the next source in the chain.
type TokenSource interface {
	Token(ctx context.Context) (token string, ok bool)
}

// EnvVarTokenSource yields the token held in an environment variable. The value
// is accepted only if it is non-empty after trimming surrounding whitespace.
type EnvVarTokenSource struct {
	Key string
}

// Token returns the trimmed value of the configured environment variable, with
// ok=true only when that value is non-empty.
func (s EnvVarTokenSource) Token(context.Context) (string, bool) {
	token := strings.TrimSpace(os.Getenv(s.Key))
	return token, token != ""
}

// CLITokenSource borrows the token stored by an installed gh CLI by
// running `gh auth token`. A missing binary, an unauthenticated gh, or any
// non-zero exit yields ok=false — never a hard failure.
type CLITokenSource struct {
	// run executes the command and returns its stdout. It is fake-able for
	// tests; when nil the real os/exec runner (ghRun) is used.
	run func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Token runs `gh auth token` and returns its trimmed output. Any error from the
// runner, or empty output, yields ok=false.
func (s CLITokenSource) Token(ctx context.Context) (string, bool) {
	run := s.run
	if run == nil {
		run = ghRun
	}
	out, err := run(ctx, "gh", "auth", "token")
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(string(out))
	return token, token != ""
}

// ghRun is the real gh runner and the only code in the token chain that touches
// os/exec. It locates gh on PATH, bounds the call at one second, and discards
// stderr so an unauthenticated gh prints nothing to the user's terminal.
func ghRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stderr = io.Discard
	return cmd.Output()
}

// StaticTokenSource always yields its fixed token (ok=true), even when empty.
// Placed last in a chain it guarantees the walk always resolves; a zero Value
// makes anonymous access explicit. Mirrors oauth2.StaticTokenSource (the field
// is named Value rather than Token because the interface method takes that
// name).
type StaticTokenSource struct {
	Value string
}

// Token always returns the fixed value with ok=true.
func (s StaticTokenSource) Token(context.Context) (string, bool) {
	return s.Value, true
}

// resolveToken walks the sources in order and returns the first token whose
// source reports ok. The ordering of the chain lives here and nowhere else; no
// source knows about any other. The empty string means anonymous access.
func resolveToken(ctx context.Context, sources []TokenSource) string {
	for _, src := range sources {
		if token, ok := src.Token(ctx); ok {
			return token
		}
	}
	return ""
}

// defaultTokenSources is the resolution chain decided in ADR 0007: an explicit
// GITHUB_TOKEN, then the gh CLI's stored credentials, then anonymous.
func defaultTokenSources() []TokenSource {
	return []TokenSource{
		EnvVarTokenSource{Key: "GITHUB_TOKEN"},
		CLITokenSource{},
		StaticTokenSource{Value: ""},
	}
}
