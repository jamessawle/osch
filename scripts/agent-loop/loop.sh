#!/usr/bin/env bash
# scripts/agent-loop/loop.sh
#
# Personal development automation: poll GitHub for issues labelled
# `ready-for-agent`, dispatch each to `claude -p` in an isolated git
# worktree, open a PR if the run produced commits.
#
# See scripts/agent-loop/README.md for full docs.

set -euo pipefail

# --- Configuration -----------------------------------------------------------

POLL_INTERVAL="${POLL_INTERVAL:-60}"

LABEL_QUEUE="ready-for-agent"
LABEL_PROGRESS="agent:in-progress"
LABEL_FAILED="agent:failed"
LABEL_AUTHORED="agent:authored"

REPO_ROOT="$(git rev-parse --show-toplevel)"
WORKTREE_BASE="$(dirname "$REPO_ROOT")"
REPO_NAME="$(basename "$REPO_ROOT")"

# Agent-only permission restrictions. Loaded via `claude --settings` on every
# `claude -p` invocation so they layer on top of the interactive baseline in
# .claude/settings.json (deny rules are unioned across settings sources). This
# file lives next to the loop, not the worktree, so the sandbox policy is fixed
# by the loop rather than by whatever branch the agent happens to check out.
# See README.md ("Permissions") for the rationale.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_SETTINGS="${SCRIPT_DIR}/settings.json"

# --- Helpers -----------------------------------------------------------------

log() {
    printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*"
}

die() {
    printf '[FATAL] %s\n' "$*" >&2
    exit 1
}

require_tool() {
    command -v "$1" >/dev/null 2>&1 || die "required tool not found on PATH: $1"
}

# Print the agent brief for an issue: the body of the most recent comment that
# contains a "## Agent Brief" heading. Empty output means no brief was posted
# (older issues, or ones queued by hand) — callers fall back to the body alone.
# Runs with the loop's own gh auth, so the agent sandbox denies don't apply.
fetch_agent_brief() {
    local n="$1"
    gh issue view "$n" --json comments \
        --jq '[.comments[] | select((.body // "") | contains("## Agent Brief"))] | last | .body // ""' \
        2>/dev/null || true
}

# --- Preflight ---------------------------------------------------------------

require_tool gh
require_tool git
require_tool claude
require_tool jq

gh auth status >/dev/null 2>&1 || die "gh is not authenticated. Run 'gh auth login'."

# Fail loudly if the agent settings file is missing or invalid JSON. In
# `claude -p` mode a settings file that fails validation is silently ignored,
# which would drop the sandbox denies without any error — so we guard here
# rather than discover it after a runaway agent has escaped.
[ -f "$AGENT_SETTINGS" ] || die "agent settings file not found: $AGENT_SETTINGS"
jq empty "$AGENT_SETTINGS" >/dev/null 2>&1 || die "agent settings file is not valid JSON: $AGENT_SETTINGS"

# --- Failure handling --------------------------------------------------------

fail_issue() {
    local n="$1"
    local tail_text="$2"

    gh issue edit "$n" \
        --remove-label "$LABEL_PROGRESS" \
        --add-label "$LABEL_FAILED" >/dev/null || true

    gh issue comment "$n" --body "Agent run failed. Last output:

\`\`\`
${tail_text}
\`\`\`" >/dev/null || true
}

# --- Issue processing --------------------------------------------------------

process_issue() {
    local issue_json="$1"
    local n title body branch worktree

    n=$(echo "$issue_json" | jq -r '.number')
    title=$(echo "$issue_json" | jq -r '.title')
    body=$(echo "$issue_json" | jq -r '.body // ""')

    branch="agent/issue-${n}"
    worktree="${WORKTREE_BASE}/${REPO_NAME}-agent-${n}"

    log "Claiming issue #${n}: ${title}"
    if ! gh issue edit "$n" \
            --remove-label "$LABEL_QUEUE" \
            --add-label "$LABEL_PROGRESS" >/dev/null; then
        log "WARN: failed to claim #${n} (label race?). Skipping."
        return
    fi

    log "Fetching origin/main..."
    git -C "$REPO_ROOT" fetch origin main --quiet

    if [ -e "$worktree" ]; then
        log "WARN: worktree path ${worktree} already exists; removing"
        git -C "$REPO_ROOT" worktree remove --force "$worktree" 2>/dev/null || rm -rf "$worktree"
    fi

    log "Creating worktree at ${worktree}"
    git -C "$REPO_ROOT" worktree add "$worktree" -b "$branch" origin/main

    # The authoritative spec lives in the "## Agent Brief" comment (added by
    # /triage); the body is supporting context. Older issues with no brief
    # fall back to body alone.
    local brief brief_block=""
    brief="$(fetch_agent_brief "$n")"
    if [ -n "$brief" ]; then
        brief_block="$(cat <<EOF


Agent brief (authoritative specification):
${brief}

The agent brief is the exclusive source of truth. Where the brief and the issue body disagree on any specific value (filenames, exact strings, error rules, etc.), the brief wins without exception. Do not copy values from the body that contradict the brief.
EOF
)"
    fi

    local prompt
    prompt="$(cat <<EOF
You are implementing GitHub issue #${n}.

Issue title: ${title}

Issue body:
${body}
${brief_block}

Instructions:
- Read CLAUDE.md in the repo root for project conventions.
- Make the edits the issue requires.
- Run any relevant local checks (build, tests, gofmt, vet).
- Commit your changes with Conventional Commit messages (e.g. 'feat:', 'fix:', 'chore:', 'docs:'). Reference the issue with 'Refs #${n}' in the commit body.
- Before your final commit, walk the brief's \`Acceptance criteria\` list AC-by-AC. For each item, identify the specific line of code or test that satisfies it. If you can't, the item is not yet done.
- DO NOT push the branch and DO NOT open a pull request — the calling script will handle that.
- If an acceptance criterion is unclear, stop without committing.
EOF
)"

    local output_file
    output_file="$(mktemp)"
    log "Invoking claude -p (output captured to ${output_file})"

    local exit_code=0
    # --permission-mode dontAsk activates the persistent permissions.allow /
    # permissions.deny rules; without the flag, the settings rules are inert in
    # headless mode. --settings layers the agent-only deny list on top of the
    # repo's interactive baseline so a runaway agent stays in its sandbox.
    (cd "$worktree" && claude -p --permission-mode dontAsk --settings "$AGENT_SETTINGS" "$prompt") >"$output_file" 2>&1 || exit_code=$?

    local output_tail
    output_tail="$(tail -n 50 "$output_file")"
    rm -f "$output_file"

    if [ "$exit_code" -ne 0 ]; then
        log "Claude exited ${exit_code} for #${n}; marking failed"
        fail_issue "$n" "$output_tail"
        git -C "$REPO_ROOT" worktree remove --force "$worktree" || true
        return
    fi

    local commit_count
    commit_count="$(git -C "$worktree" rev-list origin/main..HEAD --count)"
    if [ "$commit_count" -eq 0 ]; then
        log "No new commits produced for #${n}; marking failed"
        fail_issue "$n" "$output_tail"
        git -C "$REPO_ROOT" worktree remove --force "$worktree" || true
        return
    fi

    log "Pushing ${branch} (${commit_count} commit(s))"
    if ! git -C "$worktree" push -u origin "$branch"; then
        log "Push failed for #${n}; marking failed"
        fail_issue "$n" "push failed for branch ${branch}"
        return
    fi

    # Generate the PR body via the pr-management:write-pr-description skill.
    # The skill is enabled at the project level via .claude/settings.json, so a
    # `claude -p` run from inside the worktree has access to it.
    log "Generating PR body via /pr-management:write-pr-description"
    local pr_body_file
    pr_body_file="$(mktemp)"

    local pr_prompt
    pr_prompt="$(cat <<EOF
Use the /pr-management:write-pr-description skill to produce a pull request description for the commits on the current branch (compared to origin/main). The PR is implementing GitHub issue #${n}: "${title}".

Output ONLY the PR description markdown to stdout, with no preamble, no commentary, no trailing text. Do not open the PR — the calling script will do that. Do not commit anything.

The description should cover: what changed and why (not a diff restatement), the approach chosen, and any trade-offs or follow-ups worth noting.
EOF
)"

    if ! (cd "$worktree" && claude -p --permission-mode dontAsk --settings "$AGENT_SETTINGS" "$pr_prompt") >"$pr_body_file" 2>/dev/null; then
        log "WARN: PR description generation failed; falling back to minimal body"
        printf 'Implements #%s.\n' "$n" > "$pr_body_file"
    fi

    # Strip leading/trailing blank lines and append the issue-linking line.
    # This guarantees PR-to-issue auto-close works even if the skill output
    # omitted it.
    awk 'NF{p=1} p' "$pr_body_file" | awk 'BEGIN{RS=""; FS=""} {print}' > "${pr_body_file}.tmp" && mv "${pr_body_file}.tmp" "$pr_body_file"
    printf '\n\nCloses #%s\n' "$n" >> "$pr_body_file"

    # Generate the PR title following Conventional Commits. The title lands on
    # main as the squashed commit subject and is checked by the title-lint CI,
    # so using the raw issue title (which is not type-prefixed) would always
    # fail the check.
    log "Generating PR title (Conventional Commits)"
    local pr_title_file
    pr_title_file="$(mktemp)"

    local title_prompt
    title_prompt="$(cat <<EOF
Generate a single Conventional Commits PR title for the commits on the current branch (compared to origin/main). The PR implements GitHub issue #${n}: "${title}".

Requirements:
- Format: type(scope)?: subject  (scope optional)
- Allowed types are listed in CONTRIBUTING.md at the repo root — read it.
- Subject in lowercase, no trailing period, no issue number suffix.
- Hard cap at 72 characters for the whole header.

Output ONLY the title text, a single line, with no preamble or commentary.
EOF
)"

    local pr_title
    if ! (cd "$worktree" && claude -p --permission-mode dontAsk --settings "$AGENT_SETTINGS" "$title_prompt") >"$pr_title_file" 2>/dev/null; then
        log "WARN: PR title generation failed; falling back to chore-prefixed issue title"
        printf 'chore: %s\n' "$title" > "$pr_title_file"
    fi
    pr_title="$(tr -d '\r' < "$pr_title_file" | awk 'NF{print; exit}')"
    rm -f "$pr_title_file"
    [ -z "$pr_title" ] && pr_title="chore: ${title}"

    # Validate the generated title against the same conform config the
    # commit-msg hook uses. LLM output is stochastic; even a correct prompt
    # occasionally drifts (the agent may treat an issue title like
    # "osch list: ..." as already type-prefixed, or append "(#NN)" despite
    # being told not to). Falling back to a guaranteed-valid title is safer
    # than shipping one that fails the merge gate.
    local title_check_file
    title_check_file="$(mktemp)"
    printf '%s\n' "$pr_title" > "$title_check_file"
    if ! (cd "$worktree" && go tool conform enforce --commit-msg-file "$title_check_file") >/dev/null 2>&1; then
        log "WARN: generated title \"$pr_title\" failed conform; falling back to chore-prefixed issue title"
        pr_title="chore: ${title}"
    fi
    rm -f "$title_check_file"

    log "Opening PR for #${n}"
    if ! gh pr create \
            --head "$branch" \
            --base main \
            --title "$pr_title" \
            --body-file "$pr_body_file" \
            --label "$LABEL_AUTHORED" >/dev/null; then
        log "gh pr create failed for #${n}; marking failed"
        fail_issue "$n" "gh pr create failed for branch ${branch}"
        rm -f "$pr_body_file"
        return
    fi
    rm -f "$pr_body_file"

    gh issue edit "$n" --remove-label "$LABEL_PROGRESS" >/dev/null || true
    log "Done with #${n}. Worktree retained at ${worktree} (clean up manually after merge)."
}

# --- Main loop ---------------------------------------------------------------

log "Agent loop started. Polling every ${POLL_INTERVAL}s for label '${LABEL_QUEUE}'. Ctrl-C to stop."

while true; do
    issue_json="$(gh issue list \
        --label "$LABEL_QUEUE" \
        --state open \
        --limit 1 \
        --json number,title,body \
        | jq -c '.[0] // empty')"

    if [ -z "$issue_json" ]; then
        sleep "$POLL_INTERVAL"
        continue
    fi

    process_issue "$issue_json"
done
