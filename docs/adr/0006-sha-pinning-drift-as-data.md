# 0006. SHA pinning and drift as data

- Status: Accepted
- Date: 2026-05-27

## Context

Once a schema is installed, two questions follow `osch` around for the rest of its life:

1. **Are you up to date?** Is the installed copy the same as the upstream's current version?
2. **Have you changed things?** Has the user (or any tool) edited the installed files since they were written?

The answers shape how the tool relates to a project's normal git workflow, what guarantees `osch` can offer about reproducibility, and whether refresh operations can run safely. The architectural decisions here are how an installed schema is *bound* to an upstream version, how *modifications* to the local files are detected, and what stance the tool takes when modifications exist.

## Decision

- **Pin each installed schema to an upstream commit SHA.** That SHA, not a tag or branch, is the schema's reproducible upstream identity.
- **Record per-file integrity locally at install time.** Future modification detection compares against that local record, without contacting the upstream.
- **Treat local edits as legitimate state, not as corruption.** The tool reports drift, refuses destructive operations when it would overwrite edits, and otherwise leaves the user in control of their files.

## Consequences

- "Are you up to date?" has a hard, unambiguous answer: the pinned SHA either equals the upstream's current commit or it does not. There is no version-comparison ambiguity to argue about.
- "Have you modified files?" is a fully local check. The tool can answer it without any network access at all.
- Only the up-to-date check requires contact with the upstream — a single call per source. Everything else about an installed schema is knowable from what's already on disk.
- Local edits become a legitimate, persistent state the tool will keep reporting until the user resolves it. The tool does not pressure the user to "fix" their edits; they own the choice.
- A user who deliberately tampers with both the file and the integrity record could fool the modification check. We accept this: defending against it would cost every honest user a network round-trip per check, to guard against a self-attack with no clear motive.
- Commit SHAs are opaque to humans. This is fine — they are plumbing. The user-facing surface is the tool's reporting of drift state in human-readable terms, not the SHAs themselves.
- Reproducibility is now a property of an install, not a hope. Two users installing the same schema at the same pinned SHA get bit-identical bytes; an update is an explicit, recorded change of that SHA.

## Alternatives Considered

- **Mutable references — pin to a tag, branch, or "latest always".** Rejected. Tags can be re-pointed, branches move on every push, "latest" gives a different answer to every caller. All three break the property that an installed schema has a single, reproducible upstream identity. Acceptable for ecosystems where a registry enforces tag immutability; not acceptable for direct git-host fetching where there is no central enforcer.
- **Make the pinning mechanism pluggable now.** Rejected on YAGNI grounds. Every host we expect to support is git-backed, and commit-SHA pinning is host-agnostic across all of them. If a non-SHA mechanism ever becomes useful, the manifest format (0005) is versioned and extensible; introducing pluggability before there is anything to plug in is speculative complexity.
- **Treat any local edit as corruption.** Rejected. Once installed, the bytes live in the user's repository — they own them, including the right to patch a schema while a fix lands upstream, debug a problem locally, or carry a small permanent divergence. Refusing to acknowledge this would turn `osch` into adversarial tooling and force users to eject from `osch` management for every legitimate case.
- **Compare against upstream on every check, not against a local record.** Rejected. This was the original design and would have given a marginally stronger modification check (it catches the deliberate-falsification edge case), but at the cost of network access for *every* check, including ones the user runs offline or in tight loops. Capturing integrity at install time gives the same answer in every realistic scenario at a fraction of the cost and degrades cleanly to fully offline.
