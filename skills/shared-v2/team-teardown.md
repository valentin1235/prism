# Team Teardown

Cleanup procedure after team work is complete.

---

## Steps

1. Enumerate active teammates via `TaskList` (filter for non-completed tasks) or read team config at `~/.claude/teams/{team-name}/config.json`
2. `SendMessage(type: "shutdown_request")` to each active teammate by name
3. Await `shutdown_response(approve=true)` from each
4. `TeamDelete`
