# Assignment Matrix

## Parameters

| Placeholder | Description | Examples |
|-------------|-------------|---------|
| `{DOMAIN}` | Analysis domain | `"prd"` / `"incident"` / `"plan"` |

---

## Step 1: Build Assignment Table

For each analyst (including Devil's Advocate), compile:

```
## Analysis Assignment Matrix ({DOMAIN})

| # | Analyst | Perspective | Ontologies | Analysis Angle |
|---|---------|-------------|------------|----------------|
| 1 | {analyst-slug} (sonnet) | {perspective-name} | doc: {path}, web: {url} | {specific description of how this perspective analyzes these docs} |
| 2 | {analyst-slug} (sonnet) | {perspective-name} | doc: {path} | {specific description} |
| 3 | {analyst-slug} (opus) | {perspective-name} | (none) | {analysis based on expertise, no reference docs} |
| DA | devil-advocate (opus) | Full Verification | (all available pool entries) | Cross-verify analyst coverage and identify missed evidence |
```

Analysis Angle MUST be specific to the perspective × ontology combination. Generic descriptions like "review documents" or "analyze for issues" are NOT acceptable.

## Step 2: Display and Verify

Output the table, then verify:

- [ ] Every analyst has exactly one perspective assigned
- [ ] Ontology assignments match the Scope Mapping output exactly
- [ ] Analysis Angle is specific (not generic)
- [ ] DA row present with full pool access
- [ ] No `unavailable` pool entries assigned to any analyst
- [ ] Ready to proceed to analyst spawn
