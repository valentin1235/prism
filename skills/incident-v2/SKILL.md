---
name: incident-v2
description: Multi-perspective agent team incident postmortem with ontology-scoped analysis
version: 1.0.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, AskUserQuestion, Read, Glob, Grep, Bash, WebFetch, WebSearch, mcp__ontology-docs__search_files, mcp__ontology-docs__read_file, mcp__ontology-docs__read_text_file, mcp__ontology-docs__read_multiple_files, mcp__ontology-docs__list_directory, mcp__ontology-docs__directory_tree
---

# Table of Contents

- [Archetype Index](#archetype-index)
- [Phase 0: Problem Intake](#phase-0-problem-intake)
- [Phase 0.5: Perspective Generation](#phase-05-perspective-generation)
- [Phase 0.6: Collect External References](#phase-06-collect-external-references)
- [Phase 0.7: Ontology Scope Mapping](#phase-07-ontology-scope-mapping)
- [Phase 1: Team Formation](#phase-1-team-formation)
- [Phase 2: Analysis Execution](#phase-2-analysis-execution)
- [Phase 2.5: Conditional Tribunal](#phase-25-conditional-tribunal)
- [Phase 3: Synthesis & Report](#phase-3-synthesis--report)
- [Phase 4: Cleanup](#phase-4-cleanup)

Prompt templates and report template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

## Prerequisite: Agent Team Mode (HARD GATE)

→ Read and execute `../shared/prerequisite-gate.md`. Set `{PROCEED_TO}` = "Phase 0".

## Archetype Index

### Core Archetypes

| ID | Lens | Model | Agent Type |
|----|------|-------|------------|
| `timeline` | Timeline | Sonnet | `architect-medium` |
| `root-cause` | Root Cause | Opus | `architect` |
| `systems` | Systems & Architecture | Opus | `architect` |
| `impact` | Impact | Sonnet | `architect-medium` |

### Extended Archetypes

| ID | Lens | Model | Agent Type | When |
|----|------|-------|------------|------|
| `security` | Security & Threat | Opus | `architect` | Breaches, data leaks, compliance |
| `data-integrity` | Data Integrity | Opus | `architect` | Corruption, replication failures |
| `performance` | Performance & Capacity | Sonnet | `architect-medium` | Latency, resource exhaustion |
| `deployment` | Deployment & Change | Sonnet | `architect-medium` | Post-deploy failures, config drift |
| `network` | Network & Connectivity | Sonnet | `architect-medium` | Partitions, DNS, LB issues |
| `concurrency` | Concurrency & Race | Opus | `architect` | Race conditions, deadlocks |
| `dependency` | External Dependency | Sonnet | `architect-medium` | Third-party failures |
| `ux` | User Experience | Sonnet | `architect-medium` | User-facing degradation, error UX, journey disruption |
| `custom` | Custom | Auto | Auto | Novel failure modes |

Team size: 3 min (DA + 2) — 6 max (DA + 5). Devil's Advocate is ALWAYS present.

---

## Phase 0: Problem Intake

MUST complete ALL steps. Skipping intake → unfocused analysis.

### Step 0.1: Collect Incident

`AskUserQuestion`:
- "What is the incident? Describe symptoms, affected systems, and business impact."
- Header: "Incident"
- Options: "I'll describe it now" / "I have logs/links" / "It's in a document/ticket"

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
| SEV1 OR Active | **FAST TRACK**: Deploy Timeline + Root Cause + Systems + Impact + DA immediately → Phase 1 |
| Otherwise | **PERSPECTIVE TRACK**: Continue below |

### Perspective Track

**0.5.1** Select 3-5 archetypes using Seed Analysis mapping + Archetype Index.

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

## Phase 0.6: Collect External References

`AskUserQuestion`:
```
question: "분석에 참고할 외부 링크(URL)가 있나요? 장애 관련 문서, 모니터링 대시보드, 이슈 트래커 링크 등을 온톨로지 풀에 추가할 수 있습니다."
header: "External References"
options:
  - label: "링크 추가"
    description: "참고할 URL을 입력합니다"
  - label: "없음 — 바로 진행"
    description: "ontology-docs MCP 문서만으로 진행합니다"
```

If user selects "링크 추가":
1. Collect URLs from user input (comma or newline separated)
2. Store as `{WEB_LINKS}` list (e.g., `["https://...", "https://..."]`)
3. Ask again: "더 추가할 링크가 있나요?" — repeat until user says no

If user selects "없음 — 바로 진행":
- Set `{WEB_LINKS}` = `[]`

## Phase 0.7: Ontology Scope Mapping

→ Read and execute `../shared/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = `optional`
- `{UNMAPPED_POLICY}` = `allowed`
- `{WEB_LINKS}` = (collected from Phase 0.6, default `[]`)

If `ONTOLOGY_AVAILABLE=false` → skip to Phase 1. All analysts get `{ONTOLOGY_SCOPE}` = "N/A — ontology-docs not available".

---

## Phase 1: Team Formation

### Step 1.1

```
TeamCreate(team_name: "incident-analysis-{short-id}", description: "Incident: {summary}")
```

### Step 1.2

Create tasks: one per perspective + DA + Synthesis.

### Step 1.3: Spawn ALL Teammates in Parallel

MUST read prompt files before spawning. Files are relative to this SKILL.md's directory.

| Agent | Prompt File | Section |
|-------|-------------|---------|
| Devil's Advocate (ALWAYS) | `prompts/devil-advocate.md` + `shared/da-evaluation-protocol.md` | full file + inline protocol |
| Timeline | `prompts/core-archetypes.md` | § Timeline Lens |
| Root Cause | `prompts/core-archetypes.md` | § Root Cause Lens |
| Systems & Architecture | `prompts/core-archetypes.md` | § Systems Lens |
| Impact | `prompts/core-archetypes.md` | § Impact Lens |
| Security / Data Integrity / Performance | `prompts/extended-archetypes.md` | § respective section |
| Deployment / Network / Concurrency / Dependency / UX | `prompts/extended-archetypes.md` | § respective section |
| Custom | `prompts/extended-archetypes.md` | § Custom Lens |

**Spawn pattern:**
```
Task(
  subagent_type="oh-my-claudecode:{agent_type}",
  name="{archetype-id}-analyst",
  team_name="incident-analysis-{id}",
  model="{model}",
  prompt="{prompt from file with {INCIDENT_CONTEXT} and {ONTOLOGY_SCOPE} replaced}"
)
```

→ Apply worker preamble from `../shared/worker-preamble.md` to each analyst prompt with:
- `{TEAM_NAME}` = `"incident-analysis-{short-id}"`
- `{WORKER_NAME}` = `"{archetype-id}-analyst"`
- `{WORK_ACTION}` = `"Investigate the incident from your assigned perspective. Answer ALL key questions with evidence and code references."`

MUST replace `{INCIDENT_CONTEXT}` in every prompt with actual Phase 0 details.
MUST replace `{ONTOLOGY_SCOPE}` with the **perspective-specific scoped reference** from Phase 0.7 — NOT a generic reference. DA gets the full-scope reference.

---

## Phase 2: Analysis Execution

### Step 2.1: Monitor & Coordinate

Monitor via `TaskList`. Forward findings between analysts. Unblock stuck analysts.

### Step 2.2: Clarity Enforcement

→ Apply `../shared/clarity-enforcement.md` with `{EVIDENCE_FORMAT}` = `"file:function:line"`.

### Step 2.3: Cross-Perspective Validation

| Signal | Action |
|--------|--------|
| Convergence | Note for synthesis — strengthens confidence |
| Divergence | Route to DA + analysts for resolution |
| Blind spot | Targeted follow-up or spawn specialist |

### Step 2.4: DA Challenge-Response Loop

The DA evaluates analyst findings using the evaluation protocol (`shared/da-evaluation-protocol.md`). The orchestrator mediates a multi-round loop:

**Round 1:**
1. DA receives analyst findings and produces Fallacy Check Results (per-claim verdicts with severity)
2. Orchestrator extracts FAIL items (BLOCKING and MAJOR)
3. Orchestrator forwards each FAIL item to the responsible analyst via `SendMessage`, including the fallacy name and explanation
4. Analysts respond with corrected reasoning, additional evidence, or acknowledged limitations

**Round N (if needed):**
5. Orchestrator forwards analyst responses to DA for re-evaluation
6. DA marks each item: RESOLVED / PARTIALLY RESOLVED / UNRESOLVED
7. If UNRESOLVED items remain → repeat from step 3

**Termination:**

| DA Aggregate Verdict | Condition | Action |
|---------------------|-----------|--------|
| SUFFICIENT | Zero BLOCKING + all MAJOR resolved or acknowledged | → Proceed to Phase 2 Exit Gate |
| NOT SUFFICIENT | BLOCKING items remain | → Continue loop (next round) |
| NEEDS TRIBUNAL | BLOCKING persists after 2 rounds | → Proceed to Phase 2.5 |

Orchestrator tracks round count. Maximum 2 challenge-response rounds before escalation.

### Phase 2 Exit Gate

MUST NOT proceed until ALL verified:

- [ ] All perspective key questions answered
- [ ] No unexplained timeline gaps
- [ ] ≥1 root cause hypothesis with strong evidence + code references
- [ ] DA Aggregate Verdict is SUFFICIENT (zero BLOCKING, all MAJOR resolved or acknowledged)
- [ ] All cross-perspective discrepancies resolved
- [ ] Impact quantified with actual data

If DA Aggregate Verdict is NEEDS TRIBUNAL → skip Exit Gate, proceed directly to Phase 2.5.
If ANY other item fails → create follow-up tasks, continue Phase 2. Error: "Cannot synthesize: {item} not satisfied."

---

## Phase 2.5: Conditional Tribunal

### Trigger (ANY one)

1. DA marks "NEEDS TRIBUNAL"
2. ≥2 unresolved cross-perspective contradictions
3. User requests tribunal

If NONE → announce "DA: analysis sufficient. Proceeding to report." → skip to Phase 3.

### Execution

1. Compile findings package (~10-15K tokens): summary, key findings, recommendations, DA Fallacy Check Results, contradictions, trigger reason
2. Shut down completed analysts (keep DA)
3. Read `prompts/tribunal.md` for critic prompts
3.5. Replace placeholders in tribunal prompts:
   - `{FINDINGS_PACKAGE}` → compiled findings from step 1
   - `{TRIGGER_REASON}` → specific tribunal trigger condition from Trigger check
   - `{INCIDENT_CONTEXT}` → Phase 0 details
4. Spawn UX Critic (Sonnet) + Engineering Critic (Opus) in parallel
5. Collect independent reviews
6. Consensus round:

| Level | Condition | Label |
|-------|-----------|-------|
| Strong | 2/2 APPROVE | `[Unanimous]` |
| Caveat | 1 APPROVE, 1 CONDITIONAL | `[Approved w/caveat]` |
| Split | 1+ REJECT | `[No consensus]` → user decision |

Split → share rationale, 1 final round only. Still split → present to user.

7. Compile verdict, shut down critics, proceed to Phase 3.

---

## Phase 3: Synthesis & Report

### Step 3.1

Integrate all analyst findings.

### Step 3.2

Read `templates/report.md` and fill all sections with synthesized findings.

### Step 3.3

`AskUserQuestion`:
- "Is the analysis complete?"
- Options: "Complete" / "Need deeper investigation" / "Request Tribunal" / "Add recommendations" / "Share with team"

Deeper investigation → Phase 2. Tribunal → Phase 2.5.

---

## Phase 4: Cleanup

→ Execute `../shared/team-teardown.md`.

---

## Gate Summary

```
Phase 0 ──[5-item gate]──→ Phase 0.5 ──→ Phase 0.6 ──→ Phase 0.7 ──[exit gate]──→ Phase 1 ──→ Phase 2 ──[6-item gate]──→ Phase 2.5? ──→ Phase 3 ──→ Phase 4
                                                        ↓ (ONTOLOGY_AVAILABLE=false)
                                                        └──→ Phase 1 (skip 0.7)
```

Every gate specifies exact missing items. Fix before proceeding.
