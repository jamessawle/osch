# 0004. Decentralised schema sources, GitHub first

- Status: Accepted
- Date: 2026-05-27

## Context

`osch` could plausibly source schemas in several different ways: from a central registry the project would operate, from a curated list of approved repositories, or from arbitrary upstream repositories with no central coordination. Each model carries different costs, governance properties, and limits on who can publish.

The OpenSpec schemas the tool installs are themselves small folders of files that live alongside whatever project owns them. Their natural home is a version-control repository. Choosing where `osch` is willing to read them from sets the boundary on who can publish a schema, what infrastructure the project needs to operate, and how the CLI's source-identifying syntax evolves.

## Decision

Schemas are sourced from upstream repositories with no central coordination — anyone can publish a schema by putting it under a `schemas/` folder in a repository the tool can read. GitHub is the only supported host today, and the CLI source argument is the bare `<owner>/<repo>` form.

The project commits to keeping the *internal* source-fetching abstraction host-agnostic so that adding another host later (Bitbucket, GitLab, …) is additive rather than a rewrite. When a second host is implemented, the CLI grammar will gain a `<host>:<owner>/<repo>` prefixed form, and the existing `<owner>/<repo>` form will continue to mean GitHub for backwards compatibility. No host-prefix syntax is introduced until that need is real.

## Consequences

- The project runs no registry infrastructure and curates no list of approved schemas. Discovery happens through whatever channels schema authors choose (their READMEs, blog posts, organic search), and `osch` stays out of that question.
- Anyone with a GitHub repository can publish a schema, public or private. There is no review, gatekeeping, or rate-limiting that the project controls; users get whatever quality the schema author chose to ship. Private-repository access depends on the consumer having a valid token (see 0007).
- The internal source-fetching abstraction must stay host-agnostic from the start, even though only GitHub is wired. Coupling the rest of `osch` to a GitHub-shaped `Client` interface would create rework the day a second host lands.
- The CLI's source-argument grammar is currently flat: `<owner>/<repo>`, no prefix. We do *not* introduce a `github:` alias today — there is no second host to be symmetric with yet, and adding syntax surface for an unused case is unnecessary. When the second host arrives, the prefix grammar is introduced then.
- "Private repository support" today effectively means GitHub-private. Each new host added later brings its own auth story; the ADR's "decentralised" claim is partial until more than one host is implemented.
- A schema being broken or abandoned by its author is a user-visible problem, not a project problem. `osch list`'s drift detection (0006) is the only mechanism that surfaces this; responsibility for choosing good schemas sits with the user.

## Alternatives Considered

- **A central `osch.dev`-style registry.** Rejected. Running a registry means standing up and maintaining infrastructure — domain, hosting, an authority for who can publish — none of which the project is willing to own. A registry would also become a single point of failure for an otherwise file-based tool, and a curation/governance question we have no interest in answering.
- **Lock the design to GitHub permanently.** Rejected. Even though GitHub is the only host today, baking a GitHub-shaped `Client` interface and a GitHub-only CLI grammar into the design would make adding a second host a breaking change. Host-agnostic internals cost very little now and avoid that bind.
- **Introduce `github:<owner>/<repo>` syntax immediately, in anticipation of other hosts.** Rejected for the initial release. Adding grammar surface area for cases that don't yet exist is unnecessary; the bare form is enough until there is something to be symmetric with.
- **Any git URL via `git clone`.** Rejected for now. It would make `osch` work against any git host without further code changes, but it forces a hard dependency on the `git` binary, pulls down the entire repository to read a small folder, complicates auth (per-host SSH/HTTPS configuration), and turns "fetch a schema" from a single HTTP request into a clone-and-walk. The host-agnostic abstraction leaves this option open if it ever becomes the right answer for a particular host.
