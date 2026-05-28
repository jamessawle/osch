package list

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mkSchema(t *testing.T, root, name string, tracked bool) {
	t.Helper()
	dir := filepath.Join(root, "openspec", "schemas", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if tracked {
		writeFile(t, filepath.Join(dir, ".osch.json"), `{"name":"`+name+`"}`)
	}
}

func writeConfig(t *testing.T, root, body string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "openspec", "config.yaml"), body)
}

const emptyOutput = "No OpenSpec schemas installed\n"

func TestListMissingOpenspec(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	if buf.String() != emptyOutput {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestListMissingSchemasDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "openspec"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	if buf.String() != emptyOutput {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestListEmptySchemasDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "openspec", "schemas"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	if buf.String() != emptyOutput {
		t.Errorf("expected empty-state message, got: %q", buf.String())
	}
}

func TestListOneTrackedActive(t *testing.T) {
	dir := t.TempDir()
	mkSchema(t, dir, "widget", true)
	writeConfig(t, dir, "schema: widget\n")

	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	fields := schemaRow(t, buf.String(), "widget")
	if fields[1] != "*" {
		t.Errorf("expected ACTIVE=*, got %q in %q", fields[1], buf.String())
	}
	if fields[2] != "yes" {
		t.Errorf("expected TRACKED=yes, got %q in %q", fields[2], buf.String())
	}
}

func TestListOneTrackedInactive(t *testing.T) {
	dir := t.TempDir()
	mkSchema(t, dir, "widget", true)
	writeConfig(t, dir, "schema: gadget\n")

	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	fields := schemaRow(t, buf.String(), "widget")
	if fields[1] != "" {
		t.Errorf("expected ACTIVE empty, got %q in %q", fields[1], buf.String())
	}
	if fields[2] != "yes" {
		t.Errorf("expected TRACKED=yes, got %q", fields[2])
	}
}

func TestListOneUntracked(t *testing.T) {
	dir := t.TempDir()
	mkSchema(t, dir, "widget", false)

	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	fields := schemaRow(t, buf.String(), "widget")
	if fields[1] != "" {
		t.Errorf("expected ACTIVE empty (no config), got %q", fields[1])
	}
	if fields[2] != "no" {
		t.Errorf("expected TRACKED=no, got %q", fields[2])
	}
}

func TestListMix(t *testing.T) {
	dir := t.TempDir()
	mkSchema(t, dir, "alpha", true)
	mkSchema(t, dir, "beta", false)
	mkSchema(t, dir, "gamma", true)
	writeConfig(t, dir, "schema: gamma\n")

	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	out := buf.String()

	alpha := schemaRow(t, out, "alpha")
	if alpha[1] != "" || alpha[2] != "yes" {
		t.Errorf("alpha row wrong: %v", alpha)
	}
	beta := schemaRow(t, out, "beta")
	if beta[1] != "" || beta[2] != "no" {
		t.Errorf("beta row wrong: %v", beta)
	}
	gamma := schemaRow(t, out, "gamma")
	if gamma[1] != "*" || gamma[2] != "yes" {
		t.Errorf("gamma row wrong: %v", gamma)
	}

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), out)
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[1]), "alpha") ||
		!strings.HasPrefix(strings.TrimSpace(lines[2]), "beta") ||
		!strings.HasPrefix(strings.TrimSpace(lines[3]), "gamma") {
		t.Errorf("rows not sorted: %q", out)
	}
}

func TestListConfigWithoutSchemaKey(t *testing.T) {
	dir := t.TempDir()
	mkSchema(t, dir, "widget", true)
	writeConfig(t, dir, "other: thing\n")

	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	fields := schemaRow(t, buf.String(), "widget")
	if fields[1] != "" {
		t.Errorf("expected ACTIVE empty, got %q", fields[1])
	}
}

// A malformed config.yaml must not be a hard failure — it's an OpenSpec-owned
// file, and the inventory view is not a config validator.
func TestListMalformedConfig(t *testing.T) {
	dir := t.TempDir()
	mkSchema(t, dir, "widget", true)
	writeConfig(t, dir, "schema: [not-a-string\n  - broken: yaml\n")

	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	fields := schemaRow(t, buf.String(), "widget")
	if fields[1] != "" {
		t.Errorf("expected ACTIVE empty for malformed config, got %q", fields[1])
	}
}

// Files and other non-directory entries in openspec/schemas/ are silently
// ignored — only direct subdirectories are listed.
func TestListIgnoresNonDirectoryEntries(t *testing.T) {
	dir := t.TempDir()
	mkSchema(t, dir, "widget", true)
	writeFile(t, filepath.Join(dir, "openspec", "schemas", ".DS_Store"), "junk")
	writeFile(t, filepath.Join(dir, "openspec", "schemas", "README.md"), "# notes\n")

	var buf bytes.Buffer
	if err := List(dir, &buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, ".DS_Store") || strings.Contains(out, "README.md") {
		t.Errorf("non-directory entry leaked into output: %q", out)
	}
	if !strings.Contains(out, "widget") {
		t.Errorf("expected widget row, got %q", out)
	}
}

// An unreadable openspec/schemas/ is a hard failure — every I/O error other
// than fs.ErrNotExist must surface.
func TestListSchemasDirUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics required")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
	dir := t.TempDir()
	schemasDir := filepath.Join(dir, "openspec", "schemas")
	if err := os.MkdirAll(schemasDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(schemasDir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(schemasDir, 0o755) })

	var buf bytes.Buffer
	err := List(dir, &buf)
	if err == nil {
		t.Fatalf("expected error for unreadable schemas dir, got nil; output=%q", buf.String())
	}
}

// schemaRow returns the whitespace-split fields of the table row whose first
// column equals name. tabwriter pads with spaces, so Fields collapses runs of
// whitespace and gives us [NAME, ACTIVE, TRACKED] — except ACTIVE may have
// collapsed away when blank, so we re-expand below.
func schemaRow(t *testing.T, out, name string) [3]string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), name+" ") &&
			strings.TrimSpace(line) != name {
			continue
		}
		fields := strings.Fields(line)
		switch len(fields) {
		case 3:
			return [3]string{fields[0], fields[1], fields[2]}
		case 2:
			return [3]string{fields[0], "", fields[1]}
		default:
			t.Fatalf("unexpected field count for row %q: %v", line, fields)
		}
	}
	t.Fatalf("row for %q not found in %q", name, out)
	return [3]string{}
}
