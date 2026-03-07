---
name: incident-v3
description: Multi-perspective agent team incident postmortem with ontology-scoped analysis, Socratic DA sidecar verification, and mathematical ambiguity scoring. Use this skill for incident analysis, postmortem reports, outage investigation, or root cause analysis that requires verified multi-perspective findings with hallucination detection.
version: 3.0.0
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
- [Phase 1: Analyst Spawn](#phase-1-analyst-spawn)
- [Phase 2: Socratic Verification Loop](#phase-2-socratic-verification-loop)
- [Phase 2.5: Tribunal Decision](#phase-25-tribunal-decision)
- [Phase 3: Synthesis & Report](#phase-3-synthesis--report)
- [Phase 4: Cleanup](#phase-4-cleanup)

Prompt templates and report template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

## Artifact Persistence

MUST persist phase outputs to `.omc/state/incident-{short-id}/` (created in Phase 0, Step 0.4). On deeper investigation re-entry, agents MUST `Read` artifact files — do NOT rely solely on prompt context.

| File | Written | Read By |
|------|---------|---------|
| `perspectives.md` | Orchestrator (Phase 0.6) | All agents |
| `context.md` | Orchestrator (Phase 0.8) | All agents |
| `da-qa-{analyst-id}-round-{N}.md` | Orchestrator (Phase 2) | DA, Scorer |
| `ambiguity-{analyst-id}.json` | Scorer (Phase 2) | Orchestrator |
| `verified-findings-{analyst-id}.md` | Orchestrator (Phase 2) | Phase 3 synthesis |
| `analyst-findings.md` | Orchestrator (Phase 2 exit) | Phase 3 synthesis |
| `prior-iterations.md` | Each re-entry (append) | All agents (cumulative) |
| `ontology-catalog.md` | Orchestrator (Phase 0.7) | Analysts |
| `ontology-scope-analyst.md` | Orchestrator (Phase 0.7) | Analysts |

## Prerequisite: Agent Team Mode (HARD GATE)

→ Read and execute `../shared-v3/prerequisite-gate.md`. Set `{PROCEED_TO}` = "Phase 0".

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

### Verification Agents (per analyst)

| Role | Model | Agent Type | Purpose |
|------|-------|------------|---------|
| Socratic DA | Opus | `critic` | Sidecar interviewer — reduces ambiguity via Q&A |
| Ambiguity Scorer | Sonnet | `analyst` | Scores clarity of verified findings |

Team size: 2 min analysts — 5 max analysts. Each analyst gets a sidecar DA + scorer.

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
| SEV1 OR Active | **FAST_TRACK** | Lock 4 core archetypes (Timeline + Root Cause + Systems + Impact). Skip Phase 0.5 seed-analyst and Phase 0.6 perspective approval. |
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

→ Apply worker preamble from `../shared-v3/worker-preamble.md` with:
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

→ Read and execute `../shared-v3/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"incident analysis"`
- `{STATE_DIR}` = `.omc/state/incident-{short-id}`

If `ONTOLOGY_AVAILABLE=false` → all analysts get `{ONTOLOGY_SCOPE}` = "N/A — ontology scope not available. Analyze using available evidence only."

**Note:** DA agents do NOT receive ontology scope. They work only with analyst-provided findings.

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

## Phase 1: Analyst Spawn

Team already exists from Phase 0.5.

### Step 1.1: Create Analyst Tasks

Create one task per perspective. NO central DA task — DAs are spawned as sidecars in Phase 2.

### Step 1.2: Pre-assign Owners

MUST pre-assign owners via `TaskUpdate(owner="{analyst-name}")` BEFORE spawning.

### Step 1.3: Spawn Analysts in Parallel

Spawn all analyst agents in parallel.

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
  prompt="{prompt from file with {INCIDENT_CONTEXT} and {ONTOLOGY_SCOPE} replaced}"
)
```

→ Apply worker preamble from `../shared-v3/worker-preamble.md` to each analyst prompt with:
- `{TEAM_NAME}` = `"incident-analysis-{short-id}"`
- `{WORKER_NAME}` = `"{archetype-id}-analyst"`
- `{WORK_ACTION}` = `"Investigate the incident from your assigned perspective. Answer ALL key questions with evidence and code references. If ontology docs are available (see REFERENCE DOCUMENTS), explore them for relevant policies and documentation."`

MUST replace `{INCIDENT_CONTEXT}` in every prompt with actual Phase 0 details (from `context.md`).
MUST replace `{ONTOLOGY_SCOPE}`: Orchestrator `Read`s `.omc/state/incident-{short-id}/ontology-scope-analyst.md` and injects file contents. If file not found, inject "N/A — ontology scope not available. Analyze using available evidence only."

### Phase 1 Exit Gate

MUST NOT proceed until:

- [ ] All analyst tasks created and owners pre-assigned
- [ ] All analysts spawned in parallel

---

## Phase 2: Socratic Verification Loop

This phase runs a per-analyst verification pipeline: Analyst → Socratic DA → Ambiguity Scorer. The orchestrator manages the loop.

### Step 2.1: Collect Analyst Findings

As each analyst completes (monitor via `TaskList`), collect their findings from `SendMessage`.

**Immediately persist** each analyst's findings to:
`.omc/state/incident-{short-id}/raw-findings-{analyst-id}.md`

Apply clarity enforcement (`../shared-v3/clarity-enforcement.md` with `{EVIDENCE_FORMAT}` = `"file:function:line"`) before proceeding. Max 2 rework cycles per analyst.

### Step 2.2: Spawn Sidecar DA (per analyst)

For each analyst that passes clarity enforcement, spawn a Socratic DA sidecar.

Read `prompts/devil-advocate.md` (relative to this SKILL.md).

```
Task(
  subagent_type="oh-my-claudecode:critic",
  name="da-{analyst-id}",
  team_name="incident-analysis-{short-id}",
  model="opus",
  run_in_background=true,
  prompt="{DA prompt with placeholders replaced}"
)
```

→ Apply worker preamble with:
- `{WORKER_NAME}` = `"da-{analyst-id}"`
- `{WORK_ACTION}` = `"Ask Socratic questions to reduce ambiguity in the analyst's findings. Do NOT reference ontology documents. Work only with what the analyst provides."`

Placeholder replacements:
- `{ANALYST_NAME}` → the paired analyst's name
- `{ANALYST_FINDINGS}` → the analyst's findings
- `{PRIOR_QA}` → empty string for Round 1
- `{ROUND_NUMBER}` → "1"
- `{INCIDENT_CONTEXT}` → Phase 0 details

**Spawn DAs in parallel** as analysts complete — do not wait for all analysts.

### Step 2.3: Socratic Q&A Loop

The orchestrator mediates the conversation between DA and analyst:

**Round 1:**
1. DA receives analyst findings, produces 2-4 questions targeting ambiguity
2. Orchestrator receives DA questions via `SendMessage`
3. Orchestrator forwards questions to the paired analyst via `SendMessage`
4. Analyst responds with clarifications
5. **Persist Q&A**: Write to `.omc/state/incident-{short-id}/da-qa-{analyst-id}-round-1.md`

**Round N:**
6. Orchestrator compiles all prior Q&A into `{PRIOR_QA}`
7. Sends updated prompt to DA (or spawns new DA instance with accumulated context)
8. DA either asks more questions or declares COMPLETE
9. **Persist Q&A**: Write to `.omc/state/incident-{short-id}/da-qa-{analyst-id}-round-{N}.md`

**DA declares COMPLETE** → proceed to Step 2.4 for this analyst.

**Max rounds: 3** per analyst (prevent infinite loops). After 3 rounds, proceed to scoring regardless.

### Step 2.4: Ambiguity Scoring

For each analyst whose DA has completed, spawn an Ambiguity Scorer.

Read `prompts/ambiguity-scorer.md` (relative to this SKILL.md).

```
Task(
  subagent_type="oh-my-claudecode:analyst",
  name="scorer-{analyst-id}",
  team_name="incident-analysis-{short-id}",
  model="sonnet",
  run_in_background=true,
  prompt="{scorer prompt with placeholders replaced}"
)
```

Placeholder replacements:
- `{ANALYST_NAME}` → analyst name
- `{ANALYST_FINDINGS}` → analyst's findings (post-Q&A updated version)
- `{DA_QA_HISTORY}` → compiled Q&A from all rounds (read from persisted files)
- `{INCIDENT_CONTEXT}` → Phase 0 details

**Persist score**: Write scorer's JSON response to `.omc/state/incident-{short-id}/ambiguity-{analyst-id}.json`

### Step 2.5: Threshold Check

Parse the scorer's JSON output:

| Condition | Action |
|-----------|--------|
| `ambiguity ≤ 0.2` | **PASS** — mark analyst as verified. Write final findings to `.omc/state/incident-{short-id}/verified-findings-{analyst-id}.md` |
| `ambiguity > 0.2` AND loop count < 3 | **RETRY** — send `improvement_hint` from scorer to DA. Return to Step 2.3 with accumulated Q&A. Increment loop count. |
| `ambiguity > 0.2` AND loop count ≥ 3 | **FORCE PASS** — mark as verified with caveat. Note in findings: "Ambiguity score {X} exceeds threshold after 3 rounds. Lowest dimension: {dimension}." |

### Step 2.6: Compile Verified Findings

After ALL analysts are verified (PASS or FORCE PASS):

1. Compile all verified findings into `.omc/state/incident-{short-id}/analyst-findings.md`
2. Include ambiguity scores summary table
3. Flag any FORCE PASS analysts for user attention

### Phase 2 Exit Gate

MUST NOT proceed until ALL verified:

- [ ] All analysts have completed findings
- [ ] All DA Q&A rounds persisted to files
- [ ] All ambiguity scores computed and persisted
- [ ] All analysts verified (PASS or FORCE PASS)
- [ ] Compiled findings written to `analyst-findings.md`

---

## Phase 2.5: Tribunal Decision

Tribunal is now a user decision, not an automatic trigger.

### Step 2.5.1: Present Summary

Show the user a summary of verified findings:
- Per-analyst ambiguity scores
- Any FORCE PASS analysts (highlighted)
- Key findings overview

### Step 2.5.2: Ask User

```
AskUserQuestion(
  header: "Tribunal",
  question: "All analyst findings have been verified through Socratic Q&A. Would you like to send them to a Tribunal for additional review?",
  options: [
    "Skip — proceed to report",
    "Request Tribunal"
  ]
)
```

### Step 2.5.3: If Tribunal Requested

1. Compile findings package (~10-15K tokens):
   - **Incident Summary**: 2-3 sentence recap
   - **Key Findings by Perspective**: top 3 findings per analyst with ambiguity scores
   - **Recommendations**: all proposed recommendations
   - **FORCE PASS items**: analysts that didn't meet the threshold
2. Shut down completed analysts and DAs
3. Read `prompts/tribunal.md` for critic prompts
4. Replace placeholders:
   - `{FINDINGS_PACKAGE}` → compiled findings
   - `{TRIGGER_REASON}` → "User requested tribunal review"
   - `{INCIDENT_CONTEXT}` → Phase 0 details
5. Spawn UX Critic (Sonnet) + Engineering Critic (Opus) in parallel
6. Collect reviews, run consensus round:

| Level | Condition | Label |
|-------|-----------|-------|
| Strong | 2/2 APPROVE | `[Unanimous]` |
| Caveat | 1 APPROVE, 1 CONDITIONAL | `[Approved w/caveat]` |
| Split | 1+ REJECT | `[No consensus]` → user decision |

Split → share rationale, 1 final round only. Still split → present to user via `AskUserQuestion`.

7. Compile verdict, shut down critics, proceed to Phase 3.

### If Skip

Proceed directly to Phase 3.

---

## Phase 3: Synthesis & Report

### Step 3.1

Integrate all verified analyst findings. Read from `.omc/state/incident-{short-id}/analyst-findings.md`.

### Step 3.2

Read `templates/report.md` and fill all sections with synthesized findings.

### Step 3.3

`AskUserQuestion`:
- "Is the analysis complete?"
- Options: "Complete" / "Need deeper investigation" / "Add recommendations" / "Share with team"

**Deeper investigation re-entry (max 2 loops):**

Before re-entry, increment `investigation_loops` counter in `.omc/state/incident-{short-id}/context.md`. If counter ≥ 2, inform user: "Maximum investigation depth reached. Proceeding with current findings." and auto-select "Complete".

1. Write current findings to `.omc/state/incident-{short-id}/analyst-findings.md`
2. Append iteration summary to `prior-iterations.md`
3. Identify gaps via `AskUserQuestion` (header: "Investigation Gaps"):
   - "Add new perspective" → spawn new analyst only (existing findings preserved)
   - "Re-examine with focus" → user specifies focus area → targeted follow-up tasks
4. New analyst runs → full Socratic DA + Scorer verification
5. Return to Phase 3 synthesis with expanded findings

---

## Phase 4: Cleanup

→ Execute `../shared-v3/team-teardown.md`.

---

## Gate Summary

```
Prerequisite → Phase 0 [intake, severity, track, session ID]
→ Phase 0.5 [TeamCreate + seed-analyst]
→ Phase 0.6 [perspective approval]
→ Phase 0.7 [ontology]
→ Phase 0.8 [context + state files]
→ Phase 1 [spawn analysts]
→ Phase 2 [Socratic DA + Ambiguity Scorer loop per analyst]
→ Phase 2.5 [AskUser: tribunal?]
→ Phase 3 [report]
→ Phase 4 [cleanup]
```

FAST_TRACK shortcut: Phase 0 → Phase 0.5 [TeamCreate only, skip seed-analyst] → Phase 0.6 [skip, 4-core locked] → Phase 0.7 → Phase 0.8 → Phase 1.

Every gate specifies exact missing items. Fix before proceeding.
