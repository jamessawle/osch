// Package github is the GitHub implementation of source.Client. It reads
// schema folders from GitHub repositories via the REST API and maps GitHub's
// HTTP status codes onto the provider-agnostic error vocabulary in internal/source.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jamessawle/osch/internal/source"
)

const defaultBaseURL = "https://api.github.com"

// HTTPClient talks to the GitHub REST API. The zero value is not usable; build
// one with NewHTTPClient.
type HTTPClient struct {
	BaseURL *url.URL
	HTTP    *http.Client

	// token is the GitHub token resolved once at construction (empty means
	// anonymous). get() sends it as a Bearer credential when non-empty.
	token string
}

// NewClient returns the GitHub source.Client pointed at the public GitHub API.
func NewClient() source.Client {
	return NewHTTPClient()
}

// NewHTTPClient returns an HTTPClient pointed at the public GitHub API with a
// sensible request timeout. It resolves a GitHub token once, via the default
// chain (GITHUB_TOKEN, then gh CLI, then anonymous), and caches it on the
// client for the lifetime of the invocation.
func NewHTTPClient() *HTTPClient {
	return newHTTPClient(defaultTokenSources())
}

// newHTTPClient builds an HTTPClient resolving its token from the given chain.
// It is the seam tests use to inject fake token sources rather than reading the
// environment or spawning gh.
func newHTTPClient(sources []TokenSource) *HTTPClient {
	base, _ := url.Parse(defaultBaseURL)
	return &HTTPClient{
		BaseURL: base,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		token:   resolveToken(context.Background(), sources),
	}
}

// contentEntry is one item in a GitHub "contents" directory listing.
type contentEntry struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// ListSchemas returns the names of files under schemas/ at the ref's default
// branch HEAD. All failures are reported as *source.ClientError with a friendly
// message. It first confirms the repo is reachable so a 404 on the contents
// endpoint can be reported as "no schemas/ folder" rather than "not found".
func (c *HTTPClient) ListSchemas(ctx context.Context, ref source.Ref) ([]string, error) {
	repoStatus, _, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s", ref.Owner, ref.Name))
	if err != nil {
		return nil, source.NetworkError(ref, err)
	}
	if repoStatus == http.StatusNotFound {
		return nil, source.NotFoundError(ref)
	}
	if repoStatus != http.StatusOK {
		return nil, source.NetworkError(ref, fmt.Errorf("unexpected status %d from GitHub", repoStatus))
	}

	status, body, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/contents/schemas", ref.Owner, ref.Name))
	if err != nil {
		return nil, source.NetworkError(ref, err)
	}
	if status == http.StatusNotFound {
		return nil, source.NoSchemasError(ref)
	}
	if status != http.StatusOK {
		return nil, source.NetworkError(ref, fmt.Errorf("unexpected status %d from GitHub", status))
	}

	var entries []contentEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, source.NetworkError(ref, fmt.Errorf("could not parse GitHub response: %w", err))
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Type == "file" {
			names = append(names, e.Name)
		}
	}
	if len(names) == 0 {
		return nil, source.EmptySchemasError(ref)
	}
	return names, nil
}

// get performs a GET against the API path and returns the status code and body.
// A non-nil error indicates a transport-level failure (DNS, timeout, refused).
func (c *HTTPClient) get(ctx context.Context, path string) (int, []byte, error) {
	ref := &url.URL{Path: path}
	full := c.BaseURL.ResolveReference(ref)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full.String(), nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}
