# 0005. Per-schema `.osch.json` manifest

- Status: Accepted
- Date: 2026-05-27

## Context

For `osch list` and `osch update` to work, the tool needs persistent install metadata per schema: which upstream repository it came from, which commit SHA it was pinned to, and what manifest format the data is written in. Without this, `osch` would have no way to refresh a schema or report drift; the schema folder on disk would be just files with no provenance.

That metadata has to live somewhere. The choice of *where* shapes how portable schema folders are, whether OpenSpec can ignore `osch` entirely, and how install state and on-disk files stay (or fail to stay) in sync.

## Decision

Each installed schema carries its own manifest at `openspec/schemas/<name>/.osch.json`, inside the schema folder it describes. The file contains `$schema`, `schema_version`, `source`, `name`, `sha`, and a `files` map of per-file content hashes captured at install time (see 0006 for why those hashes live here). It is intended to be committed to the user's repository alongside the schema files it accompanies.

## Consequences

- Schema folders are self-describing and portable: copy or move the folder and the install metadata travels with it. No external lookup is needed to know where a folder came from.
- The manifest and the files it tracks live atomically. There is no central index that could drift away from per-folder reality, no class of bug where the index says one thing and the disk says another.
- OpenSpec is unaware of `.osch.json` — it is a dotfile inside `schemas/<name>/`, and OpenSpec's schema reader ignores dotfiles. `osch` coexists with OpenSpec without modifying any user-facing schema files. If OpenSpec ever changed that behaviour, this ADR would need revisiting.
- The manifest is part of the user's repository. Pull request diffs and git history for the schema will include `.osch.json` changes on each `osch update` (the `sha` field bumps). This is intentional — the user's history records both the upstream version they pulled and when.
- Format evolution is gated by `schema_version`. Future `osch` versions must handle older versions or refuse cleanly; that lever is the only forward-compatibility mechanism we have.
- Users (or other tools) can edit or delete `.osch.json`. Deleted is treated as "untracked" by `osch list`; hand-edited is the user's problem and not a state `osch` will detect or repair. This follows from the same drift-as-data principle that governs the schema files themselves (0006).

## Alternatives Considered

- **A single project-level `openspec/osch.json` listing every installed schema.** Rejected. Schemas could no longer be moved or copied between projects independently — the central index would need updating in lockstep, and would also become a new piece of state that can disagree with the per-folder reality on disk. The per-schema manifest removes that class of bug entirely.
- **A user-level / global manifest file (for example, `~/.config/osch/state.json`).** Rejected. OpenSpec itself has no global configuration model — it is purely project-local — so a global `osch` file would have nothing to anchor to and would need to invent its own project-to-schema mapping, paths to maintain across machine moves, and a new failure mode where the global file disagrees with on-disk reality. It would add complexity for no benefit the per-folder manifest doesn't already provide.
- **Embed `osch` metadata inside the schema's own files (for example, a tooling key inside `manifest.schema.json` or another file the schema already defines).** Rejected. Would require modifying user-facing schema files to carry `osch`'s install state, coupling the tool to OpenSpec's file format and forcing upstream schema authors to be aware of `osch`. Keeping our metadata in a separate dotfile preserves the property that an OpenSpec project sees no difference between an `osch`-managed schema and a hand-written one.
