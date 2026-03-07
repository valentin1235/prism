---
name: plan-v2
description: Multi-perspective planning with ontology-scoped analysis and committee consensus debate
version: 1.0.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, Read, Glob, Grep, Bash, Write, WebFetch, WebSearch, ToolSearch, ListMcpResourcesTool, mcp__ontology-docs__list_allowed_directories, mcp__ontology-docs__directory_tree, mcp__ontology-docs__list_directory, mcp__ontology-docs__read_file, mcp__ontology-docs__read_text_file, mcp__ontology-docs__read_multiple_files, mcp__ontology-docs__search_files
---

# Table of Contents

- [Prerequisite](#prerequisite)
- [Phase 0: Input Analysis & Context Gathering](#phase-0-input-analysis--context-gathering)
- [Phase 0.5: Team Creation & Seed Analysis](#phase-05-team-creation--seed-analysis)
- [Phase 0.6: Perspective Approval](#phase-06-perspective-approval)
- [Phase 0.7: Ontology Scope Mapping](#phase-07-ontology-scope-mapping)
- [Phase 0.8: Context & State Files](#phase-08-context--state-files)
- [Phase 1: Task Creation & Spawn](#phase-1-task-creation--spawn)
- [Phase 3: Parallel Multi-Perspective Analysis](#phase-3-parallel-multi-perspective-analysis)
- [Phase 4: Devil's Advocate Evaluation](#phase-4-devils-advocate-evaluation)
- [Phase 5: Committee Debate](#phase-5-committee-debate)
- [Phase 6: Plan Output](#phase-6-plan-output)
- [Phase 7: Cleanup](#phase-7-cleanup)

Prompt templates and output template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

---

## Prerequisite

→ Read and execute `../shared-v2/prerequisite-gate.md`. Set `{PROCEED_TO}` = "Phase 0".

## Artifact Persistence

MUST persist phase outputs to `.omc/state/plan-{short-id}/` (created in Phase 0, Step 0.5). On feedback loop re-entry, agents MUST `Read` artifact files — do NOT rely solely on prompt context.

| File | Written | Read By |
|------|---------|---------|
| `perspectives.md` | Orchestrator (Phase 0.6) | All agents |
| `context.md` | Orchestrator (Phase 0.8) | All agents |
| `ontology-catalog.md` | Orchestrator (Phase 0.7) | All analysts |
| `ontology-scope-analyst.md` | Orchestrator (Phase 0.7) | All analysts, committee |
| `ontology-scope-da.md` | Orchestrator (Phase 0.7) | DA |
| `analyst-findings.md` | Phase 3 exit | DA |
| `da-evaluation.md` | Phase 4 exit (DA) | Committee |
| `da-verified-briefing.md` | Phase 4 exit (orchestrator) | Committee |
| `committee-debate.md` | Phase 5 exit | Analysts (re-entry) |
| `prior-iterations.md` | Each iteration (append) | All (cumulative history) |

## Phase 0: Input Analysis & Context Gathering

Orchestrator handles intake directly — NOT delegated. Reference steps: `docs/delegated-phases.md` § Phase 0.

### Step 0.1: Detect Input Type

| Input Type | Detection | Action |
|-----------|-----------|--------|
| File path | Argument matches file path pattern (`.md`, `.txt`, `.doc`, etc.) | `Read` the file |
| URL | Argument contains `http://` or `https://` | `WebFetch` to retrieve content |
| Text prompt | Argument is plain text (not path, not URL) | Parse as requirements |
| No argument + context | Empty invocation, prior conversation exists | Summarize recent conversation context |
| No argument + no context | Empty invocation, no prior conversation | → Step 0.1b: Interactive Goal Elicitation |
| Mixed | Combination of above | Process each, then merge |

If file not found → error: `"Input file not found: {path}"`. Ask user to provide valid path.

**Step 0.1b: Interactive Goal Elicitation**

When invoked with no argument AND no usable conversation context, ask the user directly:

`AskUserQuestion` (header: "Planning Goal", question: "What would you like to plan? Describe your goal, or provide a file path / URL.", options: 2-4 domain-relevant suggestions inferred from the current working directory if possible, e.g. "New feature", "Architecture redesign", "Migration plan"). Always allow free-text via the implicit "Other" option.

Once the user responds, treat their answer as **Text prompt** input and continue to Step 0.2.

**Hell Mode**: If argument contains `--hell` or `hell` → activate Hell Mode (unanimous consensus required, no iteration limit). Announce: "Hell Mode activated — committee MUST reach 3/3 unanimous consensus."

### Step 0.2: Language Detection

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

If ANY required element is missing, use `AskUserQuestion` per element (header: element name, options: "{inferred value}" / "Not applicable"). Maximum 3 rounds.

### Step 0.5: Generate Session ID and State Directory

Generate `{short-id}`: `Bash(uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8)`. Generate ONCE and reuse throughout all phases.

Create state directory: `Bash(mkdir -p .omc/state/plan-{short-id})`

### Phase 0 Exit Gate

MUST NOT proceed until ALL documented:

- [ ] Goal clearly stated (1-2 sentences, no ambiguity)
- [ ] Scope defined (explicit in/out boundaries)
- [ ] Constraints identified (at minimum: timeline, technical)
- [ ] Input language detected → report language locked
- [ ] Raw input preserved for analyst reference
- [ ] `{short-id}` generated and state directory created

---

## Phase 0.5: Team Creation & Seed Analysis

### Step 0.5.1: Create Team

```
TeamCreate(team_name: "plan-committee-{short-id}", description: "Plan: {goal summary}")
```

### Step 0.5.2: Spawn Seed Analyst

Read `prompts/seed-analyst.md` (relative to this SKILL.md).

Create seed-analyst task via `TaskCreate`, pre-assign owner via `TaskUpdate(owner="seed-analyst")`, then spawn:

```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="seed-analyst",
  team_name="plan-committee-{short-id}",
  model="opus",
  run_in_background=true,
  prompt="{worker preamble + seed-analyst prompt with placeholders replaced}"
)
```

→ Apply worker preamble from `../shared-v2/worker-preamble.md` with:
- `{TEAM_NAME}` = `"plan-committee-{short-id}"`
- `{WORKER_NAME}` = `"seed-analyst"`
- `{WORK_ACTION}` = `"Analyze the planning input, evaluate dimensions, and generate perspective recommendations. If input references codebase, investigate using Grep/Read/Bash. Report findings via SendMessage."`

Placeholder replacements in seed-analyst prompt:
- `{PLAN_CONTEXT}` → Phase 0 extracted context (goal, scope, constraints)
- `{INPUT_TYPE}` → detected input type from Step 0.1
- `{RAW_INPUT}` → original input content
- `{REPORT_LANGUAGE}` → detected report language

### Step 0.5.3: Receive Seed Analyst Results

Wait for seed-analyst to send results via `SendMessage`. The message contains:
- **Analysis Summary**: input characteristics, files examined, recent changes
- **Dimension Evaluation**: domain, complexity, risk, stakeholders, timeline, novelty
- **Perspectives**: 3-6 perspective candidates with ID, Name, Scope, Key Questions, Model, Agent Type, Rationale

### Step 0.5.4: Shutdown Seed Analyst

After receiving results: `SendMessage(type: "shutdown_request", recipient: "seed-analyst", content: "Seed analysis complete.")`.

### Phase 0.5 Exit Gate

MUST NOT proceed until:

- [ ] Team created
- [ ] Seed-analyst results received
- [ ] Seed-analyst shut down

---

## Phase 0.6: Perspective Approval

### Step 0.6.1: Present Perspectives

`AskUserQuestion` (header: "Perspectives", question: "I recommend these {N} perspectives for analysis. How to proceed?", options: "Proceed" / "Add perspective" / "Remove perspective" / "Modify perspective")

Include seed-analyst's analysis summary for user context.

### Step 0.6.2: Iterate Until Approved

Repeat until user selects "Proceed". Warn if <3 perspectives: "Fewer than 3 perspectives may produce a shallow plan. Continue anyway?"

### Step 0.6.3: Write Perspectives

Write locked roster to `.omc/state/plan-{short-id}/perspectives.md`:

```markdown
# Perspectives — Locked Roster

## Report Language
{detected language}

## Hell Mode
{true / false}

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

→ Read and execute `../shared-v2/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"analysis"`
- `{STATE_DIR}` = `.omc/state/plan-{short-id}`

If `ONTOLOGY_AVAILABLE=false` → all analysts get `{ONTOLOGY_SCOPE}` = "N/A — ontology scope not available."

---

## Phase 0.8: Context & State Files

### Step 0.8.1: Write Context File

Write `.omc/state/plan-{short-id}/context.md` with:
- Goal, scope, constraints, stakeholders, success criteria
- Existing context (prior decisions, dependencies)
- Seed-analyst analysis summary (key findings, dimension evaluation)
- Hell Mode flag
- Report language

### Phase 0.8 Exit Gate

MUST NOT proceed until:

- [ ] `perspectives.md` written with valid roster
- [ ] `context.md` written with structured summary
- [ ] Ontology scope mapping complete (check for `ontology-scope-analyst.md`) or explicitly skipped

---

## Phase 1: Task Creation & Spawn

Team already exists from Phase 0.5.

### Step 1.1: Create Tasks

Create tasks in this order:

1. **Per-perspective analyst tasks** (N tasks)
   - Subject: `{perspective-name} Analysis`
   - Description: Include perspective scope, key questions, raw input context, and **ontology scope mapping**
   - ActiveForm: `Analyzing {perspective-name}`

2. **Devil's Advocate task** (1 task)
   - Subject: `Devil's Advocate Evaluation`
   - `addBlockedBy`: ALL analyst task IDs
   - Description: Evaluate analyst reasoning for logical fallacies and overclaims (logic auditor, NOT synthesizer)
   - ActiveForm: `Evaluating analyst reasoning`

3. **Committee tasks** (3 tasks)
   - UX Critic, Engineering Critic, Planner
   - `addBlockedBy`: DA task ID
   - ActiveForm: `{role} evaluating plan`

### Step 1.2: Pre-assign Owners

MUST pre-assign owners via `TaskUpdate(owner="{worker-name}")` BEFORE spawning:

| Task | Owner Name |
|------|-----------|
| {perspective-id} Analysis | `{perspective-id}-analyst` |
| DA Evaluation | `devils-advocate` |
| UX Critic | `ux-critic` |
| Engineering Critic | `engineering-critic` |
| Planner | `planner` |

### Feedback Loop Re-entry (Phase 5.8)

When consensus fails and a new perspective is needed, orchestrator handles directly:

1. Identify new perspective from Gap Analysis (requires committee debate context)
2. Create task for NEW perspective only (Step 1.1-1.2 pattern)
3. Ontology files already exist — no re-run needed

---

## Phase 3: Parallel Multi-Perspective Analysis

### Step 3.1: Read Analyst Prompt Template

Read `prompts/analyst.md` (relative to this SKILL.md).

### Step 3.2: Spawn ALL Analysts in Parallel

For each perspective, spawn an analyst:

```
Task(
  subagent_type="oh-my-claudecode:{agent_type}",
  name="{perspective-id}-analyst",
  team_name="plan-committee-{short-id}",
  model="{model}",
  run_in_background=true,
  prompt="{analyst prompt with placeholders replaced}"
)
```

Placeholder replacements:
- `{PERSPECTIVE_NAME}` → perspective name
- `{PERSPECTIVE_SCOPE}` → perspective scope description
- `{KEY_QUESTIONS}` → numbered list of key questions
- `{PLAN_CONTEXT}` → full Phase 0 extracted context from `context.md` (goal, scope, constraints, raw input)
- `{ONTOLOGY_SCOPE}` → Orchestrator `Read`s `.omc/state/plan-{short-id}/ontology-scope-analyst.md` and injects file contents here. If file not found, inject "N/A — ontology scope not available."

### Step 3.3: Analyst Prompt Structure

→ Apply worker preamble from `../shared-v2/worker-preamble.md` with:
- `{TEAM_NAME}` = `"plan-committee-{short-id}"`
- `{WORKER_NAME}` = `"{perspective-id}-analyst"`
- `{WORK_ACTION}` = `"Analyze the planning context from your assigned perspective. Answer ALL key questions with evidence and reasoning. If ontology docs are available (see ONTOLOGY SCOPE), explore them through your perspective's lens."`

Followed by the perspective-specific prompt from `prompts/analyst.md`.

### Step 3.4: Monitor & Coordinate

Monitor via `TaskList`. Forward relevant cross-perspective findings between analysts when:
- One analyst's finding directly impacts another's scope
- A contradiction emerges between perspectives

### Step 3.5: Clarity Enforcement

→ Apply `../shared-v2/clarity-enforcement.md` with `{EVIDENCE_FORMAT}` = `"concrete evidence or reasoning"`.

### Phase 3 Exit Gate

MUST NOT proceed until ALL verified:

- [ ] All analyst tasks in `completed` status
- [ ] ALL key questions answered per perspective
- [ ] No unresolved cross-perspective contradictions
- [ ] Each finding backed by evidence or reasoning

If ANY fails → create follow-up tasks, continue Phase 3. Error: "Cannot synthesize: {item} not satisfied."

Write compiled analyst findings to `.omc/state/plan-{short-id}/analyst-findings.md`.

---

## Phase 4: Devil's Advocate Evaluation

### Step 4.1: Read DA Prompt

Read `prompts/devil-advocate.md` (relative to this SKILL.md) + `../shared-v2/da-evaluation-protocol.md`.

### Step 4.2: Spawn Devil's Advocate

```
Task(
  subagent_type="oh-my-claudecode:critic",
  name="devils-advocate",
  team_name="plan-committee-{short-id}",
  model="opus",
  run_in_background=true,
  prompt="{DA prompt with placeholders replaced}"
)
```

Placeholder replacements:
- `{ALL_ANALYST_FINDINGS}` → compiled findings from all analysts
- `{PLAN_CONTEXT}` → Phase 0 context
- `{PRIOR_ITERATION_CONTEXT}` → empty string on first pass; on feedback loop iterations, include previous DA evaluation + committee debate results + gap analysis
- `{ONTOLOGY_SCOPE}` → Orchestrator `Read`s `.omc/state/plan-{short-id}/ontology-scope-da.md` and injects file contents here. If file not found, inject "N/A — ontology scope not available."

DA receives analyst findings for **evaluation** — NOT synthesis tasks. DA is a logic auditor only.

### Step 4.3: DA Challenge-Response Loop

Orchestrator-mediated loop, max 2 rounds:

1. **Round 1**: DA evaluates analyst findings → produces verdict table with BLOCKING/MAJOR/MINOR issues
2. **If NOT SUFFICIENT**: Orchestrator forwards DA challenges to relevant analysts (via `SendMessage`)
3. **Round 2**: Analysts respond → orchestrator sends responses to DA → DA re-evaluates → updated verdict
4. **Termination**:
   - All BLOCKING resolved → **SUFFICIENT** → proceed to Step 4.4
   - BLOCKING persists after 2 rounds → **NEEDS TRIBUNAL** → record as open question for committee
   - MAJOR unresolved after 2 rounds → record as acknowledged limitation

Write DA evaluation to `.omc/state/plan-{short-id}/da-evaluation.md`.

### Step 4.4: Orchestrator Synthesis

After DA verdict is SUFFICIENT (or NEEDS TRIBUNAL items recorded), orchestrator compiles committee briefing:

1. **Merge & deduplicate** analyst findings by theme
2. **Triage DA Unanswered Questions** — parse DA's output for BLOCKING_QUESTION and DEFERRED_QUESTION classifications:
   - **BLOCKING_QUESTION**: Orchestrator MUST resolve BEFORE compiling briefing. Resolution methods (in priority order):
     a. **Tool-based verification**: Use available tools (Bash, Grep, Read, MCP tools, WebSearch) to answer the question directly
     b. **Forward to analyst**: Send question to the relevant analyst via `SendMessage` for targeted investigation
     c. **AskUserQuestion**: If the question cannot be resolved by tools or analysts, ask the user (header: "DA Open Question")
   - **DEFERRED_QUESTION**: Record in briefing as "Open Items for Committee" — does NOT block
3. **Compile briefing package** containing:
   - DA-verified findings (with verdict status)
   - DA aggregate verdict and any NEEDS TRIBUNAL items
   - Cross-analyst contradictions identified by DA
   - Resolved BLOCKING_QUESTIONs with answers
   - DEFERRED_QUESTIONs as open items
4. Write briefing to `.omc/state/plan-{short-id}/da-verified-briefing.md`

### Phase 4 Exit Gate

MUST NOT proceed until:

- [ ] DA verdict is SUFFICIENT (or NEEDS TRIBUNAL items recorded as open questions)
- [ ] Challenge-response loop completed (max 2 rounds)
- [ ] **All DA BLOCKING_QUESTIONs resolved** (answered via tools, analysts, or user)
- [ ] Orchestrator-compiled briefing written to `da-verified-briefing.md`
- [ ] Briefing contains: DA-verified findings, aggregate verdict, resolved BLOCKING_QUESTIONs, DEFERRED_QUESTIONs

---

## Phase 5: Committee Debate

Three committee members debate the DA-verified briefing via SendMessage, with Lead as mediator.

### Step 5.1: Read Committee Prompts

Read all three prompt files (relative to this SKILL.md):
- `prompts/committee/ux-critic.md`
- `prompts/committee/engineering-critic.md`
- `prompts/committee/planner.md`

### Step 5.2: Compile Briefing Package

Compile for committee members (~10-15K tokens max):
- Planning goal and constraints (from Phase 0)
- DA-verified findings and aggregate verdict (from Phase 4 `da-verified-briefing.md`)
- Cross-analyst contradictions and open questions (from DA evaluation)
- NEEDS TRIBUNAL items (if any) for committee resolution
- DEFERRED_QUESTIONs from DA as "Open Items" — committee MUST state a position (adopt, defer, or dismiss with rationale) for each
- Ontology scope reference from Phase 0.7 (for independent verification)

### Step 5.3: Shutdown Completed Analysts

Send `shutdown_request` to all completed analysts and DA. Keep team active for committee.

### Step 5.4: Spawn Committee Members in Parallel

| Member | Agent Type | Name | Model |
|--------|-----------|------|-------|
| UX Critic | `architect-medium` | `ux-critic` | sonnet |
| Engineering Critic | `architect` | `engineering-critic` | opus |
| Planner | `planner` | `planner` | opus |

All spawn with `run_in_background=true`, `team_name="plan-committee-{short-id}"`. MUST replace `{DA_VERIFIED_BRIEFING}`, `{PLAN_CONTEXT}`, `{PRIOR_DEBATE_CONTEXT}`, and `{ONTOLOGY_SCOPE}` (orchestrator `Read`s `.omc/state/plan-{short-id}/ontology-scope-analyst.md` and injects contents; if file not found, inject "N/A") in each prompt. Committee receives the analyst variant because they evaluate the plan itself (not verifying analyst exploration coverage), so they need the same ontology context analysts had.

### Step 5.5: Collect Initial Positions

Wait for all 3 members to send their initial position via `SendMessage`.

### Step 5.6: Lead-Mediated Debate

Lead mediates (UX Critic ↔ Lead ↔ Engineering Critic, Lead ↔ Planner) — members do NOT message each other directly.

**Debate protocol:**

1. Lead receives 3 initial positions
2. Lead identifies disagreements across positions
3. Lead sends targeted cross-questions via `SendMessage` — share each member's concerns with others for response
4. Collect responses, update convergence table
5. If disagreements remain → additional targeted rounds (max 3 rounds per debate session)

**Planner = Tie-breaker**: When UX Critic and Engineering Critic reach deadlock on an item, explicitly ask Planner: "UX position: {X}. Engineering position: {Y}. As tie-breaker, propose a resolution that balances both concerns."

### Step 5.7: Consensus Check

Build convergence table for each plan element:

| Plan Element | UX Critic | Engineering Critic | Planner | Consensus |
|-------------|-----------|-------------------|---------|-----------|
| {element} | {position} | {position} | {position} | {level} |

**Consensus levels:**

| Level | Condition | Action |
|-------|-----------|--------|
| **Strong** | 3/3 agree | Proceed to Phase 6 |
| **Working** | 2/3 agree, 1 dissent documented | Proceed to Phase 6 (document dissent) |
| **Partial** | 60%+ of plan elements have Strong or Working consensus | Proceed to Phase 6 (document open items) |
| **No Consensus** | <60% of plan elements have consensus | → Feedback Loop |

**Hell Mode override**: Only Strong (3/3 unanimous) proceeds to Phase 6. Working/Partial/No Consensus ALL → Feedback Loop.

Write committee positions and debate results to `.omc/state/plan-{short-id}/committee-debate.md`.

### Step 5.8: Feedback Loop (No Consensus Path)

When consensus fails:

1. **Gap Analysis**: Lead analyzes deadlocked topics → identifies missing perspectives
2. **User Consultation** via `AskUserQuestion` (header: "Deadlock"):
   - "Add suggested perspective" → spawn new analyst, reconvene
   - "Add different perspective" → user suggests angle
   - "Force current state" → Phase 6 with partial consensus
   - "Stop" → Phase 6 with current results
3. **If adding perspective**:
   a. **Shutdown old committee**, append iteration summary to `prior-iterations.md`
   b. Add new perspective, create new task for the NEW perspective only + pre-assign owner (Step 1.1-1.2 pattern). Previously completed analyst findings are preserved in `analyst-findings.md` — do NOT re-run existing perspectives.
   c. **Phase 3 (new analyst only) → Phase 4 (DA re-runs with cumulative findings) → Phase 5** cycle repeats — DA reads `analyst-findings.md` (appended with new analyst output) + `prior-iterations.md` for full context. Re-create committee tasks (Step 1.1, item 3) + pre-assign owners (Step 1.2) before re-entering Phase 5.

**Normal mode**: max 2 feedback loops, then Phase 6 with current state. **Hell Mode**: no iteration limit — loops until 3/3 unanimous on ALL plan elements.

### Phase 5 Exit Gate

MUST NOT proceed until:

- [ ] Consensus level determined for every plan element
- [ ] All Strong/Working items have clear positions documented
- [ ] All dissenting views documented with rationale
- [ ] DEFERRED_QUESTIONs from DA listed as open items with committee positions
- [ ] Feedback loop either achieved consensus or user chose to proceed

---

## Phase 6: Plan Output

### Step 6.1: Read Output Template

Read `templates/plan-output.md` (relative to this SKILL.md).

### Step 6.2: Determine Output Path

| Input Type | Output Path |
|-----------|------------|
| File input | Same directory as input file → `plan.md` |
| URL / text / conversation | Current working directory → `plan.md` |

If file already exists → `plan-{short-id}.md` to avoid overwrite.

### Step 6.3: Synthesize Plan

Fill the output template with:
- Phase 0 context (goal, scope, constraints)
- Analyst findings (organized by perspective)
- DA evaluation (fallacy check results, aggregate verdict, open questions)
- Committee positions and consensus results
- Dissenting views (attributed)
- Open items requiring future decision
- Ontology scope mapping (catalog)

Write via `Write` tool in the detected report language.

### Step 6.4: Chat Summary

After writing the file, output to chat: Goal, File path, Consensus level (% of elements), Perspectives used, Iteration count. Then: Key Decisions (numbered, with consensus level), Top 5 Action Items (action + owner + timeline), Open Items list (or "None — full consensus achieved").

---

## Phase 7: Cleanup

→ Execute `../shared-v2/team-teardown.md`.

---

## Gate Summary

```
Prerequisite → Phase 0 [intake, context, session ID] → Phase 0.5 [TeamCreate + seed-analyst] → Phase 0.6 [perspective approval] → Phase 0.7 [ontology] → Phase 0.8 [context + state files] → Phase 1 [create tasks + spawn] → Phase 3 [4-item] → Phase 4 [5-item, DA evaluation + challenge-response loop + BLOCKING_QUESTION triage + orchestrator synthesis] → Phase 5 [consensus] → Phase 6 → Phase 7
```

No Consensus at Phase 5 → feedback loop to Phase 3. Every gate specifies exact missing items — fix before proceeding.
