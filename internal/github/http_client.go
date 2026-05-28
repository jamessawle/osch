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

// repoInfo is the subset of the repository payload we care about.
type repoInfo struct {
	DefaultBranch string `json:"default_branch"`
}

// commitInfo is the subset of the commit payload we care about.
type commitInfo struct {
	SHA string `json:"sha"`
}

// contentEntry is one item in a GitHub "contents" directory listing.
type contentEntry struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListSchemas resolves the default branch HEAD commit of ref and returns the
// names of schema directories under schemas/ at that commit. All failures are
// reported as *source.ClientError with a friendly message.
func (c *HTTPClient) ListSchemas(ctx context.Context, ref source.Ref) (string, []string, error) {
	repoStatus, repoBody, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s", ref.Owner, ref.Name), "")
	if err != nil {
		return "", nil, source.NetworkError(ref, err)
	}
	if repoStatus == http.StatusNotFound {
		return "", nil, source.NotFoundError(ref)
	}
	if repoStatus != http.StatusOK {
		return "", nil, source.NetworkError(ref, fmt.Errorf("unexpected status %d from GitHub", repoStatus))
	}
	var repo repoInfo
	if err := json.Unmarshal(repoBody, &repo); err != nil {
		return "", nil, source.NetworkError(ref, fmt.Errorf("could not parse GitHub repository response: %w", err))
	}
	if repo.DefaultBranch == "" {
		return "", nil, source.NetworkError(ref, fmt.Errorf("GitHub did not return a default branch"))
	}

	commitStatus, commitBody, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/commits/%s", ref.Owner, ref.Name, repo.DefaultBranch), "")
	if err != nil {
		return "", nil, source.NetworkError(ref, err)
	}
	if commitStatus != http.StatusOK {
		return "", nil, source.NetworkError(ref, fmt.Errorf("unexpected status %d resolving default branch commit", commitStatus))
	}
	var commit commitInfo
	if err := json.Unmarshal(commitBody, &commit); err != nil {
		return "", nil, source.NetworkError(ref, fmt.Errorf("could not parse GitHub commit response: %w", err))
	}
	if commit.SHA == "" {
		return "", nil, source.NetworkError(ref, fmt.Errorf("GitHub did not return a commit SHA"))
	}

	status, body, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/contents/schemas", ref.Owner, ref.Name), commit.SHA)
	if err != nil {
		return "", nil, source.NetworkError(ref, err)
	}
	if status == http.StatusNotFound {
		return "", nil, source.NoSchemasError(ref)
	}
	if status != http.StatusOK {
		return "", nil, source.NetworkError(ref, fmt.Errorf("unexpected status %d from GitHub", status))
	}

	var entries []contentEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return "", nil, source.NetworkError(ref, fmt.Errorf("could not parse GitHub response: %w", err))
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Type == "dir" {
			names = append(names, e.Name)
		}
	}
	if len(names) == 0 {
		return "", nil, source.EmptySchemasError(ref)
	}
	return commit.SHA, names, nil
}

// FetchSchemaFiles walks schemas/<name>/ at the given commit SHA and returns
// every file's bytes, keyed by forward-slash relative path within the schema
// folder. Failures are surfaced as *source.ClientError.
func (c *HTTPClient) FetchSchemaFiles(ctx context.Context, ref source.Ref, sha, name string) (map[string][]byte, error) {
	out := make(map[string][]byte)
	rootPath := "schemas/" + name
	if err := c.walk(ctx, ref, sha, rootPath, rootPath, out); err != nil {
		return nil, err
	}
	return out, nil
}

// walk recurses into a directory under the repo, accumulating file bytes under
// relative paths within the schema folder (rootPath).
func (c *HTTPClient) walk(ctx context.Context, ref source.Ref, sha, rootPath, dirPath string, out map[string][]byte) error {
	status, body, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/contents/%s", ref.Owner, ref.Name, dirPath), sha)
	if err != nil {
		return source.NetworkError(ref, err)
	}
	if status != http.StatusOK {
		return source.NetworkError(ref, fmt.Errorf("unexpected status %d listing %s", status, dirPath))
	}
	var entries []contentEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return source.NetworkError(ref, fmt.Errorf("could not parse GitHub response: %w", err))
	}
	for _, e := range entries {
		switch e.Type {
		case "dir":
			if err := c.walk(ctx, ref, sha, rootPath, e.Path, out); err != nil {
				return err
			}
		case "file":
			data, err := c.getRaw(ctx, fmt.Sprintf("/repos/%s/%s/contents/%s", ref.Owner, ref.Name, e.Path), sha)
			if err != nil {
				return source.NetworkError(ref, err)
			}
			rel := e.Path[len(rootPath)+1:]
			out[rel] = data
		}
	}
	return nil
}

// get performs a GET against the API path and returns the status code and body.
// When ref is non-empty it is appended as a ?ref= query parameter. A non-nil
// error indicates a transport-level failure (DNS, timeout, refused).
func (c *HTTPClient) get(ctx context.Context, path, ref string) (int, []byte, error) {
	req, err := c.newRequest(ctx, path, ref, "application/vnd.github+json")
	if err != nil {
		return 0, nil, err
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

// getRaw fetches a file's raw bytes via the contents API. A non-200 response is
// returned as an error so callers can surface it as a network problem.
func (c *HTTPClient) getRaw(ctx context.Context, path, ref string) ([]byte, error) {
	req, err := c.newRequest(ctx, path, ref, "application/vnd.github.raw")
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, path)
	}
	return io.ReadAll(resp.Body)
}

func (c *HTTPClient) newRequest(ctx context.Context, path, ref, accept string) (*http.Request, error) {
	u := c.BaseURL.ResolveReference(&url.URL{Path: path})
	if ref != "" {
		q := u.Query()
		q.Set("ref", ref)
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}
