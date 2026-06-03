package install

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
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

	offenders, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json":     hashOf(a),
		"sub/b.yaml": hashOf(b),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if len(offenders) != 0 {
		t.Errorf("expected no offenders, got %v", offenders)
	}
}

func TestCheckLocalFilesEdited(t *testing.T) {
	dir := t.TempDir()
	original := []byte("alpha\n")
	writeAt(t, dir, "a.json", []byte("EDITED\n"))

	offenders, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json": hashOf(original),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if !reflect.DeepEqual(offenders, []string{"a.json"}) {
		t.Errorf("offenders = %v, want [a.json]", offenders)
	}
}

func TestCheckLocalFilesMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	offenders, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json": hashOf([]byte("alpha\n")),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if !reflect.DeepEqual(offenders, []string{"a.json"}) {
		t.Errorf("offenders = %v, want [a.json] for missing tracked file", offenders)
	}
}

func TestCheckLocalFilesExtra(t *testing.T) {
	dir := t.TempDir()
	a := []byte("alpha\n")
	writeAt(t, dir, "a.json", a)
	writeAt(t, dir, "extra.txt", []byte("not tracked\n"))

	offenders, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json": hashOf(a),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if !reflect.DeepEqual(offenders, []string{"extra.txt"}) {
		t.Errorf("offenders = %v, want [extra.txt]", offenders)
	}
}

func TestCheckLocalFilesManifestExcluded(t *testing.T) {
	dir := t.TempDir()
	a := []byte("alpha\n")
	writeAt(t, dir, "a.json", a)
	writeAt(t, dir, ManifestFile, []byte(`{"anything":"goes"}`))

	offenders, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json": hashOf(a),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if len(offenders) != 0 {
		t.Errorf("manifest file should be excluded; got offenders=%v", offenders)
	}
}

func TestCheckLocalFilesEmptyMap(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "a.json", []byte("alpha\n"))

	offenders, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if len(offenders) == 0 {
		t.Errorf("empty Files map with files on disk must report offenders")
	}
}

func TestCheckLocalFilesIgnoresSnapshotDir(t *testing.T) {
	dir := t.TempDir()
	a := []byte("alpha\n")
	writeAt(t, dir, "a.json", a)
	// Anything inside .osch/ — gitignore marker, timestamped backup, nested
	// files — must not register as drift.
	writeAt(t, dir, SnapshotDir+"/.gitignore", []byte("*\n"))
	writeAt(t, dir, SnapshotDir+"/20260603T143012Z/a.json", []byte("PRE-UPDATE\n"))
	writeAt(t, dir, SnapshotDir+"/20260603T143012Z/sub/x.yaml", []byte("y: 1\n"))

	offenders, err := CheckLocalFiles(dir, Manifest{Files: map[string]string{
		"a.json": hashOf(a),
	}})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if len(offenders) != 0 {
		t.Errorf("expected no offenders with .osch/ present, got %v", offenders)
	}
}

func TestCheckLocalFilesNilMap(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "a.json", []byte("alpha\n"))

	offenders, err := CheckLocalFiles(dir, Manifest{})
	if err != nil {
		t.Fatalf("CheckLocalFiles: %v", err)
	}
	if len(offenders) == 0 {
		t.Errorf("nil Files map with files on disk must report offenders")
	}
}
