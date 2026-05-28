# Contributing to osch

Thanks for contributing. This guide covers the commit conventions the project
follows. For how the codebase is structured and why decisions were made, see
`docs/architecture/` and `docs/adr/`.

## Commit messages

We follow [Conventional Commits](https://www.conventionalcommits.org/). Two
layers of enforcement keep commits and PR titles in line with the convention:

- A `commit-msg` hook (installed by `make setup`) runs
  [`conform`](https://github.com/siderolabs/conform) against the message and
  rejects commits whose header doesn't match. The configuration lives in
  `.conform.yaml`.
- A GitHub Action (`.github/workflows/pr-title.yml`) fails any PR whose title
  doesn't match. Because the repo squash-merges, the PR title becomes the
  commit on `main`.

### Header format

```
<type>(<optional-scope>)!?: <subject>
```

- **Header length:** 72 characters max.
- **Scope:** optional. Use one when it usefully narrows the change.
- **`!`:** append after the type/scope to flag a breaking change.

### Types we use

| Type       | Use for                                                        |
| ---------- | -------------------------------------------------------------- |
| `feat`     | A new feature or user-visible capability.                      |
| `fix`      | A bug fix.                                                     |
| `chore`    | Maintenance that doesn't change behaviour (deps, tooling, etc).|
| `docs`     | Documentation only.                                            |
| `refactor` | A code change that neither fixes a bug nor adds a feature.     |
| `test`     | Adding or correcting tests.                                    |
| `ci`       | Changes to CI configuration or workflows.                      |
| `build`    | Changes to the build system, release tooling, or dependencies. |
| `revert`   | Reverts a previous commit.                                     |

### One logical change per commit

Keep each commit to a single logical change. Don't bundle an unrelated fix into
a feature commit, and don't split one change across several commits that don't
build or pass tests on their own.

This matters here because much of the work flows through an agent loop that
turns issues into PRs. Small, self-contained commits keep those PRs reviewable:
a reviewer can read the diff one logical step at a time instead of untangling
several concerns from a single blob.

### Reference the issue

Tie every commit back to its issue in the commit body:

- `Refs #N` — the commit relates to issue N.
- `Closes #N` — the commit (once merged) resolves issue N.

For example:

```
feat(add): friendly error messages for invalid input

Closes #28
```

This keeps issue traceability in the history without relying on PR metadata.

## Before you commit

Run `make setup` once after cloning to install the local git hooks via
[lefthook](https://github.com/evilmartians/lefthook). They run the same checks
as CI (`gofmt`, `go vet`, `golangci-lint`) on every commit. You can also run
the checks manually:

- `make lint` — `golangci-lint run`
- `make test` — `go test ./...`
- `make build` — `go build ./...`
