package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamessawle/osch/internal/source"
)

// withStdinTTY swaps the package-level TTY probe for the duration of a test.
// Activation behaviour branches on this, so every activation test pins it
// explicitly to avoid leaking the host's real TTY state into the assertions.
func withStdinTTY(t *testing.T, isTTY bool) {
	t.Helper()
	prev := stdinIsTTY
	stdinIsTTY = func() bool { return isTTY }
	t.Cleanup(func() { stdinIsTTY = prev })
}

// runAdd wires up a fake client, points "." at a temp dir, and runs `osch add`
// with the supplied extra args and stdin. It returns the stdout buffer and
// any execution error so callers can assert on both.
func runAdd(t *testing.T, workDir string, extraArgs []string, stdin string) (string, error) {
	t.Helper()
	client := &fakeClient{
		sha:   "deadbeef",
		names: []string{"widget"},
		files: map[string][]byte{"manifest.json": []byte(`{}`)},
	}
	withClient(t, client)
	cwd := chdir(t, workDir)
	t.Cleanup(cwd)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	args := append([]string{"add", "acme/widgets"}, extraArgs...)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func writeConfig(t *testing.T, workDir, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(workDir, "openspec"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	path := filepath.Join(workDir, "openspec", "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	return path
}

func readSchemaKey(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "schema:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "schema:"))
		}
	}
	return ""
}

func TestAddPromptAccepted(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "schema: old\nother: keep\n")
	withStdinTTY(t, true)

	out, err := runAdd(t, dir, nil, "y\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "Replace active schema") {
		t.Errorf("output %q should explain the replacement", out)
	}
	if !strings.Contains(out, "activated widget") {
		t.Errorf("output %q should confirm activation", out)
	}
	if got := readSchemaKey(t, path); got != "widget" {
		t.Errorf("schema key = %q, want widget", got)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "other: keep") {
		t.Errorf("other keys should be preserved, got:\n%s", data)
	}
}

func TestAddPromptDeclined(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "schema: old\n")
	withStdinTTY(t, true)

	out, err := runAdd(t, dir, nil, "n\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "activated") {
		t.Errorf("declined prompt should not activate, got %q", out)
	}
	if got := readSchemaKey(t, path); got != "old" {
		t.Errorf("schema key = %q, want old (unchanged)", got)
	}
}

func TestAddPromptDefaultEmpty(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "schema: old\n")
	withStdinTTY(t, true)

	_, err := runAdd(t, dir, nil, "\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := readSchemaKey(t, path); got != "old" {
		t.Errorf("empty response should default to no; schema key = %q, want old", got)
	}
}

func TestAddActivateFlagNonInteractive(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "schema: old\n")
	withStdinTTY(t, false)

	out, err := runAdd(t, dir, []string{"--activate"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "Replace active schema") {
		t.Errorf("--activate should not prompt, got %q", out)
	}
	if !strings.Contains(out, "activated widget") {
		t.Errorf("output %q should confirm activation", out)
	}
	if got := readSchemaKey(t, path); got != "widget" {
		t.Errorf("schema key = %q, want widget", got)
	}
}

func TestAddNoActivateFlag(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "schema: old\n")
	withStdinTTY(t, true)

	out, err := runAdd(t, dir, []string{"--no-activate"}, "y\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "Replace") || strings.Contains(out, "activated") {
		t.Errorf("--no-activate should skip prompt and activation, got %q", out)
	}
	if got := readSchemaKey(t, path); got != "old" {
		t.Errorf("schema key = %q, want old (unchanged)", got)
	}
}

func TestAddFlagConflict(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "schema: old\n")
	withStdinTTY(t, false)

	_, err := runAdd(t, dir, []string{"--activate", "--no-activate"}, "")
	if err == nil {
		t.Fatal("expected error when both --activate and --no-activate are set")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Errorf("error %q should explain the conflict", err.Error())
	}
}

func TestAddActivateMissingConfig(t *testing.T) {
	dir := t.TempDir()
	withStdinTTY(t, false)

	out, err := runAdd(t, dir, []string{"--activate"}, "")
	if err != nil {
		t.Fatalf("add should still succeed when config is missing: %v", err)
	}
	if !strings.Contains(out, "skipping activation") {
		t.Errorf("output %q should explain why activation was skipped", out)
	}
}

func TestAddNonTTYDefaultSkips(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "schema: old\n")
	withStdinTTY(t, false)

	out, err := runAdd(t, dir, nil, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "Replace") || strings.Contains(out, "activated") || strings.Contains(out, "skipping") {
		t.Errorf("non-TTY default should be silent on activation, got %q", out)
	}
	if got := readSchemaKey(t, path); got != "old" {
		t.Errorf("schema key = %q, want old (unchanged)", got)
	}
}

func TestAddActivateAlreadyActive(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, true)

	out, err := runAdd(t, dir, []string{"--activate"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "already the active schema") {
		t.Errorf("output %q should report the no-op", out)
	}
}

// Compile-time guard: source.Client is the interface our fakes satisfy.
var _ source.Client = (*fakeClient)(nil)
