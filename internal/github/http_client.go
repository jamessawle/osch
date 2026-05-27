package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://api.github.com"

// HTTPClient talks to the GitHub REST API. The zero value is not usable; build
// one with NewHTTPClient.
type HTTPClient struct {
	BaseURL *url.URL
	HTTP    *http.Client
}

// NewHTTPClient returns an HTTPClient pointed at the public GitHub API with a
// sensible request timeout.
func NewHTTPClient() *HTTPClient {
	base, _ := url.Parse(defaultBaseURL)
	return &HTTPClient{
		BaseURL: base,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// contentEntry is one item in a GitHub "contents" directory listing.
type contentEntry struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// ListSchemas returns the names of files under schemas/ at the repo's default
// branch HEAD. All failures are reported as *ClientError with a friendly
// message. It first confirms the repo is reachable so a 404 on the contents
// endpoint can be reported as "no schemas/ folder" rather than "not found".
func (c *HTTPClient) ListSchemas(ctx context.Context, repo Repo) ([]string, error) {
	repoStatus, _, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s", repo.Owner, repo.Name))
	if err != nil {
		return nil, NetworkError(repo, err)
	}
	if repoStatus == http.StatusNotFound {
		return nil, NotFoundError(repo)
	}
	if repoStatus != http.StatusOK {
		return nil, NetworkError(repo, fmt.Errorf("unexpected status %d from GitHub", repoStatus))
	}

	status, body, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/contents/schemas", repo.Owner, repo.Name))
	if err != nil {
		return nil, NetworkError(repo, err)
	}
	if status == http.StatusNotFound {
		return nil, NoSchemasError(repo)
	}
	if status != http.StatusOK {
		return nil, NetworkError(repo, fmt.Errorf("unexpected status %d from GitHub", status))
	}

	var entries []contentEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, NetworkError(repo, fmt.Errorf("could not parse GitHub response: %w", err))
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Type == "file" {
			names = append(names, e.Name)
		}
	}
	if len(names) == 0 {
		return nil, EmptySchemasError(repo)
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
