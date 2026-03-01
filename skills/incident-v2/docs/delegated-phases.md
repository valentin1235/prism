# Delegated Phases — Original Steps Reference

These phases are delegated to the setup agent (`../shared/setup-agent.md`). This file preserves the original step definitions for reference.

---

## Phase 0: Problem Intake

MUST complete ALL steps. Skipping intake → unfocused analysis.

### Step 0.1: Collect Incident

If the user provided an incident description via `$ARGUMENTS`, use it directly and skip to Step 0.2.

Otherwise, ask the user to describe the incident:

"Please describe the incident:
- What symptoms are you observing?
- Which systems are affected?
- What is the business impact?"

After receiving the user's description, proceed IMMEDIATELY to Step 0.2 — do NOT stop or wait for additional input.

### Step 0.2: Severity & Context

`AskUserQuestion` (3 questions):

1. Severity → "SEV1 — Full outage" / "SEV2 — Partial degradation" / "SEV3 — Limited impact" / "SEV4 — Minor"
2. Status → "Active — Ongoing" / "Mitigated — Temp fix" / "Resolved — Postmortem" / "Recurring — Patterns"
3. Evidence (multiSelect) → "Logs & errors" / "Metrics/dashboards" / "Code changes" / "All of the above"

### Step 0.3: Gather Evidence

Collect: error messages, stack traces, logs, event timeline, recent deploys, affected services/endpoints/regions, monitoring data, initial hypotheses.

### Phase 0 Exit Gate

MUST NOT proceed until ALL documented:

- [ ] What happened (symptoms)
- [ ] When (timeline)
- [ ] What's affected (blast radius)
- [ ] What's been tried (mitigation)
- [ ] What evidence exists (logs, metrics, code)

If ANY missing → ask user. Error: "Cannot proceed: missing {item}."

Summarize and confirm with user before continuing.

### Step 0.4: Seed Analysis (Internal — not shown to user)

Evaluate the incident across 5 dimensions, then map to archetype candidates:

| Dimension | Evaluate | Impact on Selection |
|-----------|----------|-------------------|
| Domain | infra / app / data / security / network | Maps to archetype categories |
| Failure type | crash / degradation / data loss / breach / misconfig | Determines analytical frameworks |
| Evidence available | logs / metrics / code diffs / traces | MUST NOT select perspectives without evidence |
| Complexity | single-cause / multi-factor | Simple: 2-3 perspectives. Complex: 4-5 |
| Recurrence | first-time / recurring | Recurring → add `systems` for pattern analysis |

#### Characteristic → Archetype Mapping

| Incident Characteristics | Recommended Archetypes |
|-------------------------|----------------------|
| Security breach, unauthorized access | `security` + `timeline` + `systems` |
| Data corruption, stale reads, replication lag | `data-integrity` + `root-cause` + `systems` |
| Latency spike, OOM, resource exhaustion | `performance` + `root-cause` + `systems` |
| Post-deployment failure, config drift | `deployment` + `timeline` + `root-cause` |
| Network partition, DNS failure, LB issue | `network` + `systems` + `timeline` |
| Race condition, deadlock, distributed lock | `concurrency` + `root-cause` + `systems` |
| Third-party API failure, upstream outage | `dependency` + `impact` + `timeline` |
| User-facing degradation, confusing errors, UX breakage | `ux` + `impact` + `root-cause` |
| Novel / unclassifiable | `custom` + `root-cause` + relevant core |

Use this mapping as starting point, then refine based on specific evidence.

---

## Phase 0.5: Perspective Generation

### Track Selection

| Condition | Track |
|-----------|-------|
| SEV1 OR Active | **FAST TRACK**: Lock 4 core archetypes (Timeline + Root Cause + Systems + Impact) + DA. (Note: Step 0.5.5 session ID generation is handled by orchestrator before setup agent spawn.) Then proceed to Phase 0.6 (ontology mapping runs normally). If urgency demands skipping Phase 0.6, set `{ONTOLOGY_SCOPE}` = "N/A — Fast Track, ontology mapping deferred." Then proceed to Phase 1. DA is created with `blockedBy` on all analysts — "immediately" means tasks are created together, DA executes after analysts complete. |
| Otherwise | **PERSPECTIVE TRACK**: Continue below |

### Perspective Track

**0.5.1** Select 3-5 archetypes using Seed Analysis mapping + Archetype Index. If selected archetypes exceed 5, reduce to 5 — DA is always additional (max team = DA + 5 = 6). Inform user of the cap.

Per selected archetype, document:
- **Lens Name**: From Archetype Index
- **Why this perspective**: 1-2 sentences explaining why THIS incident demands it
- **Key Questions**: 2-3 specific questions this lens will answer
- **Model**: From Archetype Index

Selection rules:
- MUST include ≥1 Core Archetype
- MUST NOT select perspectives without supporting evidence
- Fewer targeted > broad coverage

#### Perspective Quality Gate

→ Apply `../shared/perspective-quality-gate.md` with `{DOMAIN}` = "incident", `{EVIDENCE_SOURCE}` = "Available evidence".

**0.5.2** Present via `AskUserQuestion`:
- "I recommend these perspectives. How to proceed?"
- Options: "Proceed" / "Add perspective" / "Remove perspective" / "Modify perspective"

**0.5.3** Iterate until approved. Warn if <2 dynamic perspectives.

**0.5.4** Lock roster: archetype, model, key questions, rationale per perspective.

---

## Phase 0.6: Ontology Scope Mapping

→ Read and execute `../shared/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"incident analysis"`
- `{STATE_DIR}` = `.omc/state/incident-{short-id}`

If `ONTOLOGY_AVAILABLE=false` → skip to Phase 1. All analysts get `{ONTOLOGY_SCOPE}` = the following block:

```
No ontology-docs available for this analysis.
DO NOT call any mcp__ontology-docs__* tools — they will fail or timeout.
Analyze using available evidence only (logs, code, metrics, stack traces).
```

#### Phase 0.6 Exit Gate

Shared module exit gate applies. No additional incident-specific checks required.
