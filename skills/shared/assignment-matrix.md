# Assignment Matrix

Display a comprehensive assignment table showing each analyst, their perspective, selected ontologies, and analysis angle before analysis begins. This provides visibility into the analysis plan and an opportunity to catch misassignments.

## Prerequisites

- Locked perspectives (from Perspective Quality Gate)
- Per-perspective ontology mappings (from Ontology Scope Mapping Phase B)
- Analyst agent assignments (from skill's team formation — same phase that calls this module)

## Parameters

| Placeholder | Description | Examples |
|-------------|-------------|---------|
| `{DOMAIN}` | Analysis domain | `"prd"` / `"incident"` / `"plan"` |

---

## Step 1: Build Assignment Table

For each analyst (including Devil's Advocate), compile from upstream outputs:

| Field | Source | Description |
|-------|--------|-------------|
| **Analyst** | Team formation | Agent name/slug and model tier |
| **Perspective** | Quality Gate | Perspective name |
| **Ontologies** | Scope Mapping | Selected pool entries with source type indicator |
| **Analysis Angle** | Derived | 1-line: what this perspective examines in these ontologies |

**Analysis Angle derivation rule:**
The angle MUST be specific to the perspective × ontology combination. Generic descriptions like "review documents" or "analyze for issues" are NOT acceptable. It must state the concrete analytical lens applied to the specific documents.

## Step 2: Display Matrix

Output the assignment table:

```
## Analysis Assignment Matrix ({DOMAIN})

| # | Analyst | Perspective | Ontologies | Analysis Angle |
|---|---------|-------------|------------|----------------|
| 1 | {analyst-slug} (sonnet) | {perspective-name} | doc: {path}, web: {url} | {specific description of how this perspective analyzes these docs} |
| 2 | {analyst-slug} (sonnet) | {perspective-name} | doc: {path} | {specific description} |
| 3 | {analyst-slug} (opus) | {perspective-name} | (none) | {analysis based on expertise, no reference docs} |
| DA | devil-advocate (opus) | Full Verification | (all available pool entries) | Cross-verify analyst coverage and identify missed evidence |
```

## Step 3: Validation

Before proceeding to analysis, verify:

- [ ] Every analyst has exactly one perspective assigned
- [ ] Ontology assignments match the Scope Mapping output exactly
- [ ] Analysis Angle is specific to the perspective x ontology combination (not generic)
- [ ] DA row is present with full pool access
- [ ] No `unavailable` pool entries assigned to any analyst

## Exit Gate

- [ ] Assignment Matrix displayed
- [ ] All validation checks passed
- [ ] Ready to proceed to analyst spawn
