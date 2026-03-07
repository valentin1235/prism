---
name: incident-v2
description: Multi-perspective agent team incident postmortem with ontology-scoped analysis
version: 1.0.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, Read, Glob, Grep, Bash, Write, WebFetch, WebSearch, ToolSearch, ListMcpResourcesTool, mcp__ontology-docs__list_allowed_directories, mcp__ontology-docs__search_files, mcp__ontology-docs__read_file, mcp__ontology-docs__read_text_file, mcp__ontology-docs__read_multiple_files, mcp__ontology-docs__list_directory, mcp__ontology-docs__directory_tree
---

# Table of Contents

- [Archetype Index](#archetype-index)
- [Phase 0: Problem Intake](#phase-0-problem-intake)
- [Phase 0.5: Team Creation & Seed Analysis](#phase-05-team-creation--seed-analysis)
- [Phase 0.6: Perspective Approval](#phase-06-perspective-approval)
- [Phase 0.7: Ontology Scope Mapping](#phase-07-ontology-scope-mapping)
- [Phase 0.8: Context & State Files](#phase-08-context--state-files)
- [Phase 1: Analyst Task Creation & Spawn](#phase-1-analyst-task-creation--spawn)
- [Phase 2: Analysis Execution](#phase-2-analysis-execution)
- [Phase 2.5: Conditional Tribunal](#phase-25-conditional-tribunal)
- [Phase 3: Synthesis & Report](#phase-3-synthesis--report)
- [Phase 4: Cleanup](#phase-4-cleanup)

Prompt templates and report template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

## Artifact Persistence

MUST persist phase outputs to `.omc/state/incident-{short-id}/` (created in Phase 0, Step 0.4). On deeper investigation re-entry, agents MUST `Read` artifact files — do NOT rely solely on prompt context.

| File | Written | Read By |
|------|---------|---------|
| `perspectives.md` | Orchestrator (Phase 0.6) | All agents |
| `context.md` | Orchestrator (Phase 0.8) | All agents |
| `analyst-findings.md` | Phase 2 exit | DA |
| `da-evaluation.md` | Phase 2.4 exit (DA) | Phase 3 synthesis |
| `prior-iterations.md` | Each re-entry (append) | DA (cumulative history) |
| `ontology-catalog.md` | Orchestrator (Phase 0.7) | All agents |
| `ontology-scope-analyst.md` | Orchestrator (Phase 0.7) | All analysts |
| `ontology-scope-da.md` | Orchestrator (Phase 0.7) | DA |

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

Orchestrator handles intake directly — NOT delegated. Reference steps: `docs/delegated-phases.md` § Phase 0.

### Step 0.1: Collect Incident

If the user provided an incident description via `$ARGUMENTS`, use it directly. Otherwise, ask via `AskUserQuestion` (header: "Incident"): "Please describe the incident: What symptoms? Which systems affected? Business impact?"

### Step 0.2: Severity & Context

`AskUserQuestion` (3 questions):

1. Severity → "SEV1 — Full outage" / "SEV2 — Partial degradation" / "SEV3 — Limited impact" / "SEV4 — Minor"
2. Status → "Active — Ongoing" / "Mitigated — Temp fix" / "Resolved — Postmortem" / "Recurring — Patterns"
3. Evidence (multiSelect) → "Logs & errors" / "Metrics/dashboards" / "Code changes" / "All of the above"

### Step 0.3: Fast Track Check

| Condition | Track | Action |
|-----------|-------|--------|
| SEV1 OR Active | **FAST_TRACK** | Lock 4 core archetypes (Timeline + Root Cause + Systems + Impact) + DA. Skip Phase 0.5 seed-analyst and Phase 0.6 perspective approval. |
| Otherwise | **PERSPECTIVE_TRACK** | Continue with seed-analyst for dynamic perspective generation. |

### Step 0.4: Generate Session ID and State Directory

Generate `{short-id}`: `Bash(uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8)`. Generate ONCE and reuse throughout all phases.

Create state directory: `Bash(mkdir -p .omc/state/incident-{short-id})`

### Phase 0 Exit Gate

MUST NOT proceed until ALL documented:

- [ ] Incident description collected (symptoms, affected systems, impact)
- [ ] Severity and status determined
- [ ] Evidence types identified
- [ ] Track determined (FAST_TRACK or PERSPECTIVE_TRACK)
- [ ] `{short-id}` generated and state directory created

---

## Phase 0.5: Team Creation & Seed Analysis

### Step 0.5.1: Create Team

```
TeamCreate(team_name: "incident-analysis-{short-id}", description: "Incident: {summary}")
```

### Step 0.5.2: Spawn Seed Analyst

**Skip on FAST_TRACK** — proceed directly to Phase 0.6 with locked 4-core roster.

Read `prompts/seed-analyst.md` (relative to this SKILL.md).

Create seed-analyst task via `TaskCreate`, pre-assign owner via `TaskUpdate(owner="seed-analyst")`, then spawn:

```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="seed-analyst",
  team_name="incident-analysis-{short-id}",
  model="opus",
  run_in_background=true,
  prompt="{worker preamble + seed-analyst prompt with placeholders replaced}"
)
```

→ Apply worker preamble from `../shared/worker-preamble.md` with:
- `{TEAM_NAME}` = `"incident-analysis-{short-id}"`
- `{WORKER_NAME}` = `"seed-analyst"`
- `{WORK_ACTION}` = `"Actively investigate the incident using available tools (Grep, Read, Bash, MCP). Evaluate incident dimensions, map to archetype candidates, and generate perspective recommendations. Report findings via SendMessage."`

Placeholder replacements in seed-analyst prompt:
- `{INCIDENT_DESCRIPTION}` → Phase 0 incident description
- `{SEVERITY}` → Phase 0 severity
- `{STATUS}` → Phase 0 status
- `{EVIDENCE_TYPES}` → Phase 0 evidence types

### Step 0.5.3: Receive Seed Analyst Results

Wait for seed-analyst to send results via `SendMessage`. The message contains:
- **Research Summary**: evidence discovered, files examined, MCP queries, recent changes
- **Dimension Evaluation**: domain, failure type, evidence, complexity, recurrence
- **Perspectives**: 3-5 perspective candidates with ID, Name, Scope, Key Questions, Model, Agent Type, Rationale

### Step 0.5.4: Shutdown Seed Analyst

After receiving results: `SendMessage(type: "shutdown_request", recipient: "seed-analyst", content: "Seed analysis complete.")`.

### Phase 0.5 Exit Gate

MUST NOT proceed until:

- [ ] Team created
- [ ] Seed-analyst results received (or FAST_TRACK with 4-core roster locked)
- [ ] Seed-analyst shut down (or FAST_TRACK)

---

## Phase 0.6: Perspective Approval

**Skip on FAST_TRACK** — perspectives already locked. Write locked roster to `perspectives.md` and proceed.

### Step 0.6.1: Present Perspectives

`AskUserQuestion` (header: "Perspectives", question: "I recommend these {N} perspectives for analysis. How to proceed?", options: "Proceed" / "Add perspective" / "Remove perspective" / "Modify perspective")

Include seed-analyst's research summary for user context.

### Step 0.6.2: Iterate Until Approved

Repeat until user selects "Proceed". Warn if <2 dynamic perspectives.

### Step 0.6.3: Write Perspectives

Write locked roster to `.omc/state/incident-{short-id}/perspectives.md`:

```markdown
# Perspectives — Locked Roster

## Severity
{SEV1-4}

## Status
{Active/Mitigated/Resolved/Recurring}

## Track
{FAST_TRACK / PERSPECTIVE_TRACK}

## Perspectives

### {perspective-id}
- **Name:** {name}
- **Scope:** {scope}
- **Key Questions:**
  1. {question}
  2. {question}
- **Model:** {model}
- **Agent Type:** {agent type}
- **Rationale:** {rationale}
```

---

## Phase 0.7: Ontology Scope Mapping

→ Read and execute `../shared/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"incident analysis"`
- `{STATE_DIR}` = `.omc/state/incident-{short-id}`

If `ONTOLOGY_AVAILABLE=false` → all analysts get `{ONTOLOGY_SCOPE}` = "N/A — ontology scope not available. Analyze using available evidence only."

---

## Phase 0.8: Context & State Files

### Step 0.8.1: Write Context File

Write `.omc/state/incident-{short-id}/context.md` with:
- Incident summary (symptoms, timeline, blast radius, mitigation, evidence)
- Severity and status
- Evidence types
- Seed-analyst research summary (key findings, files examined, dimension evaluation)
- Report language (detect from user's input language)

### Phase 0.8 Exit Gate

MUST NOT proceed until:

- [ ] `perspectives.md` written with valid roster
- [ ] `context.md` written with structured summary
- [ ] Ontology scope mapping complete (check for `ontology-scope-analyst.md`) or explicitly skipped
- [ ] Track field recorded in `perspectives.md`

---

## Phase 1: Analyst Task Creation & Spawn

Team already exists from Phase 0.5.

### Step 1.1: Create Tasks

Create tasks: one per perspective + DA.

- **Per-perspective analyst tasks**: one per selected archetype
- **DA task**: with `addBlockedBy` depending on ALL analyst task IDs (DA cannot start until all analysts complete)

### Step 1.2: Pre-assign Owners

MUST pre-assign owners via `TaskUpdate(owner="{worker-name}")` BEFORE spawning.

### Step 1.3: Spawn Analysts in Parallel

Spawn all **analyst** agents in parallel. DA is spawned separately after analysts complete (Step 1.4).

MUST read prompt files before spawning. Files are relative to this SKILL.md's directory.

| Agent | Prompt File | Section |
|-------|-------------|---------|
| Devil's Advocate (ALWAYS) | `prompts/devil-advocate.md` + `../shared/da-evaluation-protocol.md` | full file + inline protocol |
| Timeline | `prompts/core-archetypes.md` | § Timeline Lens |
| Root Cause | `prompts/core-archetypes.md` | § Root Cause Lens |
| Systems & Architecture | `prompts/core-archetypes.md` | § Systems Lens |
| Impact | `prompts/core-archetypes.md` | § Impact Lens |
| Security | `prompts/extended-archetypes.md` | § Security Lens |
| Data Integrity | `prompts/extended-archetypes.md` | § Data Integrity Lens |
| Performance | `prompts/extended-archetypes.md` | § Performance Lens |
| UX | `prompts/extended-archetypes.md` | § UX Lens |
| Deployment | `prompts/extended-archetypes.md` | § Deployment |
| Network | `prompts/extended-archetypes.md` | § Network |
| Concurrency | `prompts/extended-archetypes.md` | § Concurrency |
| Dependency | `prompts/extended-archetypes.md` | § Dependency |
| Custom | `prompts/extended-archetypes.md` | § Custom Lens |

**Spawn pattern:**
```
Task(
  subagent_type="oh-my-claudecode:{agent_type}",
  name="{archetype-id}-analyst",
  team_name="incident-analysis-{short-id}",
  model="{model}",
  run_in_background=true,
  prompt="{prompt from file with {INCIDENT_CONTEXT} and {ONTOLOGY_SCOPE} replaced}"
)
```

→ Apply worker preamble from `../shared/worker-preamble.md` to each analyst prompt with:
- `{TEAM_NAME}` = `"incident-analysis-{short-id}"`
- `{WORKER_NAME}` = `"{archetype-id}-analyst"`
- `{WORK_ACTION}` = `"Investigate the incident from your assigned perspective. Answer ALL key questions with evidence and code references. If ontology docs are available (see REFERENCE DOCUMENTS), explore them for relevant policies and documentation."`

MUST replace `{INCIDENT_CONTEXT}` in every prompt with actual Phase 0 details (from `context.md`).
MUST replace `{ONTOLOGY_SCOPE}`: Orchestrator `Read`s `.omc/state/incident-{short-id}/ontology-scope-analyst.md` and injects file contents. If file not found, inject "N/A — ontology scope not available. Analyze using available evidence only."

### Step 1.4: Spawn Devil's Advocate

After all analysts complete (DA task `blockedBy` resolved), spawn DA.

The lead MUST compile all analyst findings into `{ALL_ANALYST_FINDINGS}` before spawning DA. Collect findings from analyst `SendMessage` reports received during Phase 2.

```
Task(
  subagent_type="oh-my-claudecode:critic",
  name="devils-advocate",
  team_name="incident-analysis-{short-id}",
  model="opus",
  run_in_background=true,
  prompt="{DA prompt with {INCIDENT_CONTEXT}, {ACTIVE_PERSPECTIVES}, {ALL_ANALYST_FINDINGS}, {PRIOR_ITERATION_CONTEXT}, and {ONTOLOGY_SCOPE} replaced}"
)
```

`{ONTOLOGY_SCOPE}` for DA: Orchestrator `Read`s `.omc/state/incident-{short-id}/ontology-scope-da.md` and injects file contents. If file not found, inject "N/A" fallback.

---

## Phase 2: Analysis Execution

### Step 2.1: Monitor & Coordinate

Monitor via `TaskList`. Forward findings between analysts. Unblock stuck analysts.

### Step 2.2: Clarity Enforcement

→ Apply `../shared/clarity-enforcement.md` with `{EVIDENCE_FORMAT}` = `"file:function:line"`.

**Rework procedure** when analyst output matches a rejection pattern (MUST occur before analyst marks task `completed` and before DA spawning in Step 1.4):
1. Send feedback via `SendMessage(recipient: "{analyst-name}", content: "{rejection message from clarity enforcement table}")`.
2. Analyst addresses feedback and re-sends findings (task remains `in_progress` throughout).
3. Max 2 rework cycles per analyst. After 2nd rejection, accept with caveat and note for DA.

### Step 2.3: Cross-Perspective Validation

| Signal | Action |
|--------|--------|
| Convergence | Note for synthesis — strengthens confidence |
| Divergence | Route to DA + analysts for resolution |
| Blind spot | Targeted follow-up or spawn specialist |

### Step 2.4: DA Challenge-Response Loop

The DA evaluates analyst findings using the evaluation protocol (`../shared/da-evaluation-protocol.md`). The orchestrator mediates a multi-round loop:

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

**How to read DA verdict**: DA's SendMessage contains a "### Tribunal Trigger Assessment" section with exactly one checked item (`[x]`). Parse the checked line to determine: `SUFFICIENT`, `NOT SUFFICIENT`, or `NEEDS TRIBUNAL`. If no checked item found, ask DA to clarify via SendMessage.

### Step 2.4.1: Triage DA Unanswered Questions

After DA verdict is SUFFICIENT, orchestrator MUST parse DA's "Unanswered Questions" section for BLOCKING_QUESTION and DEFERRED_QUESTION classifications:

- **BLOCKING_QUESTION**: Orchestrator MUST resolve BEFORE proceeding to Phase 2 Exit Gate. Resolution methods (in priority order):
  a. **Tool-based verification**: Use available tools (Bash, Grep, Read, MCP tools, WebSearch) to answer the question directly
  b. **Forward to analyst**: Send question to the relevant analyst via `SendMessage` for targeted investigation
  c. **AskUserQuestion**: If the question cannot be resolved by tools or analysts, ask the user (header: "DA Open Question")
- **DEFERRED_QUESTION**: Record in report as "Open Items" — does NOT block

Write resolved answers to `.omc/state/incident-{short-id}/analyst-findings.md` under a `## DA Resolved Questions` section header (append — do NOT overwrite existing analyst content).

### Phase 2 Exit Gate

MUST NOT proceed until ALL verified:

- [ ] All perspective key questions answered
- [ ] No unexplained timeline gaps
- [ ] ≥1 root cause hypothesis with strong evidence + code references
- [ ] DA Aggregate Verdict is SUFFICIENT (zero BLOCKING, all MAJOR resolved or acknowledged)
- [ ] **All DA BLOCKING_QUESTIONs resolved** (answered via tools, analysts, or user)
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

1. Compile findings package (~10-15K tokens) using this structure:
   - **Incident Summary**: 2-3 sentence recap from Phase 0
   - **Key Findings by Perspective**: For each analyst — perspective name, top 3 findings with evidence
   - **Root Cause Hypothesis**: Primary hypothesis with supporting evidence and code references
   - **Recommendations**: Numbered list of all proposed recommendations
   - **DA Fallacy Check Results**: Per-claim verdict table (FAIL items only — BLOCKING + MAJOR)
   - **Unresolved Contradictions**: Cross-perspective conflicts that triggered tribunal
   - **Tribunal Trigger**: Specific condition from Trigger check that activated tribunal
2. Shut down completed analysts (keep DA): for each analyst, call `SendMessage(type: "shutdown_request", recipient: "{analyst-name}", content: "Analysis complete, proceeding to tribunal.")` and await `shutdown_response(approve=true)` before proceeding. DA remains active for tribunal review.
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

Split → share rationale, 1 final round only. Still split → present to user via `AskUserQuestion` with options: "{UX Critic position}" / "{Eng Critic position}" / "Defer". If user selects "Defer", apply per-recommendation tiebreaker: adopt the position of the critic whose rationale aligns with the DA's findings for that specific recommendation.

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

**Deeper investigation re-entry (max 2 loops):**

Before re-entry, increment `investigation_loops` counter in `.omc/state/incident-{short-id}/context.md`. If counter ≥ 2, inform user: "Maximum investigation depth reached. Proceeding with current findings." and auto-select "Complete".

1. Write current findings to `.omc/state/incident-{short-id}/analyst-findings.md` and DA evaluation to `da-evaluation.md`
2. Append iteration summary to `prior-iterations.md`
3. Identify gaps via `AskUserQuestion` (header: "Investigation Gaps"):
   - "Add new perspective" → spawn new analyst only (existing findings preserved in `analyst-findings.md`)
   - "Re-examine with focus" → user specifies focus area → targeted follow-up tasks
4. New analyst runs → DA re-evaluates with cumulative findings (`analyst-findings.md` appended + `prior-iterations.md` for context via `{PRIOR_ITERATION_CONTEXT}`)
5. Return to Phase 3 synthesis with expanded findings

Tribunal → Phase 2.5.

---

## Phase 4: Cleanup

→ Execute `../shared/team-teardown.md`.

---

## Gate Summary

```
Prerequisite → Phase 0 [intake, severity, track, session ID] → Phase 0.5 [TeamCreate + seed-analyst] → Phase 0.6 [perspective approval] → Phase 0.7 [ontology] → Phase 0.8 [context + state files] → Phase 1 [create tasks + spawn analysts] → Phase 2 [7-item gate] → Phase 2.5? → Phase 3 → Phase 4
```

FAST_TRACK shortcut: Phase 0 → Phase 0.5 [TeamCreate only, skip seed-analyst] → Phase 0.6 [skip, 4-core locked] → Phase 0.7 → Phase 0.8 → Phase 1.

Every gate specifies exact missing items. Fix before proceeding.
