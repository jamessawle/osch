package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jamessawle/osch/internal/source"
)

// fakeClient is a stand-in for a source client used to drive the add command
// without touching the network. Add itself is exercised in internal/install;
// the tests here cover argument validation and provider wiring.
type fakeClient struct {
	sha   string
	names []string
	files map[string][]byte
}

func (f *fakeClient) ListSchemas(_ context.Context, _ source.Ref) (string, []string, error) {
	return f.sha, f.names, nil
}

func (f *fakeClient) FetchSchemaFiles(_ context.Context, _ source.Ref, _, _ string) (map[string][]byte, error) {
	return f.files, nil
}

// withClient swaps the package-level client factory for the duration of a
// test. Tests must call it via t.Cleanup so they do not leak state.
func withClient(t *testing.T, c source.Client) {
	t.Helper()
	prev := clientFactory
	clientFactory = func(string) (source.Client, error) { return c, nil }
	t.Cleanup(func() { clientFactory = prev })
}

func TestAddMissingArg(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"add"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when no repo argument is given")
	}
}

func TestAddTooManyArgs(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"add", "a/b", "schema", "extra"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when more than two arguments are given")
	}
}

func TestAddInvalidRepoArg(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"add", "notaslug"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid repo argument")
	}
	if !strings.Contains(err.Error(), "user/repo") {
		t.Errorf("error %q should explain the expected form", err.Error())
	}
}

func TestAddRunsAgainstFakeClient(t *testing.T) {
	client := &fakeClient{
		sha:   "deadbeef",
		names: []string{"widget"},
		files: map[string][]byte{"manifest.json": []byte(`{"k":"v"}`)},
	}
	withClient(t, client)

	dir := t.TempDir()
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"add", "acme/widgets"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(buf.String(), "acme/widgets") {
		t.Errorf("success output %q should mention the repo", buf.String())
	}
}
