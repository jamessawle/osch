# 0008. Homebrew channel is a macOS-only cask

- Status: Accepted
- Date: 2026-08-13
- Amends: [ADR-0002](0002-distribution-homebrew-and-releases.md)

## Context

[ADR-0002](0002-distribution-homebrew-and-releases.md) made the Homebrew tap at
`jamessawle/homebrew-tap` the primary distribution channel for "macOS and Linux",
with GitHub release tarballs as the secondary channel. GoReleaser (0003)
implements that as a `brews:` package, which renders a `url` + `sha256` +
`bin.install "osch"` formula into `Formula/osch.rb`.

That formula is a source-install formula in everything but name. Homebrew routes
it through `FormulaInstaller#build` and the install sandbox — a code path
designed for compiling from source — even though `osch` ships a compiled Go
binary and does no build work at install time. Two consequences follow from that
mismatch:

- The project is exposed to a failure class it has no use for. #67
  (`getcwd EPERM` on macOS Tahoe) was an upstream Homebrew sandbox bug that only
  affects the source-install path. It was fixed upstream, so this is supporting
  motivation rather than the justification.
- `brew install --build-from-source osch` is a silent no-op: it delivers the same
  prebuilt binary as a normal install. `.out-of-scope/homebrew-bottles.md`
  recorded that as an accepted limitation after rejecting bottles (#68). Bottles
  were the wrong remedy — the formula/bottle axis is for artifacts that *can* be
  built from source.

Homebrew's own package type for prebuilt artifacts is the cask. The sibling
project `sbxflow` publishes to the same tap as a cask, so keeping `osch` a
formula also means maintaining two publishing shapes and two sets of tap
conventions for the same kind of artifact.

Homebrew cannot install a cask on Linux. Choosing the cask therefore forces a
choice about the Linux Homebrew channel, which is what makes this an ADR rather
than a packaging tweak.

## Decision

Publish `osch` to the tap as a cask (`Casks/osch.rb`, via GoReleaser's
`homebrew_casks:`) rather than a formula, and accept that the Homebrew channel
becomes **macOS-only**. Linux users install from the GitHub release tarballs,
which are promoted from secondary channel to the only channel on Linux.

This amends ADR-0002's commitment to Homebrew for "macOS and Linux". Everything
else in 0002 stands: the tap remains the primary channel on macOS, tarballs
remain published for every supported OS/arch, and homebrew-core promotion remains
uncommitted. ADR-0003 is unaffected — GoReleaser remains the single owner of the
pipeline.

## Consequences

- The install command changes to `brew install --cask jamessawle/tap/osch`. This
  is a breaking change for two groups, so it ships as `v0.2.0`: existing
  formula-installed macOS users are not upgraded by `brew upgrade` (the tap no
  longer publishes a formula of that name) and must `brew uninstall osch` and
  reinstall; Linux Homebrew users lose `brew install` entirely.
- Linux users get no discovery or upgrade story through a package manager —
  manual download and manual re-download to upgrade. This is a real regression
  for them and the reason this needed recording rather than deciding quietly.
- The install-sandbox failure class disappears, and `--build-from-source` stops
  being a misleading no-op because casks have no source-install path at all.
- The linux amd64/arm64 archives keep building and the cask keeps referencing
  their checksums. This is deliberate, not an oversight:
  `brew readall --os=all --arch=all` rejects a cask that resolves no checksum on
  a platform it evaluates, and GoReleaser has no `depends_on macos:` support with
  which to declare the cask macOS-only instead.
- Because the binary is neither signed nor notarized, the cask needs a
  `postflight` hook stripping `com.apple.quarantine` and a caveat saying so.
  Gatekeeper otherwise refuses to run the installed executable. Signing and
  notarization would let both go, and are separate work.
- The release pipeline tracks GoReleaser `nightly` until 2.18.0 ships, because
  the cask formatting the tap's `brew style` job accepts requires
  goreleaser/goreleaser#6752, which is merged but unreleased. That is tracked
  debt, not a permanent posture.
- We would revisit this if Homebrew gained cask support on Linux, or if Linux
  demand justified a second channel there (a distro package, or a formula
  published alongside the cask under a different name).

## Alternatives Considered

- **Keep the formula and accept the source-install path** — rejected. It keeps
  Linux Homebrew working, which is the one thing the cask gives up, but it keeps
  the package type wrong for the artifact, keeps the sandbox failure class, keeps
  `--build-from-source` misleading, and keeps two publishing shapes in one tap.
  The Linux Homebrew audience for a pre-1.0 personal-tap CLI is small enough that
  it does not outweigh those.
- **Publish both a formula (for Linux) and a cask (for macOS)** — rejected. Two
  packages of the same name in one tap makes `brew install jamessawle/tap/osch`
  ambiguous, and distinct names (`osch` cask, `osch-linux` formula) push tap
  trivia into the install instructions. It also doubles the release surface for a
  channel we have no evidence anyone uses.
- **Bottles** — rejected previously in #68; see
  `.out-of-scope/homebrew-bottles.md`. Bottles answer "how do we avoid the source
  build?" within the formula model; the cask makes the question moot and needs no
  bespoke release machinery.
- **Drop Homebrew entirely and ship only tarballs** — rejected for the same
  reason ADR-0002 rejected it: it removes discovery and upgrade for the majority
  of users to avoid an asymmetry that affects a minority.
