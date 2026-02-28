---
name: incident
description: Multi-perspective agent team incident postmortem with devil's advocate challenge
version: 2.1.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, AskUserQuestion, Read, Glob, Grep, Bash, WebFetch, WebSearch, mcp__ontology-docs__search_files, mcp__ontology-docs__read_file, mcp__ontology-docs__list_directory, mcp__ontology-docs__directory_tree
---

# Table of Contents

- [Archetype Index](#archetype-index)
- [Phase 0: Problem Intake](#phase-0-problem-intake)
- [Phase 0.5: Perspective Generation](#phase-05-perspective-generation)
- [Phase 1: Team Formation](#phase-1-team-formation)
- [Phase 2: Analysis Execution](#phase-2-analysis-execution)
- [Phase 2.5: Conditional Tribunal](#phase-25-conditional-tribunal)
- [Phase 3: Synthesis & Report](#phase-3-synthesis--report)
- [Phase 4: Cleanup](#phase-4-cleanup)

Prompt templates and report template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

## Prerequisite: Agent Team Mode (HARD GATE)

**This gate MUST be checked before ALL other phases. Do NOT skip.**

### Step 1: Check Settings

Read `~/.claude/settings.json` using the `Read` tool and verify:

```
env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS === "1"
```

### Step 2: Decision

| Condition | Action |
|-----------|--------|
| `"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"` exists | → Proceed to Phase 0 |
| Value is `"0"` or key is missing | → **STOP immediately**, show message below |
| `~/.claude/settings.json` file does not exist | → **STOP immediately**, show message below |

### On Failure: Show This Message and STOP

If the setting is not satisfied, output the following message to the user and **terminate skill execution entirely**:

```
Agent Team Mode is not enabled.

This plugin (prism) requires Agent Team Mode because it uses multi-agent team
features (TeamCreate, TaskList, SendMessage, etc.).

How to enable:

1. Open ~/.claude/settings.json (create it if it doesn't exist)
2. Add the following to the "env" section:

   {
     "env": {
       "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
     }
   }

   If you already have an "env" section, just add this key inside it:

   "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"

3. Restart Claude Code
4. Run this skill again after restarting
```

**HARD STOP**: Do NOT proceed to Phase 0 or any subsequent phase if this gate fails. Output the message above and terminate immediately.

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

### Step 0.3.5: Documentation & Codebase Discovery

1. Discover docs structure via `mcp__ontology-docs__directory_tree` (root)
2. Identify top-level directories (e.g., frontend/, backend/, shared/, etc.)
3. Build `{CODEBASE_REFERENCE}` block:

Template:
- Search `{discovered_path_1}` for {detected domain} docs
- Search `{discovered_path_2}` for {detected domain} docs
- Trace from documentation to source code (file:line)

4. If MCP unavailable → error: "ontology-docs MCP not configured. See plugin README for setup."

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

Each selected perspective MUST pass ALL:
- [ ] **Orthogonal**: Does NOT overlap analysis scope with other selected perspectives
- [ ] **Evidence-backed**: Available evidence can answer this lens's key questions
- [ ] **Incident-specific**: Selected because THIS incident demands it, not "always useful"
- [ ] **Actionable**: Will produce concrete recommendations, not just observations

If a perspective fails any check → replace with a better-fitting archetype or drop it.

**Example** (Payment 503, SEV3, Resolved):
```
Seed: domain=app, type=degradation, evidence=logs+metrics+deploys, complexity=multi
Selected:
  1. timeline (Core) — reconstruct 503 failure sequence from logs
  2. root-cause (Core) — trace payment service error chain to code
  3. dependency (Extended) — payment gateway showed elevated latency
  + DA (mandatory)
Rejected: security (no breach indicators), performance (metrics show normal CPU/mem)
```

**0.5.2** Present via `AskUserQuestion`:
- "I recommend these perspectives. How to proceed?"
- Options: "Proceed" / "Add perspective" / "Remove perspective" / "Modify perspective"

**0.5.3** Iterate until approved. Warn if <2 dynamic perspectives.

**0.5.4** Lock roster: archetype, model, key questions, rationale per perspective.

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
  prompt="{prompt from file with {INCIDENT_CONTEXT} and {CODEBASE_REFERENCE} replaced}"
)
```

MUST replace `{INCIDENT_CONTEXT}` in every prompt with actual Phase 0 details.

---

## Phase 2: Analysis Execution

### Step 2.1: Monitor & Coordinate

Monitor via `TaskList`. Forward findings between analysts. Unblock stuck analysts.

### Step 2.2: Clarity Enforcement

MUST reject and return these patterns:

| Pattern Found | Required Response |
|---------------|------------------|
| "probably", "might", "seems like" | "Cite specific evidence. What file:line supports this?" |
| Unexplained timeline gaps | TaskCreate: "Investigate gap {time_a}–{time_b}" |
| Cross-analyst conflicts | Route to both analysts + DA |
| Unaddressed DA challenges | Forward to analyst, REQUIRE evidence-based response |
| Missing code references | "INCOMPLETE: Cite file:function:line. Re-investigate." |

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
4. Spawn UX Critic (Sonnet) + Engineering Critic (Opus) in parallel
5. Collect independent reviews
6. Consensus round:

| Level | Condition | Label |
|-------|-----------|-------|
| Strong | 3/3 APPROVE | `[Unanimous]` |
| Caveat | 2+ APPROVE, 1 CONDITIONAL | `[Approved w/caveat]` |
| Majority | 2 APPROVE, 1 REJECT | `[Majority, dissent: {critic}]` |
| Split | 2+ REJECT | `[No consensus]` → user decision |

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

1. `SendMessage(type: "shutdown_request")` to all active teammates
2. `TeamDelete`

---

## Gate Summary

```
Phase 0 ──[5-item gate]──→ Phase 0.5 ──→ Phase 1 ──→ Phase 2 ──[6-item gate]──→ Phase 2.5? ──→ Phase 3 ──→ Phase 4
```

Every gate specifies exact missing items. Fix before proceeding.
