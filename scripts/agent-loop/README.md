# Agent Loop

**This is personal development automation, not part of `osch`.**

A long-running Bash poller that watches the GitHub repo for issues labelled
`ready-for-agent`, builds a JSON **Chit** describing the task, and dispatches
it to a Go binary that runs the engineer **Chef**. The Chef does the actual
work — creating a worktree, driving `claude -p` through an
implement→validate→retry loop, and opening a PR — then returns a **Proof**
that the poller uses to apply outcome labels.

If this turns out to be useful beyond `osch`, it should be extracted to its
own repo. Until then, it lives here for proximity.

## Architecture

The loop is split into two parts:

- **`scripts/agent-loop/loop.sh`** — a thin (~150-line) orchestrator. Polls
  GitHub, claims an issue, fetches the agent brief, builds a Chit, dispatches
  to `brigade chef engineer` (`go run ./scripts/agent-loop/cmd/brigade`),
  parses the Proof, applies success/failure labels. It does not touch
  worktrees or invoke `claude` directly.
- **`scripts/agent-loop/cmd/brigade`** — a Go binary exposing a
  `chef <chef-name>` subcommand. Today only `engineer` is implemented. The
  engineer Chef owns: worktree creation/destruction, running setup commands
  from `.brigade.yml`, the 3-attempt implement→validate retry loop driven by
  `claude -p`, PR body/title generation, PR creation, and posting failure
  comments.

## Usage

```bash
./scripts/agent-loop/loop.sh
```

Press Ctrl-C to stop. Logs go to stdout — pipe to `tee` if you want a file copy.

The orchestrator runs the brigade binary via `go run ./scripts/agent-loop/cmd/brigade`,
so as long as Go is on `PATH` no separate build step is required.

## Requirements

- `gh` (GitHub CLI), authenticated against `github.com/jamessawle/osch`
- `git` ≥ 2.5 (for worktree support)
- `claude` (Claude Code CLI), logged in — invoked by the engineer Chef, not the orchestrator
- `jq`
- Go 1.25+ — to build/run `brigade`

The poller checks for these at startup and exits with an error if any are missing.

## Labels

The loop's entry gate is the `ready-for-agent` triage role — issues reach it
via `/triage` (grilled, agent-brief written), not by hand. From there the loop
manages its own runtime state in `agent:*` labels:

- `ready-for-agent` — queued; the next poll cycle will claim it (set by triage)
- `agent:in-progress` — poller has claimed it; do not touch
- `agent:failed` — last run failed; comment on the issue contains the tail of `claude` output. Re-label as `ready-for-agent` to retry.
- `agent:authored` — applied to PRs opened by the poller

## Behaviour

- Sequential: one issue per loop iteration. No concurrency.
- Spec source: the engineer Chef is prompted with the issue body **plus** the
  `## Agent Brief` comment that `/triage` posts (the authoritative contract;
  the body is context). Issues with no brief comment fall back to the body alone.
- Implement → validate → retry: each implement Chit gets up to **3 attempts**.
  After each attempt the engineer runs the `checks` from `.brigade.yml`; if any
  fail, their output is fed back into the next attempt's prompt. After 3 failed
  attempts the run fails.
- Failure semantics: a non-zero exit from the brigade binary means the Chef
  itself crashed (loop labels `agent:failed`). Exit 0 with a Proof of
  `status: "failed"` means the Chit failed cleanly — the Chef has already
  posted a failure comment, and the loop just applies the label.
- Worktrees live at `../osch-agent-issue-<N>` on branch `agent/issue-<N>`,
  created and torn down by the engineer Chef. They are **not** auto-cleaned on
  PR merge; remove them with `git worktree remove ../osch-agent-issue-<N>` once
  the PR lands.

## Chit / Proof contract

The orchestrator and Chef communicate over JSON: the orchestrator writes a
Chit to the binary's stdin; the binary writes a Proof to stdout and forwards
live logs to stderr.

**Chit** (input):

```json
{
  "kind": "implement",
  "task": {
    "ref": {"source": "github", "id": "42"},
    "title": "Add --force flag to update command",
    "description": "<issue body>",
    "specification": "<agent brief comment>"
  },
  "repo": {"path": "/abs/path/to/repo"}
}
```

**Proof — success:**

```json
{"kind": "implement", "status": "ok", "pr": {"url": "https://...", "number": 99}}
```

**Proof — failure:**

```json
{
  "kind": "implement",
  "status": "failed",
  "message": "checks failed after 3 attempts",
  "output_tail": "..."
}
```

## `.brigade.yml`

The Chef reads `.brigade.yml` from the repo root. Current contents:

```yaml
setup:
  - go mod download

checks:
  - go build ./...
  - go vet ./...
  - gofmt -l . | (! grep .)
  - go test ./...
```

- `setup` runs **once** at the start of an implement Chit, after the worktree
  is created.
- `checks` run at the end of **each** implement attempt. If any fail, their
  output is fed back into the next attempt's prompt (up to 3 attempts total).

## Permissions

Two consumers share this repo with opposite needs, so the permission config is
split:

- **`.claude/settings.json`** (repo root) — the **interactive-dev baseline**,
  committed. Pre-allows the common tools (`Read`, `Edit`, `Bash(git:*)`,
  `Bash(go:*)`, …) with **no** denies, so a maintainer's own interactive
  Claude Code session can open PRs, push branches, and develop normally.
- **Agent deny layer** — the sandbox restrictions that apply only to the
  Chef's `claude -p` runs. This file is **embedded into the brigade binary**
  at compile time (via `go:embed`) from
  `scripts/agent-loop/internal/chef/engineer/settings.json`. On each `claude -p`
  invocation the engineer writes it to a temporary `.brigade-settings.json`
  inside the worktree, passes `--settings .brigade-settings.json` to claude,
  and removes the file when the call returns.

### Why two files

Claude Code merges settings across layers and **unions the `deny` lists** — a
deny in `.claude/settings.json` cannot be undone by a more local file. If the
agent's denies lived in the committed baseline, they would also clamp down the
maintainer's interactive sessions. Keeping the baseline permissive and layering
the agent denies on top at invocation time fixes that: deny rules are unioned,
so the agent ends up with the baseline allows **plus** the agent denies, while
interactive sessions (no `--settings`) get the baseline alone.

> Mechanism reference: `claude --help` →
> `--settings <file-or-json>` ("Path to a settings JSON file … to load
> additional settings from"). Docs:
> <https://docs.anthropic.com/en/docs/claude-code/settings>.

### What the agent deny list currently restricts

The intent is to allow broadly inside the disposable worktree while denying
operations that escape it or duplicate the Chef's own work:

- **Escaping the branch/sandbox:** `git push`, `git worktree`,
  `git reset --hard`, branch delete/rename, `git checkout/switch main`,
  `git commit --amend`, `git rebase`, `git remote`, `git config`.
- **Acting on GitHub:** `gh pr create | merge | close | review`, `gh issue`,
  `gh repo`, `gh api`, `gh auth` — the Chef owns pushing and PR-opening; the
  inner `claude -p` only edits and commits.
- **Privilege / network:** `sudo`, `WebFetch`, `WebSearch`.

If a future task genuinely needs a capability that's currently blocked, extend
the allowlist (in the baseline, if both consumers should have it) by the
minimum required — and, if it's a risky operation, add a matching deny to the
embedded `scripts/agent-loop/internal/chef/engineer/settings.json` to keep the
escape hatches closed.

> **Surfaced, not acted on (per issue #47):** `sudo`, `WebFetch`, and
> `WebSearch` are denied for the agent but arguably *should* also apply to
> interactive sessions. They were left out of the baseline to avoid restricting
> interactive dev as part of this refactor. Revisit if a shared safety baseline
> is wanted.

### Smoke test

To confirm the split, from this repo (not inside an agent worktree):

```bash
# Interactive baseline: should prompt / be allowed, not hard-denied.
claude -p --permission-mode default 'Run `gh pr create --help` with the Bash tool.'

# Agent layer: same call must be denied.
claude -p --permission-mode dontAsk \
  --settings scripts/agent-loop/internal/chef/engineer/settings.json \
  'Run `gh pr create --help` with the Bash tool.'
```

## Configuration

Environment variables (all optional):

- `POLL_INTERVAL` — seconds between polls when the queue is empty. Default: `60`.
