# Delegated Phases — Original Steps Reference

These phases are delegated to the setup agent (`../shared/setup-agent.md`). This file preserves the original step definitions for reference.

---

## Phase 0: Input Analysis & Context Gathering

MUST complete ALL steps. Skipping intake → unfocused analysis, wasted committee time.

### Step 0.1: Detect Input Type

Examine the skill invocation argument(s):

| Input Type | Detection | Action |
|-----------|-----------|--------|
| File path | Argument matches file path pattern (`.md`, `.txt`, `.doc`, etc.) | `Read` the file |
| URL | Argument contains `http://` or `https://` | `WebFetch` to retrieve content |
| Text prompt | Argument is plain text (not path, not URL) | Parse as requirements |
| No argument | Empty invocation during conversation | Summarize recent conversation context |
| Mixed | Combination of above | Process each, then merge |

If file not found → error: `"Input file not found: {path}"`. Ask user to provide valid path.

**Hell Mode**: If argument contains `--hell` or `hell` → activate Hell Mode (unanimous consensus required, no iteration limit). Announce: "Hell Mode activated — committee MUST reach 3/3 unanimous consensus."

### Step 0.2: Language Detection

Detect the primary language of the input content.

| Input Language | Report Language |
|---------------|----------------|
| Korean | Korean (한글) |
| English | English |
| Mixed | Follow majority language |
| Ambiguous | `AskUserQuestion` to confirm |

Lock report language for all subsequent phases.

### Step 0.3: Extract Planning Context

Parse input to extract:

| Element | Description | Required |
|---------|-------------|----------|
| **Goal** | What the plan aims to achieve | YES |
| **Scope** | Boundaries — what's in and out | YES |
| **Constraints** | Technical, timeline, budget, team limitations | YES |
| **Stakeholders** | Who is affected, who decides | NO (infer if absent) |
| **Success criteria** | How to measure plan success | NO (derive if absent) |
| **Existing context** | Prior decisions, dependencies, codebase state | NO |

### Step 0.4: Fill Gaps via User Interview

If ANY required element is missing, use `AskUserQuestion` per element (header: element name, options: "{inferred value}" / "Not applicable"). Users can select "Other" to provide a custom value. After each answer, IMMEDIATELY proceed to the next missing element or exit gate. Maximum 3 rounds.

### Phase 0 Exit Gate

MUST NOT proceed until ALL documented:

- [ ] Goal clearly stated (1-2 sentences, no ambiguity)
- [ ] Scope defined (explicit in/out boundaries)
- [ ] Constraints identified (at minimum: timeline, technical)
- [ ] Input language detected → report language locked
- [ ] Raw input preserved for analyst reference

If ANY missing → ask user. Error: "Cannot proceed: missing {item}."

Summarize extracted context and confirm with user before continuing.

---

## Phase 1: Dynamic Perspective Generation

### Step 1.1: Seed Analysis (Internal)

Evaluate the planning context across dimensions:

| Dimension | Evaluate | Impact on Perspectives |
|-----------|----------|----------------------|
| Domain | product / technical / business / organizational | Maps to analysis domains |
| Complexity | single-system / cross-cutting / organizational | Simple: 3 perspectives. Complex: 5-6 |
| Risk profile | low / medium / high / critical | High risk → add risk-focused perspective |
| Stakeholder count | few / many / cross-org | Many → add stakeholder/change management perspective |
| Timeline | urgent / normal / long-term | Urgent → add feasibility/phasing perspective |
| Novelty | incremental / new capability / transformational | Novel → add innovation/research perspective |

### Step 1.2: Generate Perspectives

Generate 3-6 orthogonal perspectives. Per perspective, define:

```
ID: {kebab-case-slug}
Name: {Human-readable perspective name}
Scope: {What this perspective examines}
Key Questions: [2-4 specific questions this perspective will answer]
Model: sonnet (standard) or opus (complex/cross-cutting)
Agent Type: architect-medium (sonnet) or analyst (opus)
Rationale: {1-2 sentences: why THIS plan demands this perspective}
```

#### Perspective Quality Gate

→ Apply `../shared/perspective-quality-gate.md` with `{DOMAIN}` = "plan", `{EVIDENCE_SOURCE}` = "Available input content".

### Step 1.3: Present to User

`AskUserQuestion` (header: "Perspectives", question: "I recommend these {N} perspectives for analysis. How to proceed?", options: "Proceed" / "Add perspective" / "Remove perspective" / "Modify perspective")

### Step 1.4: Iterate Until Approved

Repeat 1.3 until user selects "Proceed". Warn if <3 perspectives: "Fewer than 3 perspectives may produce a shallow plan. Continue anyway?"

### Step 1.6: Ontology Scope Mapping

→ Read and execute `../shared/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"analysis"`
- `{STATE_DIR}` = `.omc/state/plan-{short-id}`

If `ONTOLOGY_AVAILABLE=false` → analysts get `{ONTOLOGY_SCOPE}` = "N/A — ontology-docs not available".

Catalog MUST show all source types (mcp, web, file) if present. (Catalog persistence handled by shared module.)
