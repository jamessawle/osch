# 0007. GitHub access and authentication

- Status: Accepted
- Date: 2026-05-27

## Context

`osch` needs to read from GitHub: the contents of repositories under their `schemas/` folders, and the current commit SHA of each repository's default branch. Some of those repositories will be private, both for organisations using internal schemas and for individual users keeping work-in-progress schemas out of public view (a property the decentralised source model in 0004 explicitly preserves).

The choices here affect three things: whether `osch` can stay a standalone binary (a property locked in by 0001–0003), whether private repositories work without per-tool credential setup, and whether the HTTP path is testable in isolation from subprocess plumbing.

## Decision

- **`osch` talks to GitHub via the HTTP API directly.** It does not shell out to the GitHub CLI (`gh`) for data access.
- **When a token is available, `osch` uses it.** The token is resolved from a fallback chain: a user-supplied environment variable, then the credentials stored by an installed `gh` CLI if one is present, then no token.
- **When no token is available, `osch` operates anonymously.** Anonymous access is supported but limited: public repositories only, and subject to GitHub's unauthenticated rate limits.

## Consequences

- `osch` remains a standalone binary. There is no runtime dependency on `gh`, no subprocess abstraction layer in the data path, and no broken behaviour for users who have never installed `gh`.
- The HTTP path is testable end-to-end against fakes and `httptest`-style harnesses, without mocking subprocess execution. The internal source client interface (0004) stays clean of operating-system concerns.
- Users who already authenticate with the `gh` CLI get private-repository access for free. They do not have to set anything up, copy tokens between tools, or maintain a separate `osch` credential file.
- Users without `gh` can opt in by providing a token through the environment. No file-based configuration is introduced.
- `gh` is treated as an *optional credential source*, not a dependency. Missing `gh`, an unauthenticated `gh`, or a `gh` that errors on credential retrieval are not `osch` errors — they degrade silently to the next step in the fallback chain.
- The auth resolution runs once per `osch` invocation. The resolved state (token or anonymous) is held for the run; the tool does not re-resolve mid-command.
- This ADR is specifically about GitHub. Each future provider added under 0004 brings its own auth story. The shape — borrow from a provider-specific CLI if installed, then env, then anonymous — can repeat per provider, but is not promised by this decision.

## Alternatives Considered

- **Shell out to `gh` for all data access.** Rejected. Adds a hard runtime dependency on `gh` (which non-Homebrew users in particular may not have), turns every API call into a subprocess invocation, makes the data path harder to test, and couples `osch` to whatever output formats `gh` chooses to emit. Borrowing `gh`'s credentials is a much lighter-touch use of the tool than depending on its data layer.
- **Anonymous-only — never authenticate.** Rejected. Makes private-repository schemas impossible to install, which directly contradicts 0004's claim that anyone with a repository, public or private, can publish a schema.
- **Require explicit token configuration only — no auto-discovery from `gh`.** Rejected. Users who already use `gh` for their day-to-day GitHub work would have to duplicate their credentials in another place. That friction is the kind that pushes people to plaintext token files in home directories, which is strictly worse than borrowing the credentials `gh` already stores securely.
