# Execution Trace: Payment Points Issue Analysis

**Task:** "우리 podo-backend에서 결제 완료 후 포인트 적립이 안 되는 이슈가 발생했어. Sentry에 NullPointerException이 찍히고 있어. 분석해줘"

**Skill Version:** analyze v4.1.0
**Simulated Session ID:** `a1b2c3d4` (would be generated via `uuidgen`)

---

## Prerequisite: Agent Team Mode (HARD GATE)

**File read:** `skills/shared-v3/prerequisite-gate.md`

**Action:** Read `~/.claude/settings.json` and verify `env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS === "1"`.

- If present and `"1"` -> proceed to Phase 0.
- If missing or `"0"` -> HARD STOP with enablement instructions.

**Assumed result:** Gate passes. Proceed to Phase 0.

---

## Phase 0: Problem Intake

**Files read:** None (orchestrator handles directly per SKILL.md).

### Step 0.1: Collect Description

User provided description via `$ARGUMENTS`, so no `AskUserQuestion` needed. Description captured as:

> "우리 podo-backend에서 결제 완료 후 포인트 적립이 안 되는 이슈가 발생했어. Sentry에 NullPointerException이 찍히고 있어. 분석해줘"

### Step 0.2: Generate Session ID and State Directory

```bash
uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8
# -> "a1b2c3d4"

mkdir -p ~/.prism/state/analyze-a1b2c3d4
```

### Phase 0 Exit Gate

- [x] Description collected (from $ARGUMENTS)
- [x] `{short-id}` = `a1b2c3d4`, state directory created

**NEXT:** Proceed to Phase 0.5.

---

## Phase 0.5: Team Creation & Seed Analysis

### Step 0.5.1: Create Team

**Files read:** None (inline instruction in SKILL.md).

```
TeamCreate(team_name: "analyze-a1b2c3d4", description: "Analysis: podo-backend 결제 후 포인트 적립 실패, NullPointerException")
```

### Step 0.5.2: Spawn Seed Analyst

**Files read at this step:**
1. `skills/analyze/prompts/seed-analyst.md` — the seed analyst prompt template
2. `skills/shared-v3/worker-preamble.md` — worker preamble template

**Agent name:** `seed-analyst`
**Subagent type:** `oh-my-claudecode:architect`
**Model:** `opus`

**Prompt assembly:**
1. Worker preamble with replacements:
   - `{TEAM_NAME}` = `"analyze-a1b2c3d4"`
   - `{WORKER_NAME}` = `"seed-analyst"`
   - `{WORK_ACTION}` = `"Actively investigate using available tools (Grep, Read, Bash, MCP). Evaluate dimensions and severity. Write findings to seed-analysis.json. Report via SendMessage."`
2. Seed analyst prompt from `prompts/seed-analyst.md` with replacements:
   - `{DESCRIPTION}` = the user's Korean description
   - `{SHORT_ID}` = `a1b2c3d4`

**Concatenated prompt structure:**
```
[Worker Preamble]
You are a TEAM WORKER in team "analyze-a1b2c3d4". Your name is "seed-analyst".
You report to the team lead ("team-lead").
== WORK PROTOCOL ==
1. TaskList -> find my assigned task -> TaskUpdate(status="in_progress")
2. Actively investigate using available tools...
3. Report findings via SendMessage to team-lead
4. TaskUpdate(status="completed")
5. On shutdown_request -> respond with shutdown_response(approve=true)

[Seed Analyst Prompt from prompts/seed-analyst.md]
You are the SEED ANALYST for an investigation team.
...
DESCRIPTION:
우리 podo-backend에서 결제 완료 후 포인트 적립이 안 되는 이슈가 발생했어...
...
(STEP 1: Active Research, STEP 2: Dimension Evaluation, OUTPUT FORMAT)
```

**Spawn:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="seed-analyst",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

### Step 0.5.3: Receive Seed Analyst Results

Wait for `SendMessage` from `seed-analyst`. Expected output (simulated):

```json
{
  "severity": "SEV2",
  "status": "Active",
  "evidence_types": ["code diffs", "source code"],
  "dimensions": {
    "domain": "app",
    "failure_type": "crash",
    "evidence_available": ["code diffs", "source code"],
    "complexity": "single-cause",
    "recurrence": "first-time"
  },
  "research": {
    "findings": [
      {
        "id": 1,
        "finding": "NullPointerException in point accrual service after payment completion",
        "source": "PointService.java:accruePoints:45",
        "tool_used": "Grep",
        "severity": "critical"
      }
    ],
    "files_examined": ["PointService.java:45 — null check missing"],
    "mcp_queries": ["sentry: NullPointerException in podo-backend"],
    "recent_changes": ["abc1234 — refactored payment callback handler"]
  }
}
```

Also written to `~/.prism/state/analyze-a1b2c3d4/seed-analysis.json`.

### Step 0.5.4: Shutdown Seed Analyst

```
SendMessage(type: "shutdown_request", recipient: "seed-analyst", content: "Seed analysis complete.")
```

### Step 0.5.5: Drain Background Task Output

`TaskList` -> `TaskOutput` for each completed task (Claude Code bug workaround #27431).

### Phase 0.5 Exit Gate

- [x] Team created
- [x] Seed-analyst results received
- [x] `seed-analysis.json` written
- [x] Seed-analyst shut down
- [x] All background task outputs drained

**NEXT:** Proceed to Phase 0.55.

---

## Phase 0.55: Perspective Generation

### Step 0.55.1: Spawn Perspective Generator

**Files read at this step:**
1. `skills/analyze/prompts/perspective-generator.md` — perspective generator prompt
2. `skills/shared-v3/worker-preamble.md` — worker preamble template

**Agent name:** `perspective-generator`
**Subagent type:** `oh-my-claudecode:architect`
**Model:** `opus`

**Prompt assembly:**
1. Worker preamble with:
   - `{TEAM_NAME}` = `"analyze-a1b2c3d4"`
   - `{WORKER_NAME}` = `"perspective-generator"`
   - `{WORK_ACTION}` = `"Read seed-analysis.json, apply archetype mapping rules and mandatory rules, generate perspective candidates. Write perspectives.json. Report via SendMessage."`
2. Perspective generator prompt with:
   - `{SHORT_ID}` = `a1b2c3d4`
   - `{DESCRIPTION}` = user's Korean description

**What the perspective generator would do (simulated):**

Given the seed analysis dimensions:
- domain=`app`, failure_type=`crash`, complexity=`single-cause`
- NullPointerException in payment->point flow
- Payment + point accrual = financial/transactional concern

**Archetype mapping (from perspective-generator.md Step 2):**
- "Payment discrepancy, billing error, revenue data mismatch" -> `financial` + `data-integrity` + `root-cause`
- But this is a crash (NullPointerException), so also: crash -> `root-cause` is essential

**Mandatory rules check (Step 3):**
- Core archetype required: `root-cause` is core -> satisfied
- Recurring -> systems: `first-time`, so N/A
- Evidence-backed only: all have evidence (NullPointerException, code path)
- Minimum perspectives: need >= 2
- Complexity scaling: `single-cause` -> 2-3 perspectives

**Simulated output — perspectives.json:**

```json
{
  "perspectives": [
    {
      "id": "root-cause",
      "name": "Root Cause",
      "scope": "Trace the NullPointerException from payment callback to point accrual code path",
      "key_questions": [
        "What object is null at the crash point in PointService?",
        "Which code change introduced the null reference?",
        "Why does the payment completion path not validate the object before passing to point accrual?"
      ],
      "model": "opus",
      "agent_type": "architect",
      "rationale": "Seed analyst found NullPointerException at PointService.java:accruePoints:45. Root cause analysis needed to trace the exact null reference and its origin."
    },
    {
      "id": "impact",
      "name": "Impact",
      "scope": "Assess how many users are affected by failed point accrual and business/revenue impact",
      "key_questions": [
        "How many payment transactions have completed without point accrual?",
        "Is there a recovery mechanism to retroactively grant missed points?"
      ],
      "model": "sonnet",
      "agent_type": "architect-medium",
      "rationale": "Point accrual failure after payment directly impacts user experience and trust. Need to quantify affected users and determine recovery path."
    },
    {
      "id": "data-integrity",
      "name": "Data Integrity",
      "scope": "Check consistency between payment records and point records — identify orphaned payments without matching point entries",
      "key_questions": [
        "Are there payment records without corresponding point records?",
        "Is the point accrual transactional with the payment, or is it a separate async operation that can silently fail?"
      ],
      "model": "opus",
      "agent_type": "architect",
      "rationale": "Seed analyst found crash in point accrual path. If payment and point accrual are not in the same transaction, there may be data inconsistency requiring reconciliation."
    }
  ],
  "rules_applied": {
    "core_archetype_included": true,
    "recurring_systems_enforced": "n/a",
    "all_evidence_backed": true,
    "min_perspectives_met": true,
    "complexity_scaling_correct": true
  },
  "selection_summary": "3 perspectives selected for single-cause crash. root-cause (core, opus) traces the NullPointerException. impact (core, sonnet) quantifies user/business damage. data-integrity (extended, opus) checks payment-point consistency."
}
```

### Steps 0.55.2-0.55.4: Receive, Shutdown, Drain

Same pattern as seed analyst. Perspective generator sends results, is shut down, task outputs drained.

### Phase 0.55 Exit Gate

- [x] Perspective generator results received
- [x] `perspectives.json` written
- [x] Perspective generator shut down
- [x] All background task outputs drained

**NEXT:** Proceed to Phase 0.6.

---

## Phase 0.6: Perspective Approval

**Files read:** `~/.prism/state/analyze-a1b2c3d4/perspectives.json`

**Action:** Present 3 perspectives to user via `AskUserQuestion`:
- "I recommend these 3 perspectives for analysis. How to proceed?"
- Options: Proceed / Add perspective / Remove perspective / Modify perspective

**Assumed result:** User selects "Proceed".

**Step 0.6.3:** Update `perspectives.json` with `"approved": true, "user_modifications": []`.

**NEXT:** Proceed to Phase 0.7.

---

## Phase 0.7: Ontology Scope Mapping

**File read:** `skills/shared-v3/ontology-scope-mapping.md`

Parameters:
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"analysis"`
- `{STATE_DIR}` = `~/.prism/state/analyze-a1b2c3d4`

This step uses MCP tools (`mcp__prism-mcp__prism_docs_roots`, `prism_docs_list`, `prism_docs_search`) to find relevant ontology documents for the podo-backend domain.

If ontology is not available: all analysts get `{ONTOLOGY_SCOPE}` = "N/A -- ontology scope not available. Analyze using available evidence only."

If available: writes `ontology-scope.json` to state directory and generates a text block for `{ONTOLOGY_SCOPE}` placeholder.

**NEXT:** Proceed to Phase 0.8.

---

## Phase 0.8: Context & State Files

**Files read:** `~/.prism/state/analyze-a1b2c3d4/seed-analysis.json` (for research_summary)

**Step 0.8.1:** Write `~/.prism/state/analyze-a1b2c3d4/context.json`:

```json
{
  "summary": "podo-backend에서 결제 완료 후 포인트 적립이 안 되는 이슈. Sentry에 NullPointerException 발생. PointService.java:accruePoints:45에서 NPE 발생 확인.",
  "research_summary": {
    "key_findings": [
      "NullPointerException in PointService.java:accruePoints:45",
      "Recent refactoring of payment callback handler (commit abc1234)"
    ],
    "files_examined": ["PointService.java:45"],
    "dimensions": "domain=app, failure_type=crash, complexity=single-cause, recurrence=first-time"
  },
  "report_language": "ko"
}
```

Note: `report_language` is detected as `"ko"` from the user's Korean input.

### Phase 0.8 Exit Gate

- [x] `perspectives.json` has `approved: true`
- [x] `context.json` written
- [x] Ontology scope mapping complete (or skipped)

**NEXT:** Proceed to Phase 1.

---

## Phase 1: Spawn Analysts (Finding Phase)

**This is the FINDING phase. Analysts investigate and write findings only. No verification happens here.**

### Files read at this step (per analyst):

For each of the 3 approved perspectives, the orchestrator reads:

1. **Archetype prompt file** (the section matching the perspective ID)
2. **Finding protocol:** `skills/analyze/prompts/finding-protocol.md`
3. **Worker preamble:** `skills/shared-v3/worker-preamble.md`

### CRITICAL DETAIL: What protocol file gets concatenated to analyst prompts in Phase 1?

**Answer: `prompts/finding-protocol.md` is concatenated, NOT `prompts/verification-protocol.md`.**

Per SKILL.md Phase 1, Step 1.1, prompt assembly order:
> 1. Read archetype section from `prompts/core-archetypes.md` or `prompts/extended-archetypes.md`
> 2. Read `prompts/finding-protocol.md`
> 3. Concatenate: `[worker preamble] + [archetype prompt] + [finding protocol]`

The finding protocol instructs:
- Investigate and answer key questions with evidence
- Write findings to `~/.prism/state/analyze-{SHORT_ID}/perspectives/{perspective-id}/findings.json`
- Report via SendMessage to team-lead
- **"Do NOT run self-verification -- that happens in a separate session."**

### Analyst 1: root-cause-analyst

**Files read:**
- `prompts/core-archetypes.md` section "Root Cause Lens"
- `prompts/finding-protocol.md`
- `shared-v3/worker-preamble.md`

**Agent name:** `root-cause-analyst`
**Subagent type:** `oh-my-claudecode:architect`
**Model:** `opus`

**Prompt assembly:**
```
[Worker Preamble]
  {TEAM_NAME} = "analyze-a1b2c3d4"
  {WORKER_NAME} = "root-cause-analyst"
  {WORK_ACTION} = "Investigate from your assigned perspective. Answer ALL key questions
    with evidence and code references. Write findings to findings.json. Report findings
    via SendMessage to team-lead. Do NOT run self-verification -- that happens in a
    separate session."

[Root Cause Lens prompt from core-archetypes.md]
  {CONTEXT} = contents of context.json
  {ONTOLOGY_SCOPE} = ontology scope text or "N/A"
  {SHORT_ID} = "a1b2c3d4"

[Finding Protocol from finding-protocol.md]
  {SHORT_ID} = "a1b2c3d4"
  perspective-id = "root-cause"
```

**Spawn:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="root-cause-analyst",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

**Findings output path:** `~/.prism/state/analyze-a1b2c3d4/perspectives/root-cause/findings.json`

### Analyst 2: impact-analyst

**Files read:**
- `prompts/core-archetypes.md` section "Impact Lens"
- `prompts/finding-protocol.md`
- `shared-v3/worker-preamble.md`

**Agent name:** `impact-analyst`
**Subagent type:** `oh-my-claudecode:architect-medium`
**Model:** `sonnet`

**Prompt assembly:** Same pattern, with:
- `{WORKER_NAME}` = `"impact-analyst"`
- Impact Lens archetype prompt
- `perspective-id` = `"impact"`

**Findings output path:** `~/.prism/state/analyze-a1b2c3d4/perspectives/impact/findings.json`

### Analyst 3: data-integrity-analyst

**Files read:**
- `prompts/extended-archetypes.md` section "Data Integrity Lens"
- `prompts/finding-protocol.md`
- `shared-v3/worker-preamble.md`

**Agent name:** `data-integrity-analyst`
**Subagent type:** `oh-my-claudecode:architect`
**Model:** `opus`

**Prompt assembly:** Same pattern, with:
- `{WORKER_NAME}` = `"data-integrity-analyst"`
- Data Integrity Lens archetype prompt
- `perspective-id` = `"data-integrity"`

**Findings output path:** `~/.prism/state/analyze-a1b2c3d4/perspectives/data-integrity/findings.json`

### All 3 spawned in parallel with `run_in_background=true`.

### Phase 1 Exit Gate

- [x] All 3 analyst tasks created and owners pre-assigned
- [x] All 3 analysts spawned in parallel

**NEXT:** Read `docs/later-phases.md` and proceed to Phase 2.

---

## Phase 2: Collect Findings & Spawn Verification Sessions

**File read at this point:** `skills/analyze/docs/later-phases.md` (read ONLY when entering Phase 2, per SKILL.md instruction).

Phase 2 has **two distinct stages**: Stage A and Stage B.

---

### Stage A: Collect Findings

#### Step 2A.1: Wait for Analyst Findings

Monitor via `TaskList`. Wait for all 3 finding analysts to:
1. Write their `findings.json` files
2. Send findings to team-lead via `SendMessage`

Expected `SendMessage` receipts:
- From `root-cause-analyst`: findings about NullPointerException code path
- From `impact-analyst`: findings about affected users and business impact
- From `data-integrity-analyst`: findings about payment-point data consistency

#### Step 2A.2: Drain Background Task Outputs

`TaskList` -> `TaskOutput` for each completed task (#27431 workaround).

#### Step 2A.3: Shutdown Finding Analysts

**For each of the 3 analysts:**

```
SendMessage(type: "shutdown_request", recipient: "root-cause-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "impact-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "data-integrity-analyst", content: "Finding phase complete.")
```

Wait for shutdown acknowledgment, then drain task outputs again.

#### Stage A Exit Gate

- [x] All 3 analyst findings received via `SendMessage`
- [x] All 3 `findings.json` files written:
  - `~/.prism/state/analyze-a1b2c3d4/perspectives/root-cause/findings.json`
  - `~/.prism/state/analyze-a1b2c3d4/perspectives/impact/findings.json`
  - `~/.prism/state/analyze-a1b2c3d4/perspectives/data-integrity/findings.json`
- [x] All 3 finding analysts shut down
- [x] All background task outputs drained

**CRITICAL STATE TRANSITION:** All finding sessions are now TERMINATED. The finding analyst agents no longer exist. Stage B spawns entirely NEW agent sessions.

**NEXT:** Proceed to Stage B.

---

### Stage B: Spawn Verification Sessions

#### Step 2B.1: Spawn Verification Sessions

**Files read at this step (per verifier):**

For each of the 3 perspectives, the orchestrator reads:

1. **Archetype prompt file** (SAME archetype section as Phase 1)
2. **Verification protocol:** `skills/analyze/prompts/verification-protocol.md` (NOT finding-protocol.md)
3. **Worker preamble:** `skills/shared-v3/worker-preamble.md`

### CRITICAL DETAIL: Protocol file difference between Phase 1 and Phase 2

| Phase | Protocol File Concatenated | Purpose |
|-------|---------------------------|---------|
| Phase 1 (Finding) | `prompts/finding-protocol.md` | Investigate, write findings, do NOT self-verify |
| Phase 2 Stage B (Verification) | `prompts/verification-protocol.md` | Read findings.json, run `prism_interview` MCP loop, report verified findings |

### CRITICAL DETAIL: Finding sessions and Verification sessions are SEPARATE

**Yes, they are completely separate agent sessions:**

1. **Finding sessions** (Phase 1): Named `{perspective-id}-analyst` (e.g., `root-cause-analyst`). These agents investigate, write `findings.json`, then are SHUT DOWN in Stage A.
2. **Verification sessions** (Phase 2 Stage B): Named `{perspective-id}-verifier` (e.g., `root-cause-verifier`). These are NEW agent spawns that READ the `findings.json` written by the finding sessions, then run MCP-based Socratic verification (`prism_interview`).

The verification protocol explicitly states: "You are the same analyst who produced findings in a previous session. Your findings are saved at: `~/.prism/state/analyze-{SHORT_ID}/perspectives/{perspective-id}/findings.json`. Read this file first to recall your findings before starting verification."

This means the verification agent is a fresh agent that must READ the artifact file to "recall" what was found -- it does NOT have the finding session's context in memory.

### Agent Name Summary

| Perspective | Finding Session Agent Name | Verification Session Agent Name |
|-------------|---------------------------|-------------------------------|
| root-cause | `root-cause-analyst` | `root-cause-verifier` |
| impact | `impact-analyst` | `impact-verifier` |
| data-integrity | `data-integrity-analyst` | `data-integrity-verifier` |

### Verifier 1: root-cause-verifier

**Files read:**
- `prompts/core-archetypes.md` section "Root Cause Lens"
- `prompts/verification-protocol.md`
- `shared-v3/worker-preamble.md`

**Agent name:** `root-cause-verifier`
**Subagent type:** `oh-my-claudecode:architect`
**Model:** `opus`

**Prompt assembly:**
```
[Worker Preamble]
  {TEAM_NAME} = "analyze-a1b2c3d4"
  {WORKER_NAME} = "root-cause-verifier"
  {WORK_ACTION} = "Read your findings from findings.json. Run self-verification via
    MCP tools (prism_interview). Re-investigate with tools as needed to answer interview
    questions. Report verified findings via SendMessage to team-lead."

[Root Cause Lens prompt from core-archetypes.md]
  {CONTEXT} = contents of context.json
  {ONTOLOGY_SCOPE} = ontology scope text or "N/A"
  {SHORT_ID} = "a1b2c3d4"

[Verification Protocol from verification-protocol.md]
  {SHORT_ID} = "a1b2c3d4"
  perspective-id = "root-cause"
```

**Spawn:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="root-cause-verifier",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

**What the verifier does (governed by verification-protocol.md):**
1. Read `~/.prism/state/analyze-a1b2c3d4/perspectives/root-cause/findings.json`
2. Start interview: `mcp__prism-mcp__prism_interview(context_id="analyze-a1b2c3d4", perspective_id="root-cause", topic="root-cause findings verification...")`
3. Answer + Integrated Score Loop: answer questions, re-investigate if needed, submit responses
4. Loop until `continue: false`
5. Report verified findings via `SendMessage` with rounds, score, verdict

### Verifier 2: impact-verifier

**Files read:**
- `prompts/core-archetypes.md` section "Impact Lens"
- `prompts/verification-protocol.md`
- `shared-v3/worker-preamble.md`

**Agent name:** `impact-verifier`
**Subagent type:** `oh-my-claudecode:architect-medium`
**Model:** `sonnet`

Same pattern with `perspective-id` = `"impact"`.

### Verifier 3: data-integrity-verifier

**Files read:**
- `prompts/extended-archetypes.md` section "Data Integrity Lens"
- `prompts/verification-protocol.md`
- `shared-v3/worker-preamble.md`

**Agent name:** `data-integrity-verifier`
**Subagent type:** `oh-my-claudecode:architect`
**Model:** `opus`

Same pattern with `perspective-id` = `"data-integrity"`.

### All 3 verifiers spawned in parallel with `run_in_background=true`.

#### Step 2B.2: Wait for Verified Findings

Monitor via `TaskList`. Each verifier sends `SendMessage` with:
- context_id, perspective_id, rounds, score, verdict (PASS/FORCE PASS)
- Verified findings refined through Q&A
- Key Q&A clarifications

#### Step 2B.3: Drain Background Task Outputs

`TaskList` -> `TaskOutput` for each completed task.

#### Step 2B.4: Shutdown Verifiers

```
SendMessage(type: "shutdown_request", recipient: "root-cause-verifier", content: "Verification complete.")
SendMessage(type: "shutdown_request", recipient: "impact-verifier", content: "Verification complete.")
SendMessage(type: "shutdown_request", recipient: "data-integrity-verifier", content: "Verification complete.")
```

#### Step 2B.5: Persist Verified Results

Write verified findings for each perspective:
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-root-cause.md`
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-impact.md`
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-data-integrity.md`

#### Step 2B.6: Compile Verified Findings

Write compiled findings to `~/.prism/state/analyze-a1b2c3d4/analyst-findings.md` containing:
- All verified findings from all 3 perspectives
- Ambiguity scores summary table (perspective_id, goal, constraints, criteria, weighted_total, verdict)
- FORCE PASS flags if any

### Phase 2 Exit Gate

- [x] All 3 verifiers completed and sent verified findings
- [x] All background task outputs drained
- [x] All 3 verifiers shut down
- [x] All verified findings persisted
- [x] Compiled findings written to `analyst-findings.md`

**NEXT:** Proceed to Phase 3.

---

## Phase 3: Synthesis & Report

**Files read:**
- `~/.prism/state/analyze-a1b2c3d4/analyst-findings.md`
- `skills/analyze/templates/report.md`

### Step 3.1: Integrate all verified findings from `analyst-findings.md`.

### Step 3.2: Read `templates/report.md` template, fill all sections.

### Step 3.3: `AskUserQuestion` — "Is the analysis complete?"
- Options: Complete / Need deeper investigation / Add recommendations / Share with team

If "Need deeper investigation" -> re-entry loop (max 2), appending to `prior-iterations.md`.

---

## Phase 4: Cleanup

**File read:** `skills/shared-v3/team-teardown.md`

Execute team teardown to clean up the `analyze-a1b2c3d4` team.

---

## Summary: Complete File Reference Map

### Files read by orchestrator at each phase:

| Phase | Files Read |
|-------|-----------|
| Prerequisite | `shared-v3/prerequisite-gate.md`, `~/.claude/settings.json` |
| Phase 0 | (none -- inline) |
| Phase 0.5 | `prompts/seed-analyst.md`, `shared-v3/worker-preamble.md` |
| Phase 0.55 | `prompts/perspective-generator.md`, `shared-v3/worker-preamble.md` |
| Phase 0.6 | `~/.prism/state/analyze-{id}/perspectives.json` |
| Phase 0.7 | `shared-v3/ontology-scope-mapping.md` |
| Phase 0.8 | `~/.prism/state/analyze-{id}/seed-analysis.json` |
| Phase 1 | Per analyst: archetype from `prompts/core-archetypes.md` or `prompts/extended-archetypes.md` + `prompts/finding-protocol.md` + `shared-v3/worker-preamble.md` |
| Phase 2 entry | `docs/later-phases.md` |
| Phase 2 Stage B | Per verifier: archetype from `prompts/core-archetypes.md` or `prompts/extended-archetypes.md` + `prompts/verification-protocol.md` + `shared-v3/worker-preamble.md` |
| Phase 3 | `templates/report.md`, `analyst-findings.md` |
| Phase 4 | `shared-v3/team-teardown.md` |

### Protocol concatenation difference:

| Session Type | Archetype Prompt | + Protocol File |
|-------------|-----------------|-----------------|
| Finding (Phase 1) | core/extended-archetypes.md | **finding-protocol.md** |
| Verification (Phase 2B) | core/extended-archetypes.md (same) | **verification-protocol.md** |

### Agent naming convention:

| Session Type | Name Pattern | Examples |
|-------------|-------------|----------|
| Seed Analyst | `seed-analyst` | `seed-analyst` |
| Perspective Generator | `perspective-generator` | `perspective-generator` |
| Finding Analysts | `{perspective-id}-analyst` | `root-cause-analyst`, `impact-analyst`, `data-integrity-analyst` |
| Verification Sessions | `{perspective-id}-verifier` | `root-cause-verifier`, `impact-verifier`, `data-integrity-verifier` |

### Session separation confirmation:

Finding sessions and verification sessions are **completely separate agent instances**. The finding agents are shut down (Stage A, Step 2A.3) BEFORE the verification agents are spawned (Stage B, Step 2B.1). The verification agents recover context by reading the `findings.json` artifact file from disk, not from shared memory or session state. This is a deliberate design for hallucination detection -- the verifier cannot "remember" things that were not actually written to the findings file.
