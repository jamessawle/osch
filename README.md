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
- `UPSTREAM` — `up-to-date` when the pinned SHA matches the upstream default-branch HEAD, `behind` when it differs, or `unknown` when the upstream cannot be resolved (network error, repo gone, etc.). Blank for untracked rows. Multiple schemas from the same source share a single upstream lookup within one invocation.

Pass `--offline` to skip all upstream lookups; every tracked row's `UPSTREAM` reads `unknown` and the command still exits 0.

If `openspec/schemas/` is missing or empty, prints `No OpenSpec schemas installed` and exits 0.

### Update an installed schema

```
osch update <schema>
```

Reads `openspec/schemas/<schema>/.osch.json`, resolves the upstream default branch's HEAD commit, and overwrites the local schema folder with the upstream bytes at that SHA. The manifest is rewritten with the new SHA and a refreshed per-file SHA-256 `files` map; files removed upstream are deleted locally and files added upstream are written locally. If the pinned SHA already matches upstream the command is a no-op and reports "already up to date". If the schema folder or its `.osch.json` is missing the command aborts with a non-zero exit. This slice always overwrites local edits — refusing to overwrite modified files lands in a follow-up.

### Remove an installed schema

```
osch remove <schema> [--yes]
```

Deletes `openspec/schemas/<schema>/` recursively from the current directory, whether or not the schema was originally installed by `osch add` (the `.osch.json` manifest is incidental). Interactively (stdin is a TTY) it prompts `y/N`; `--yes` skips the prompt. When stdin is not a TTY and `--yes` is not set the command aborts rather than silently proceeding. If the folder does not exist, the command exits non-zero with a clear message. The schema argument must be a plain folder name — path separators and `..` are rejected.

Clearing the active key in `openspec/config.yaml` when the removed schema is the active one is not yet implemented.

## Status

`osch` is pre-1.0 and under active development. Breaking changes are possible between releases until a 1.0 release is tagged.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Released under the [MIT License](LICENSE).
