package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamessawle/osch/internal/install"
)

// seedTrackedSchema writes a tracked schema folder under root with manifest SHA and
// initial files mirroring what `osch add` would leave on disk.
func seedTrackedSchema(t *testing.T, root, name, sha string, files map[string][]byte) {
	t.Helper()
	dir := filepath.Join(root, "openspec", "schemas", name)
	hashes, err := install.WriteFiles(dir, files)
	if err != nil {
		t.Fatalf("seed write files %s: %v", name, err)
	}
	if err := install.WriteManifest(dir, install.Manifest{
		Schema:        install.ManifestSchemaURL,
		SchemaVersion: install.ManifestSchemaVersion,
		Source:        "acme/" + name,
		Name:          name,
		SHA:           sha,
		Files:         hashes,
	}); err != nil {
		t.Fatalf("seed manifest %s: %v", name, err)
	}
}

func TestUpdateNoArgNoSchemasInstalled(t *testing.T) {
	dir := t.TempDir()
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(buf.String(), "no tracked schemas to update") {
		t.Errorf("stdout %q should announce no tracked schemas", buf.String())
	}
}

func TestUpdateRunsAgainstFakeClient(t *testing.T) {
	client := &fakeClient{
		sha:   "newsha7890123",
		names: []string{"widget"},
		files: map[string][]byte{"manifest.json": []byte(`{"k":"v2"}`)},
	}
	withClient(t, client)

	dir := t.TempDir()
	schemaDir := filepath.Join(dir, "openspec", "schemas", "widget")
	seedBytes := []byte(`{"k":"v1"}`)
	if _, err := install.WriteFiles(schemaDir, map[string][]byte{"manifest.json": seedBytes}); err != nil {
		t.Fatalf("seed files: %v", err)
	}
	seedHash := sha256.Sum256(seedBytes)
	if err := install.WriteManifest(schemaDir, install.Manifest{
		Schema:        install.ManifestSchemaURL,
		SchemaVersion: install.ManifestSchemaVersion,
		Source:        "acme/widgets",
		Name:          "widget",
		SHA:           "oldsha1234567",
		Files:         map[string]string{"manifest.json": hex.EncodeToString(seedHash[:])},
	}); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update", "widget"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "widget\n") {
		t.Errorf("stdout %q should start the section with the schema name", out)
	}
	if !strings.Contains(out, "updated oldsha1 → newsha7") {
		t.Errorf("stdout %q should show short SHAs in updated line", out)
	}
	got, err := os.ReadFile(filepath.Join(schemaDir, "manifest.json"))
	if err != nil || string(got) != `{"k":"v2"}` {
		t.Errorf("manifest.json after update = %q err=%v", got, err)
	}
}

func TestUpdateBatchAllUpToDate(t *testing.T) {
	client := &fakeClient{sha: "samesha", names: []string{"a", "b"}}
	withClient(t, client)

	dir := t.TempDir()
	seedTrackedSchema(t, dir, "alpha", "samesha", map[string][]byte{"f.json": []byte("1")})
	seedTrackedSchema(t, dir, "beta", "samesha", map[string][]byte{"f.json": []byte("2")})
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "alpha\n  up to date at samesha") {
		t.Errorf("stdout %q missing alpha section", out)
	}
	if !strings.Contains(out, "beta\n  up to date at samesha") {
		t.Errorf("stdout %q missing beta section", out)
	}
	if strings.Index(out, "alpha") > strings.Index(out, "beta") {
		t.Errorf("schemas should be alphabetical; got %q", out)
	}
}

func TestUpdateBatchMixedUpdatedAndUpToDate(t *testing.T) {
	client := &fakeClient{
		sha:   "newsha",
		files: map[string][]byte{"f.json": []byte("UPSTREAM")},
	}
	withClient(t, client)

	dir := t.TempDir()
	seedTrackedSchema(t, dir, "alpha", "oldsha", map[string][]byte{"f.json": []byte("local")})
	seedTrackedSchema(t, dir, "beta", "newsha", map[string][]byte{"f.json": []byte("synced")})
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "alpha\n  updated oldsha → newsha") {
		t.Errorf("alpha section missing/wrong: %q", out)
	}
	if !strings.Contains(out, "beta\n  up to date at newsha") {
		t.Errorf("beta section missing/wrong: %q", out)
	}
}

func TestUpdateBatchRefusalNonZeroButOthersContinue(t *testing.T) {
	client := &fakeClient{
		sha:   "newsha",
		files: map[string][]byte{"f.json": []byte("UPSTREAM")},
	}
	withClient(t, client)

	dir := t.TempDir()
	// alpha is clean, beta has a local edit so should refuse, gamma is clean.
	seedTrackedSchema(t, dir, "alpha", "oldsha", map[string][]byte{"f.json": []byte("local-a")})
	seedTrackedSchema(t, dir, "beta", "oldsha", map[string][]byte{"f.json": []byte("local-b")})
	seedTrackedSchema(t, dir, "gamma", "oldsha", map[string][]byte{"f.json": []byte("local-g")})
	// Edit beta to trigger refusal.
	if err := os.WriteFile(filepath.Join(dir, "openspec", "schemas", "beta", "f.json"), []byte("EDITED"), 0o644); err != nil {
		t.Fatalf("edit beta: %v", err)
	}
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected non-zero exit when one schema refused")
	}
	out := buf.String()
	if !strings.Contains(out, "alpha\n  updated") {
		t.Errorf("alpha should have been updated despite beta refusing: %q", out)
	}
	if !strings.Contains(out, "beta\n  refused: local modifications") {
		t.Errorf("beta should report refusal: %q", out)
	}
	if !strings.Contains(out, "gamma\n  updated") {
		t.Errorf("gamma should have been updated despite beta refusing: %q", out)
	}
	// alpha and gamma should be updated on disk.
	for _, n := range []string{"alpha", "gamma"} {
		got, _ := os.ReadFile(filepath.Join(dir, "openspec", "schemas", n, "f.json"))
		if string(got) != "UPSTREAM" {
			t.Errorf("%s f.json = %q, want UPSTREAM", n, got)
		}
	}
}

func TestUpdateBatchFailureFromUnparseableManifestNonZero(t *testing.T) {
	client := &fakeClient{
		sha:   "newsha",
		files: map[string][]byte{"f.json": []byte("UPSTREAM")},
	}
	withClient(t, client)

	dir := t.TempDir()
	seedTrackedSchema(t, dir, "alpha", "oldsha", map[string][]byte{"f.json": []byte("a")})
	// Plant a folder with a broken .osch.json manifest.
	brokenDir := filepath.Join(dir, "openspec", "schemas", "broken")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("mkdir broken: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, install.ManifestFile), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("plant broken manifest: %v", err)
	}
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected non-zero exit when a schema fails")
	}
	out := buf.String()
	if !strings.Contains(out, "alpha\n  updated") {
		t.Errorf("alpha should have processed despite broken sibling: %q", out)
	}
	if !strings.Contains(out, "broken\n  failed:") {
		t.Errorf("broken should report failure: %q", out)
	}
}

func TestUpdateBatchForceAppliesToEverySchema(t *testing.T) {
	client := &fakeClient{
		sha:   "newsha",
		files: map[string][]byte{"f.json": []byte("UPSTREAM")},
	}
	withClient(t, client)

	dir := t.TempDir()
	seedTrackedSchema(t, dir, "alpha", "oldsha", map[string][]byte{"f.json": []byte("a")})
	seedTrackedSchema(t, dir, "beta", "oldsha", map[string][]byte{"f.json": []byte("b")})
	// Edit both so both would refuse without --force.
	for _, n := range []string{"alpha", "beta"} {
		if err := os.WriteFile(filepath.Join(dir, "openspec", "schemas", n, "f.json"), []byte("EDITED"), 0o644); err != nil {
			t.Fatalf("edit %s: %v", n, err)
		}
	}
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update", "--force"})
	if err := root.Execute(); err != nil {
		t.Fatalf("force batch update: %v", err)
	}
	for _, n := range []string{"alpha", "beta"} {
		got, _ := os.ReadFile(filepath.Join(dir, "openspec", "schemas", n, "f.json"))
		if string(got) != "UPSTREAM" {
			t.Errorf("%s f.json = %q, want UPSTREAM", n, got)
		}
	}
}

func TestUpdateBatchIgnoresFoldersWithoutManifest(t *testing.T) {
	client := &fakeClient{sha: "samesha"}
	withClient(t, client)

	dir := t.TempDir()
	seedTrackedSchema(t, dir, "alpha", "samesha", map[string][]byte{"f.json": []byte("a")})
	// Folder with no .osch.json — must be skipped.
	if err := os.MkdirAll(filepath.Join(dir, "openspec", "schemas", "untracked"), 0o755); err != nil {
		t.Fatalf("mkdir untracked: %v", err)
	}
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "untracked") {
		t.Errorf("untracked folder should not appear in output: %q", out)
	}
	if !strings.Contains(out, "alpha\n  up to date") {
		t.Errorf("alpha section missing: %q", out)
	}
}

func TestUpdateBatchEmptySchemasDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "openspec", "schemas"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(buf.String(), "no tracked schemas to update") {
		t.Errorf("stdout %q should announce no tracked schemas", buf.String())
	}
}

func TestUpdateTooManyArgs(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update", "a", "b"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when more than one argument is given")
	}
}
