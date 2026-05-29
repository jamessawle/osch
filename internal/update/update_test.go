package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamessawle/osch/internal/install"
	"github.com/jamessawle/osch/internal/source"
)

type fakeClient struct {
	sha   string
	names []string
	files map[string][]byte

	listCalls  int
	fetchCalls int
}

func (f *fakeClient) ListSchemas(_ context.Context, _ source.Ref) (string, []string, error) {
	f.listCalls++
	return f.sha, f.names, nil
}

func (f *fakeClient) FetchSchemaFiles(_ context.Context, _ source.Ref, _, _ string) (map[string][]byte, error) {
	f.fetchCalls++
	return f.files, nil
}

func factoryFor(c source.Client) ClientFactory {
	return func(source.Ref) (source.Client, error) { return c, nil }
}

// seed installs a schema under workDir with the supplied manifest SHA and
// initial files. It mirrors the bytes-on-disk shape `osch add` would leave.
func seed(t *testing.T, workDir, name, sha string, files map[string][]byte) {
	t.Helper()
	dir := filepath.Join(workDir, "openspec", "schemas", name)
	hashes, err := install.WriteFiles(dir, files)
	if err != nil {
		t.Fatalf("seed write files: %v", err)
	}
	m := install.Manifest{
		Schema:        install.ManifestSchemaURL,
		SchemaVersion: install.ManifestSchemaVersion,
		Source:        "acme/widgets",
		Name:          name,
		SHA:           sha,
		Files:         hashes,
	}
	if err := install.WriteManifest(dir, m); err != nil {
		t.Fatalf("seed write manifest: %v", err)
	}
}

func TestUpdateRefreshesToNewSHA(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"manifest.json": []byte(`{"v":1}`),
		"old.yaml":      []byte("gone: true\n"),
	})

	client := &fakeClient{
		sha:   "newsha",
		names: []string{"widget"},
		files: map[string][]byte{
			"manifest.json": []byte(`{"v":2}`),
			"new/sub.yaml":  []byte("hello: world\n"),
		},
	}
	var buf bytes.Buffer
	if err := Update(context.Background(), factoryFor(client), workDir, "widget", &buf); err != nil {
		t.Fatalf("update: %v", err)
	}

	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	if _, err := os.Stat(filepath.Join(dir, "old.yaml")); !os.IsNotExist(err) {
		t.Errorf("old.yaml should have been removed, stat err=%v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err != nil || string(got) != `{"v":2}` {
		t.Errorf("manifest.json = %q, err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "new", "sub.yaml")); err != nil || string(got) != "hello: world\n" {
		t.Errorf("new/sub.yaml = %q, err=%v", got, err)
	}

	m, err := install.ReadManifest(dir)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if m.SHA != "newsha" {
		t.Errorf("manifest SHA = %q, want newsha", m.SHA)
	}
	if len(m.Files) != 2 {
		t.Errorf("files map has %d entries, want 2: %v", len(m.Files), m.Files)
	}
	if _, ok := m.Files["old.yaml"]; ok {
		t.Errorf("old.yaml should be dropped from files map: %v", m.Files)
	}
	for rel, data := range client.files {
		want := sha256.Sum256(data)
		if m.Files[rel] != hex.EncodeToString(want[:]) {
			t.Errorf("hash for %s = %q, want %q", rel, m.Files[rel], hex.EncodeToString(want[:]))
		}
	}
	if !strings.Contains(buf.String(), "newsha") {
		t.Errorf("stdout %q should mention the new SHA", buf.String())
	}
}

func TestUpdateNoOpWhenAlreadyUpToDate(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "samesha", map[string][]byte{
		"keep.json": []byte(`{"k":"v"}`),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	manifestPath := filepath.Join(dir, install.ManifestFile)
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	keepBefore, err := os.ReadFile(filepath.Join(dir, "keep.json"))
	if err != nil {
		t.Fatalf("read keep.json: %v", err)
	}

	client := &fakeClient{
		sha:   "samesha",
		names: []string{"widget"},
		files: map[string][]byte{"unused.json": []byte("nope")},
	}
	var buf bytes.Buffer
	if err := Update(context.Background(), factoryFor(client), workDir, "widget", &buf); err != nil {
		t.Fatalf("update: %v", err)
	}
	if client.fetchCalls != 0 {
		t.Errorf("FetchSchemaFiles should not be called when SHAs match; got %d calls", client.fetchCalls)
	}
	manifestAfter, _ := os.ReadFile(manifestPath)
	if !bytes.Equal(manifestBefore, manifestAfter) {
		t.Errorf("manifest should not be rewritten; before=%q after=%q", manifestBefore, manifestAfter)
	}
	keepAfter, _ := os.ReadFile(filepath.Join(dir, "keep.json"))
	if !bytes.Equal(keepBefore, keepAfter) {
		t.Errorf("file should not be rewritten")
	}
	if _, err := os.Stat(filepath.Join(dir, "unused.json")); !os.IsNotExist(err) {
		t.Errorf("upstream files should not be written on no-op; stat err=%v", err)
	}
	if !strings.Contains(buf.String(), "up to date") {
		t.Errorf("stdout %q should announce no-op", buf.String())
	}
}

func TestUpdateMissingSchema(t *testing.T) {
	workDir := t.TempDir()
	client := &fakeClient{sha: "x", names: []string{"widget"}}
	var buf bytes.Buffer
	err := Update(context.Background(), factoryFor(client), workDir, "widget", &buf)
	if err == nil {
		t.Fatal("expected error when schema is not installed")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error %q should mention not installed", err.Error())
	}
}

func TestUpdateMissingManifest(t *testing.T) {
	workDir := t.TempDir()
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	client := &fakeClient{sha: "x", names: []string{"widget"}}
	var buf bytes.Buffer
	err := Update(context.Background(), factoryFor(client), workDir, "widget", &buf)
	if err == nil {
		t.Fatal("expected error when manifest is missing")
	}
	if !strings.Contains(err.Error(), install.ManifestFile) {
		t.Errorf("error %q should mention the manifest file", err.Error())
	}
}

func TestUpdateAddsAndRemovesFiles(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"keep.json":     []byte("keep1"),
		"drop.yaml":     []byte("dropme"),
		"nested/old.md": []byte("old"),
	})
	client := &fakeClient{
		sha:   "newsha",
		names: []string{"widget"},
		files: map[string][]byte{
			"keep.json":      []byte("keep2"),
			"nested/new.md":  []byte("new"),
			"brand/new.json": []byte("{}"),
		},
	}
	var buf bytes.Buffer
	if err := Update(context.Background(), factoryFor(client), workDir, "widget", &buf); err != nil {
		t.Fatalf("update: %v", err)
	}
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	if _, err := os.Stat(filepath.Join(dir, "drop.yaml")); !os.IsNotExist(err) {
		t.Errorf("drop.yaml should be removed; stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nested", "old.md")); !os.IsNotExist(err) {
		t.Errorf("nested/old.md should be removed")
	}
	for rel, want := range client.files {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("file %s: got %q err=%v want %q", rel, got, err, want)
		}
	}
	m, err := install.ReadManifest(dir)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if len(m.Files) != len(client.files) {
		t.Errorf("files map has %d entries, want %d: %v", len(m.Files), len(client.files), m.Files)
	}
	if _, ok := m.Files["drop.yaml"]; ok {
		t.Errorf("drop.yaml should not be in files map")
	}
	if _, ok := m.Files["brand/new.json"]; !ok {
		t.Errorf("brand/new.json should be in files map: %v", m.Files)
	}
}

func TestUpdateRejectsInvalidName(t *testing.T) {
	client := &fakeClient{sha: "x"}
	var buf bytes.Buffer
	err := Update(context.Background(), factoryFor(client), t.TempDir(), "../escape", &buf)
	if err == nil {
		t.Fatal("expected error for invalid schema name")
	}
}
