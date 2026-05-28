package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestListNoSchemasDir(t *testing.T) {
	dir := t.TempDir()
	cwd := chdir(t, dir)
	defer cwd()

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(buf.String(), "No OpenSpec schemas installed") {
		t.Errorf("expected no-schemas message, got %q", buf.String())
	}
}
