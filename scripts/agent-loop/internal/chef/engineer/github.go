package engineer

import "context"

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
