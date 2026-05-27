# Agent loop

`scripts/agent-loop/loop.sh` is a personal development-automation script. It is
**not** part of the shipped `osch` binary. It watches GitHub for issues the
maintainer has labelled ready for automated implementation, drives Claude Code
to do the work in an isolated git worktree, and opens a pull request when the
run produces commits.

See [`scripts/agent-loop/README.md`](../../scripts/agent-loop/README.md) for the
operational details (labels, permissions, configuration).

## Labels

The loop moves an issue through a small label state machine:

- `agent:implement` — queued; the loop claims these.
- `agent:in-progress` — claimed and being worked on.
- `agent:failed` — the run errored or produced no commits.
- `agent:authored` — applied to the opened PR.

## Sequence

```mermaid
sequenceDiagram
    autonumber
    participant Loop as loop.sh (poller)
    participant GH as GitHub
    participant WT as git worktree
    participant Claude as claude -p
    participant PR as Pull request

    loop every POLL_INTERVAL seconds
        Loop->>GH: list open issues labelled agent:implement
        alt no queued issue
            Note over Loop: sleep, then poll again
        else issue found
            Loop->>GH: relabel agent:implement → agent:in-progress (claim)
            Loop->>GH: fetch origin/main
            Loop->>WT: worktree add (branch agent/issue-N from origin/main)
            Loop->>Claude: run prompt in worktree (sandboxed --settings)
            Claude->>WT: edit files, run checks, commit
            Claude-->>Loop: exit code + captured output

            alt non-zero exit OR zero new commits
                Loop->>GH: relabel → agent:failed, comment with output tail
                Loop->>WT: worktree remove --force
            else commits produced
                Loop->>GH: push branch agent/issue-N
                Loop->>Claude: generate PR body via write-pr-description skill
                Loop->>PR: gh pr create (label agent:authored, "Closes #N")
                Loop->>GH: remove agent:in-progress from issue
                Note over WT: worktree retained for manual cleanup after merge
            end
        end
    end
```

## Notes

- **Isolation.** Each issue runs in its own worktree (`<repo>-agent-<N>` beside
  the repo root) on a dedicated `agent/issue-<N>` branch cut from `origin/main`,
  so concurrent or repeated runs don't collide.
- **Sandboxing.** Every `claude -p` call runs with `--permission-mode dontAsk`
  and an agent-only `--settings` file (`scripts/agent-loop/settings.json`) that
  layers deny rules on top of the interactive baseline. The settings file lives
  next to the loop, not in the worktree, so the agent can't widen its own
  sandbox by editing a checked-out file.
- **Failure is loud but non-fatal.** A failed run relabels the issue and
  comments the last 50 lines of output; the loop continues to the next issue
  rather than exiting.
