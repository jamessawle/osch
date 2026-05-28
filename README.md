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
osch add <owner>/<repo> [--activate | --no-activate]
```

Resolves the upstream's default-branch HEAD, copies every file under `schemas/<name>/` into `openspec/schemas/<name>/` in the current repository, and writes a `.osch.json` manifest alongside each installed schema pinning the upstream commit and a SHA-256 of each file.

After a successful install, `osch` offers to set the new schema as active by writing the top-level `schema:` key in `openspec/config.yaml`. Interactively (stdin is a TTY) it prompts `y/N`; when stdin is not a TTY the prompt is skipped silently. `--activate` activates without prompting and `--no-activate` skips both prompt and activation; passing both is an error. `osch` never creates `openspec/config.yaml` — if the file is absent, activation is skipped with a message and the install itself still succeeds. The writer round-trips the file through a YAML decode/encode cycle and so does not preserve comments, blank lines, or key order; other top-level keys' values are kept.

Only upstreams that expose a single schema directory under `schemas/` are supported today; multi-schema upstreams will follow.

### List installed schemas

```
osch list
```

Scans `openspec/schemas/` in the current directory and prints a table with one row per installed schema:

- `NAME` — the schema folder name.
- `ACTIVE` — marked with `*` when the schema name matches the top-level `schema:` key in `openspec/config.yaml`.
- `TRACKED` — `yes` when the folder contains a `.osch.json` manifest (i.e. was installed by `osch`), otherwise `no`.

If `openspec/schemas/` is missing or empty, prints `No OpenSpec schemas installed` and exits 0.

## Status

`osch` is pre-1.0 and under active development. Breaking changes are possible between releases until a 1.0 release is tagged.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Released under the [MIT License](LICENSE).
