package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionPlain(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"version"}, &buf); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "osch ") {
		t.Errorf("unexpected plain output: %q", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("plain output should not be JSON: %q", out)
	}
}

func TestVersionJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"version", "--json"}, &buf); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v (output=%q)", err, buf.String())
	}
	for _, key := range []string{"version", "commit", "date"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in JSON output: %v", key, got)
		}
	}
}
