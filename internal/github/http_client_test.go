package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jamessawle/osch/internal/source"
)

// newTestClient points an HTTPClient at a test server. It resolves to the
// anonymous token so tests never read the environment or spawn gh.
func newTestClient(t *testing.T, srv *httptest.Server) *HTTPClient {
	t.Helper()
	c := newHTTPClient([]TokenSource{StaticTokenSource{Value: ""}})
	base, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	c.BaseURL = base
	c.HTTP = srv.Client()
	return c
}

func TestHTTPClientSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"full_name":"acme/widgets"}`))
		case "/repos/acme/widgets/contents/schemas":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"type":"file","name":"a.json"},{"type":"file","name":"b.json"}]`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "a.json" || got[1] != "b.json" {
		t.Errorf("got %v, want [a.json b.json]", got)
	}
}

func TestHTTPClientSendsAuthorizationWhenTokenResolved(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/repos/acme/widgets":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"full_name":"acme/widgets"}`))
		case "/repos/acme/widgets/contents/schemas":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"type":"file","name":"a.json"}]`))
		}
	}))
	defer srv.Close()

	c := newHTTPClient([]TokenSource{StaticTokenSource{Value: "secret-token"}})
	base, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	c.BaseURL = base
	c.HTTP = srv.Client()

	if _, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer secret-token")
	}
}

func TestHTTPClientOmitsAuthorizationWhenAnonymous(t *testing.T) {
	var sawAuthHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Header["Authorization"]; ok {
			sawAuthHeader = true
		}
		switch r.URL.Path {
		case "/repos/acme/widgets":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"full_name":"acme/widgets"}`))
		case "/repos/acme/widgets/contents/schemas":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"type":"file","name":"a.json"}]`))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawAuthHeader {
		t.Error("anonymous client sent an Authorization header")
	}
}

func TestHTTPClientRepoNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "ghost"})
	assertKind(t, err, source.KindNotFound)
}

func TestHTTPClientNoSchemasFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"full_name":"acme/widgets"}`))
		case "/repos/acme/widgets/contents/schemas":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	assertKind(t, err, source.KindNoSchemas)
}

func TestHTTPClientEmptySchemasFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"full_name":"acme/widgets"}`))
		case "/repos/acme/widgets/contents/schemas":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	assertKind(t, err, source.KindEmptySchemas)
}

func TestHTTPClientNetworkError(t *testing.T) {
	// Start a server and immediately close it so the connection is refused.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	c := newTestClient(t, srv)
	srv.Close()

	_, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	assertKind(t, err, source.KindNetwork)
}

func TestHTTPClientContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	_, err := c.ListSchemas(ctx, source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	assertKind(t, err, source.KindNetwork)
}

func assertKind(t *testing.T, err error, want source.ErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error of kind %d, got nil", want)
	}
	ce, ok := err.(*source.ClientError)
	if !ok {
		t.Fatalf("expected *source.ClientError, got %T: %v", err, err)
	}
	if ce.Kind != want {
		t.Errorf("got kind %d, want %d (msg: %s)", ce.Kind, want, ce.Error())
	}
}
