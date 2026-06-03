package engineer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	defaultBaseRef       = "main"
	defaultRemote        = "origin"
	defaultBaseBranch    = "main"
	maxImplementAttempts = 3
)

// ImplementInput is the typed handler input. The wire layer translates
// a Chit into this struct.
type ImplementInput struct {
	TaskRef       TaskRef
	Title         string
	Description   string
	Specification string
	RepoPath      string
}

// TaskRef identifies an external task (source + id).
type TaskRef struct {
	Source string
	ID     string
}

// ImplementResult is the success outcome of a successful implement Chit.
type ImplementResult struct {
	PR PRInfo
}

// TitleValidator returns nil if the title satisfies the project's
// Conventional Commits policy. The default impl shells out to `go tool
// conform`; tests inject a stub so they don't spawn processes.
type TitleValidator func(ctx context.Context, repoPath, title string) error

// Deps bundles the external collaborators. Production wires real impls;
// tests inject fakes. NewRunID returns a per-run nonce used to stamp the
// agent branch name so successive attempts against the same issue never
// collide on a stale remote ref. If nil, a default ULID generator is used.
type Deps struct {
	Claude        ClaudeRunner
	Shell         ShellRunner
	Git           GitRunner
	GH            GitHubClient
	NewRunID      func() string
	ValidateTitle TitleValidator
	Stderr        io.Writer // optional; used for non-fatal warnings (post-failure errors, etc.)
}

// RunImplement executes the implement Chit end-to-end. Returns a result
// on success, or an error on Chit failure. The caller (engineer.Run)
// translates the (result, error) pair into a Proof.
func RunImplement(ctx context.Context, in ImplementInput, deps Deps) (ImplementResult, error) {
	if in.TaskRef.Source != "github" {
		return ImplementResult{}, fmt.Errorf("unsupported task ref source %q", in.TaskRef.Source)
	}

	newRunID := deps.NewRunID
	if newRunID == nil {
		newRunID = defaultRunID
	}
	validate := deps.ValidateTitle
	if validate == nil {
		validate = defaultTitleValidator
	}
	layout := DeriveWorktreeLayout(in.RepoPath, in.TaskRef.ID, newRunID())

	if err := boardSetup(ctx, deps.Git, in.RepoPath, layout); err != nil {
		postFailure(ctx, deps, in, "board setup: "+err.Error(), "")
		return ImplementResult{}, err
	}

	// On success, drop the worktree; on failure, leave it for debugging.
	// The deferred-bool pattern keeps both paths in one place and avoids
	// littering each early-return with cleanup.
	succeeded := false
	defer func() {
		if succeeded {
			_ = deps.Git.WorktreeRemove(ctx, in.RepoPath, layout.WorktreePath)
		}
	}()

	cfg, err := LoadConfig(layout.WorktreePath)
	if err != nil {
		postFailure(ctx, deps, in, "config: "+err.Error(), "")
		return ImplementResult{}, err
	}

	if err := runSetupCommands(ctx, deps.Shell, layout.WorktreePath, cfg.Setup); err != nil {
		postFailure(ctx, deps, in, "setup: "+err.Error(), lastTail(err))
		return ImplementResult{}, err
	}

	if err := implementLoop(ctx, deps, in, layout, cfg); err != nil {
		postFailure(ctx, deps, in, "implement: "+err.Error(), lastTail(err))
		return ImplementResult{}, err
	}

	body := generatePRBody(ctx, deps.Claude, in, layout)
	title := generatePRTitle(ctx, deps.Claude, in, layout, validate)

	if err := deps.Git.Push(ctx, layout.WorktreePath, defaultRemote, layout.BranchName); err != nil {
		postFailure(ctx, deps, in, "push: "+err.Error(), "")
		return ImplementResult{}, err
	}

	pr, err := deps.GH.CreatePR(ctx, layout.WorktreePath, CreatePROpts{
		Title:   title,
		Body:    appendCloses(body, in.TaskRef.ID),
		BaseRef: defaultBaseBranch,
		HeadRef: layout.BranchName,
		Labels:  []string{"agent:authored"},
	})
	if err != nil {
		postFailure(ctx, deps, in, "pr create: "+err.Error(), "")
		return ImplementResult{}, err
	}

	succeeded = true
	return ImplementResult{PR: pr}, nil
}

// closesRefRE matches GitHub's issue-closing keywords (close|closes|closed|
// fix|fixes|fixed|resolve|resolves|resolved) followed by a #<id> reference.
// Case-insensitive. Used to detect whether the LLM-generated body already
// closes the issue we'd otherwise append for.
var closesRefRE = regexp.MustCompile(`(?i)\b(close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#(\d+)\b`)

func appendCloses(body, issueID string) string {
	for _, m := range closesRefRE.FindAllStringSubmatch(body, -1) {
		if len(m) >= 3 && m[2] == issueID {
			return body
		}
	}
	return body + "\n\nCloses #" + issueID
}

func boardSetup(ctx context.Context, git GitRunner, repoPath string, layout WorktreeLayout) error {
	// Always prune first — git can hold a stale worktree entry pointing at a
	// directory that was removed externally; without prune the subsequent
	// `worktree add` will fail with "already registered".
	if err := git.Prune(ctx, repoPath); err != nil {
		return fmt.Errorf("worktree prune: %w", err)
	}
	if _, err := os.Stat(layout.WorktreePath); err == nil {
		if err := git.WorktreeRemove(ctx, repoPath, layout.WorktreePath); err != nil {
			// Remove may fail if the directory exists but isn't registered as a
			// worktree (e.g. a previous run partially failed). Fall back to a
			// plain filesystem remove so the subsequent add can succeed.
			if rmErr := os.RemoveAll(layout.WorktreePath); rmErr != nil {
				return fmt.Errorf("remove stale worktree: %w (git remove: %v)", rmErr, err)
			}
		}
	}
	if err := git.Fetch(ctx, repoPath, defaultRemote, defaultBaseRef); err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	if err := git.WorktreeAdd(ctx, repoPath, layout.WorktreePath, layout.BranchName, defaultRemote+"/"+defaultBaseRef); err != nil {
		return fmt.Errorf("worktree add: %w", err)
	}
	return nil
}

func runSetupCommands(ctx context.Context, shell ShellRunner, workdir string, cmds []string) error {
	for _, cmd := range cmds {
		out, err := shell.Run(ctx, workdir, cmd)
		if err != nil {
			return tailErr(fmt.Errorf("%s: %w", cmd, err), out)
		}
	}
	return nil
}

func implementLoop(ctx context.Context, deps Deps, in ImplementInput, layout WorktreeLayout, cfg Config) error {
	var failingChecks string
	for attempt := 1; attempt <= maxImplementAttempts; attempt++ {
		prompt := buildImplementPrompt(in, cfg.Checks, failingChecks)
		out, err := deps.Claude.Run(ctx, layout.WorktreePath, prompt)
		if err != nil {
			return tailErr(fmt.Errorf("claude attempt %d: %w", attempt, err), out)
		}

		failures := runChecks(ctx, deps.Shell, layout.WorktreePath, cfg.Checks)
		if len(failures) == 0 {
			break
		}
		failingChecks = formatFailures(failures)
		if attempt == maxImplementAttempts {
			return tailErr(errors.New("checks failing after max attempts"), failingChecks)
		}
	}

	commits, err := deps.Git.CommitCount(ctx, layout.WorktreePath, defaultRemote+"/"+defaultBaseRef)
	if err != nil {
		return fmt.Errorf("commit count: %w", err)
	}
	if commits == 0 {
		return errors.New("implement loop produced zero new commits")
	}
	return nil
}

type checkFailure struct {
	Command string
	Output  string
}

func runChecks(ctx context.Context, shell ShellRunner, workdir string, checks []string) []checkFailure {
	var failures []checkFailure
	for _, cmd := range checks {
		out, err := shell.Run(ctx, workdir, cmd)
		if err != nil {
			failures = append(failures, checkFailure{Command: cmd, Output: out})
		}
	}
	return failures
}

func formatFailures(fs []checkFailure) string {
	var b strings.Builder
	for _, f := range fs {
		b.WriteString("--- FAILED: ")
		b.WriteString(f.Command)
		b.WriteString(" ---\n")
		b.WriteString(f.Output)
		b.WriteString("\n")
	}
	return b.String()
}

func buildImplementPrompt(in ImplementInput, checks []string, failingChecks string) string {
	var b strings.Builder
	b.WriteString("You are implementing GitHub issue #")
	b.WriteString(in.TaskRef.ID)
	b.WriteString(".\n\nIssue title: ")
	b.WriteString(in.Title)
	b.WriteString("\n\nIssue body:\n")
	b.WriteString(in.Description)
	b.WriteString("\n")
	if in.Specification != "" {
		b.WriteString("\nAgent brief (authoritative specification):\n")
		b.WriteString(in.Specification)
		b.WriteString("\n\nThe agent brief is the exclusive source of truth. Where it disagrees with the issue body on any specific value, the brief wins.\n")
	}
	b.WriteString("\nInstructions:\n")
	b.WriteString("- Read CLAUDE.md in the repo root for project conventions.\n")
	b.WriteString("- Make the edits the issue requires.\n")
	b.WriteString("- Stage and commit your changes with a Conventional Commits message (e.g. 'feat:', 'fix:', 'chore:', 'docs:'). Reference the issue with 'Refs #")
	b.WriteString(in.TaskRef.ID)
	b.WriteString("' in the commit body.\n")
	b.WriteString("- DO NOT push the branch and DO NOT open a pull request — the calling process will handle that.\n")
	b.WriteString("- Before your final commit, walk the brief's `Acceptance criteria` list AC-by-AC. For each item, identify the specific line of code or test that satisfies it.\n\n")
	b.WriteString("After committing, verify your work by running these checks (they must all pass):\n")
	for _, c := range checks {
		b.WriteString("  $ ")
		b.WriteString(c)
		b.WriteString("\n")
	}
	if failingChecks != "" {
		b.WriteString("\nPrior attempt left the following checks failing. Fix them and commit the fix:\n")
		b.WriteString(failingChecks)
	}
	return b.String()
}

func generatePRBody(ctx context.Context, claude ClaudeRunner, in ImplementInput, layout WorktreeLayout) string {
	prompt := "Use the /pr-management:write-pr-description skill to produce a pull request description " +
		"for the commits on the current branch (compared to origin/main). The PR implements GitHub issue #" +
		in.TaskRef.ID + ": \"" + in.Title + "\".\n\n" +
		"Output ONLY the PR description markdown to stdout, with no preamble, no commentary, no trailing text, " +
		"and DO NOT wrap the output in code fences. The description should cover what changed and why " +
		"(not a diff restatement), the approach chosen, and any trade-offs or follow-ups worth noting."
	out, err := claude.Run(ctx, layout.WorktreePath, prompt)
	if err != nil || strings.TrimSpace(out) == "" {
		return "Implements #" + in.TaskRef.ID + "."
	}
	return stripCodeFence(strings.TrimSpace(out))
}

// stripCodeFence removes a single outer ```lang ... ``` fence if one wraps
// the entire string. It does not parse inner fences (a code example inside
// the body is left untouched).
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// drop the opening fence line
	nl := strings.IndexByte(s, '\n')
	if nl < 0 {
		return s
	}
	inner := s[nl+1:]
	inner = strings.TrimRight(inner, "\n")
	if !strings.HasSuffix(inner, "```") {
		return s
	}
	return strings.TrimRight(inner[:len(inner)-3], "\n")
}

func generatePRTitle(ctx context.Context, claude ClaudeRunner, in ImplementInput, layout WorktreeLayout, validate TitleValidator) string {
	// Prefer the issue title verbatim if the maintainer already wrote a valid
	// Conventional Commits title — no point asking claude to re-derive what's
	// already correct, and it avoids a double-prefix like "chore: docs(...)".
	if validate(ctx, in.RepoPath, in.Title) == nil {
		return in.Title
	}
	prompt := "Generate a single Conventional Commits PR title for the commits on the current branch " +
		"(compared to origin/main). The PR implements GitHub issue #" + in.TaskRef.ID + ": \"" + in.Title + "\".\n\n" +
		"Requirements:\n" +
		"- Format: type(scope)?: subject  (scope optional)\n" +
		"- Subject in lowercase, no trailing period, no issue number suffix.\n" +
		"- Hard cap at 72 characters.\n\n" +
		"Output ONLY the title text, a single line, with no preamble or commentary."
	out, err := claude.Run(ctx, layout.WorktreePath, prompt)
	generated := strings.TrimSpace(out)
	if err == nil && generated != "" && validate(ctx, in.RepoPath, generated) == nil {
		return generated
	}
	// Both the issue title and the generated title were invalid; the issue
	// title was already validated above, so go straight to the fallback.
	return "chore: " + in.Title
}

func postFailure(ctx context.Context, deps Deps, in ImplementInput, msg, tail string) {
	body := "Agent loop failed: " + msg
	if tail != "" {
		body += "\n\n```\n" + tail + "\n```"
	}
	if err := deps.GH.CommentIssue(ctx, in.RepoPath, in.TaskRef.ID, body); err != nil && deps.Stderr != nil {
		// Failure to post the comment is itself a real signal worth surfacing
		// — the operator otherwise sees only the label without explanation.
		_, _ = fmt.Fprintf(deps.Stderr, "postFailure: comment failed for issue #%s: %v\n", in.TaskRef.ID, err)
	}
}

type tailErrWrap struct {
	err  error
	tail string
}

func (t tailErrWrap) Error() string { return t.err.Error() }
func (t tailErrWrap) Unwrap() error { return t.err }

func tailErr(err error, tail string) error { return tailErrWrap{err: err, tail: tail} }
func lastTail(err error) string {
	var t tailErrWrap
	if errors.As(err, &t) {
		return t.tail
	}
	return ""
}
