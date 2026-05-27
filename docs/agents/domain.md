# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root (the domain glossary), if it exists.
- **`docs/adr/`** — read ADRs that touch the area you're about to work in (currently 0001–0007: language, distribution, release tooling, decentralised sources, per-schema manifest, SHA-pinning, GitHub access/auth).

If any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The producer skill (`/grill-with-docs`) creates `CONTEXT.md` lazily when terms actually get resolved.

## Layout

osch is **single-context**: one `CONTEXT.md` + `docs/adr/` at the repo root. There is no `CONTEXT-MAP.md` and no per-context ADR directories.

```
/
├── CONTEXT.md          (created lazily by grill-with-docs)
├── docs/adr/           (0001-…-0007-…)
├── cmd/osch/           (CLI entrypoint + Cobra commands)
└── internal/           (source abstraction, etc.)
```

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `CONTEXT.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal — either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/grill-with-docs`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0006 (SHA-pinning, drift as data) — but worth reopening because…_
