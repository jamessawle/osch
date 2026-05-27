package github

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRepoValid(t *testing.T) {
	repo, err := ParseRepo("openspec/schemas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Owner != "openspec" || repo.Name != "schemas" {
		t.Errorf("got %+v, want {openspec schemas}", repo)
	}
}

func TestParseRepoInvalid(t *testing.T) {
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
			_, err := ParseRepo(arg)
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

func TestClientErrorMessages(t *testing.T) {
	repo := Repo{Owner: "acme", Name: "widgets"}
	cases := []struct {
		name string
		err  *ClientError
		want []string
	}{
		{"not found", NotFoundError(repo), []string{"acme/widgets", "not found"}},
		{"no schemas", NoSchemasError(repo), []string{"acme/widgets", "schemas/"}},
		{"empty schemas", EmptySchemasError(repo), []string{"acme/widgets", "empty"}},
		{"network", NetworkError(repo, errors.New("dial tcp: timeout")), []string{"acme/widgets", "dial tcp: timeout"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			for _, want := range tc.want {
				if !strings.Contains(msg, want) {
					t.Errorf("message %q should contain %q", msg, want)
				}
			}
		})
	}
}

func TestClientErrorUnwrap(t *testing.T) {
	underlying := errors.New("boom")
	err := NetworkError(Repo{Owner: "a", Name: "b"}, underlying)
	if !errors.Is(err, underlying) {
		t.Errorf("NetworkError should unwrap to the underlying error")
	}
}

func TestRepoString(t *testing.T) {
	if got := (Repo{Owner: "a", Name: "b"}).String(); got != "a/b" {
		t.Errorf("Repo.String() = %q, want %q", got, "a/b")
	}
}
