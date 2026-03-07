# Perspective Quality Gate

Quality checklist that each selected perspective must pass before being locked.

## Parameters

| Placeholder | Description | Examples |
|-------------|-------------|---------|
| `{DOMAIN}` | Analysis domain | `"incident"` / `"plan"` |
| `{EVIDENCE_SOURCE}` | Evidence basis expression | `"Available evidence"` / `"Available input content"` |

---

## Quality Gate

Each selected perspective MUST pass ALL:
- [ ] **Orthogonal**: Does NOT overlap analysis scope with other selected perspectives
- [ ] **Evidence-backed**: {EVIDENCE_SOURCE} can answer this perspective's key questions
- [ ] **{DOMAIN}-specific**: Selected because THIS {DOMAIN} demands it, not "generically useful"
- [ ] **Actionable**: Will produce concrete recommendations/plan elements, not just observations

If a perspective fails any check → replace with a better-fitting option or drop it.
