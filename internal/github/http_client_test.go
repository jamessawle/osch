package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

// successHandler answers a minimal repo / commit / contents flow with one schema
// directory named "widget". Tests pick this up via newTestClient.
func successHandler(t *testing.T, schemaDirs ...string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/acme/widgets/commits/main":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sha":"abc123"}`))
		case "/repos/acme/widgets/contents/schemas":
			parts := make([]string, 0, len(schemaDirs))
			for _, d := range schemaDirs {
				parts = append(parts, `{"type":"dir","name":"`+d+`","path":"schemas/`+d+`"}`)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("[" + strings.Join(parts, ",") + "]"))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}

func TestHTTPClientSuccess(t *testing.T) {
	srv := httptest.NewServer(successHandler(t, "widget", "gadget"))
	defer srv.Close()

	c := newTestClient(t, srv)
	sha, names, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "abc123" {
		t.Errorf("sha = %q, want abc123", sha)
	}
	if len(names) != 2 || names[0] != "widget" || names[1] != "gadget" {
		t.Errorf("got %v, want [widget gadget]", names)
	}
}

func TestHTTPClientSendsAuthorizationWhenTokenResolved(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/repos/acme/widgets":
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/acme/widgets/commits/main":
			_, _ = w.Write([]byte(`{"sha":"abc123"}`))
		case "/repos/acme/widgets/contents/schemas":
			_, _ = w.Write([]byte(`[{"type":"dir","name":"widget","path":"schemas/widget"}]`))
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

	if _, _, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"}); err != nil {
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
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/acme/widgets/commits/main":
			_, _ = w.Write([]byte(`{"sha":"abc123"}`))
		case "/repos/acme/widgets/contents/schemas":
			_, _ = w.Write([]byte(`[{"type":"dir","name":"widget","path":"schemas/widget"}]`))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"}); err != nil {
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
	_, _, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "ghost"})
	assertKind(t, err, source.KindNotFound)
}

func TestHTTPClientNoSchemasFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets":
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/acme/widgets/commits/main":
			_, _ = w.Write([]byte(`{"sha":"abc123"}`))
		case "/repos/acme/widgets/contents/schemas":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, _, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	assertKind(t, err, source.KindNoSchemas)
}

func TestHTTPClientEmptySchemasFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets":
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/acme/widgets/commits/main":
			_, _ = w.Write([]byte(`{"sha":"abc123"}`))
		case "/repos/acme/widgets/contents/schemas":
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, _, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	assertKind(t, err, source.KindEmptySchemas)
}

func TestHTTPClientNetworkError(t *testing.T) {
	// Start a server and immediately close it so the connection is refused.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	c := newTestClient(t, srv)
	srv.Close()

	_, _, err := c.ListSchemas(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
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
	_, _, err := c.ListSchemas(ctx, source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	assertKind(t, err, source.KindNetwork)
}

func TestHTTPClientFetchSchemaFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets/contents/schemas/widget":
			_, _ = w.Write([]byte(`[
				{"type":"file","name":"a.json","path":"schemas/widget/a.json"},
				{"type":"dir","name":"sub","path":"schemas/widget/sub"}
			]`))
		case "/repos/acme/widgets/contents/schemas/widget/sub":
			_, _ = w.Write([]byte(`[{"type":"file","name":"b.json","path":"schemas/widget/sub/b.json"}]`))
		case "/repos/acme/widgets/contents/schemas/widget/a.json":
			_, _ = w.Write([]byte(`A`))
		case "/repos/acme/widgets/contents/schemas/widget/sub/b.json":
			_, _ = w.Write([]byte(`B`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	files, err := c.FetchSchemaFiles(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"}, "abc123", "widget")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(files["a.json"]); got != "A" {
		t.Errorf("a.json = %q, want A", got)
	}
	if got := string(files["sub/b.json"]); got != "B" {
		t.Errorf("sub/b.json = %q, want B", got)
	}
	if len(files) != 2 {
		t.Errorf("got %d files, want 2: %v", len(files), files)
	}
}

func TestHTTPClientLatestSHA(t *testing.T) {
	var sawContents bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets":
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/acme/widgets/commits/main":
			_, _ = w.Write([]byte(`{"sha":"abc123"}`))
		case "/repos/acme/widgets/contents/schemas":
			sawContents = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	sha, err := c.LatestSHA(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "widgets"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "abc123" {
		t.Errorf("sha = %q, want abc123", sha)
	}
	if sawContents {
		t.Error("LatestSHA must not list /contents/schemas")
	}
}

func TestHTTPClientLatestSHARepoNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.LatestSHA(context.Background(), source.Ref{Provider: source.ProviderGitHub, Owner: "acme", Name: "ghost"})
	assertKind(t, err, source.KindNotFound)
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
