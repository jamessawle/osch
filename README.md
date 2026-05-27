# osch

CLI for managing OpenSpec schemas across repos.

## Setup

After cloning, run:

```
make setup
```

This installs the local git hooks via [lefthook](https://github.com/evilmartians/lefthook) so the same checks CI runs (`gofmt`, `go vet`, `golangci-lint`) fire on every commit.

## Common tasks

- `make lint` — runs `golangci-lint run` (mirrors CI).
- `make test` — runs `go test ./...` (mirrors CI).
- `make build` — runs `go build ./...`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for commit conventions.
