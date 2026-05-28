package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedSchema(t *testing.T, workDir, name string, withManifest bool) string {
	t.Helper()
	dir := filepath.Join(workDir, "openspec", "schemas", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("x: 1\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if withManifest {
		if err := os.WriteFile(filepath.Join(dir, ".osch.json"), []byte("{}"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	return dir
}

func runRemove(t *testing.T, workDir string, extraArgs []string, stdin string) (string, error) {
	t.Helper()
	cwd := chdir(t, workDir)
	t.Cleanup(cwd)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	args := append([]string{"remove"}, extraArgs...)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestRemoveMissingArg(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"remove"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when no schema argument is given")
	}
}

func TestRemoveYesFlagSkipsPrompt(t *testing.T) {
	dir := t.TempDir()
	target := seedSchema(t, dir, "widget", true)
	withStdinTTY(t, false) // --yes should work even non-interactively

	out, err := runRemove(t, dir, []string{"widget", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "Remove schema") {
		t.Errorf("--yes should not prompt, got %q", out)
	}
	if !strings.Contains(out, "removed widget") {
		t.Errorf("output %q should confirm removal", out)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target still exists: %v", err)
	}
}

func TestRemovePromptAccepted(t *testing.T) {
	dir := t.TempDir()
	target := seedSchema(t, dir, "widget", true)
	withStdinTTY(t, true)

	out, err := runRemove(t, dir, []string{"widget"}, "y\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "Remove schema") {
		t.Errorf("output %q should ask for confirmation", out)
	}
	if !strings.Contains(out, "removed widget") {
		t.Errorf("output %q should confirm removal", out)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target still exists: %v", err)
	}
}

func TestRemovePromptDeclinedLeavesFiles(t *testing.T) {
	dir := t.TempDir()
	target := seedSchema(t, dir, "widget", true)
	withStdinTTY(t, true)

	out, err := runRemove(t, dir, []string{"widget"}, "n\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "removed") {
		t.Errorf("declined prompt should not remove, got %q", out)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("target should still exist: %v", err)
	}
}

func TestRemovePromptDefaultEmpty(t *testing.T) {
	dir := t.TempDir()
	target := seedSchema(t, dir, "widget", true)
	withStdinTTY(t, true)

	_, err := runRemove(t, dir, []string{"widget"}, "\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("empty response defaults to no; target should still exist: %v", err)
	}
}

func TestRemoveNonTTYWithoutYesAborts(t *testing.T) {
	dir := t.TempDir()
	target := seedSchema(t, dir, "widget", true)
	withStdinTTY(t, false)

	_, err := runRemove(t, dir, []string{"widget"}, "")
	if err == nil {
		t.Fatal("expected error when non-TTY and --yes is not set")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error %q should mention --yes", err.Error())
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("target should still exist after aborted run: %v", statErr)
	}
}

func TestRemoveMissingFolder(t *testing.T) {
	dir := t.TempDir()
	withStdinTTY(t, false)

	_, err := runRemove(t, dir, []string{"widget", "--yes"}, "")
	if err == nil {
		t.Fatal("expected error for missing folder")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error %q should explain missing schema", err.Error())
	}
}

func TestRemoveUntrackedSchema(t *testing.T) {
	dir := t.TempDir()
	target := seedSchema(t, dir, "widget", false) // no manifest
	withStdinTTY(t, false)

	out, err := runRemove(t, dir, []string{"widget", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "removed widget") {
		t.Errorf("output %q should confirm removal", out)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target still exists: %v", err)
	}
}

func TestRemoveRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	withStdinTTY(t, false)

	_, err := runRemove(t, dir, []string{"../etc", "--yes"}, "")
	if err == nil {
		t.Fatal("expected error for path-traversal argument")
	}
}
