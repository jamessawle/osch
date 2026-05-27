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

Two consumers share this repo with opposite needs, so the permission config is
split across two files:

- **`.claude/settings.json`** (repo root) — the **interactive-dev baseline**,
  committed. It pre-allows the common tools (`Read`, `Edit`, `Bash(git:*)`,
  `Bash(go:*)`, …) and carries **no** denies, so a maintainer's own
  interactive Claude Code session can open PRs, push branches, and do normal
  development. This is the layer that applies when you run `claude` yourself in
  this repo.
- **`scripts/agent-loop/settings.json`** — the **agent-only deny list**,
  committed next to the loop. It carries the sandbox restrictions and applies
  *only* to the loop's `claude -p` runs.

### Why two files

Claude Code merges settings across layers and **unions the `deny` lists** — a
deny in `.claude/settings.json` cannot be undone by a more local file. So if
the agent's denies lived in the committed baseline, they would also clamp down
the maintainer's interactive sessions (this is how `gh pr create` got blocked
when opening PR #46). Inverting the model fixes that: the committed baseline is
permissive, and the loop layers its stricter file on top at invocation time.

### How the loop loads it

`loop.sh` passes `--settings "$AGENT_SETTINGS"` (pointing at
`scripts/agent-loop/settings.json`) on every `claude -p` call, alongside
`--permission-mode dontAsk`. The `--settings` flag loads an *additional*
settings file that merges on top of the baseline; because deny rules are
unioned, the agent ends up with the baseline allows **plus** the agent denies,
while interactive sessions (no `--settings`) get the baseline alone.

> Mechanism reference: `claude --help` →
> `--settings <file-or-json>` ("Path to a settings JSON file … to load
> additional settings from"). Docs:
> <https://docs.anthropic.com/en/docs/claude-code/settings>.

The file is resolved relative to `loop.sh` itself (not the worktree under
test), so the sandbox policy is fixed by the loop rather than by whatever
branch an agent happens to check out. `loop.sh` also validates the file is
present and parses as JSON at startup and aborts if not — in `-p` mode an
invalid settings file is *silently ignored*, which would drop the denies
without warning.

### What the agent deny list currently restricts

The intent is to allow broadly inside the disposable worktree while denying
operations that escape it or duplicate the loop's own work:

- **Escaping the branch/sandbox:** `git push`, `git worktree`, `git
  reset --hard`, branch delete/rename, `git checkout/switch main`, `git commit
  --amend`, `git rebase`, `git remote`, `git config`.
- **Acting on GitHub:** `gh pr create | merge | close | review`, `gh issue`,
  `gh repo`, `gh api`, `gh auth` — the loop owns pushing and PR-opening; the
  agent only edits and commits.
- **Privilege / network:** `sudo`, `WebFetch`, `WebSearch`.

If a future task genuinely needs a capability that's currently blocked, extend
the allowlist (in the baseline, if both consumers should have it) by the
minimum required — and, if it's a risky operation, add a matching deny rule to
`scripts/agent-loop/settings.json` to keep the escape hatches closed.

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
  --settings scripts/agent-loop/settings.json \
  'Run `gh pr create --help` with the Bash tool.'
```

## Configuration

Environment variables (all optional):

- `POLL_INTERVAL` — seconds between polls when the queue is empty. Default: `60`.
