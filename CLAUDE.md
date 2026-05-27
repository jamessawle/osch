# osch

CLI for managing OpenSpec schemas across repos. Public OSS, MIT.

## Where to look
- How the system is structured today: `docs/architecture/`
- Why decisions were made: `docs/adr/`
- Definition of done: pre-commit hooks must pass; CI must be green
- Issue context: the issue body and acceptance criteria are authoritative

## Working principle
If an acceptance criterion is unclear, stop and comment on the issue rather than guess.

## PR descriptions
When opening a pull request (either manually or via the agent loop), use the
`/pr-management:write-pr-description` skill to generate the body. The skill
is enabled at the project level via `.claude/settings.json`.
