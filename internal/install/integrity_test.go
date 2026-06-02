package install

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func hashOf(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func writeAt(t *testing.T, dir, rel string, data []byte) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
}

func TestCheckLocalFilesClean(t *testing.T) {
	dir := t.TempDir()
	a := []byte("alpha\n")
	b := []byte("b: 1\n")
	writeAt(t, dir, "a.json", a)
	writeAt(t, dir, "sub/b.yaml", b)
	writeAt(t, dir, ManifestFile, []byte("{}"))

	clean, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json":     hashOf(a),
		"sub/b.yaml": hashOf(b),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if !clean {
		t.Errorf("expected clean=true")
	}
}

func TestCheckLocalFilesEdited(t *testing.T) {
	dir := t.TempDir()
	original := []byte("alpha\n")
	writeAt(t, dir, "a.json", []byte("EDITED\n"))

	clean, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json": hashOf(original),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if clean {
		t.Errorf("expected clean=false for edited file")
	}
}

func TestCheckLocalFilesMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	clean, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json": hashOf([]byte("alpha\n")),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if clean {
		t.Errorf("expected clean=false for missing tracked file")
	}
}

func TestCheckLocalFilesExtra(t *testing.T) {
	dir := t.TempDir()
	a := []byte("alpha\n")
	writeAt(t, dir, "a.json", a)
	writeAt(t, dir, "extra.txt", []byte("not tracked\n"))

	clean, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json": hashOf(a),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if clean {
		t.Errorf("expected clean=false for extra file")
	}
}

func TestCheckLocalFilesManifestExcluded(t *testing.T) {
	dir := t.TempDir()
	a := []byte("alpha\n")
	writeAt(t, dir, "a.json", a)
	writeAt(t, dir, ManifestFile, []byte(`{"anything":"goes"}`))

	clean, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json": hashOf(a),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if !clean {
		t.Errorf("manifest file should be excluded from comparison; got clean=false")
	}
}

func TestCheckLocalFilesEmptyMap(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "a.json", []byte("alpha\n"))

	clean, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if clean {
		t.Errorf("empty Files map must report modified (pre-hash-era manifest)")
	}
}

func TestCheckLocalFilesNilMap(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "a.json", []byte("alpha\n"))

	clean, err := CheckLocalFiles(dir, Manifest{})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if clean {
		t.Errorf("nil Files map must report modified")
	}
}
