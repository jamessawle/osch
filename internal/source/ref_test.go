package source

import (
	"strings"
	"testing"
)

func TestParseRefValid(t *testing.T) {
	ref, err := ParseRef("openspec/schemas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "openspec" || ref.Name != "schemas" {
		t.Errorf("got %+v, want {openspec schemas}", ref)
	}
	if ref.Host != HostGitHub {
		t.Errorf("got host %q, want %q", ref.Host, HostGitHub)
	}
}

func TestParseRefInvalid(t *testing.T) {
	cases := map[string]string{
		"empty":          "",
		"no slash":       "noslash",
		"missing name":   "owner/",
		"missing owner":  "/repo",
		"too many parts": "owner/repo/extra",
		"whitespace":     "own er/repo",
		"only slash":     "/",
	}
	for name, arg := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseRef(arg)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", arg)
			}
			// Message must be friendly: mention the expected form and not look like a stack trace.
			if !strings.Contains(err.Error(), "user/repo") {
				t.Errorf("error %q should mention the expected user/repo form", err.Error())
			}
		})
	}
}

func TestRefString(t *testing.T) {
	if got := (Ref{Host: HostGitHub, Owner: "a", Name: "b"}).String(); got != "a/b" {
		t.Errorf("Ref.String() = %q, want %q", got, "a/b")
	}
}
