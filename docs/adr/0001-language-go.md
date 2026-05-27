# 0001. Language: Go

- Status: Accepted
- Date: 2026-05-27

## Context

`osch` is a small CLI whose job is narrow: read schema folders from GitHub, write them to disk, and edit a small YAML config. The implementation language needs to fit a tool of this size and shape — predictable, low-ceremony, and able to express HTTP, JSON, filesystem, and subprocess work without leaning on a heavy ecosystem.

The work has no concurrency requirement, no heavy compute, and no need for a sophisticated type system — schemas are opaque bytes from `osch`'s perspective. Interactive CLI use favours fast startup over peak throughput.

## Decision

Implement `osch` in Go.

## Consequences

- The codebase is committed to a stdlib-first approach. Pulling in a third-party library is now an explicit, justified choice rather than the default.
- Implementation and test code will be wordier than equivalents in languages with sugar for error propagation (e.g. Rust's `?`). We accept that as the cost of explicit error flow.
- Schema contents are opaque to `osch` today. If that ever changes, Go's JSON and type-system ergonomics for schema-shaped data become the first place we'd feel friction; this ADR would need revisiting at that point.

## Alternatives Considered

- **Rust.** Rejected. The language is more complex than this tool warrants. Lifetimes, the borrow checker, and a more demanding ecosystem would cost real implementation effort that `osch`'s workload does not earn back.
- **Node.** Rejected. A useful Node CLI brings the npm ecosystem along with it: a `package.json`, lockfile churn, transitive-dependency surface, and the editorial overhead of choosing between competing libraries for things Go's stdlib provides. The runtime characteristics (slower cold-start than Go in CLI use) are a smaller, related cost.
- **Bash.** Rejected. Too primitive for the size of the work. Structured JSON handling, GitHub API access, testable error reporting, and a clean YAML edit pass all become disproportionate effort in Bash.
