# 0003. Release tooling: GoReleaser

- Status: Accepted
- Date: 2026-05-27

## Context

The distribution decision (0002) commits the project to publishing tagged releases as GitHub release tarballs for several OS/architecture combinations and to opening a pull request against a Homebrew tap with an updated formula on each release. That is a multi-step pipeline — cross-compilation, archive packaging, checksum generation, changelog assembly, formula rendering, tap PR creation — and the project needs a single tool to own it.

For a Go project distributed through Homebrew, this combination of responsibilities has a well-established solution in the Go ecosystem, and reaching for it brings strong network effects: existing examples to copy, ongoing maintenance, and predictable behaviour for anyone familiar with similar Go releases.

## Decision

Use [GoReleaser](https://goreleaser.com) as the single tool that drives `osch` releases. Configuration lives in `.goreleaser.yaml`; releases run via a GitHub Actions workflow triggered by `v*.*.*` tags.

## Consequences

- The release pipeline is concentrated in one file (`.goreleaser.yaml`) and one workflow. Future changes to release shape — archive layout, additional architectures, formula tweaks — go through GoReleaser's configuration model rather than hand-rolled shell.
- The project accepts vendor lock-in to GoReleaser's configuration schema. Moving to a different tool later would mean rebuilding the cross-compile matrix, archive logic, checksum and changelog handling, and tap-PR creation from scratch.
- Release health is now coupled to GoReleaser's own health. A regression in GoReleaser, or a change to its config schema across a major version, has to be addressed before the next release can ship.
- "Tag a commit, the pipeline does the rest" is a property we now depend on. Diagnosing release failures will usually start from GoReleaser's logs rather than from individual shell steps.
- Cross-compilation, archive shapes, checksums, changelog grouping, and Homebrew formula generation are all driven by the same tool. Convenient in practice, but it means a single configuration file influences several otherwise separable concerns.

## Alternatives Considered

- **Hand-rolled shell scripts in GitHub Actions.** Rejected. Would require reinventing the cross-compile matrix, archive packaging, checksum file format, changelog grouping, formula rendering, and tap-PR creation. The maintenance surface is large and provides no benefit over an established tool that already handles all of these.
- **nFPM.** Rejected. nFPM targets Linux native package formats (`.deb`, `.rpm`, `.apk`) — useful for a different distribution strategy, but it does not address Homebrew formula generation or the broader release-orchestration shape that 0002 needs.
- **A GitHub Actions matrix build with no orchestrator.** Rejected. Solves the cross-compile step but leaves archive packaging, checksums, formula generation, and tap-PR creation as bespoke workflow code. Essentially the hand-rolled option with a thinner wrapper.
