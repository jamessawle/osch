# Architecture

This directory describes the **current state** of `osch` — what exists today and
how the pieces fit together. It is descriptive, not aspirational.

Decisions, with their alternatives weighed and trade-offs recorded, live in
[`docs/adr/`](../adr/). When the two overlap, the ADRs explain *why* and these
documents explain *what is*.

## The rule

**If a description here goes stale, fix it as part of the change that made it
stale.** A pull request that changes how the system behaves is not complete
until the matching document here reflects the new behaviour. Treat these files
like code: review them, and don't let them drift.

## Contents

- [overview.md](overview.md) — what `osch` is and its top-level components.
- [package-layout.md](package-layout.md) — the directory tree and the purpose
  of each package.
- [agent-loop.md](agent-loop.md) — how the issue-to-PR automation loop works.
- [release-flow.md](release-flow.md) — how a tag becomes an installable release.
