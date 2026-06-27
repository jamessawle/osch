# Homebrew Bottles

`osch` does **not** ship Homebrew bottles. The Homebrew formula is the
`url`-source formula that GoReleaser's `brews:` block renders into the
`jamessawle/homebrew-tap` tap — a tarball with `bin.install "osch"`, installed
through Homebrew's source-install code path rather than poured from a
`bottle do … sha256 …` block.

## Why this is out of scope

The request (move to bottles) is technically accurate about the install path:
a `url` + `bin.install` formula does route through `FormulaInstaller#build`
and the install sandbox, the channel Homebrew designed for building from
source. But the case for switching does not hold up at this project's scale:

- **The benefit is theoretical for our distribution shape.** We publish to a
  *personal tap* (`jamessawle/homebrew-tap`), not homebrew-core. The
  `brew audit --strict` "should be a bottle" posture only matters if we pursue
  core promotion, and ADR 0002 explicitly leaves that **uncommitted**. So the
  headline benefit isn't one we've signed up to need.

- **The cost is large and structural.** GoReleaser has **no native bottle
  support** — its `brews:` directive only emits `url`-source formulas. Bottles
  are normally produced by Homebrew's own `brew test-bot` / `brew bottle`
  machinery on per-macOS-version runners (`arm64_sonoma`, `arm64_sequoia`, …).
  Delivering them from a third-party tap would mean standing up net-new bespoke
  CI to build, host (`root_url`), checksum, and stitch bottle blocks per
  OS/arch and macOS version. That is precisely the hand-rolled release
  machinery ADR 0003 was written to avoid by making GoReleaser the single
  owner of the pipeline.

- **The sandbox-regression motivation is weak.** The incident that prompted
  this (#67) was fixed upstream in Homebrew/brew#22440. The risk is
  low-frequency and self-heals as `brew` updates; it doesn't justify a
  structural change to the release pipeline.

If the project ever commits to homebrew-core promotion (revisiting ADR 0002),
this should be reopened as a deliberate design exercise — including a
superseding/amending ADR against 0003, since bottle building introduces
release machinery GoReleaser cannot own.

## Known accepted limitation

`brew install --build-from-source osch` is currently a silent no-op — it
delivers the same prebuilt binary as a normal install. We are choosing to live
with this at present rather than make the flag meaningful or unsupported.

## Prior requests

- #68 — "Ship Homebrew bottles instead of URL-source formula"
