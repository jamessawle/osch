# Package layout

The current directory tree, with a one-sentence purpose for each package or
significant directory.

```
.
├── cmd/
│   └── osch/             # CLI entry point: argument parsing and command dispatch.
├── internal/
│   └── github/           # Read-only GitHub REST client and shared friendly-error type.
├── docs/
│   ├── adr/              # Architecture Decision Records — why decisions were made.
│   └── architecture/     # Current-state descriptions of the system (this directory).
├── scripts/
│   └── agent-loop/       # Issue-to-PR development-automation loop (not shipped in the binary).
└── .github/
    └── workflows/        # CI (build, lint, test) and release (tag → GoReleaser) workflows.
```

## Packages

- **`cmd/osch`** — `main` package; defines `run`, the `version`/`add` command
  handlers, and the build-stamped `version`/`commit`/`date` variables.
- **`internal/github`** — the `github` package; defines `Client`, the
  `HTTPClient` implementation, `Repo`/`ParseRepo`, and the `ClientError` shape.
  `internal/` keeps it private to this module.

## Notable root files

- `go.mod` — module definition (`github.com/jamessawle/osch`).
- `.goreleaser.yaml` — release build, archive, changelog, and Homebrew-tap config.
- `Makefile` — `setup`, `build`, `lint`, `test` shortcuts that mirror CI.
- `lefthook.yml` — local git hooks running `gofmt`, `go vet`, `golangci-lint`.
- `.golangci.yaml` — linter configuration.
