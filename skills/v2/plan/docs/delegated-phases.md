# Early Phases — Reference Tables

This file provides reference tables for seed analysis dimension evaluation and perspective generation. These tables are used by:
- **Orchestrator**: Phase 0 intake (SKILL.md § Phase 0)
- **Seed-analyst**: Input analysis and perspective generation (`prompts/seed-analyst.md`)

> **Note**: Phase 0 and seed analysis are NOT delegated to setup-agent for plan-v2. The orchestrator handles intake directly, and seed-analyst runs as a team member. See `prompts/seed-analyst.md` for the seed-analyst prompt.

---

## Dimension Evaluation

Evaluate the planning context across 6 dimensions:

| Dimension | Values | Impact on Perspectives |
|-----------|--------|----------------------|
| Domain | product / technical / business / organizational | Maps to analysis domains |
| Complexity | single-system / cross-cutting / organizational | Simple: 3 perspectives. Complex: 5-6 |
| Risk profile | low / medium / high / critical | High risk → add risk-focused perspective |
| Stakeholder count | few / many / cross-org | Many → add stakeholder/change management perspective |
| Timeline | urgent / normal / long-term | Urgent → add feasibility/phasing perspective |
| Novelty | incremental / new capability / transformational | Novel → add innovation/research perspective |

---

## Perspective Generation Rules

Generate 3-6 orthogonal perspectives. Per perspective, define:

```
ID: {kebab-case-slug}
Name: {Human-readable perspective name}
Scope: {What this perspective examines}
Key Questions: [2-4 specific questions this perspective will answer]
Model: sonnet (standard) or opus (complex/cross-cutting)
Agent Type: architect-medium (sonnet) or architect (opus)
Rationale: {1-2 sentences: why THIS plan demands this perspective}
```

### Perspective Quality Gate

→ Apply `../shared/perspective-quality-gate.md` with `{DOMAIN}` = "plan", `{EVIDENCE_SOURCE}` = "Available input content".

---

## Input Type Detection

| Input Type | Detection | Action |
|-----------|-----------|--------|
| File path | Argument matches file path pattern (`.md`, `.txt`, `.doc`, etc.) | `Read` the file |
| URL | Argument contains `http://` or `https://` | `WebFetch` to retrieve content |
| Text prompt | Argument is plain text (not path, not URL) | Parse as requirements |
| No argument | Empty invocation during conversation | Summarize recent conversation context |
| Mixed | Combination of above | Process each, then merge |

## Planning Context Elements

| Element | Description | Required |
|---------|-------------|----------|
| **Goal** | What the plan aims to achieve | YES |
| **Scope** | Boundaries — what's in and out | YES |
| **Constraints** | Technical, timeline, budget, team limitations | YES |
| **Stakeholders** | Who is affected, who decides | NO (infer if absent) |
| **Success criteria** | How to measure plan success | NO (derive if absent) |
| **Existing context** | Prior decisions, dependencies, codebase state | NO |
