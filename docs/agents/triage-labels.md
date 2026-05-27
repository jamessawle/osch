# Triage Labels

The skills speak in terms of five canonical triage roles. This file maps those roles to the actual label strings used in this repo's issue tracker.

| Label in mattpocock/skills | Label in our tracker | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | Requires human implementation            |
| `wontfix`                  | `wontfix`            | Will not be actioned                     |

Category roles map 1:1 to the existing GitHub defaults: `bug` and `enhancement`.

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), use the corresponding label string from this table.

## Relationship to the agent loop

`ready-for-agent` is the entry gate for `scripts/agent-loop/loop.sh` — the loop polls for it, then manages its own runtime state in `agent:in-progress`, `agent:failed`, and `agent:authored`. Those `agent:*` labels are loop runtime states, **not** triage roles; don't apply them during triage.

Edit the right-hand column to match whatever vocabulary you actually use.
