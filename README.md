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
osch add <owner>/<repo>
```

Resolves the upstream's default-branch HEAD, copies every file under `schemas/<name>/` into `openspec/schemas/<name>/` in the current repository, and writes a `.osch.json` manifest alongside each installed schema pinning the upstream commit and a SHA-256 of each file.

Only upstreams that expose a single schema directory under `schemas/` are supported today; multi-schema upstreams will follow.

## Status

`osch` is pre-1.0 and under active development. Breaking changes are possible between releases until a 1.0 release is tagged.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Released under the [MIT License](LICENSE).
