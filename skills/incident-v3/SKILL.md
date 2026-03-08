---
name: incident-v3
description: Multi-perspective agent team incident postmortem with ontology-scoped analysis and MCP-based Socratic verification with mathematical ambiguity scoring. Use this skill for incident analysis, postmortem reports, outage investigation, or root cause analysis that requires verified multi-perspective findings with hallucination detection.
version: 4.0.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, TaskOutput, Read, Glob, Grep, Bash, Write, WebFetch, WebSearch, ToolSearch, ListMcpResourcesTool, mcp__prism-mcp__prism_docs_roots, mcp__prism-mcp__prism_docs_list, mcp__prism-mcp__prism_docs_read, mcp__prism-mcp__prism_docs_search, mcp__prism-mcp__prism_interview---

# Incident Postmortem v3

Prompt templates and report template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

Later phases (Phase 2+) are in `docs/later-phases.md`. Read that file ONLY when entering Phase 2.

## Artifact Persistence

Persist phase outputs to `~/.prism/state/incident-{short-id}/` (created in Phase 0, Step 0.4). On deeper investigation re-entry, agents MUST `Read` artifact files — do NOT rely solely on prompt context.

| File | Written | Read By |
|------|---------|---------|
| `seed-analysis.json` | Seed Analyst (Phase 0.5) | Perspective Generator (Phase 0.55), Orchestrator |
| `perspectives.json` | Perspective Generator (Phase 0.55), updated by Orchestrator (Phase 0.6) | Orchestrator (Phase 0.6, 0.8, 1, 3) |
| `context.json` | Orchestrator (Phase 0.8) | Orchestrator (Phase 1 `{INCIDENT_CONTEXT}` injection, Phase 3 re-entry) |
| `~/.prism/state/incident-{short-id}/perspectives/{perspective-id}/findings.json` | Analyst (Phase 2) | MCP prism_interview |
| `verified-findings-{perspective-id}.md` | Orchestrator (Phase 2) | Phase 3 synthesis |
| `analyst-findings.md` | Orchestrator (Phase 2 exit) | Phase 3 synthesis |
| `prior-iterations.md` | Each re-entry (append) | All agents (cumulative) |
| `ontology-scope.json` | Orchestrator (Phase 0.7) | Analysts (via `{ONTOLOGY_SCOPE}` injection) |

## Prerequisite: Agent Team Mode (HARD GATE)

> Read and execute `../shared-v3/prerequisite-gate.md`. Set `{PROCEED_TO}` = "Phase 0".

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
| `ux` | User Experience | Sonnet | `architect-medium` | User-facing degradation, error UX |
| `financial` | Financial & Compliance | Opus | `architect` | Payment discrepancies, billing errors, revenue impact, audit trail gaps |
| `custom` | Custom | Auto | Auto | Novel failure modes |

### Verification (MCP-based)

| Tool | Purpose |
|------|---------|
| `prism_interview` | Socratic interviewer with integrated scoring — reads analyst findings, asks probing questions, auto-scores after each answer. Returns `{continue: true/false, score, question?, reason?}` |

Team size: 2 min analysts, no hard max (typically 3-5; complex incidents may need more). Verification runs via MCP tools, not sidecar agents.

---

## Phase 0: Problem Intake

Orchestrator handles intake directly — NOT delegated.

### Step 0.1: Collect Incident

If the user provided an incident description via `$ARGUMENTS`, use it directly. Otherwise, ask via `AskUserQuestion` (header: "Incident"): "Please describe the incident: What symptoms? Which systems affected? Business impact?"

### Step 0.2: Generate Session ID and State Directory

Generate `{short-id}`: `Bash(uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8)`. Generate ONCE and reuse throughout all phases.

> **Naming note:** `{short-id}` in path templates (e.g., `incident-{short-id}/`) and `{INCIDENT_SHORT_ID}` in prompt placeholders refer to the same value. Use `{short-id}` when constructing paths, `{INCIDENT_SHORT_ID}` when replacing placeholders in agent prompts.

Create state directory: `Bash(mkdir -p ~/.prism/state/incident-{short-id})`

### Phase 0 Exit Gate

MUST NOT proceed until ALL documented:

- [ ] Incident description collected
- [ ] `{short-id}` generated and state directory created

Severity and status are NOT collected here — the seed-analyst will determine these automatically during active investigation in Phase 0.5.

→ **NEXT ACTION: Proceed to Phase 0.5 Step 0.5.1 — Create team.**

---

## Phase 0.5: Team Creation & Seed Analysis

### Step 0.5.1: Create Team

```
TeamCreate(team_name: "incident-analysis-{short-id}", description: "Incident: {summary}")
```

### Step 0.5.2: Spawn Seed Analyst

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

> Apply worker preamble from `../shared-v3/worker-preamble.md` with:
- `{TEAM_NAME}` = `"incident-analysis-{short-id}"`
- `{WORKER_NAME}` = `"seed-analyst"`
- `{WORK_ACTION}` = `"Actively investigate the incident using available tools (Grep, Read, Bash, MCP). Evaluate incident dimensions and severity. Write findings to seed-analysis.json. Report via SendMessage."`

Placeholder replacements in seed-analyst prompt:
- `{INCIDENT_DESCRIPTION}` → Phase 0 incident description
- `{INCIDENT_SHORT_ID}` → Phase 0 short-id

### Step 0.5.3: Receive Seed Analyst Results

Wait for seed-analyst to send results via `SendMessage`. The message contains a JSON object with:
- `severity`, `status`
- `dimensions`: domain, failure_type, evidence_available, complexity, recurrence
- `research`: findings (with source and tool_used), files_examined, mcp_queries, recent_changes

The seed analyst also writes this JSON to `~/.prism/state/incident-{short-id}/seed-analysis.json`.

Note: The seed analyst focuses on research and dimension evaluation only — perspective generation is handled separately in Phase 0.55.

### Step 0.5.4: Shutdown Seed Analyst

After receiving results: `SendMessage(type: "shutdown_request", recipient: "seed-analyst", content: "Seed analysis complete.")`.

### Step 0.5.5: Drain Background Task Output

**CRITICAL (Claude Code bug workaround — [#27431](https://github.com/anthropics/claude-code/issues/27431)):** MCP tool calls hang indefinitely when completed background agents have unread output. Before proceeding to any phase that uses MCP tools (Phase 0.7), drain all completed background task output.

1. Call `TaskList` to find all completed background tasks
2. For each completed task, call `TaskOutput` to read and discard the output
3. This clears the internal notification queue and prevents MCP transport deadlock

### Phase 0.5 Exit Gate

MUST NOT proceed until:

- [ ] Team created
- [ ] Seed-analyst results received (JSON with severity, status, dimensions, research)
- [ ] `seed-analysis.json` written to state directory
- [ ] Seed-analyst shut down
- [ ] All background task outputs drained via `TaskOutput`

→ **NEXT ACTION: Proceed to Phase 0.55 — Perspective Generation.**

---

## Phase 0.55: Perspective Generation

### Step 0.55.1: Spawn Perspective Generator

Read `prompts/perspective-generator.md` (relative to this SKILL.md).

Create task via `TaskCreate`, pre-assign owner via `TaskUpdate(owner="perspective-generator")`, then spawn:

```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="perspective-generator",
  team_name="incident-analysis-{short-id}",
  model="opus",
  run_in_background=true,
  prompt="{worker preamble + perspective-generator prompt with placeholders replaced}"
)
```

> Apply worker preamble from `../shared-v3/worker-preamble.md` with:
- `{TEAM_NAME}` = `"incident-analysis-{short-id}"`
- `{WORKER_NAME}` = `"perspective-generator"`
- `{WORK_ACTION}` = `"Read seed-analysis.json, apply archetype mapping rules and mandatory rules, generate perspective candidates. Write perspectives.json. Report via SendMessage."`

Placeholder replacements:
- `{INCIDENT_SHORT_ID}` → Phase 0 short-id
- `{INCIDENT_DESCRIPTION}` → Phase 0 incident description

### Step 0.55.2: Receive Perspective Generator Results

Wait for perspective-generator to send results via `SendMessage`. The message contains a JSON object with:
- `perspectives`: array of perspective candidates (id, name, scope, key_questions, model, agent_type, rationale)
- `rules_applied`: which mandatory rules were checked and enforced
- `selection_summary`: reasoning for the selection

The perspective generator also writes this JSON to `~/.prism/state/incident-{short-id}/perspectives.json`.

### Step 0.55.3: Shutdown Perspective Generator

After receiving results: `SendMessage(type: "shutdown_request", recipient: "perspective-generator", content: "Perspective generation complete.")`.

### Step 0.55.4: Drain Background Task Output

Same as Step 0.5.5 — drain all completed background task outputs via `TaskList` → `TaskOutput`.

### Phase 0.55 Exit Gate

MUST NOT proceed until:

- [ ] Perspective generator results received
- [ ] `perspectives.json` written to state directory
- [ ] Perspective generator shut down
- [ ] All background task outputs drained

→ **NEXT ACTION: Proceed to Phase 0.6 — Perspective Approval.**

---

## Phase 0.6: Perspective Approval

### Step 0.6.1: Present Perspectives

Read `~/.prism/state/incident-{short-id}/perspectives.json` and present to user.

`AskUserQuestion` (header: "Perspectives", question: "I recommend these {N} perspectives for analysis. How to proceed?", options: "Proceed" / "Add perspective" / "Remove perspective" / "Modify perspective")

Include seed-analyst's research summary (from `seed-analysis.json`) for user context. Show `rules_applied` so user knows which mandatory rules were enforced.

### Step 0.6.2: Iterate Until Approved

Repeat until user selects "Proceed". Warn if <2 dynamic perspectives.

### Step 0.6.3: Update Perspectives

Update `~/.prism/state/incident-{short-id}/perspectives.json` in-place — add approval metadata and apply any user modifications:

```json
{
  "perspectives": [...],
  "rules_applied": {...},
  "selection_summary": "...",
  "approved": true,
  "user_modifications": ["description of changes, if any"]
}
```

The `perspectives` array, `rules_applied`, and `selection_summary` fields are preserved from Phase 0.55. The orchestrator adds `approved` and `user_modifications` (empty array if no changes).

→ **NEXT ACTION: Proceed to Phase 0.7 — Ontology Scope Mapping.**

---

## Phase 0.7: Ontology Scope Mapping

> Read and execute `../shared-v3/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"incident analysis"`
- `{STATE_DIR}` = `~/.prism/state/incident-{short-id}`

If `ONTOLOGY_AVAILABLE=false` → all analysts get `{ONTOLOGY_SCOPE}` = "N/A — ontology scope not available. Analyze using available evidence only."

→ **NEXT ACTION: Proceed to Phase 0.8 — Write context file.**

---

## Phase 0.8: Context & State Files

### Step 0.8.1: Write Context File

Write `~/.prism/state/incident-{short-id}/context.json`:

```json
{
  "incident_summary": "Symptoms, timeline, blast radius, mitigation, evidence",
  "research_summary": {
    "key_findings": ["finding1", "finding2"],
    "files_examined": ["path1", "path2"],
    "dimensions": "domain, failure_type, complexity, recurrence from seed-analysis.json"
  },
  "report_language": "detected from user's input language"
}
```

### Phase 0.8 Exit Gate

MUST NOT proceed until:

- [ ] `perspectives.json` updated with approved=true
- [ ] `context.json` written with structured summary
- [ ] Ontology scope mapping complete (check for `ontology-scope.json`) or explicitly skipped

→ **NEXT ACTION: Proceed to Phase 1 — Spawn analysts.**

---

## Phase 1: Spawn Analysts

Team already exists from Phase 0.5. Spawn all analyst agents in parallel. Each analyst runs self-verification via MCP tools (prism_interview) before reporting.

### Step 1.1: Spawn Analysts

MUST read prompt files before spawning. Files are relative to this SKILL.md's directory.

| Agent | Prompt File | Section |
|-------|-------------|---------|
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
| Financial & Compliance | `prompts/extended-archetypes.md` | § Financial Lens |
| Custom | `prompts/extended-archetypes.md` | § Custom Lens |

**Spawn pattern:**

```
Task(
  subagent_type="oh-my-claudecode:{agent_type}",
  name="{archetype-id}-analyst",
  team_name="incident-analysis-{short-id}",
  model="{model}",
  run_in_background=true,
  prompt="{analyst prompt with {INCIDENT_CONTEXT}, {ONTOLOGY_SCOPE}, {INCIDENT_SHORT_ID} replaced}"
)
```

> Apply worker preamble with `{WORK_ACTION}` = `"Investigate the incident from your assigned perspective. Answer ALL key questions with evidence and code references. Run self-verification via MCP tools (prism_interview). Report verified findings via SendMessage to team-lead."`

MUST replace `{INCIDENT_CONTEXT}` from `context.json`.
MUST replace `{ONTOLOGY_SCOPE}` by reading `ontology-scope.json` and generating a text block per Phase B of ontology-scope-mapping.md (or "N/A" if not found).
MUST replace `{INCIDENT_SHORT_ID}` with the incident's `{short-id}`. Analysts construct their own session path: `incident-{short-id}/perspectives/{perspective-id}`.

### Phase 1 Exit Gate

MUST NOT proceed until:

- [ ] All analyst tasks created and owners pre-assigned
- [ ] All analysts spawned in parallel

→ **NEXT ACTION: Read `docs/later-phases.md` and proceed to Phase 2 — MCP Socratic Verification.**

---

## Gate Summary

```
Prerequisite → Phase 0 [intake, session ID]
→ Phase 0.5 [TeamCreate + seed-analyst (research, dimensions → seed-analysis.json) + drain]
→ Phase 0.55 [perspective-generator (seed-analysis.json → perspectives.json) + drain]
→ Phase 0.6 [perspective approval (user reviews perspectives.json → update with approved)]
→ Phase 0.7 [ontology]
→ Phase 0.8 [context + state files]
→ Phase 1 [spawn analysts]
→ Phase 2 [collect verified findings — analysts self-verify via prism_interview] ← docs/later-phases.md
→ Phase 3 [report] ← docs/later-phases.md
→ Phase 4 [cleanup] ← docs/later-phases.md
```

Every gate specifies exact missing items. Fix before proceeding.
