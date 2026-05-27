# Overview

`osch` is a command-line tool for managing [OpenSpec](https://openspec.dev)
schemas across repositories. Schemas are not published to a central registry;
they live in the `schemas/` folder of ordinary GitHub repositories, and `osch`
reads them from there. See [ADR 0004](../adr/0004-decentralised-sources-github-first.md)
for why sources are decentralised and GitHub-first.

The tool is written in Go (see [ADR 0001](../adr/0001-language-go.md)),
distributed as a single static binary via GitHub Releases and a Homebrew tap
(see [ADR 0002](../adr/0002-distribution-homebrew-and-releases.md)).

## Top-level components

### CLI (`cmd/osch`)

The entry point and command dispatch. `run` parses `os.Args` and routes to a
handler. Commands available today:

- **(no args)** / `version` — prints the build version, commit, and date.
  `version --json` emits the same as JSON. The version values are stamped into
  the binary at release time by GoReleaser via `-ldflags`.
- **`add <user/repo>`** — validates the argument, then reports how many schema
  files the upstream repository exposes under `schemas/`.

All command handlers take their dependencies (e.g. the GitHub client) and an
output writer as arguments, so they can be driven directly from tests without
spawning a process.

### GitHub client (`internal/github`)

A small read-only client over the GitHub REST API. It resolves the `schemas/`
folder at a repository's default-branch HEAD and lists the files in it. Failures
are normalised into a single `ClientError` type with a friendly,
non-stacktrace message — distinguishing "repo not found", "no `schemas/`
folder", "empty `schemas/` folder", and transport-level network errors — so
every command surfaces upstream problems consistently.

### Agent loop (`scripts/agent-loop`)

A development-automation script, not part of the shipped binary. It polls
GitHub for issues labelled `agent:implement`, hands each to `claude -p` in an
isolated git worktree, and opens a pull request if the run produced commits.
See [agent-loop.md](agent-loop.md).

### Release pipeline

A tag push triggers a GitHub Actions workflow that runs GoReleaser, which builds
the binaries, publishes a GitHub Release, and opens a formula-update PR against
the Homebrew tap. See [release-flow.md](release-flow.md).
