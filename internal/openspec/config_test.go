package openspec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSchemaMissingFile(t *testing.T) {
	name, exists, err := ReadSchema(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Errorf("exists = true for missing file")
	}
	if name != "" {
		t.Errorf("name = %q, want empty", name)
	}
}

func TestReadSchemaPresent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("schema: foo\nother: bar\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	name, exists, err := ReadSchema(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Errorf("exists = false for present file")
	}
	if name != "foo" {
		t.Errorf("name = %q, want foo", name)
	}
}

func TestWriteSchemaPreservesOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("schema: old\nother: keep\nnested:\n  a: 1\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := WriteSchema(path, "new"); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "schema: new") {
		t.Errorf("schema not updated, got:\n%s", body)
	}
	if !strings.Contains(body, "other: keep") {
		t.Errorf("sibling key dropped, got:\n%s", body)
	}
	if !strings.Contains(body, "a: 1") {
		t.Errorf("nested value dropped, got:\n%s", body)
	}
}

func TestWriteSchemaMissingFile(t *testing.T) {
	err := WriteSchema(filepath.Join(t.TempDir(), "config.yaml"), "x")
	if err == nil {
		t.Fatal("expected error when config is absent (osch must not create it)")
	}
}
