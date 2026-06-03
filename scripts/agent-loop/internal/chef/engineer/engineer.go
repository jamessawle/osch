package engineer

import (
	"context"
	"errors"
	"io"

	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef"
)

// Run is the engineer Chef entry point. It dispatches on chit.Kind and
// returns a Proof reflecting the outcome. A non-nil error returned from
// Run signals a Chef-internal crash (not a Chit failure); the caller
// should exit non-zero in that case.
func Run(ctx context.Context, c chef.Chit, deps Deps) (chef.Proof, error) {
	switch c.Kind {
	case "implement":
		return runImplementAsProof(ctx, c, deps)
	default:
		return chef.Proof{
			Kind:    c.Kind,
			Status:  chef.StatusFailed,
			Message: "unsupported kind: " + c.Kind,
		}, nil
	}
}

func runImplementAsProof(ctx context.Context, c chef.Chit, deps Deps) (chef.Proof, error) {
	in := ImplementInput{
		TaskRef:       TaskRef{Source: c.Task.Ref.Source, ID: c.Task.Ref.ID},
		Title:         c.Task.Title,
		Description:   c.Task.Description,
		Specification: c.Task.Specification,
		RepoPath:      c.Repo.Path,
	}
	res, err := RunImplement(ctx, in, deps)
	if err != nil {
		return chef.Proof{
			Kind:       "implement",
			Status:     chef.StatusFailed,
			Message:    err.Error(),
			OutputTail: lastTail(err),
		}, nil
	}
	return chef.Proof{
		Kind:   "implement",
		Status: chef.StatusOK,
		PR:     &chef.ProofPR{URL: res.PR.URL, Number: res.PR.Number},
	}, nil
}

// ProductionDeps wires the real ClaudeRunner / ShellRunner / GitRunner
// / GitHubClient. stderr is forwarded to subprocess stderr for live logs.
func ProductionDeps(stderr io.Writer) Deps {
	return Deps{
		Claude: realClaude{stderr: stderr},
		Shell:  realShell{stderr: stderr},
		Git:    realGit{stderr: stderr},
		GH:     realGH{stderr: stderr},
	}
}

type realClaude struct{ stderr io.Writer }
type realShell struct{ stderr io.Writer }
type realGit struct{ stderr io.Writer }
type realGH struct{ stderr io.Writer }

func (r realClaude) Run(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("realClaude not implemented yet")
}
func (r realShell) Run(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("realShell not implemented yet")
}
func (r realGit) Fetch(_ context.Context, _, _, _ string) error {
	return errors.New("realGit not implemented yet")
}
func (r realGit) WorktreeAdd(_ context.Context, _, _, _, _ string) error {
	return errors.New("realGit not implemented yet")
}
func (r realGit) WorktreeRemove(_ context.Context, _, _ string) error {
	return errors.New("realGit not implemented yet")
}
func (r realGit) Push(_ context.Context, _, _, _ string) error {
	return errors.New("realGit not implemented yet")
}
func (r realGit) CommitCount(_ context.Context, _, _ string) (int, error) {
	return 0, errors.New("realGit not implemented yet")
}
func (r realGH) CreatePR(_ context.Context, _ string, _ CreatePROpts) (PRInfo, error) {
	return PRInfo{}, errors.New("realGH not implemented yet")
}
func (r realGH) CommentIssue(_ context.Context, _, _, _ string) error {
	return errors.New("realGH not implemented yet")
}
