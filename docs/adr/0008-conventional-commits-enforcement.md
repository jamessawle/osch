# 0008. Enforce Conventional Commits via PR-title lint and local commit-msg hook

- Status: Accepted
- Date: 2026-05-28

## Context

`CONTRIBUTING.md` says the project follows [Conventional Commits](https://www.conventionalcommits.org/), but nothing in the repo enforces this. Recent merges to `main` (e.g. `Bump goreleaser-action v6 -> v7 for Node 24 runtime`, `Pin dev tools in go.mod and run them via go tool`) do not carry a `feat:` / `fix:` / `chore:` prefix.

The repo uses squash-merging, so the PR title becomes the commit message on `main`. Enforcement has to land at the PR title to actually protect `main`'s history. PR titles, however, are often pre-filled from the branch's commit messages, so individual commits also need to follow the convention for the inferred title to be valid.

## Decision

- **Allowed types:** `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `build`, `perf`. This is the standard Conventional Commits set; `build` and `perf` are added on top of the seven types `CONTRIBUTING.md` already listed because they cover changes (build system, performance work) that don't fit cleanly under `chore` or `refactor`.
- **Scopes are optional.** A scope can be added in parentheses (`feat(add): …`) when it usefully narrows the change, but it is never required. The project is small enough that mandatory scopes would add friction without improving history.
- **Header length: 72 characters max.** Same limit as the canonical Git commit subject convention; long enough for a real summary, short enough to render in `git log --oneline` and PR lists without truncation.
- **Header-only enforcement.** Body and footer rules (e.g. `Refs #N`) stay as written guidance in `CONTRIBUTING.md`. The automated check covers only the header, because that is what becomes the squash-merged commit subject.
- **Two enforcement points, no history cleanup.**
  - **PR title (CI).** A GitHub Action runs on `pull_request` events and fails the check if the title doesn't match. This is the layer that guarantees `main`'s history is conventional.
  - **`commit-msg` hook (local).** A `lefthook` `commit-msg` hook rejects local commits whose subject doesn't match, so the branch contains conventional commits and PR titles can be inferred cleanly from them.
  - Existing non-conforming history on `main` is left as-is. Squash-merging has already collapsed it; rewriting history would invalidate every contributor's local clone for no real gain.

## Consequences

- Contributors discover format errors at `git commit` time rather than after pushing and opening a PR. The CI check remains as a backstop for commits made outside the local hook path (web UI edits, force-pushes from a clone without hooks installed).
- The set of allowed types is now load-bearing: it lives in three places (`CONTRIBUTING.md`, the lefthook script, and the workflow inputs) and they must agree. `CONTRIBUTING.md` is the source of truth; the other two reference the same list explicitly so a single review of all three catches drift.
- The local hook is implemented as a small shell script rather than via `commitlint`. `commitlint` is the more common Conventional Commits tool, but it pulls in a Node toolchain that the project doesn't otherwise use; a ~20-line regex check covers the header-only rules we care about. If we ever need body/footer rules or per-type scope policies, revisit this and adopt `commitlint`.
- The CI check uses `amannn/action-semantic-pull-request`. It is the de-facto standard for this job, configurable via inputs (types list, subject pattern), and does not require any in-repo Node setup.

## Alternatives Considered

- **`commitlint` for the local hook.** Rejected for now — see above. The decision is reversible; the shell script is self-contained and easy to delete in favour of `commitlint` if scope grows.
- **Local hook only, no CI check.** Rejected. Local hooks can be skipped (`--no-verify`) and aren't installed on first clone until `make setup` runs. Without CI, a single un-hooked clone can land a non-conforming title on `main`.
- **CI check only, no local hook.** Rejected. The PR title is often pre-filled from the branch's first commit; if commits don't follow the convention, contributors hit a red CI on every PR and have to rename the title by hand. Catching it at commit time is cheaper.
- **Rewrite existing history to be conventional.** Rejected. Squash-merging already collapsed the per-PR history; rewriting `main` invalidates every clone and provides no forward-looking benefit.
