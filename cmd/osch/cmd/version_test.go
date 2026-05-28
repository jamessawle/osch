package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestVersionPlain(t *testing.T) {
	out, err := runRoot(t, "version")
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.HasPrefix(out, "osch ") {
		t.Errorf("unexpected plain output: %q", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("plain output should not be JSON: %q", out)
	}
}

func TestVersionJSON(t *testing.T) {
	out, err := runRoot(t, "version", "--json")
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v (output=%q)", err, out)
	}
	for _, key := range []string{"version", "commit", "date"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in JSON output: %v", key, got)
		}
	}
}
