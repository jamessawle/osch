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

func TestRemoveResetsActiveSchema(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	cfg := writeConfig(t, dir, "schema: widget\nother: keep\n")
	withStdinTTY(t, false)

	out, err := runRemove(t, dir, []string{"widget", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "active schema reset to spec-driven") {
		t.Errorf("output %q should report active schema reset", out)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "schema: spec-driven") {
		t.Errorf("schema not reset to default, got:\n%s", body)
	}
	if !strings.Contains(body, "other: keep") {
		t.Errorf("sibling key dropped, got:\n%s", body)
	}
}

func TestRemoveInactiveSchemaLeavesConfigUntouched(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	original := "schema: other-schema\nother: keep\n"
	cfg := writeConfig(t, dir, original)
	withStdinTTY(t, false)

	out, err := runRemove(t, dir, []string{"widget", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "active schema reset") {
		t.Errorf("inactive schema should not trigger reset message, got %q", out)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != original {
		t.Errorf("config changed; got:\n%s\nwant:\n%s", string(data), original)
	}
}

func TestRemoveMissingConfigIsNoOp(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	withStdinTTY(t, false)

	out, err := runRemove(t, dir, []string{"widget", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "active schema reset") {
		t.Errorf("missing config should not trigger reset message, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "openspec", "config.yaml")); !os.IsNotExist(err) {
		t.Errorf("config should not be created, stat err: %v", err)
	}
}

func TestRemoveMalformedConfigIsNoOpForReset(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	malformed := "schema: : bad\n  - not valid\n"
	cfg := writeConfig(t, dir, malformed)
	withStdinTTY(t, false)

	out, err := runRemove(t, dir, []string{"widget", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "active schema reset") {
		t.Errorf("malformed config should not trigger reset message, got %q", out)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != malformed {
		t.Errorf("malformed config rewritten; got:\n%s", string(data))
	}
}

func TestRemoveActivePromptSelectByIndex(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	seedSchema(t, dir, "gadget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, true)

	// Confirm with "y", then pick index 1 (gadget — sorted before spec-driven).
	out, err := runRemove(t, dir, []string{"widget"}, "y\n1\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "Choose the new active schema") {
		t.Errorf("output %q should show the menu", out)
	}
	if !strings.Contains(out, "1) gadget") {
		t.Errorf("output %q should list gadget at index 1", out)
	}
	if !strings.Contains(out, "active schema set to gadget") {
		t.Errorf("output %q should confirm selection", out)
	}
	if got := readSchemaKey(t, cfg); got != "gadget" {
		t.Errorf("schema key = %q, want gadget", got)
	}
}

func TestRemoveActivePromptSelectByName(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	seedSchema(t, dir, "gadget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, true)

	out, err := runRemove(t, dir, []string{"widget"}, "y\ngadget\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "active schema set to gadget") {
		t.Errorf("output %q should confirm selection", out)
	}
	if got := readSchemaKey(t, cfg); got != "gadget" {
		t.Errorf("schema key = %q, want gadget", got)
	}
}

func TestRemoveActivePromptEmptyDefaultsToSpecDriven(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	seedSchema(t, dir, "gadget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, true)

	out, err := runRemove(t, dir, []string{"widget"}, "y\n\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "active schema reset to spec-driven") {
		t.Errorf("output %q should report spec-driven fallback", out)
	}
	if got := readSchemaKey(t, cfg); got != "spec-driven" {
		t.Errorf("schema key = %q, want spec-driven", got)
	}
}

func TestRemoveActivePromptSelectSpecDrivenByName(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	seedSchema(t, dir, "gadget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, true)

	out, err := runRemove(t, dir, []string{"widget"}, "y\nspec-driven\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "active schema reset to spec-driven") {
		t.Errorf("output %q should report spec-driven selection", out)
	}
	if got := readSchemaKey(t, cfg); got != "spec-driven" {
		t.Errorf("schema key = %q, want spec-driven", got)
	}
}

func TestRemoveActivePromptInvalidRetriesThenFallsBack(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	seedSchema(t, dir, "gadget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, true)

	// Three invalid lines exhausts the retry budget; the loop falls back to
	// spec-driven rather than blocking on a fourth read.
	out, err := runRemove(t, dir, []string{"widget"}, "y\nbogus\n99\nnope\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Count(out, "invalid selection") != promptRetries {
		t.Errorf("expected %d invalid-selection messages, got %q", promptRetries, out)
	}
	if !strings.Contains(out, "active schema reset to spec-driven") {
		t.Errorf("output %q should fall back to spec-driven", out)
	}
	if got := readSchemaKey(t, cfg); got != "spec-driven" {
		t.Errorf("schema key = %q, want spec-driven", got)
	}
}

func TestRemoveActivateFlagHappyPath(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	seedSchema(t, dir, "gadget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, false)

	out, err := runRemove(t, dir, []string{"widget", "--yes", "--activate", "gadget"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "Choose the new active schema") {
		t.Errorf("--activate should skip the menu, got %q", out)
	}
	if !strings.Contains(out, "active schema set to gadget") {
		t.Errorf("output %q should confirm activation", out)
	}
	if got := readSchemaKey(t, cfg); got != "gadget" {
		t.Errorf("schema key = %q, want gadget", got)
	}
}

func TestRemoveActivateFlagUnknownAbortsBeforeDeletion(t *testing.T) {
	dir := t.TempDir()
	target := seedSchema(t, dir, "widget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, false)

	_, err := runRemove(t, dir, []string{"widget", "--yes", "--activate", "nope"}, "")
	if err == nil {
		t.Fatal("expected error for unknown --activate target")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error %q should mention not installed", err.Error())
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("widget folder must not be deleted on validation failure: %v", statErr)
	}
	if got := readSchemaKey(t, cfg); got != "widget" {
		t.Errorf("schema key changed to %q; should still be widget", got)
	}
}

func TestRemoveActivateSpecDrivenAllowed(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, false)

	// spec-driven is not under openspec/schemas/ but the flag must still
	// accept it because OpenSpec ships it as the default.
	out, err := runRemove(t, dir, []string{"widget", "--yes", "--activate", "spec-driven"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "active schema reset to spec-driven") {
		t.Errorf("output %q should report spec-driven", out)
	}
	if got := readSchemaKey(t, cfg); got != "spec-driven" {
		t.Errorf("schema key = %q, want spec-driven", got)
	}
}

func TestRemoveNoActivateFlagSkipsPrompt(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	seedSchema(t, dir, "gadget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, true)

	// stdin pretends to be a TTY but --no-activate should bypass the menu.
	out, err := runRemove(t, dir, []string{"widget", "--yes", "--no-activate"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "Choose the new active schema") {
		t.Errorf("--no-activate should skip the menu, got %q", out)
	}
	if !strings.Contains(out, "active schema reset to spec-driven") {
		t.Errorf("output %q should report spec-driven fallback", out)
	}
	if got := readSchemaKey(t, cfg); got != "spec-driven" {
		t.Errorf("schema key = %q, want spec-driven", got)
	}
}

func TestRemoveActivateFlagsConflict(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, false)

	_, err := runRemove(t, dir, []string{"widget", "--yes", "--activate", "gadget", "--no-activate"}, "")
	if err == nil {
		t.Fatal("expected error when both --activate and --no-activate are set")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Errorf("error %q should explain the conflict", err.Error())
	}
}

func TestRemoveNonTTYDefaultsToSpecDriven(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	seedSchema(t, dir, "gadget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, false)

	out, err := runRemove(t, dir, []string{"widget", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "Choose the new active schema") {
		t.Errorf("non-TTY should skip the menu, got %q", out)
	}
	if !strings.Contains(out, "active schema reset to spec-driven") {
		t.Errorf("output %q should report spec-driven fallback", out)
	}
	if got := readSchemaKey(t, cfg); got != "spec-driven" {
		t.Errorf("schema key = %q, want spec-driven", got)
	}
}

func TestRemoveNoOtherSchemasSilentlyFallsBack(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	cfg := writeConfig(t, dir, "schema: widget\n")
	withStdinTTY(t, true) // even on a TTY, no menu when nothing else is installed

	out, err := runRemove(t, dir, []string{"widget"}, "y\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "Choose the new active schema") {
		t.Errorf("no other schemas should mean no menu, got %q", out)
	}
	if !strings.Contains(out, "active schema reset to spec-driven") {
		t.Errorf("output %q should report spec-driven fallback", out)
	}
	if got := readSchemaKey(t, cfg); got != "spec-driven" {
		t.Errorf("schema key = %q, want spec-driven", got)
	}
}

func TestRemoveInactiveSchemaNoPromptEvenWithOthers(t *testing.T) {
	dir := t.TempDir()
	seedSchema(t, dir, "widget", true)
	seedSchema(t, dir, "gadget", true)
	original := "schema: gadget\n"
	cfg := writeConfig(t, dir, original)
	withStdinTTY(t, true)

	out, err := runRemove(t, dir, []string{"widget"}, "y\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, "Choose the new active schema") {
		t.Errorf("removing a non-active schema must not prompt, got %q", out)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != original {
		t.Errorf("config changed; got:\n%s\nwant:\n%s", string(data), original)
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
