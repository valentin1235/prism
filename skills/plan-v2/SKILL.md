---
name: plan-v2
description: Multi-perspective planning with ontology-scoped analysis and committee consensus debate
version: 1.0.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, AskUserQuestion, Read, Glob, Grep, Bash, Write, WebFetch, WebSearch, ToolSearch, ListMcpResourcesTool, mcp__ontology-docs__list_allowed_directories, mcp__ontology-docs__directory_tree, mcp__ontology-docs__list_directory, mcp__ontology-docs__read_file, mcp__ontology-docs__read_text_file, mcp__ontology-docs__read_multiple_files, mcp__ontology-docs__search_files
---

# Table of Contents

- [Prerequisite](#prerequisite)
- [Phase 0: Input Analysis & Context Gathering](#phase-0-input-analysis--context-gathering)
- [Phase 1: Dynamic Perspective Generation](#phase-1-dynamic-perspective-generation)
- [Phase 2: Team Formation](#phase-2-team-formation)
- [Phase 3: Parallel Multi-Perspective Analysis](#phase-3-parallel-multi-perspective-analysis)
- [Phase 4: Devil's Advocate Evaluation](#phase-4-devils-advocate-evaluation)
- [Phase 5: Committee Debate](#phase-5-committee-debate)
- [Phase 6: Plan Output](#phase-6-plan-output)
- [Phase 7: Cleanup](#phase-7-cleanup)

Prompt templates and output template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

---

## Prerequisite

→ Read and execute `../shared/prerequisite-gate.md`. Set `{PROCEED_TO}` = "Phase 0".

## Artifact Persistence

MUST persist phase outputs to `.omc/state/plan-{short-id}/` (created in Phase 1, Step 1.5.1). On feedback loop re-entry, agents MUST `Read` artifact files — do NOT rely solely on prompt context.

| File | Written | Read By |
|------|---------|---------|
| `setup-complete.md` | Setup agent (last) | Orchestrator (before reading other files) |
| `perspectives.md` | Setup agent | Orchestrator (Phase 2) |
| `context.md` | Setup agent | All agents |
| `ontology-catalog.md` | Setup agent | All analysts |
| `ontology-scope-analyst.md` | Setup agent | All analysts, committee |
| `ontology-scope-da.md` | Setup agent | DA |
| `analyst-findings.md` | Phase 3 exit | DA |
| `da-evaluation.md` | Phase 4 exit (DA) | Committee |
| `da-verified-briefing.md` | Phase 4 exit (orchestrator) | Committee |
| `committee-debate.md` | Phase 5 exit | Analysts (re-entry) |
| `prior-iterations.md` | Each iteration (append) | All (cumulative history) |

## Phase 0: Input Analysis & Context Gathering

> → Delegated to setup agent. See "Setup Agent Invocation" below.
> Original steps 0.1-0.4 and exit gate are executed by the setup agent
> in an isolated session. Orchestrator reads output files after completion.

<details>
<summary>Original Phase 0 steps (executed by setup agent)</summary>

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

</details>

---

## Phase 1: Dynamic Perspective Generation

> → Delegated to setup agent (Steps 1.1-1.6).
> Exception: Step 1.5.1 (Session ID generation) remains in orchestrator.

<details>
<summary>Original Phase 1 steps (executed by setup agent)</summary>

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

</details>

### Step 1.5.1: Generate Session ID and State Directory

Generate `{short-id}`: `Bash(uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8)` (e.g., `a3f7b2c1`). Generate ONCE and reuse throughout all phases.

Create state directory: `Bash(mkdir -p .omc/state/plan-{short-id})`

### Setup Agent Invocation

**Step A: Spawn setup agent**

```
Task(
  subagent_type="oh-my-claudecode:deep-executor",
  model="opus",
  prompt="Read and execute skills/shared/setup-agent.md with:
    {SKILL_NAME} = 'plan'
    {STATE_DIR} = '.omc/state/plan-{short-id}'
    {SHORT_ID} = '{short-id}'
    {INPUT_CONTEXT} = '{user arguments / conversation context}'
    {CALLER_CONTEXT} = 'analysis'
    {AVAILABILITY_MODE} = 'optional'
    {SKILL_PATH} = '{absolute path to skills/plan-v2/SKILL.md}'
    {FAST_TRACK_ELIGIBLE} = false"
)
```

NO `team_name` — runs in isolated session. Blocks until setup agent completes and returns.

**Step B: Verify setup completion**

- Read `{STATE_DIR}/setup-complete.md` — verify it exists and lists expected files
- If missing → error: "Setup agent failed. Please retry."

**Step C: Read setup outputs**

- Read `.omc/state/plan-{short-id}/perspectives.md` → parse perspective roster
- Verify `perspectives.md` exists and contains valid roster
- If missing → error: "Perspectives file missing. Please retry."

### Phase 1 Exit Gate

MUST NOT proceed until:

- [ ] `setup-complete.md` sentinel verified
- [ ] `perspectives.md` parsed with 3-6 perspectives
- [ ] Ontology scope mapping complete (check for `ontology-scope-analyst.md`) or explicitly skipped

### Feedback Loop Re-entry (Phase 5.8) — EXEMPT from Setup Agent

Phase 5.8 feedback loop re-entry is explicitly EXEMPT from the setup agent pattern. When consensus fails and a new perspective is needed:

1. The orchestrator identifies the new perspective from Gap Analysis (it has committee debate context)
2. Orchestrator creates a new task for the NEW perspective only using the Phase 2.2-2.3 pattern
3. Ontology files already exist from initial setup — no re-run needed
4. No setup agent is spawned — the orchestrator handles this directly because it requires committee debate context that a fresh agent would not have

---

## Phase 2: Team Formation

### Step 2.1: Create Team

```
TeamCreate(team_name: "plan-committee-{short-id}", description: "Plan: {goal summary}")
```
(State directory already exists from Step 1.5.1.)

Phase 0 context is already available in `.omc/state/plan-{short-id}/context.md` (written by setup agent). Read it for team context reference.

### Step 2.2: Create Tasks

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

### Step 2.3: Pre-assign Owners

MUST pre-assign owners via `TaskUpdate(owner="{worker-name}")` BEFORE spawning:

| Task | Owner Name |
|------|-----------|
| {perspective-id} Analysis | `{perspective-id}-analyst` |
| DA Evaluation | `devils-advocate` |
| UX Critic | `ux-critic` |
| Engineering Critic | `engineering-critic` |
| Planner | `planner` |

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
- `{PLAN_CONTEXT}` → full Phase 0 extracted context (goal, scope, constraints, raw input)
- `{ONTOLOGY_SCOPE}` → Orchestrator `Read`s `.omc/state/plan-{short-id}/ontology-scope-analyst.md` and injects file contents here. If file not found, inject "N/A — ontology scope not available."

### Step 3.3: Analyst Prompt Structure

→ Apply worker preamble from `../shared/worker-preamble.md` with:
- `{TEAM_NAME}` = `"plan-committee-{short-id}"`
- `{WORKER_NAME}` = `"{perspective-id}-analyst"`
- `{WORK_ACTION}` = `"Analyze the planning context from your assigned perspective. Answer ALL key questions with evidence and reasoning. If ontology docs are available (see ONTOLOGY SCOPE), explore them through your perspective's lens."`

Followed by the perspective-specific prompt from `prompts/analyst.md`.

### Step 3.4: Monitor & Coordinate

Monitor via `TaskList`. Forward relevant cross-perspective findings between analysts when:
- One analyst's finding directly impacts another's scope
- A contradiction emerges between perspectives

### Step 3.5: Clarity Enforcement

→ Apply `../shared/clarity-enforcement.md` with `{EVIDENCE_FORMAT}` = `"concrete evidence or reasoning"`.

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

Read `prompts/devil-advocate.md` (relative to this SKILL.md) + `shared/da-evaluation-protocol.md`.

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
2. **Compile briefing package** containing:
   - DA-verified findings (with verdict status)
   - DA aggregate verdict and any NEEDS TRIBUNAL items
   - Cross-analyst contradictions identified by DA
   - Open questions from DA evaluation
3. Write briefing to `.omc/state/plan-{short-id}/da-verified-briefing.md`

### Phase 4 Exit Gate

MUST NOT proceed until:

- [ ] DA verdict is SUFFICIENT (or NEEDS TRIBUNAL items recorded as open questions)
- [ ] Challenge-response loop completed (max 2 rounds)
- [ ] Orchestrator-compiled briefing written to `da-verified-briefing.md`
- [ ] Briefing contains: DA-verified findings, aggregate verdict, open questions

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
- Ontology scope reference from Phase 1.6 (for independent verification)

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
   b. Add new perspective, create new task for the NEW perspective only + pre-assign owner (Phase 2.2-2.3 pattern). Previously completed analyst findings are preserved in `analyst-findings.md` — do NOT re-run existing perspectives.
   c. **Phase 3 (new analyst only) → Phase 4 (DA re-runs with cumulative findings) → Phase 5** cycle repeats — DA reads `analyst-findings.md` (appended with new analyst output) + `prior-iterations.md` for full context. Re-create committee tasks (Step 2.2, item 3) + pre-assign owners (Step 2.3) before re-entering Phase 5.

**Normal mode**: max 2 feedback loops, then Phase 6 with current state. **Hell Mode**: no iteration limit — loops until 3/3 unanimous on ALL plan elements.

### Phase 5 Exit Gate

MUST NOT proceed until:

- [ ] Consensus level determined for every plan element
- [ ] All Strong/Working items have clear positions documented
- [ ] All dissenting views documented with rationale
- [ ] Open items (if any) clearly listed
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

→ Execute `../shared/team-teardown.md`.

---

## Gate Summary

```
Prerequisite → Setup Agent [Phase 0 + Phase 1] → Phase 1 Exit Gate → Phase 2 → Phase 3 [4-item] → Phase 4 [4-item, DA evaluation + challenge-response loop + orchestrator synthesis] → Phase 5 [consensus] → Phase 6 → Phase 7
                                                                                                      ↓ (ONTOLOGY_AVAILABLE=false)
                                                                                                      └─ Analysts get {ONTOLOGY_SCOPE}="N/A", proceed normally
```

No Consensus at Phase 5 → feedback loop to Phase 3. Every gate specifies exact missing items — fix before proceeding.
