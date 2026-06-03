package engineer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

// GitHubClient drives the gh CLI for PR creation and issue commenting.
type GitHubClient interface {
	CreatePR(ctx context.Context, workdir string, opts CreatePROpts) (PRInfo, error)
	CommentIssue(ctx context.Context, repoPath, issueID, body string) error
}

// CreatePROpts specifies the options for creating a pull request.
type CreatePROpts struct {
	Title   string
	Body    string
	BaseRef string
	HeadRef string
	Labels  []string
}

// PRInfo contains information about a created pull request.
type PRInfo struct {
	URL    string
	Number int
}

type realGH struct{ stderr io.Writer }

func (r realGH) CreatePR(ctx context.Context, workdir string, opts CreatePROpts) (PRInfo, error) {
	args := []string{
		"pr", "create",
		"--title", opts.Title,
		"--body", opts.Body,
		"--base", opts.BaseRef,
		"--head", opts.HeadRef,
	}
	for _, l := range opts.Labels {
		args = append(args, "--label", l)
	}
	// First create, then read back url+number via `gh pr view` against the
	// branch we just pushed. gh pr create's --json flag is not universally
	// supported across versions, but `gh pr view --json` is — and this also
	// gives a typed source of truth instead of scraping a URL.
	createCmd := exec.CommandContext(ctx, "gh", args...)
	createCmd.Dir = workdir
	var createOut bytes.Buffer
	createErr := newTailBuffer()
	createCmd.Stdout = &createOut
	createCmd.Stderr = io.MultiWriter(createErr, r.stderr)
	if err := createCmd.Run(); err != nil {
		return PRInfo{}, fmt.Errorf("gh pr create: %w: %s", err, createErr.String())
	}

	viewCmd := exec.CommandContext(ctx, "gh", "pr", "view", opts.HeadRef, "--json", "url,number")
	viewCmd.Dir = workdir
	var viewOut bytes.Buffer
	viewErr := newTailBuffer()
	viewCmd.Stdout = &viewOut
	viewCmd.Stderr = io.MultiWriter(viewErr, r.stderr)
	if err := viewCmd.Run(); err != nil {
		return PRInfo{}, fmt.Errorf("gh pr view: %w: %s", err, viewErr.String())
	}

	var parsed struct {
		URL    string `json:"url"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal(viewOut.Bytes(), &parsed); err != nil {
		return PRInfo{}, fmt.Errorf("parse gh pr view output: %w", err)
	}
	if parsed.Number == 0 || parsed.URL == "" {
		return PRInfo{}, fmt.Errorf("gh pr view returned empty url/number: %s", viewOut.String())
	}
	return PRInfo{URL: parsed.URL, Number: parsed.Number}, nil
}

func (r realGH) CommentIssue(ctx context.Context, repoPath, issueID, body string) error {
	cmd := exec.CommandContext(ctx, "gh", "issue", "comment", issueID, "--body", body)
	cmd.Dir = repoPath
	cmd.Stderr = r.stderr
	return cmd.Run()
}
