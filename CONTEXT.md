# Context

Glossary of domain terms used in `osch`. Definitions only — no implementation
details, no decisions (those live in `docs/adr/`).

## Schema

A named bundle of OpenSpec workflow files (e.g. `spec-driven`). On disk, a
schema lives at `openspec/schemas/<name>/` inside a user's project. `osch`
installs, updates, and removes schemas from upstream repositories.

## Active schema

The schema OpenSpec is currently using in a project. Determined by the
top-level `schema:` string in `openspec/config.yaml` (an OpenSpec-owned
file). `osch` reads this file freely, and writes only the `schema:` key
when activating or deactivating a schema during `add`/`remove`. All other
keys (`context`, `rules`, …) are left untouched. At most one schema is
active at a time. The active schema may or may not also be installed under
`openspec/schemas/<name>/`; OpenSpec also resolves user-level and built-in
schemas, which `osch` does not manage.

## Tracked schema

A schema folder `openspec/schemas/<name>/` that contains a `.osch.json` file.
`osch` only manages tracked schemas — an untracked folder is one a user (or
another tool) created by hand, and `osch list` shows it but
`osch update`/`osch remove` will not touch it. Presence is the only signal at
the inventory level; whether the manifest parses or matches the expected
shape is each downstream command's problem (see ADR 0005's stance that
hand-edited manifests are the user's problem, not a state `osch` repairs).

## OpenSpec config file

`openspec/config.yaml` (singular, project-local). Owned by OpenSpec.
`osch` reads it to determine the active schema and writes only the
top-level `schema:` key (string, names the active schema) during
`add`/`remove`. Other keys (`context`, `rules`, …) are ignored on read
and preserved on write.
