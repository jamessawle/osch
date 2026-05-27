# Agent Loop

**This is personal development automation, not part of `osch`.**

A long-running Bash poller that watches the GitHub repo for issues labelled
`agent:implement`, dispatches each one to a local `claude -p` invocation
in an isolated git worktree, and opens a PR if the run produces commits.

If this turns out to be useful beyond `osch`, it should be extracted to its
own repo. Until then, it lives here for proximity.

## Usage

```bash
./scripts/agent-loop/loop.sh
```

Press Ctrl-C to stop. Logs go to stdout — pipe to `tee` if you want a file copy.

## Requirements

- `gh` (GitHub CLI), authenticated against `github.com/jamessawle/osch`
- `git` ≥ 2.5 (for worktree support)
- `claude` (Claude Code CLI), logged in
- `jq`

The poller checks for all of these at startup and exits with an error if any are missing.

## Labels

State machine lives in GitHub labels:

- `agent:implement` — queued; the next poll cycle will claim it
- `agent:in-progress` — poller has claimed it; do not touch
- `agent:failed` — last run failed; comment on the issue contains the tail of `claude` output. Re-label as `agent:implement` to retry.
- `agent:authored` — applied to PRs opened by the poller

## Behaviour

- Sequential: one issue per loop iteration. No concurrency.
- No retries: a failed run flips to `agent:failed` and the poller moves on.
- Worktrees live at `../osch-agent-<N>` and are NOT auto-cleaned on PR merge.
  Clean them manually with `git worktree remove ../osch-agent-<N>` once the PR lands.
- Claude does the editing and committing. The poller does the pushing and PR-opening.

## Permissions

The agent runs inside a disposable worktree, so the goal is to allow broadly
within that sandbox while denying operations that escape it — pushing
branches, opening or closing PRs, mutating issue state, touching remotes,
reaching out to the network, or duplicating work the loop already does. The
concrete allow/deny lists live in `.claude/settings.json` under the
`permissions` key, paired with `--permission-mode dontAsk` on each
`claude -p` invocation (without that flag the settings rules are inert in
headless mode).

If a future task genuinely needs a capability that's currently blocked,
extend the allowlist by the minimum required — and, if it's a risky
operation, add a matching deny rule to keep the escape hatches closed.

## Configuration

Environment variables (all optional):

- `POLL_INTERVAL` — seconds between polls when the queue is empty. Default: `60`.
