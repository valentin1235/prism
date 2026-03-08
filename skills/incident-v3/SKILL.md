---
name: incident-v3
description: Multi-perspective agent team incident postmortem with ontology-scoped analysis and MCP-based Socratic verification with mathematical ambiguity scoring. Use this skill for incident analysis, postmortem reports, outage investigation, or root cause analysis that requires verified multi-perspective findings with hallucination detection.
version: 4.0.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, TaskOutput, Read, Glob, Grep, Bash, Write, WebFetch, WebSearch, ToolSearch, ListMcpResourcesTool, mcp__ontology-docs__list_allowed_directories, mcp__ontology-docs__search_files, mcp__ontology-docs__read_file, mcp__ontology-docs__read_text_file, mcp__ontology-docs__read_multiple_files, mcp__ontology-docs__list_directory, mcp__ontology-docs__directory_tree, mcp__prism__prism_interview, mcp__prism__prism_score
---

# Incident Postmortem v3

Prompt templates and report template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

Later phases (Phase 2+) are in `docs/later-phases.md`. Read that file ONLY when entering Phase 2.

## Artifact Persistence

Persist phase outputs to `~/.prism/state/incident-{short-id}/` (created in Phase 0, Step 0.4). On deeper investigation re-entry, agents MUST `Read` artifact files — do NOT rely solely on prompt context.

| File | Written | Read By |
|------|---------|---------|
| `perspectives.md` | Orchestrator (Phase 0.6) | All agents |
| `context.md` | Orchestrator (Phase 0.8) | All agents |
| `~/.prism/state/incident-{short-id}/perspectives/{perspective-id}/findings.json` | Analyst (Phase 2) | MCP prism_interview |
| `verified-findings-{perspective-id}.md` | Orchestrator (Phase 2) | Phase 3 synthesis |
| `analyst-findings.md` | Orchestrator (Phase 2 exit) | Phase 3 synthesis |
| `prior-iterations.md` | Each re-entry (append) | All agents (cumulative) |
| `ontology-catalog.md` | Orchestrator (Phase 0.7) | Analysts |
| `ontology-scope-analyst.md` | Orchestrator (Phase 0.7) | Analysts |

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
| `custom` | Custom | Auto | Auto | Novel failure modes |

### Verification (MCP-based)

| Tool | Purpose |
|------|---------|
| `prism_interview` | Socratic interviewer — reads analyst findings, asks probing questions to reduce ambiguity |
| `prism_score` | Ambiguity scorer — evaluates clarity on 3 axes (Goal 40%, Constraints 30%, Criteria 30%) |

Team size: 2 min analysts, no hard max (typically 3-5; complex incidents may need more). Verification runs via MCP tools, not sidecar agents.

---

## Phase 0: Problem Intake

Orchestrator handles intake directly — NOT delegated.

### Step 0.1: Collect Incident

If the user provided an incident description via `$ARGUMENTS`, use it directly. Otherwise, ask via `AskUserQuestion` (header: "Incident"): "Please describe the incident: What symptoms? Which systems affected? Business impact?"

### Step 0.2: Generate Session ID and State Directory

Generate `{short-id}`: `Bash(uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8)`. Generate ONCE and reuse throughout all phases.

Create state directory: `Bash(mkdir -p ~/.prism/state/incident-{short-id})`

### Phase 0 Exit Gate

MUST NOT proceed until ALL documented:

- [ ] Incident description collected
- [ ] `{short-id}` generated and state directory created

Severity, status, and evidence types are NOT collected here — the seed-analyst will determine these automatically during active investigation in Phase 0.5.

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
- `{WORK_ACTION}` = `"Actively investigate the incident using available tools (Grep, Read, Bash, MCP). Evaluate incident dimensions, map to archetype candidates, and generate perspective recommendations. Report findings via SendMessage."`

Placeholder replacements in seed-analyst prompt:
- `{INCIDENT_DESCRIPTION}` → Phase 0 incident description

### Step 0.5.3: Receive Seed Analyst Results

Wait for seed-analyst to send results via `SendMessage`. The message contains:
- **Research Summary**: evidence discovered, files examined, MCP queries, recent changes
- **Assessed Context**: severity (SEV1-4), status (Active/Mitigated/Resolved/Recurring), evidence types found
- **Dimension Evaluation**: domain, failure type, evidence, complexity, recurrence
- **Perspectives**: perspective candidates with ID, Name, Scope, Key Questions, Model, Agent Type, Rationale

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
- [ ] Seed-analyst results received
- [ ] Seed-analyst shut down
- [ ] All background task outputs drained via `TaskOutput`

→ **NEXT ACTION: Proceed to Phase 0.6 Step 0.6.1 — Present perspectives to user.**

---

## Phase 0.6: Perspective Approval

### Step 0.6.1: Present Perspectives

`AskUserQuestion` (header: "Perspectives", question: "I recommend these {N} perspectives for analysis. How to proceed?", options: "Proceed" / "Add perspective" / "Remove perspective" / "Modify perspective")

Include seed-analyst's research summary for user context.

### Step 0.6.2: Iterate Until Approved

Repeat until user selects "Proceed". Warn if <2 dynamic perspectives.

### Step 0.6.3: Write Perspectives

Write locked roster to `~/.prism/state/incident-{short-id}/perspectives.md`:

```markdown
# Perspectives — Locked Roster

## Severity
{SEV1-4}

## Status
{Active/Mitigated/Resolved/Recurring}

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

Write `~/.prism/state/incident-{short-id}/context.md` with:
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

→ **NEXT ACTION: Proceed to Phase 1 — Spawn analysts.**

---

## Phase 1: Spawn Analysts

Team already exists from Phase 0.5. Spawn all analyst agents in parallel. Each analyst runs self-verification via MCP tools (prism_interview + prism_score) before reporting.

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

> Apply worker preamble with `{WORK_ACTION}` = `"Investigate the incident from your assigned perspective. Answer ALL key questions with evidence and code references. Run self-verification via MCP tools (prism_interview + prism_score). Report verified findings via SendMessage to team-lead."`

MUST replace `{INCIDENT_CONTEXT}` from `context.md`.
MUST replace `{ONTOLOGY_SCOPE}` from `ontology-scope-analyst.md` (or "N/A" if not found).
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
→ Phase 0.5 [TeamCreate + seed-analyst (severity, status, evidence) + drain background tasks]
→ Phase 0.6 [perspective approval]
→ Phase 0.7 [ontology]
→ Phase 0.8 [context + state files]
→ Phase 1 [spawn analysts]
→ Phase 2 [collect verified findings — analysts self-verify via prism_interview + prism_score] ← docs/later-phases.md
→ Phase 3 [report] ← docs/later-phases.md
→ Phase 4 [cleanup] ← docs/later-phases.md
```

Every gate specifies exact missing items. Fix before proceeding.
