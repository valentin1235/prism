---
name: plan
description: Analyzes input from multiple dynamically-generated perspectives, synthesizes via Devil's Advocate, and produces actionable execution plans through 3-person committee debate with consensus enforcement
version: 1.0.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, AskUserQuestion, Read, Glob, Grep, Bash, Write, WebFetch, WebSearch
---

# Table of Contents

- [Prerequisite](#prerequisite)
- [Phase 0: Input Analysis & Context Gathering](#phase-0-input-analysis--context-gathering)
- [Phase 1: Dynamic Perspective Generation](#phase-1-dynamic-perspective-generation)
- [Phase 2: Team Formation](#phase-2-team-formation)
- [Phase 3: Parallel Multi-Perspective Analysis](#phase-3-parallel-multi-perspective-analysis)
- [Phase 4: Devil's Advocate Synthesis](#phase-4-devils-advocate-synthesis)
- [Phase 5: Committee Debate](#phase-5-committee-debate)
- [Phase 6: Plan Output](#phase-6-plan-output)
- [Phase 7: Cleanup](#phase-7-cleanup)

Prompt templates and output template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

---

## Prerequisite

HARD GATE — MUST verify Agent Teams is enabled:
```json
// ~/.claude/settings.json → "env" → "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
```
If missing → inform user and offer setup. Do NOT proceed without this.

## Artifact Persistence

MUST persist phase outputs to `.omc/state/plan-{short-id}/` (create in Phase 2). On feedback loop re-entry, agents MUST `Read` artifact files — do NOT rely solely on prompt context.

| File | Written | Read By |
|------|---------|---------|
| `context.md` | Phase 2 | All agents |
| `analyst-findings.md` | Phase 3 exit | DA |
| `da-synthesis.md` | Phase 4 exit | Committee |
| `committee-debate.md` | Phase 5 exit | Analysts (re-entry) |
| `prior-iterations.md` | Each iteration (append) | All (cumulative history) |

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

If ANY required element is missing, use `AskUserQuestion` per element (header: element name, options: inferred value / "I'll describe it differently"). Maximum 3 rounds.

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

Each perspective MUST pass ALL:
- [ ] **Orthogonal**: Does NOT overlap analysis scope with other selected perspectives
- [ ] **Input-grounded**: Available input content can answer this perspective's key questions
- [ ] **Plan-specific**: Selected because THIS plan demands it, not "generically useful"
- [ ] **Actionable**: Will produce concrete plan elements, not just observations

If a perspective fails any check → replace or drop it.

### Step 1.3: Present to User

`AskUserQuestion`:
```
question: "I recommend these {N} perspectives for analysis. How to proceed?"
header: "Perspectives"
options:
  - label: "Proceed"
    description: "Start analysis with these perspectives"
  - label: "Add perspective"
    description: "I want an additional analysis angle"
  - label: "Remove perspective"
    description: "Drop one of the proposed perspectives"
  - label: "Modify perspective"
    description: "Adjust scope or questions of a perspective"
```

### Step 1.4: Iterate Until Approved

Repeat 1.3 until user selects "Proceed". Warn if <3 perspectives: "Fewer than 3 perspectives may produce a shallow plan. Continue anyway?"

### Step 1.5: Lock Roster

Lock final perspective roster: ID, name, scope, key questions, model, agent type, rationale.

### Phase 1 Exit Gate

MUST NOT proceed until:

- [ ] 3-6 orthogonal perspectives defined
- [ ] Each passes Quality Gate
- [ ] User approved the roster
- [ ] Roster locked (no further changes)

---

## Phase 2: Team Formation

### Step 2.1: Create Team

```
TeamCreate(team_name: "plan-committee-{short-id}", description: "Plan: {goal summary}")
Bash(mkdir -p .omc/state/plan-{short-id})
```

Write Phase 0 extracted context to `.omc/state/plan-{short-id}/context.md`. MUST include Hell Mode flag if active.

### Step 2.2: Create Tasks

Create tasks in this order:

1. **Per-perspective analyst tasks** (N tasks)
   - Subject: `{perspective-name} Analysis`
   - Description: Include perspective scope, key questions, and raw input context
   - ActiveForm: `Analyzing {perspective-name}`

2. **Devil's Advocate task** (1 task)
   - Subject: `Devil's Advocate Synthesis`
   - `addBlockedBy`: ALL analyst task IDs
   - Description: Synthesize all analyst findings, challenge assumptions, identify gaps
   - ActiveForm: `Synthesizing and challenging findings`

3. **Committee tasks** (3 tasks)
   - UX Critic, Engineering Critic, Planner
   - `addBlockedBy`: DA task ID
   - ActiveForm: `{role} evaluating plan`

### Step 2.3: Pre-assign Owners

MUST pre-assign owners via `TaskUpdate(owner="{worker-name}")` BEFORE spawning:

| Task | Owner Name |
|------|-----------|
| {perspective-id} Analysis | `{perspective-id}-analyst` |
| DA Synthesis | `devils-advocate` |
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
- `{CODEBASE_REFERENCE}` → codebase search instructions (if plan involves code), otherwise "N/A — this plan does not involve codebase analysis"

### Step 3.3: Analyst Prompt Structure

Worker preamble (shared by all analysts):
```
You are a TEAM WORKER in team "plan-committee-{short-id}". Your name is "{perspective-id}-analyst".
You report to the team lead ("team-lead").

== WORK PROTOCOL ==
1. TaskList → find my assigned task → TaskUpdate(status="in_progress")
2. Analyze the planning context from your assigned perspective
3. Answer ALL key questions with evidence and reasoning
4. Report findings via SendMessage to team-lead
5. TaskUpdate(status="completed")
6. On shutdown_request → respond with shutdown_response(approve=true)
```

Followed by the perspective-specific prompt from `prompts/analyst.md`.

### Step 3.4: Monitor & Coordinate

Monitor via `TaskList`. Forward relevant cross-perspective findings between analysts when:
- One analyst's finding directly impacts another's scope
- A contradiction emerges between perspectives

### Step 3.5: Clarity Enforcement

MUST reject and return these patterns:

| Pattern Found | Required Response |
|---------------|------------------|
| "probably", "might", "seems like" | "Provide concrete evidence or reasoning. What supports this?" |
| Unsupported claims | "INCOMPLETE: Cite specific data, reference, or logical chain." |
| Scope drift | "Stay within your perspective scope: {scope}. Redirect to {correct-analyst}." |
| Missing key question answers | "Key question unanswered: {question}. Address before completing." |

### Phase 3 Exit Gate

MUST NOT proceed until ALL verified:

- [ ] All analyst tasks in `completed` status
- [ ] ALL key questions answered per perspective
- [ ] No unresolved cross-perspective contradictions
- [ ] Each finding backed by evidence or reasoning

If ANY fails → create follow-up tasks, continue Phase 3. Error: "Cannot synthesize: {item} not satisfied."

Write compiled analyst findings to `.omc/state/plan-{short-id}/analyst-findings.md`.

---

## Phase 4: Devil's Advocate Synthesis

### Step 4.1: Read DA Prompt

Read `prompts/devil-advocate.md` (relative to this SKILL.md).

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
- `{PRIOR_ITERATION_CONTEXT}` → empty string on first pass; on feedback loop iterations, include previous DA synthesis + committee debate results + gap analysis

### Step 4.3: Monitor DA

Wait for DA completion. DA output = **synthesis package** for committee. Write to `da-synthesis.md`.

### Phase 4 Exit Gate

MUST NOT proceed until:

- [ ] DA task in `completed` status
- [ ] Synthesis package contains: merged findings, challenges, blind spots, risk assessment, preliminary recommendations
- [ ] All analyst challenges have DA responses

---

## Phase 5: Committee Debate

Three committee members debate the synthesis package via SendMessage, with Lead as mediator.

### Step 5.1: Read Committee Prompts

Read all three prompt files (relative to this SKILL.md):
- `prompts/committee/ux-critic.md`
- `prompts/committee/engineering-critic.md`
- `prompts/committee/planner.md`

### Step 5.2: Compile Briefing Package

Compile for committee members (~10-15K tokens max):
- Planning goal and constraints (from Phase 0)
- Synthesis package (from Phase 4 DA output)
- Key disagreements and open questions
- Preliminary recommendations to evaluate

### Step 5.3: Shutdown Completed Analysts

Send `shutdown_request` to all completed analysts and DA. Keep team active for committee.

### Step 5.4: Spawn Committee Members in Parallel

| Member | Agent Type | Name | Model |
|--------|-----------|------|-------|
| UX Critic | `architect-medium` | `ux-critic` | sonnet |
| Engineering Critic | `architect` | `engineering-critic` | opus |
| Planner | `planner` | `planner` | opus |

All spawn with `run_in_background=true`, `team_name="plan-committee-{short-id}"`. MUST replace `{SYNTHESIS_PACKAGE}`, `{PLAN_CONTEXT}`, `{PRIOR_DEBATE_CONTEXT}` in each prompt.

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
   b. Add new perspective, create new tasks + pre-assign owners (Phase 2.2-2.3 pattern)
   c. **Phase 3 → 4 → 5** cycle repeats — agents read artifact files for cumulative context

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
- DA synthesis (challenges, blind spots, risk)
- Committee positions and consensus results
- Dissenting views (attributed)
- Open items requiring future decision

Write via `Write` tool in the detected report language.

### Step 6.4: Chat Summary

After writing the file, output to chat: Goal, File path, Consensus level (% of elements), Perspectives used, Iteration count. Then: Key Decisions (numbered, with consensus level), Top 5 Action Items (action + owner + timeline), Open Items list (or "None — full consensus achieved").

---

## Phase 7: Cleanup

1. `SendMessage(type: "shutdown_request")` to all active committee members
2. Await `shutdown_response(approve=true)` from each
3. `TeamDelete`

---

## Gate Summary

Prerequisite → Phase 0 [5-item] → Phase 1 [4-item] → Phase 2 → Phase 3 [4-item] → Phase 4 [3-item] → Phase 5 [consensus] → Phase 6 → Phase 7. No Consensus at Phase 5 → feedback loop to Phase 3. Every gate specifies exact missing items — fix before proceeding.
