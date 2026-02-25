# Team Teardown

Cleanup procedure after team work is complete.

---

## Steps

1. `SendMessage(type: "shutdown_request")` to all active teammates
2. Await `shutdown_response(approve=true)` from each
3. `TeamDelete`
