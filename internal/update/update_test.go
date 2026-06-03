package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func (f *fakeClient) LatestSHA(_ context.Context, _ source.Ref) (string, error) {
	return f.sha, nil
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
	res, err := Update(context.Background(), factoryFor(client), workDir, "widget", false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if res.Status != StatusUpdated {
		t.Fatalf("status = %v, want StatusUpdated", res.Status)
	}
	if res.NewSHA != "newsha" {
		t.Errorf("NewSHA = %q, want newsha", res.NewSHA)
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
	res, err := Update(context.Background(), factoryFor(client), workDir, "widget", false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if res.Status != StatusUpToDate {
		t.Errorf("status = %v, want StatusUpToDate", res.Status)
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
}

func TestUpdateMissingSchema(t *testing.T) {
	workDir := t.TempDir()
	client := &fakeClient{sha: "x", names: []string{"widget"}}
	res, err := Update(context.Background(), factoryFor(client), workDir, "widget", false)
	if err == nil {
		t.Fatal("expected error when schema is not installed")
	}
	if res.Status != StatusFailed {
		t.Errorf("status = %v, want StatusFailed", res.Status)
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
	_, err := Update(context.Background(), factoryFor(client), workDir, "widget", false)
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
	if _, err := Update(context.Background(), factoryFor(client), workDir, "widget", false); err != nil {
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

// snapshotDir captures bytes + mtime for every file under dir keyed by absolute
// path. It is used to assert no file was touched during a refused update.
func snapshotDir(t *testing.T, dir string) map[string]struct {
	data  []byte
	mtime int64
} {
	t.Helper()
	out := map[string]struct {
		data  []byte
		mtime int64
	}{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[path] = struct {
			data  []byte
			mtime int64
		}{data, info.ModTime().UnixNano()}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return out
}

func assertSnapshotUnchanged(t *testing.T, dir string, before map[string]struct {
	data  []byte
	mtime int64
}) {
	t.Helper()
	after := snapshotDir(t, dir)
	if len(after) != len(before) {
		t.Errorf("file set changed: before=%d files, after=%d files", len(before), len(after))
	}
	for path, b := range before {
		a, ok := after[path]
		if !ok {
			t.Errorf("file disappeared: %s", path)
			continue
		}
		if !bytes.Equal(a.data, b.data) {
			t.Errorf("file %s bytes changed: before=%q after=%q", path, b.data, a.data)
		}
		if a.mtime != b.mtime {
			t.Errorf("file %s mtime changed", path)
		}
	}
}

func TestUpdateRefusesWhenFileEdited(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"a.json":     []byte("alpha\n"),
		"sub/b.yaml": []byte("b: 1\n"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	// Hand-edit one tracked file.
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte("EDITED\n"), 0o644); err != nil {
		t.Fatalf("edit: %v", err)
	}
	before := snapshotDir(t, dir)

	client := &fakeClient{sha: "newsha", files: map[string][]byte{"a.json": []byte("upstream\n")}}
	res, err := Update(context.Background(), factoryFor(client), workDir, "widget", false)
	if err != nil {
		t.Fatalf("refusal should not surface as error: %v", err)
	}
	if res.Status != StatusRefused {
		t.Fatalf("status = %v, want StatusRefused", res.Status)
	}
	if !containsString(res.Offenders, "a.json") {
		t.Errorf("offenders %v should list a.json", res.Offenders)
	}
	if client.listCalls != 0 || client.fetchCalls != 0 {
		t.Errorf("no network calls expected on refusal; list=%d fetch=%d", client.listCalls, client.fetchCalls)
	}
	assertSnapshotUnchanged(t, dir, before)
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func TestUpdateRefusesWhenFileDeleted(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"a.json":     []byte("alpha\n"),
		"sub/b.yaml": []byte("b: 1\n"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	if err := os.Remove(filepath.Join(dir, "sub", "b.yaml")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	before := snapshotDir(t, dir)

	client := &fakeClient{sha: "newsha", files: map[string][]byte{"a.json": []byte("upstream\n")}}
	res, err := Update(context.Background(), factoryFor(client), workDir, "widget", false)
	if err != nil {
		t.Fatalf("refusal should not surface as error: %v", err)
	}
	if res.Status != StatusRefused {
		t.Fatalf("status = %v, want StatusRefused", res.Status)
	}
	if !containsString(res.Offenders, "sub/b.yaml") {
		t.Errorf("offenders %v should list sub/b.yaml", res.Offenders)
	}
	if client.listCalls != 0 || client.fetchCalls != 0 {
		t.Errorf("no network calls expected on refusal; list=%d fetch=%d", client.listCalls, client.fetchCalls)
	}
	assertSnapshotUnchanged(t, dir, before)
}

func TestUpdateRefusesWhenExtraFile(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"a.json": []byte("alpha\n"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}
	before := snapshotDir(t, dir)

	client := &fakeClient{sha: "newsha", files: map[string][]byte{"a.json": []byte("upstream\n")}}
	res, err := Update(context.Background(), factoryFor(client), workDir, "widget", false)
	if err != nil {
		t.Fatalf("refusal should not surface as error: %v", err)
	}
	if res.Status != StatusRefused {
		t.Fatalf("status = %v, want StatusRefused", res.Status)
	}
	if !containsString(res.Offenders, "extra.txt") {
		t.Errorf("offenders %v should list extra.txt", res.Offenders)
	}
	if client.listCalls != 0 || client.fetchCalls != 0 {
		t.Errorf("no network calls expected on refusal; list=%d fetch=%d", client.listCalls, client.fetchCalls)
	}
	assertSnapshotUnchanged(t, dir, before)
}

type failingListClient struct {
	fakeClient
	listErr error
}

func (f *failingListClient) ListSchemas(_ context.Context, _ source.Ref) (string, []string, error) {
	f.listCalls++
	return "", nil, f.listErr
}

func TestUpdatePropagatesUpstreamResolveError(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"a.json": []byte("alpha\n"),
	})
	client := &failingListClient{listErr: errors.New("network down")}
	_, err := Update(context.Background(), factoryFor(client), workDir, "widget", false)
	if err == nil {
		t.Fatal("expected error when upstream HEAD resolve fails")
	}
	if !strings.Contains(err.Error(), "network down") {
		t.Errorf("error %q should wrap underlying cause", err.Error())
	}
	if !strings.Contains(err.Error(), "widget") {
		t.Errorf("error %q should mention schema name", err.Error())
	}
	if client.fetchCalls != 0 {
		t.Errorf("FetchSchemaFiles should not be called when list fails; got %d", client.fetchCalls)
	}
}

func TestUpdateCreatesSnapshotBeforeOverwrite(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"keep.json": []byte("KEEP-OLD"),
		"drop.yaml": []byte("DROP-OLD"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	manifestBytes, err := os.ReadFile(filepath.Join(dir, install.ManifestFile))
	if err != nil {
		t.Fatalf("read pre-update manifest: %v", err)
	}

	fixed := time.Date(2026, 6, 3, 14, 30, 12, 0, time.UTC)
	prev := snapshotClock
	snapshotClock = func() time.Time { return fixed }
	t.Cleanup(func() { snapshotClock = prev })

	client := &fakeClient{
		sha:   "newsha",
		names: []string{"widget"},
		files: map[string][]byte{"keep.json": []byte("KEEP-NEW")},
	}
	res, err := Update(context.Background(), factoryFor(client), workDir, "widget", false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	snapDir := filepath.Join(dir, install.SnapshotDir, "20260603T143012Z")
	if res.SnapshotPath != snapDir {
		t.Errorf("Result.SnapshotPath = %q, want %q", res.SnapshotPath, snapDir)
	}
	got, err := os.ReadFile(filepath.Join(snapDir, "keep.json"))
	if err != nil || string(got) != "KEEP-OLD" {
		t.Errorf("snapshot keep.json = %q err=%v, want pre-update bytes", got, err)
	}
	got, err = os.ReadFile(filepath.Join(snapDir, "drop.yaml"))
	if err != nil || string(got) != "DROP-OLD" {
		t.Errorf("snapshot drop.yaml = %q err=%v, want pre-update bytes", got, err)
	}
	got, err = os.ReadFile(filepath.Join(snapDir, install.ManifestFile))
	if err != nil || !bytes.Equal(got, manifestBytes) {
		t.Errorf("snapshot manifest mismatch err=%v", err)
	}

	gi, err := os.ReadFile(filepath.Join(dir, install.SnapshotDir, ".gitignore"))
	if err != nil || string(gi) != "*\n" {
		t.Errorf(".osch/.gitignore = %q err=%v, want \"*\\n\"", gi, err)
	}
}

func TestUpdateNoOpDoesNotSnapshot(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "samesha", map[string][]byte{
		"keep.json": []byte("k"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	client := &fakeClient{sha: "samesha", names: []string{"widget"}}
	if _, err := Update(context.Background(), factoryFor(client), workDir, "widget", false); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, install.SnapshotDir)); !os.IsNotExist(err) {
		t.Errorf(".osch/ should not exist after no-op update; stat err=%v", err)
	}
}

func TestUpdateSnapshotExcludesNestedSnapshotDir(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"keep.json": []byte("KEEP-OLD"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	// Pre-existing snapshot from a previous update — must not be copied
	// into the new snapshot.
	priorSnap := filepath.Join(dir, install.SnapshotDir, "20260101T000000Z")
	if err := os.MkdirAll(priorSnap, 0o755); err != nil {
		t.Fatalf("mkdir prior snap: %v", err)
	}
	if err := os.WriteFile(filepath.Join(priorSnap, "evidence.txt"), []byte("prior"), 0o644); err != nil {
		t.Fatalf("write prior: %v", err)
	}

	fixed := time.Date(2026, 6, 3, 14, 30, 12, 0, time.UTC)
	prev := snapshotClock
	snapshotClock = func() time.Time { return fixed }
	t.Cleanup(func() { snapshotClock = prev })

	client := &fakeClient{
		sha:   "newsha",
		names: []string{"widget"},
		files: map[string][]byte{"keep.json": []byte("KEEP-NEW")},
	}
	if _, err := Update(context.Background(), factoryFor(client), workDir, "widget", false); err != nil {
		t.Fatalf("update: %v", err)
	}

	snapDir := filepath.Join(dir, install.SnapshotDir, "20260603T143012Z")
	if _, err := os.Stat(filepath.Join(snapDir, install.SnapshotDir)); !os.IsNotExist(err) {
		t.Errorf("new snapshot should not nest .osch/; stat err=%v", err)
	}
	// Prior snapshot should still be there (pruneStale must not remove it).
	if _, err := os.Stat(filepath.Join(priorSnap, "evidence.txt")); err != nil {
		t.Errorf("prior snapshot wiped: %v", err)
	}
}

func TestUpdateAbortsWhenSnapshotFails(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"keep.json": []byte("KEEP-OLD"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	fixed := time.Date(2026, 6, 3, 14, 30, 12, 0, time.UTC)
	prev := snapshotClock
	snapshotClock = func() time.Time { return fixed }
	t.Cleanup(func() { snapshotClock = prev })

	// Plant a regular file where the timestamped snapshot folder would be so
	// MkdirAll fails. CheckLocalFiles skips .osch/ recursively, so this does
	// not register as drift.
	if err := os.MkdirAll(filepath.Join(dir, install.SnapshotDir), 0o755); err != nil {
		t.Fatalf("mkdir .osch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, install.SnapshotDir, "20260603T143012Z"), []byte("blocked"), 0o644); err != nil {
		t.Fatalf("plant blocker: %v", err)
	}
	keepBefore, err := os.ReadFile(filepath.Join(dir, "keep.json"))
	if err != nil {
		t.Fatalf("read keep.json: %v", err)
	}
	manifestBefore, mErr := os.ReadFile(filepath.Join(dir, install.ManifestFile))
	if mErr != nil {
		t.Fatalf("read manifest: %v", mErr)
	}

	client := &fakeClient{
		sha:   "newsha",
		names: []string{"widget"},
		files: map[string][]byte{"keep.json": []byte("KEEP-NEW")},
	}
	_, err = Update(context.Background(), factoryFor(client), workDir, "widget", false)
	if err == nil {
		t.Fatal("expected error when snapshot creation fails")
	}
	if !strings.Contains(err.Error(), "snapshot") {
		t.Errorf("error %q should mention snapshot failure", err.Error())
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "keep.json")); !bytes.Equal(got, keepBefore) {
		t.Errorf("keep.json = %q, want %q (no overwrite on snapshot failure)", got, keepBefore)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, install.ManifestFile)); !bytes.Equal(got, manifestBefore) {
		t.Errorf("manifest rewritten on snapshot failure")
	}
}

func TestUpdateRejectsInvalidName(t *testing.T) {
	client := &fakeClient{sha: "x"}
	_, err := Update(context.Background(), factoryFor(client), t.TempDir(), "../escape", false)
	if err == nil {
		t.Fatal("expected error for invalid schema name")
	}
}

func TestUpdateForceWithEditedFilesSucceedsAndSnapshotsOriginal(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"a.json": []byte("alpha\n"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte("EDITED\n"), 0o644); err != nil {
		t.Fatalf("edit: %v", err)
	}

	fixed := time.Date(2026, 6, 3, 14, 30, 12, 0, time.UTC)
	prev := snapshotClock
	snapshotClock = func() time.Time { return fixed }
	t.Cleanup(func() { snapshotClock = prev })

	client := &fakeClient{
		sha:   "newsha",
		names: []string{"widget"},
		files: map[string][]byte{"a.json": []byte("UPSTREAM\n")},
	}
	res, err := Update(context.Background(), factoryFor(client), workDir, "widget", true)
	if err != nil {
		t.Fatalf("force update: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.json"))
	if err != nil || string(got) != "UPSTREAM\n" {
		t.Errorf("a.json = %q err=%v, want UPSTREAM", got, err)
	}
	snap := filepath.Join(dir, install.SnapshotDir, "20260603T143012Z", "a.json")
	snapGot, err := os.ReadFile(snap)
	if err != nil || string(snapGot) != "EDITED\n" {
		t.Errorf("snapshot a.json = %q err=%v, want EDITED (the user's pre-update bytes)", snapGot, err)
	}
	wantSnap := filepath.Join(dir, install.SnapshotDir, "20260603T143012Z")
	if res.SnapshotPath != wantSnap {
		t.Errorf("Result.SnapshotPath = %q, want %q", res.SnapshotPath, wantSnap)
	}
}

func TestUpdateForceWithExtraFilesSucceedsAndPrunesExtras(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"a.json": []byte("alpha\n"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}

	fixed := time.Date(2026, 6, 3, 14, 30, 12, 0, time.UTC)
	prev := snapshotClock
	snapshotClock = func() time.Time { return fixed }
	t.Cleanup(func() { snapshotClock = prev })

	client := &fakeClient{
		sha:   "newsha",
		names: []string{"widget"},
		files: map[string][]byte{"a.json": []byte("UPSTREAM\n")},
	}
	if _, err := Update(context.Background(), factoryFor(client), workDir, "widget", true); err != nil {
		t.Fatalf("force update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "extra.txt")); !os.IsNotExist(err) {
		t.Errorf("extra.txt should have been pruned; stat err=%v", err)
	}
	snapExtra := filepath.Join(dir, install.SnapshotDir, "20260603T143012Z", "extra.txt")
	got, err := os.ReadFile(snapExtra)
	if err != nil || string(got) != "untracked\n" {
		t.Errorf("snapshot extra.txt = %q err=%v, want untracked", got, err)
	}
}

func TestUpdateForceWithMissingTrackedFileRestoresUpstream(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"a.json":     []byte("alpha\n"),
		"sub/b.yaml": []byte("b: 1\n"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	if err := os.Remove(filepath.Join(dir, "sub", "b.yaml")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	client := &fakeClient{
		sha:   "newsha",
		names: []string{"widget"},
		files: map[string][]byte{
			"a.json":     []byte("alpha\n"),
			"sub/b.yaml": []byte("b: 2\n"),
		},
	}
	if _, err := Update(context.Background(), factoryFor(client), workDir, "widget", true); err != nil {
		t.Fatalf("force update: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "sub", "b.yaml"))
	if err != nil || string(got) != "b: 2\n" {
		t.Errorf("sub/b.yaml = %q err=%v, want upstream restored", got, err)
	}
}

func TestUpdateForceOnCleanSchemaBehavesLikeNoFlag(t *testing.T) {
	build := func(t *testing.T) (string, *fakeClient) {
		t.Helper()
		work := t.TempDir()
		seed(t, work, "widget", "oldsha", map[string][]byte{
			"a.json": []byte("alpha\n"),
		})
		client := &fakeClient{
			sha:   "newsha",
			names: []string{"widget"},
			files: map[string][]byte{"a.json": []byte("UPSTREAM\n")},
		}
		return work, client
	}

	workA, clientA := build(t)
	if _, err := Update(context.Background(), factoryFor(clientA), workA, "widget", false); err != nil {
		t.Fatalf("no-flag update: %v", err)
	}
	workB, clientB := build(t)
	if _, err := Update(context.Background(), factoryFor(clientB), workB, "widget", true); err != nil {
		t.Fatalf("force update: %v", err)
	}

	gotA, _ := os.ReadFile(filepath.Join(workA, "openspec", "schemas", "widget", "a.json"))
	gotB, _ := os.ReadFile(filepath.Join(workB, "openspec", "schemas", "widget", "a.json"))
	if !bytes.Equal(gotA, gotB) {
		t.Errorf("force on clean diverged from no-flag: %q vs %q", gotA, gotB)
	}
	mA, err := install.ReadManifest(filepath.Join(workA, "openspec", "schemas", "widget"))
	if err != nil {
		t.Fatalf("read manifest A: %v", err)
	}
	mB, err := install.ReadManifest(filepath.Join(workB, "openspec", "schemas", "widget"))
	if err != nil {
		t.Fatalf("read manifest B: %v", err)
	}
	if mA.SHA != mB.SHA {
		t.Errorf("SHA diverged: %q vs %q", mA.SHA, mB.SHA)
	}
}

func TestUpdateForceDoesNotMaskUpstreamResolveError(t *testing.T) {
	workDir := t.TempDir()
	seed(t, workDir, "widget", "oldsha", map[string][]byte{
		"a.json": []byte("alpha\n"),
	})
	dir := filepath.Join(workDir, "openspec", "schemas", "widget")
	// Edit so that without --force we'd refuse before resolving upstream;
	// with --force the resolve error must still propagate.
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte("EDITED\n"), 0o644); err != nil {
		t.Fatalf("edit: %v", err)
	}
	client := &failingListClient{listErr: errors.New("network down")}
	_, err := Update(context.Background(), factoryFor(client), workDir, "widget", true)
	if err == nil {
		t.Fatal("expected error when upstream resolve fails under --force")
	}
	if !strings.Contains(err.Error(), "network down") {
		t.Errorf("error %q should wrap underlying cause", err.Error())
	}
}
