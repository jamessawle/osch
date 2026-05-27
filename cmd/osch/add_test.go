package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jamessawle/osch/internal/github"
)

// fakeClient is a stand-in for the GitHub client used to exercise each error
// path of the add command without touching the network.
type fakeClient struct {
	names []string
	err   error
}

func (f fakeClient) ListSchemas(_ context.Context, _ github.Repo) ([]string, error) {
	return f.names, f.err
}

func TestRunAddSuccess(t *testing.T) {
	var buf bytes.Buffer
	client := fakeClient{names: []string{"a.json", "b.json"}}
	err := runAdd(context.Background(), client, []string{"acme/widgets"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "acme/widgets") {
		t.Errorf("success output %q should mention the repo", buf.String())
	}
}

func TestRunAddMissingArg(t *testing.T) {
	var buf bytes.Buffer
	// No client call should happen; nil client proves it.
	err := runAdd(context.Background(), nil, nil, &buf)
	if err == nil {
		t.Fatal("expected error when no repo argument is given")
	}
}

func TestRunAddTooManyArgs(t *testing.T) {
	var buf bytes.Buffer
	err := runAdd(context.Background(), nil, []string{"a/b", "c/d"}, &buf)
	if err == nil {
		t.Fatal("expected error when more than one argument is given")
	}
}

func TestRunAddInvalidRepoArg(t *testing.T) {
	var buf bytes.Buffer
	// nil client: parsing must fail before any client call.
	err := runAdd(context.Background(), nil, []string{"notaslug"}, &buf)
	if err == nil {
		t.Fatal("expected error for invalid repo argument")
	}
	if !strings.Contains(err.Error(), "user/repo") {
		t.Errorf("error %q should explain the expected form", err.Error())
	}
}

func TestRunAddUpstreamErrors(t *testing.T) {
	repo := github.Repo{Owner: "acme", Name: "widgets"}
	cases := []struct {
		name string
		err  error
	}{
		{"not found", github.NotFoundError(repo)},
		{"no schemas", github.NoSchemasError(repo)},
		{"empty schemas", github.EmptySchemasError(repo)},
		{"network", github.NetworkError(repo, context.DeadlineExceeded)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			client := fakeClient{err: tc.err}
			err := runAdd(context.Background(), client, []string{"acme/widgets"}, &buf)
			if err == nil {
				t.Fatalf("expected error for %s path", tc.name)
			}
			if buf.Len() != 0 {
				t.Errorf("no success output expected on error, got %q", buf.String())
			}
		})
	}
}

func TestRunDispatchesAddErrorsNonZero(t *testing.T) {
	// Routing "add" with a bad argument must return a non-nil error so main
	// exits non-zero, without needing the network.
	var buf bytes.Buffer
	if err := run([]string{"add", "notaslug"}, &buf); err == nil {
		t.Fatal("expected run to return an error for invalid add argument")
	}
}
