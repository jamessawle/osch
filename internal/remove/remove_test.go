package remove

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"widget", false},
		{"my-schema", false},
		{"", true},
		{".", true},
		{"..", true},
		{"../etc", true},
		{"foo/bar", true},
		{`foo\bar`, true},
		{"foo..bar", true},
	}
	for _, c := range cases {
		err := ValidateName(c.name)
		if c.wantErr && err == nil {
			t.Errorf("ValidateName(%q) = nil; want error", c.name)
		}
		if !c.wantErr && err != nil {
			t.Errorf("ValidateName(%q) = %v; want nil", c.name, err)
		}
	}
}

func TestRemoveDeletesTree(t *testing.T) {
	workDir := t.TempDir()
	target := Dir(workDir, "widget")
	if err := os.MkdirAll(filepath.Join(target, "sub"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "sub", "a.yaml"), []byte("x: 1\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := Remove(workDir, "widget"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target still exists or unexpected err: %v", err)
	}
}

func TestRemoveMissingFolderErrors(t *testing.T) {
	workDir := t.TempDir()
	err := Remove(workDir, "widget")
	if err == nil {
		t.Fatal("expected error for missing folder")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error %q should explain missing schema", err.Error())
	}
}

func TestRemoveUntrackedSchema(t *testing.T) {
	workDir := t.TempDir()
	target := Dir(workDir, "widget")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// no .osch.json manifest written
	if err := Remove(workDir, "widget"); err != nil {
		t.Fatalf("Remove without manifest should succeed: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target still exists: %v", err)
	}
}

func TestRemoveRejectsTraversal(t *testing.T) {
	workDir := t.TempDir()
	if err := Remove(workDir, "../etc"); err == nil {
		t.Fatal("expected error for path-traversal argument")
	}
}
