package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamessawle/osch/internal/install"
)

func TestUpdateMissingArg(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"update"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when no schema argument is given")
	}
}

func TestUpdateRunsAgainstFakeClient(t *testing.T) {
	client := &fakeClient{
		sha:   "newsha",
		names: []string{"widget"},
		files: map[string][]byte{"manifest.json": []byte(`{"k":"v2"}`)},
	}
	withClient(t, client)

	dir := t.TempDir()
	schemaDir := filepath.Join(dir, "openspec", "schemas", "widget")
	if _, err := install.WriteFiles(schemaDir, map[string][]byte{"manifest.json": []byte(`{"k":"v1"}`)}); err != nil {
		t.Fatalf("seed files: %v", err)
	}
	if err := install.WriteManifest(schemaDir, install.Manifest{
		Schema:        install.ManifestSchemaURL,
		SchemaVersion: install.ManifestSchemaVersion,
		Source:        "acme/widgets",
		Name:          "widget",
		SHA:           "oldsha",
		Files:         map[string]string{"manifest.json": "ignored"},
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
	if !strings.Contains(buf.String(), "newsha") {
		t.Errorf("stdout %q should mention the new SHA", buf.String())
	}
	got, err := os.ReadFile(filepath.Join(schemaDir, "manifest.json"))
	if err != nil || string(got) != `{"k":"v2"}` {
		t.Errorf("manifest.json after update = %q err=%v", got, err)
	}
}
