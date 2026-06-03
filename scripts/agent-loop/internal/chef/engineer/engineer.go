package engineer

import (
	"context"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef"
)

// defaultRunID returns a lowercase ULID for use as the per-run branch nonce.
// Each call uses a fresh entropy source seeded from the current time, which
// is sufficient for the engineer's single-process, sequential use.
func defaultRunID() string {
	t := time.Now().UTC()
	entropy := rand.New(rand.NewSource(t.UnixNano()))
	return strings.ToLower(ulid.MustNew(ulid.Timestamp(t), entropy).String())
}

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
