# 0002. Distribution: Homebrew tap and GitHub releases

- Status: Accepted, amended by [ADR-0008](0008-homebrew-cask-macos-only.md)
- Date: 2026-05-27

> **Amendment:** [ADR-0008](0008-homebrew-cask-macos-only.md) narrows the
> Homebrew channel below to **macOS only**. `osch` is published to the tap as a
> cask rather than a formula, and Homebrew cannot install a cask on Linux, so
> Linux users are served by the GitHub release tarballs alone. The rest of this
> ADR stands.

## Context

`osch` needs to reach developers on macOS and Linux. The implementation language (0001) is already chosen to avoid pushing a runtime install onto the user, so distribution should preserve that property: install one thing, get a working binary.

A scalable distribution channel needs more than a download URL — it needs discovery, version pinning, signature/checksum integrity, and a path for users to upgrade without re-finding the project. Among the options that fit a small open-source CLI without dragging in a language-specific package manager, only a handful provide that across both target platforms.

Windows is not in scope for this project.

## Decision

Distribute `osch` through a Homebrew tap at `jamessawle/homebrew-tap` as the primary channel, with raw GitHub release tarballs (per OS/architecture) as the secondary channel for users without Homebrew.

## Consequences

- We commit to maintaining the tap repository and the release pipeline that publishes formulas to it. The tap becomes a piece of project infrastructure that must stay green for releases to land.
- macOS and Linux users with Homebrew get install, pin, and upgrade through commands they already know (`brew install jamessawle/tap/osch`, `brew upgrade`). No bespoke commands or trust prompts.
- Users without Homebrew install from a GitHub release tarball — manual download, no upgrade story, no automatic verification beyond the published checksums. This is an accepted limitation, not a placeholder for more channels.
- The tap is a deliberate first step; promotion to `homebrew-core` later is possible but not committed. Core would bring wider discoverability at the cost of its review process and stability requirements.
- The project is tied to Homebrew as an ecosystem. If Homebrew's policy or operation shifted in a way that no longer suited a tap-published CLI, this ADR would need revisiting.

## Alternatives Considered

- **`go install github.com/jamessawle/osch/...`** — rejected. It requires the user to have a Go toolchain installed, which is a language-runtime dependency in everything but name — the same property 0001 set out to avoid, just shifted to the Go ecosystem. It also offers no upgrade UX beyond re-running `go install`, and binds version metadata to Go's module versioning conventions, which is awkward for a tool whose releases we want to control independently.
- **`curl | sh` install script** — rejected as the primary channel. It does not scale: every first-run user trusts an arbitrary script over the network, there is no built-in integrity check, and there is no upgrade path without re-running the script. It also draws legitimate objections from security-minded users.
- **GitHub release tarballs as the *only* channel** — rejected as primary. Tarballs are the secondary channel already; relying on them alone would leave no discovery or upgrade mechanism, and would push the responsibility for version awareness entirely onto each user.
