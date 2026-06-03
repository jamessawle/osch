package engineer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
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
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = workdir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = io.MultiWriter(&buf, r.stderr)
	if err := cmd.Run(); err != nil {
		return PRInfo{}, fmt.Errorf("gh pr create: %w: %s", err, buf.String())
	}
	url := strings.TrimSpace(buf.String())

	lines := strings.Split(url, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "https://") {
			url = lines[i]
			break
		}
	}

	parts := strings.Split(url, "/")
	var number int
	if len(parts) > 0 {
		_ = json.Unmarshal([]byte(parts[len(parts)-1]), &number)
	}
	return PRInfo{URL: url, Number: number}, nil
}

func (r realGH) CommentIssue(ctx context.Context, repoPath, issueID, body string) error {
	cmd := exec.CommandContext(ctx, "gh", "issue", "comment", issueID, "--body", body)
	cmd.Dir = repoPath
	cmd.Stderr = r.stderr
	return cmd.Run()
}
