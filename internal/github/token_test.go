package github

import (
	"context"
	"errors"
	"testing"
)

// fakeRun returns a runner func that yields fixed output and error, standing in
// for `gh auth token` without spawning a process.
func fakeRun(out string, err error) func(context.Context, string, ...string) ([]byte, error) {
	return func(context.Context, string, ...string) ([]byte, error) {
		return []byte(out), err
	}
}

func TestResolveTokenEnvWinsOverGh(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	sources := []TokenSource{
		EnvVarTokenSource{Key: "GITHUB_TOKEN"},
		CLITokenSource{run: fakeRun("gh-token", nil)},
		StaticTokenSource{Value: ""},
	}
	if got := resolveToken(context.Background(), sources); got != "env-token" {
		t.Errorf("got %q, want env-token", got)
	}
}

func TestResolveTokenGhUsedWhenEnvUnset(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	sources := []TokenSource{
		EnvVarTokenSource{Key: "GITHUB_TOKEN"},
		CLITokenSource{run: fakeRun("gh-token\n", nil)},
		StaticTokenSource{Value: ""},
	}
	if got := resolveToken(context.Background(), sources); got != "gh-token" {
		t.Errorf("got %q, want gh-token", got)
	}
}

func TestResolveTokenAnonymousWhenBothUnset(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	sources := []TokenSource{
		EnvVarTokenSource{Key: "GITHUB_TOKEN"},
		CLITokenSource{run: fakeRun("", nil)}, // gh authenticated but empty
		StaticTokenSource{Value: ""},
	}
	if got := resolveToken(context.Background(), sources); got != "" {
		t.Errorf("got %q, want empty (anonymous)", got)
	}
}

func TestResolveTokenGhFailingIsSilentFallback(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	sources := []TokenSource{
		EnvVarTokenSource{Key: "GITHUB_TOKEN"},
		CLITokenSource{run: fakeRun("", errors.New("gh: not logged in"))},
		StaticTokenSource{Value: ""},
	}
	if got := resolveToken(context.Background(), sources); got != "" {
		t.Errorf("got %q, want empty (anonymous)", got)
	}
}

func TestEnvVarTokenSourceTrimsAndRejectsBlank(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "  spaced-token  ")
	token, ok := EnvVarTokenSource{Key: "GITHUB_TOKEN"}.Token(context.Background())
	if !ok || token != "spaced-token" {
		t.Errorf("got (%q, %v), want (spaced-token, true)", token, ok)
	}

	t.Setenv("GITHUB_TOKEN", "   ")
	blank := EnvVarTokenSource{Key: "GITHUB_TOKEN"}
	if token, ok := blank.Token(context.Background()); ok {
		t.Errorf("whitespace-only token accepted: (%q, %v)", token, ok)
	}
}

func TestCLITokenSourceTrimsOutput(t *testing.T) {
	token, ok := CLITokenSource{run: fakeRun("  gh-token\n", nil)}.Token(context.Background())
	if !ok || token != "gh-token" {
		t.Errorf("got (%q, %v), want (gh-token, true)", token, ok)
	}
}
