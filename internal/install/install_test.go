package install

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamessawle/osch/internal/source"
)

type fakeClient struct {
	sha   string
	names []string
	files map[string][]byte

	listErr  error
	fetchErr error
}

func (f *fakeClient) ListSchemas(_ context.Context, _ source.Ref) (string, []string, error) {
	return f.sha, f.names, f.listErr
}

func (f *fakeClient) FetchSchemaFiles(_ context.Context, _ source.Ref, _, _ string) (map[string][]byte, error) {
	return f.files, f.fetchErr
}

func ref() source.Ref {
	return source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"}
}

func TestAddSingleSchemaHappyPath(t *testing.T) {
	client := &fakeClient{
		sha:   "deadbeef",
		names: []string{"widget"},
		files: map[string][]byte{
			"manifest.json":    []byte(`{"hello":"world"}`),
			"sub/example.yaml": []byte("a: 1\n"),
		},
	}
	workDir := t.TempDir()
	var buf bytes.Buffer
	name, err := Add(context.Background(), client, ref(), workDir, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "widget" {
		t.Errorf("Add returned name %q, want widget", name)
	}

	for rel, want := range client.files {
		got, err := os.ReadFile(filepath.Join(workDir, "openspec", "schemas", "widget", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("file %s: got %q, want %q", rel, got, want)
		}
	}

	manifestPath := filepath.Join(workDir, "openspec", "schemas", "widget", ".osch.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var got Manifest
	if err := json.Unmarshal(manifestBytes, &got); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if got.Schema != ManifestSchemaURL {
		t.Errorf("manifest $schema = %q, want %q", got.Schema, ManifestSchemaURL)
	}
	if got.SchemaVersion != ManifestSchemaVersion {
		t.Errorf("schema_version = %d, want %d", got.SchemaVersion, ManifestSchemaVersion)
	}
	if got.Source != "acme/widgets" {
		t.Errorf("source = %q, want acme/widgets", got.Source)
	}
	if got.Name != "widget" {
		t.Errorf("name = %q, want widget", got.Name)
	}
	if got.SHA != "deadbeef" {
		t.Errorf("sha = %q, want deadbeef", got.SHA)
	}
	if len(got.Files) != len(client.files) {
		t.Errorf("files map has %d entries, want %d", len(got.Files), len(client.files))
	}
	for rel, data := range client.files {
		want := sha256.Sum256(data)
		if got.Files[rel] != hex.EncodeToString(want[:]) {
			t.Errorf("hash for %s = %q, want %q", rel, got.Files[rel], hex.EncodeToString(want[:]))
		}
	}
}

func TestAddRefusesWhenTargetExists(t *testing.T) {
	workDir := t.TempDir()
	existing := filepath.Join(workDir, "openspec", "schemas", "widget")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	client := &fakeClient{sha: "deadbeef", names: []string{"widget"}, files: map[string][]byte{"a.json": []byte("x")}}
	var buf bytes.Buffer
	_, err := Add(context.Background(), client, ref(), workDir, &buf)
	if err == nil {
		t.Fatal("expected refusal when target folder exists")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("error %q should explain the refusal", err.Error())
	}
}

func TestAddRefusesWhenUpstreamHasMultipleSchemas(t *testing.T) {
	client := &fakeClient{sha: "deadbeef", names: []string{"widget", "gadget"}}
	var buf bytes.Buffer
	_, err := Add(context.Background(), client, ref(), t.TempDir(), &buf)
	if err == nil {
		t.Fatal("expected error when upstream has multiple schemas")
	}
}

func TestManifestSerializationStableOrder(t *testing.T) {
	m := Manifest{
		Schema:        ManifestSchemaURL,
		SchemaVersion: ManifestSchemaVersion,
		Source:        "acme/widgets",
		Name:          "widget",
		SHA:           "abc",
		Files:         map[string]string{"b.json": "h2", "a.json": "h1"},
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if idx1, idx2 := bytes.Index(data, []byte(`"a.json"`)), bytes.Index(data, []byte(`"b.json"`)); idx1 < 0 || idx2 < 0 || idx1 > idx2 {
		t.Errorf("expected a.json before b.json in serialized output, got: %s", data)
	}
}
