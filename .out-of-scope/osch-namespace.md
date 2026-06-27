# Single `.osch/` namespace per schema

osch does **not** consolidate all osch-owned files into a single `.osch/`
directory per schema. The on-disk layout stays split by intent:

- **Committed state** — the per-schema manifest at
  `openspec/schemas/<name>/.osch.json` (ADR 0005), committed alongside the
  schema for provenance.
- **Local-only state** — scratch artefacts under
  `openspec/schemas/<name>/.osch/`, wholly git-ignored by a self-ignoring
  `.osch/.gitignore` (`*`). Today this holds only `osch update --force`
  backups (#36).

The proposal was to move the committed manifest *into* that `.osch/` directory
(e.g. `.osch/manifest.json`) and express the committed-vs-ignored split via
`.gitignore` negation inside one folder.

## Why this is out of scope

This is speculative consolidation with a real migration cost and no functional
benefit — and it would arguably make the layout worse:

- **The boundary gets less clear, not more.** Today the split is obvious from
  the filesystem: a committed top-level dotfile (`.osch.json`) versus a
  wholly-ignored dot-directory (`.osch/`). Collapsing both into one directory
  moves the commit/ignore boundary *inside* it, expressed through `.gitignore`
  negation (`*` plus `!manifest.json`). That's a footgun — easier to
  accidentally commit a backup, or to fail to commit the manifest.
- **Real migration cost, zero functional gain.** It's a consistency/future-
  proofing change, not a bug fix. It tensions an explicit, recent ADR (0005)
  and would need a superseding/amending ADR plus a migration story for existing
  installs.
- **The "more internal artefacts coming" premise is speculative.** Today there
  is exactly one local artefact (backups), and it already has a tidy home under
  `.osch/`. ADR 0005's portability and atomicity properties still hold as-is.

This is rejected as *speculative consolidation*, not as a bad idea in
principle.

## Reopen this if any of these become true

(None are true today.)

1. **A second local-only artefact type appears.** Backups (#36) are currently
   the only local artefact, and they already have a clean home. If osch grows a
   second kind of local state (caches, logs, lockfiles, …), the "one canonical
   namespace" argument starts earning its keep — revisit then.
2. **The current split causes a real, observed problem.** If users actually
   trip over the top-level-dotfile-vs-ignored-directory distinction —
   accidentally committing backups, or losing/failing to commit `.osch.json` —
   that's concrete evidence the layout is wrong, not speculative.
3. **ADR 0005's own revisit trigger fires.** If OpenSpec ever stops ignoring
   dotfiles/dot-directories (the condition ADR 0005 already names), the whole
   on-disk layout has to be reconsidered anyway — fold this consolidation into
   that work.
4. **A migration mechanism exists for an unrelated reason.** The migration cost
   is a primary objection. If an `osch migrate` step ships for something else,
   that objection largely evaporates and the consolidation becomes cheap to do
   cleanly.

## Prior requests

- #78 — "Consolidate osch-owned files into a single .osch/ namespace per schema"
