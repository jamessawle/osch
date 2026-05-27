package source

import (
	"errors"
	"strings"
	"testing"
)

func TestClientErrorMessages(t *testing.T) {
	ref := Ref{Provider: ProviderGitHub, Owner: "acme", Name: "widgets"}
	cases := []struct {
		name string
		err  *ClientError
		want []string
	}{
		{"not found", NotFoundError(ref), []string{"acme/widgets", "not found"}},
		{"no schemas", NoSchemasError(ref), []string{"acme/widgets", "schemas/"}},
		{"empty schemas", EmptySchemasError(ref), []string{"acme/widgets", "empty"}},
		{"network", NetworkError(ref, errors.New("dial tcp: timeout")), []string{"acme/widgets", "dial tcp: timeout"}},
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
	err := NetworkError(Ref{Provider: ProviderGitHub, Owner: "a", Name: "b"}, underlying)
	if !errors.Is(err, underlying) {
		t.Errorf("NetworkError should unwrap to the underlying error")
	}
}
