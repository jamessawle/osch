#!/usr/bin/env bash
# scripts/agent-loop/loop.sh
#
# Personal development automation: poll GitHub for issues labelled
# `ready-for-agent`, dispatch each to the brigade engineer Chef, and label
# the outcome on the issue.
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
require_tool jq

gh auth status >/dev/null 2>&1 || die "gh is not authenticated. Run 'gh auth login'."

# --- Issue processing --------------------------------------------------------

process_issue() {
    local issue_json="$1"
    local n title body

    n=$(echo "$issue_json" | jq -r '.number')
    title=$(echo "$issue_json" | jq -r '.title')
    body=$(echo "$issue_json" | jq -r '.body // ""')

    log "Claiming issue #${n}: ${title}"
    if ! gh issue edit "$n" \
            --remove-label "$LABEL_QUEUE" \
            --add-label "$LABEL_PROGRESS" >/dev/null; then
        log "WARN: failed to claim #${n} (label race?). Skipping."
        return
    fi

    # The authoritative spec lives in the "## Agent Brief" comment (added by
    # /triage); the body is supporting context. Older issues with no brief
    # fall back to body alone.
    local brief
    brief="$(fetch_agent_brief "$n")"

    # Build the Chit JSON
    local chit
    chit=$(jq -n \
        --arg kind "implement" \
        --arg source "github" \
        --arg id "$n" \
        --arg title "$title" \
        --arg description "$body" \
        --arg spec "$brief" \
        --arg repo "$REPO_ROOT" \
        '{
            kind: $kind,
            task: { ref: { source: $source, id: $id }, title: $title, description: $description, specification: $spec },
            repo: { path: $repo }
        }')

    # Dispatch — stderr passes through; stdout is the Proof.
    local proof chef_exit=0
    proof=$(printf '%s' "$chit" | go run ./scripts/agent-loop/cmd/brigade chef engineer) || chef_exit=$?

    if [ "$chef_exit" -ne 0 ]; then
        log "CHEF CRASHED for issue $n (exit $chef_exit)"
        gh issue edit "$n" --add-label "$LABEL_FAILED" --remove-label "$LABEL_PROGRESS" >/dev/null || true
        return
    fi

    local status
    status=$(printf '%s' "$proof" | jq -r '.status')
    case "$status" in
        ok)
            local url
            url=$(printf '%s' "$proof" | jq -r '.pr.url')
            log "PR created for issue $n: $url"
            gh issue edit "$n" --remove-label "$LABEL_PROGRESS" >/dev/null || true
            ;;
        failed)
            local msg
            msg=$(printf '%s' "$proof" | jq -r '.message')
            log "Chit failed for issue $n: $msg"
            gh issue edit "$n" --add-label "$LABEL_FAILED" --remove-label "$LABEL_PROGRESS" >/dev/null || true
            ;;
        *)
            log "Unparseable Proof for issue $n"
            gh issue edit "$n" --add-label "$LABEL_FAILED" --remove-label "$LABEL_PROGRESS" >/dev/null || true
            ;;
    esac
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
