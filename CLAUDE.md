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

## Agent skills

Configuration the mattpocock engineering skills (`triage`, `to-issues`,
`grill-with-docs`, …) read from. Edit `docs/agents/*.md` directly to change it.

### Issue tracker
Issues live in `jamessawle/osch` GitHub Issues, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels
The five canonical triage roles map 1:1 to GitHub labels of the same name
(`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`);
`ready-for-agent` is the agent loop's entry gate. See `docs/agents/triage-labels.md`.

### Domain docs
Single-context: one `CONTEXT.md` (created lazily by `grill-with-docs`) + `docs/adr/` at the root. See `docs/agents/domain.md`.
