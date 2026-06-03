# osch

[![CI](https://github.com/jamessawle/osch/actions/workflows/ci.yml/badge.svg)](https://github.com/jamessawle/osch/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/jamessawle/osch)](https://github.com/jamessawle/osch/releases/latest)
[![License: MIT](https://img.shields.io/github/license/jamessawle/osch)](LICENSE)

`osch` is a command-line tool for managing [OpenSpec](https://github.com/Fission-AI/OpenSpec) schemas across repositories, giving you a single consistent workflow for working with specs wherever they live.

## Install

```
brew install jamessawle/tap/osch
```

> **Note:** The Homebrew formula is published by the release pipeline. If the command above fails because the formula doesn't exist yet, the pipeline may not have run for a release — build from source instead (see below).

## Build from source

Requires a recent [Go](https://go.dev/dl/) toolchain.

```
go build -o osch ./cmd/osch
```

This produces an `osch` binary in the current directory.

## Usage

### Add a schema from an upstream repository

```
osch add <owner>/<repo> [schema] [--activate | --no-activate]
```

Resolves the upstream's default-branch HEAD, copies every file under `schemas/<name>/` into `openspec/schemas/<name>/` in the current repository, and writes a `.osch.json` manifest alongside each installed schema pinning the upstream commit and a SHA-256 of each file.

When the upstream publishes more than one schema, pass the schema name as the second argument to pick which one to install. Omitting the argument against a multi-schema upstream — or passing a name that is not published — aborts before any files are written and prints the list of available schemas so the command can be re-run.

After a successful install, `osch` offers to set the new schema as active by writing the top-level `schema:` key in `openspec/config.yaml`. Interactively (stdin is a TTY) it prompts `y/N`; when stdin is not a TTY the prompt is skipped silently. `--activate` activates without prompting and `--no-activate` skips both prompt and activation; passing both is an error. `osch` never creates `openspec/config.yaml` — if the file is absent, activation is skipped with a message and the install itself still succeeds. The writer round-trips the file through a YAML decode/encode cycle and so does not preserve comments, blank lines, or key order; other top-level keys' values are kept.

### List installed schemas

```
osch list [--offline]
```

Scans `openspec/schemas/` in the current directory and prints a table with one row per installed schema:

- `NAME` — the schema folder name.
- `ACTIVE` — marked with `*` when the schema name matches the top-level `schema:` key in `openspec/config.yaml`.
- `TRACKED` — `yes` when the folder contains a `.osch.json` manifest (i.e. was installed by `osch`), otherwise `no`.
- `SOURCE` — the manifest's `source` field (e.g. `owner/repo`); blank for untracked rows.
- `SHA` — the first 7 characters of the manifest's pinned commit SHA; blank for untracked rows.
- `FILES` — `clean` when every file under the schema folder matches the per-file SHA-256 recorded in the manifest at install time; `modified` when any file's hash differs, any tracked file is missing locally, any extra file is present in the schema folder, or the manifest has no `files` map. The check is fully local — `--offline` does not affect it. Blank for untracked rows.
- `UPSTREAM` — `up-to-date` when the pinned SHA matches the upstream default-branch HEAD, `behind` when it differs, or `unknown` when the upstream cannot be resolved (network error, repo gone, etc.). Blank for untracked rows. Multiple schemas from the same source share a single upstream lookup within one invocation.

Pass `--offline` to skip all upstream lookups; every tracked row's `UPSTREAM` reads `unknown` and the command still exits 0.

If `openspec/schemas/` is missing or empty, prints `No OpenSpec schemas installed` and exits 0.

### Update an installed schema

```
osch update <schema>
```

Reads `openspec/schemas/<schema>/.osch.json`, resolves the upstream default branch's HEAD commit, and overwrites the local schema folder with the upstream bytes at that SHA. The manifest is rewritten with the new SHA and a refreshed per-file SHA-256 `files` map; files removed upstream are deleted locally and files added upstream are written locally. If the pinned SHA already matches upstream the command is a no-op and reports "already up to date". If the schema folder or its `.osch.json` is missing the command aborts with a non-zero exit.

Before any network call, `osch update` checks the local schema against the per-file SHA-256 hashes in `.osch.json`. If any tracked file's content has changed, a tracked file is missing, or an extra untracked file is present (excluding `.osch.json`), the command aborts with a non-zero exit and an error that lists every offending path. No files are written, deleted, or otherwise touched on refusal. The check is fully offline — a refusal makes zero upstream calls. A `--force` override lands in a follow-up.

### Remove an installed schema

```
osch remove <schema> [--yes] [--activate <name> | --no-activate]
```

Deletes `openspec/schemas/<schema>/` recursively from the current directory, whether or not the schema was originally installed by `osch add` (the `.osch.json` manifest is incidental). Interactively (stdin is a TTY) it prompts `y/N`; `--yes` skips the prompt. When stdin is not a TTY and `--yes` is not set the command aborts rather than silently proceeding. If the folder does not exist, the command exits non-zero with a clear message. The schema argument must be a plain folder name — path separators and `..` are rejected.

If the removed schema is the one named in `openspec/config.yaml`'s top-level `schema` key, `osch` picks a replacement so the project is never left pointing at a missing folder. The replacement is determined as follows:

- If `--activate <name>` is set, the key is rewritten to `<name>`. The target must either be `spec-driven` (OpenSpec's default) or another installed schema; an unknown target aborts the whole command **before** the folder is deleted so a typo does not cost you the schema.
- If `--no-activate` is set, or stdin is not a TTY, the key falls back silently to `spec-driven`.
- Otherwise, if at least one other schema remains installed, `osch` shows a numbered menu of those schemas plus `spec-driven` and accepts either a 1-based index or the schema name. Empty input (just Enter) selects `spec-driven`. Invalid input re-prompts up to three times before falling back to `spec-driven`.
- If no other schemas are installed, the menu is skipped and the key falls back silently to `spec-driven`.

The success line distinguishes the activation outcome: `removed <name> (active schema set to <chosen>)` for a non-default selection, or `removed <name> (active schema reset to spec-driven)` for the fallback. Passing both `--activate` and `--no-activate` is an error. If `openspec/config.yaml` is absent or unparseable, no rewrite is attempted and the deletion still succeeds.

## Status

`osch` is pre-1.0 and under active development. Breaking changes are possible between releases until a 1.0 release is tagged.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Released under the [MIT License](LICENSE).
