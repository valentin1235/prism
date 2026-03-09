# Execution Trace: Chat Timeout 504 / DB Connection Pool Exhaustion

**Task:** "최근 배포 이후 앱에서 채팅 메시지 전송 시 간헐적으로 504 타임아웃이 발생하고 있어. 서버 로그에서 DB connection pool exhaustion 로그도 보여. 원인 분석해줘"

**Skill version:** 4.1.0 (`/Users/heechul/prism/skills/analyze/SKILL.md`)

---

## Prerequisite: Agent Team Mode (HARD GATE)

**File read:** `/Users/heechul/prism/skills/shared-v3/prerequisite-gate.md`

- Orchestrator reads `~/.claude/settings.json`
- Checks `env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS === "1"`
- `{PROCEED_TO}` = "Phase 0"
- If gate passes, proceed to Phase 0. If not, HARD STOP with setup instructions.

---

## Phase 0: Problem Intake

**Files read:** None (orchestrator handles directly per SKILL.md line 43)

### Step 0.1: Collect Description

User provided description via `$ARGUMENTS`, so it is used directly:
> "최근 배포 이후 앱에서 채팅 메시지 전송 시 간헐적으로 504 타임아웃이 발생하고 있어. 서버 로그에서 DB connection pool exhaustion 로그도 보여. 원인 분석해줘"

No `AskUserQuestion` needed.

### Step 0.2: Generate Session ID and State Directory

```bash
uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8
# Example output: "a1b2c3d4"
mkdir -p ~/.prism/state/analyze-a1b2c3d4
```

`{short-id}` = `a1b2c3d4` (reused throughout all phases)

### Phase 0 Exit Gate

- [x] Description collected (from $ARGUMENTS)
- [x] `{short-id}` generated and state directory created

Proceed to Phase 0.5.

---

## Phase 0.5: Team Creation & Seed Analysis

### Step 0.5.1: Create Team

```
TeamCreate(team_name: "analyze-a1b2c3d4", description: "Analysis: 채팅 메시지 504 타임아웃 + DB connection pool exhaustion")
```

### Step 0.5.2: Spawn Seed Analyst

**Files read at this step:**
1. `/Users/heechul/prism/skills/shared-v3/worker-preamble.md` -- worker preamble template
2. `/Users/heechul/prism/skills/analyze/prompts/seed-analyst.md` -- seed analyst prompt

**Prompt assembly:**
```
[Worker Preamble with:
  {TEAM_NAME} = "analyze-a1b2c3d4"
  {WORKER_NAME} = "seed-analyst"
  {WORK_ACTION} = "Actively investigate using available tools (Grep, Read, Bash, MCP). Evaluate dimensions and severity. Write findings to seed-analysis.json. Report via SendMessage."
]
+
[seed-analyst.md prompt with:
  {DESCRIPTION} = "최근 배포 이후 앱에서 채팅 메시지 전송 시 간헐적으로 504 타임아웃이 발생하고 있어. 서버 로그에서 DB connection pool exhaustion 로그도 보여. 원인 분석해줘"
  {SHORT_ID} = "a1b2c3d4"
]
```

**Spawn call:**
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

**Seed analyst would:**
- Grep codebase for "connection pool", "504", "timeout", chat message sending code paths
- Read DB connection config files, chat service source code
- `git log --oneline --since="7 days ago"` to find recent deployment changes
- ToolSearch for Grafana/Sentry/Loki MCP tools if available
- Write `~/.prism/state/analyze-a1b2c3d4/seed-analysis.json` with:
  - severity: likely SEV2 (user-facing intermittent failure)
  - status: Active
  - dimensions: { domain: "app", failure_type: "degradation", evidence_available: ["code diffs", "git history", "source code", "logs"], complexity: "multi-factor", recurrence: "first-time" }
  - research findings with file:function:line references

### Step 0.5.3-0.5.5: Receive, Shutdown, Drain

- Wait for SendMessage from seed-analyst
- SendMessage(type: "shutdown_request", recipient: "seed-analyst", content: "Seed analysis complete.")
- TaskList -> TaskOutput for all completed tasks

### Phase 0.5 Exit Gate

- [x] Team created
- [x] Seed-analyst results received
- [x] seed-analysis.json written
- [x] Seed-analyst shut down
- [x] Background task outputs drained

---

## Phase 0.55: Perspective Generation

**Files read at this step:**
1. `/Users/heechul/prism/skills/shared-v3/worker-preamble.md`
2. `/Users/heechul/prism/skills/analyze/prompts/perspective-generator.md`

**Prompt assembly:**
```
[Worker Preamble with:
  {TEAM_NAME} = "analyze-a1b2c3d4"
  {WORKER_NAME} = "perspective-generator"
  {WORK_ACTION} = "Read seed-analysis.json, apply archetype mapping rules and mandatory rules, generate perspective candidates. Write perspectives.json. Report via SendMessage."
]
+
[perspective-generator.md prompt with:
  {SHORT_ID} = "a1b2c3d4"
  {DESCRIPTION} = <original user description>
]
```

**Spawn call:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="perspective-generator",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

**Perspective generator would apply archetype mapping:**

Given the characteristics (latency spike, resource exhaustion, post-deployment failure), the mapping table in perspective-generator.md recommends:
- "Latency spike, OOM, resource exhaustion" -> `performance` + `root-cause` + `systems`
- "Post-deployment failure, config drift" -> `deployment` + `timeline` + `root-cause`

**Mandatory rules applied:**
1. Core archetype required: `root-cause` included (yes)
2. Recurring -> systems: recurrence is "first-time", so N/A
3. Evidence-backed only: all perspectives backed by seed findings
4. Minimum perspectives: >= 2 (yes)
5. Complexity scaling: "multi-factor" -> 3-5 perspectives

**Likely perspective selection (written to `~/.prism/state/analyze-a1b2c3d4/perspectives.json`):**

| ID | Lens | Model | Agent Type |
|----|------|-------|------------|
| `root-cause` | Root Cause | opus | architect |
| `performance` | Performance & Capacity | sonnet | architect-medium |
| `systems` | Systems & Architecture | opus | architect |
| `timeline` | Timeline | sonnet | architect-medium |

(4 perspectives for multi-factor complexity)

### Step 0.55.3-0.55.4: Shutdown, Drain

- SendMessage(type: "shutdown_request", recipient: "perspective-generator")
- TaskList -> TaskOutput drain

---

## Phase 0.6: Perspective Approval

**File read:** `~/.prism/state/analyze-a1b2c3d4/perspectives.json`

Orchestrator presents perspectives to user with AskUserQuestion. User selects "Proceed".

Orchestrator updates perspectives.json with `"approved": true, "user_modifications": []`.

---

## Phase 0.7: Ontology Scope Mapping

**File read:** `/Users/heechul/prism/skills/shared-v3/ontology-scope-mapping.md`

Parameters:
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"analysis"`
- `{STATE_DIR}` = `~/.prism/state/analyze-a1b2c3d4`

Execution:
1. ToolSearch -> load `mcp__prism-mcp__prism_docs_roots`
2. Call `prism_docs_roots` to discover ontology doc directories
3. ToolSearch for MCP servers (grafana, sentry, clickhouse, etc.) -> present to user
4. AskUserQuestion for external sources
5. AskUserQuestion for pool confirmation
6. Write `~/.prism/state/analyze-a1b2c3d4/ontology-scope.json`

---

## Phase 0.8: Context & State Files

**Files read:** `~/.prism/state/analyze-a1b2c3d4/seed-analysis.json` (for research summary)

**File written:** `~/.prism/state/analyze-a1b2c3d4/context.json`:
```json
{
  "summary": "최근 배포 이후 채팅 메시지 전송 시 간헐적 504 타임아웃 발생. 서버 로그에 DB connection pool exhaustion 확인.",
  "research_summary": {
    "key_findings": ["DB connection pool exhaustion in server logs", "504 timeout on chat message send API", "Recent deployment correlated with onset"],
    "files_examined": ["<paths from seed analysis>"],
    "dimensions": "domain: app, failure_type: degradation, complexity: multi-factor, recurrence: first-time"
  },
  "report_language": "Korean"
}
```

Note: `report_language` detected as Korean from the user's input.

### Phase 0.8 Exit Gate

- [x] perspectives.json has approved=true
- [x] context.json written
- [x] Ontology scope mapping complete

---

## Phase 1: Spawn Analysts (Finding Phase)

**This is the FINDING phase. Analysts investigate and write findings only. No verification happens here.**

### Files read at this step (per analyst):

For EACH of the 4 analysts, the orchestrator reads:
1. **Worker preamble:** `/Users/heechul/prism/skills/shared-v3/worker-preamble.md`
2. **Archetype prompt:** the relevant section from the archetype file
3. **Finding protocol:** `/Users/heechul/prism/skills/analyze/prompts/finding-protocol.md` **<-- THIS IS THE PROTOCOL CONCATENATED TO EACH ANALYST PROMPT**

### Prompt assembly order (per analyst):

```
[worker preamble] + [archetype prompt section] + [finding-protocol.md]
```

Then replace placeholders: `{CONTEXT}`, `{ONTOLOGY_SCOPE}`, `{SHORT_ID}`

### Analyst spawn details:

#### 1. Root Cause Analyst (Finding Session)

- **Archetype file read:** `/Users/heechul/prism/skills/analyze/prompts/core-archetypes.md` section "Root Cause Lens"
- **Protocol file concatenated:** `/Users/heechul/prism/skills/analyze/prompts/finding-protocol.md`
- **Agent name:** `root-cause-analyst`
- **Model:** opus
- **Subagent type:** `oh-my-claudecode:architect`
- **Worker preamble {WORK_ACTION}:** "Investigate from your assigned perspective. Answer ALL key questions with evidence and code references. Write findings to findings.json. Report findings via SendMessage to team-lead. Do NOT run self-verification -- that happens in a separate session."
- **Findings path:** `~/.prism/state/analyze-a1b2c3d4/perspectives/root-cause/findings.json`

#### 2. Performance Analyst (Finding Session)

- **Archetype file read:** `/Users/heechul/prism/skills/analyze/prompts/extended-archetypes.md` section "Performance Lens"
- **Protocol file concatenated:** `/Users/heechul/prism/skills/analyze/prompts/finding-protocol.md`
- **Agent name:** `performance-analyst`
- **Model:** sonnet
- **Subagent type:** `oh-my-claudecode:architect-medium`
- **Findings path:** `~/.prism/state/analyze-a1b2c3d4/perspectives/performance/findings.json`

#### 3. Systems Analyst (Finding Session)

- **Archetype file read:** `/Users/heechul/prism/skills/analyze/prompts/core-archetypes.md` section "Systems Lens"
- **Protocol file concatenated:** `/Users/heechul/prism/skills/analyze/prompts/finding-protocol.md`
- **Agent name:** `systems-analyst`
- **Model:** opus
- **Subagent type:** `oh-my-claudecode:architect`
- **Findings path:** `~/.prism/state/analyze-a1b2c3d4/perspectives/systems/findings.json`

#### 4. Timeline Analyst (Finding Session)

- **Archetype file read:** `/Users/heechul/prism/skills/analyze/prompts/core-archetypes.md` section "Timeline Lens"
- **Protocol file concatenated:** `/Users/heechul/prism/skills/analyze/prompts/finding-protocol.md`
- **Agent name:** `timeline-analyst`
- **Model:** sonnet
- **Subagent type:** `oh-my-claudecode:architect-medium`
- **Findings path:** `~/.prism/state/analyze-a1b2c3d4/perspectives/timeline/findings.json`

### Key detail: finding-protocol.md governs all finding sessions

The finding protocol (`prompts/finding-protocol.md`) is concatenated to EVERY analyst prompt in Phase 1. It instructs:
1. Data Source Constraint -- only use data sources listed in "Reference Documents" (i.e., `{ONTOLOGY_SCOPE}`)
2. Task Lifecycle -- TaskGet -> in_progress -> investigate -> write findings -> SendMessage -> completed
3. Investigation steps: Investigate -> Write findings.json -> Report via SendMessage
4. Explicitly states: "Do NOT run self-verification (prism_interview) -- that happens in a separate session."

All 4 analysts are spawned in parallel with `run_in_background=true`.

### Phase 1 Exit Gate

- [x] All 4 analyst tasks created with owners pre-assigned
- [x] All 4 analysts spawned in parallel

---

## Phase 2: Collect Findings & Spawn Verification Sessions

**File read at Phase 2 entry:** `/Users/heechul/prism/skills/analyze/docs/later-phases.md`

This file is read ONLY when entering Phase 2 (per SKILL.md line 12: "Read that file ONLY when entering Phase 2.").

---

### Stage A: Collect Findings and Shut Down Finding Sessions

#### Step 2A.1: Wait for Analyst Findings

Orchestrator monitors via `TaskList`. Waits for all 4 analysts to:
1. Write their `findings.json` to their respective paths
2. Send findings to team-lead via `SendMessage`

Expected SendMessage from each analyst (per finding-protocol.md):
```markdown
## Findings -- {perspective-id}

### Session
- context_id: analyze-a1b2c3d4
- perspective_id: {perspective-id}

### Findings
{findings with evidence}
```

#### Step 2A.2: Drain Background Task Outputs

For each completed analyst task: `TaskList` -> `TaskOutput`

#### Step 2A.3: Shutdown Finding Analysts

For each of the 4 finding analysts:
```
SendMessage(type: "shutdown_request", recipient: "root-cause-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "performance-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "systems-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "timeline-analyst", content: "Finding phase complete.")
```

Wait for shutdown acknowledgment, then drain task outputs again.

#### Stage A Exit Gate

- [x] All 4 analyst findings received via SendMessage
- [x] All 4 findings.json files written
- [x] All 4 finding analysts shut down
- [x] All background task outputs drained

**Key observation: All finding sessions are TERMINATED before Stage B begins. Finding and verification are strictly sequential stages.**

---

### Stage B: Spawn Verification Sessions (NEW, SEPARATE sessions)

#### Step 2B.1: Spawn Verification Sessions

**For EACH perspective, a NEW agent session is spawned.** These are entirely separate from the finding sessions that were just shut down.

**Files read at this step (per verifier):**
1. **Worker preamble:** `/Users/heechul/prism/skills/shared-v3/worker-preamble.md`
2. **Archetype prompt:** Same archetype section as Phase 1 (same file, same section)
3. **Verification protocol:** `/Users/heechul/prism/skills/analyze/prompts/verification-protocol.md` **<-- THIS REPLACES finding-protocol.md**

**Prompt assembly order (per verifier):**
```
[worker preamble] + [archetype prompt section] + [verification-protocol.md]
```

Then replace placeholders: `{CONTEXT}`, `{ONTOLOGY_SCOPE}`, `{SHORT_ID}`

#### Verifier spawn details:

##### 1. Root Cause Verifier (Verification Session)

- **Archetype file read:** `/Users/heechul/prism/skills/analyze/prompts/core-archetypes.md` section "Root Cause Lens"
- **Protocol file concatenated:** `/Users/heechul/prism/skills/analyze/prompts/verification-protocol.md`
- **Agent name:** `root-cause-verifier`
- **Model:** opus
- **Subagent type:** `oh-my-claudecode:architect`
- **Worker preamble {WORK_ACTION}:** "Read your findings from findings.json. Run self-verification via MCP tools (prism_interview). Re-investigate with tools as needed to answer interview questions. Report verified findings via SendMessage to team-lead."

##### 2. Performance Verifier (Verification Session)

- **Archetype file read:** `/Users/heechul/prism/skills/analyze/prompts/extended-archetypes.md` section "Performance Lens"
- **Protocol file concatenated:** `/Users/heechul/prism/skills/analyze/prompts/verification-protocol.md`
- **Agent name:** `performance-verifier`
- **Model:** sonnet
- **Subagent type:** `oh-my-claudecode:architect-medium`

##### 3. Systems Verifier (Verification Session)

- **Archetype file read:** `/Users/heechul/prism/skills/analyze/prompts/core-archetypes.md` section "Systems Lens"
- **Protocol file concatenated:** `/Users/heechul/prism/skills/analyze/prompts/verification-protocol.md`
- **Agent name:** `systems-verifier`
- **Model:** opus
- **Subagent type:** `oh-my-claudecode:architect`

##### 4. Timeline Verifier (Verification Session)

- **Archetype file read:** `/Users/heechul/prism/skills/analyze/prompts/core-archetypes.md` section "Timeline Lens"
- **Protocol file concatenated:** `/Users/heechul/prism/skills/analyze/prompts/verification-protocol.md`
- **Agent name:** `timeline-verifier`
- **Model:** sonnet
- **Subagent type:** `oh-my-claudecode:architect-medium`

All 4 verifiers spawned in parallel with `run_in_background=true`.

#### What each verification session does (governed by verification-protocol.md):

1. **Read findings:** Read `~/.prism/state/analyze-a1b2c3d4/perspectives/{perspective-id}/findings.json` (written by the finding session)
2. **Start interview:** Call `mcp__prism-mcp__prism_interview(context_id="analyze-a1b2c3d4", perspective_id="{perspective-id}", topic="{perspective-id} findings verification -- ...")`
3. **Answer + Score loop:** For each question from the MCP interviewer, answer with evidence, submit, check continue flag
4. **Report:** Send verified findings to team-lead via SendMessage with session metadata (rounds, score, verdict)

#### Step 2B.2-2B.4: Wait, Drain, Shutdown

- Wait for all 4 verifiers to complete
- TaskList -> TaskOutput drain
- SendMessage shutdown_request to each verifier:
  - `root-cause-verifier`, `performance-verifier`, `systems-verifier`, `timeline-verifier`

#### Step 2B.5: Persist Verified Results

For each perspective, write:
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-root-cause.md`
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-performance.md`
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-systems.md`
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-timeline.md`

#### Step 2B.6: Compile Verified Findings

Write `~/.prism/state/analyze-a1b2c3d4/analyst-findings.md` with all verified findings and ambiguity scores summary table.

### Phase 2 Exit Gate

- [x] All 4 verifiers completed and sent verified findings
- [x] All background task outputs drained
- [x] All 4 verifiers shut down
- [x] All verified findings persisted
- [x] Compiled findings written to analyst-findings.md

---

## Summary of Key Questions

### 1. What prompt files are read at each phase?

| Phase | Files Read |
|-------|-----------|
| Prerequisite | `shared-v3/prerequisite-gate.md` |
| Phase 0 | None (orchestrator handles directly) |
| Phase 0.5 | `shared-v3/worker-preamble.md`, `prompts/seed-analyst.md` |
| Phase 0.55 | `shared-v3/worker-preamble.md`, `prompts/perspective-generator.md` |
| Phase 0.6 | `~/.prism/state/analyze-{id}/perspectives.json` (artifact, not prompt file) |
| Phase 0.7 | `shared-v3/ontology-scope-mapping.md` |
| Phase 0.8 | `~/.prism/state/analyze-{id}/seed-analysis.json` (artifact) |
| Phase 1 | `shared-v3/worker-preamble.md`, `prompts/core-archetypes.md` or `prompts/extended-archetypes.md` (per analyst), **`prompts/finding-protocol.md`** |
| Phase 2 entry | `docs/later-phases.md` |
| Phase 2 Stage B | `shared-v3/worker-preamble.md`, `prompts/core-archetypes.md` or `prompts/extended-archetypes.md` (per verifier), **`prompts/verification-protocol.md`** |

### 2. What protocol file gets concatenated to analyst prompts in Phase 1?

**`/Users/heechul/prism/skills/analyze/prompts/finding-protocol.md`**

It is appended after the archetype prompt section for every analyst in Phase 1. The assembly order is:
```
[worker preamble] + [archetype section] + [finding-protocol.md]
```

This protocol explicitly forbids self-verification: "Do NOT run self-verification (prism_interview) -- that happens in a separate session."

### 3. The exact Phase 2 flow

**Stage A (Collecting findings, shutting down):**
1. Wait for all finding analysts to complete (TaskList monitoring)
2. Receive findings via SendMessage from each analyst
3. Drain background task outputs (TaskList -> TaskOutput)
4. Shutdown each finding analyst via SendMessage(type: "shutdown_request")
5. Drain again after shutdown acknowledgment
6. Verify all findings.json files exist

**Stage B (Spawning verification sessions):**
1. For each perspective, spawn a NEW verification session (new Task with new agent)
2. Each verifier reads findings.json from disk (written by the now-terminated finding session)
3. Each verifier runs MCP prism_interview loop (Socratic verification with integrated scoring)
4. Wait for all verifiers to complete
5. Drain background task outputs
6. Shutdown each verifier
7. Persist verified findings (per-perspective .md files)
8. Compile all verified findings into analyst-findings.md

### 4. Are finding sessions and verification sessions separate?

**Yes, they are strictly separate.**

- Finding sessions (Phase 1) are spawned, run to completion, and then **shut down** in Stage A of Phase 2.
- Verification sessions (Stage B of Phase 2) are spawned AFTER all finding sessions are terminated.
- The two session types share data only through the filesystem: `findings.json` written by finding sessions is read by verification sessions.
- The verification protocol (`verification-protocol.md` line 13) states: "You are the same analyst who produced findings in a previous session."
- A finding session and its corresponding verification session are different agent instances. They are NOT the same running process.

### 5. Agent names for finding vs verification sessions

| Perspective | Finding Session Agent Name | Verification Session Agent Name |
|-------------|---------------------------|--------------------------------|
| root-cause | `root-cause-analyst` | `root-cause-verifier` |
| performance | `performance-analyst` | `performance-verifier` |
| systems | `systems-analyst` | `systems-verifier` |
| timeline | `timeline-analyst` | `timeline-verifier` |

The naming convention is:
- Finding: `{perspective-id}-analyst` (from SKILL.md Phase 1 spawn pattern, line 309)
- Verification: `{perspective-id}-verifier` (from later-phases.md Stage B spawn pattern, line 70)

---

## File Reference Index

All paths relative to `/Users/heechul/prism/skills/analyze/`:

| File | Role |
|------|------|
| `SKILL.md` | Main orchestration (Phases 0 through 1, gate summary) |
| `docs/later-phases.md` | Phases 2 through 4 |
| `prompts/seed-analyst.md` | Seed analyst prompt (Phase 0.5) |
| `prompts/perspective-generator.md` | Perspective generation prompt (Phase 0.55) |
| `prompts/core-archetypes.md` | Timeline, Root Cause, Systems, Impact archetype prompts |
| `prompts/extended-archetypes.md` | Performance, Security, Data Integrity, UX, Deployment, Network, Concurrency, Dependency, Financial, Custom archetype prompts |
| `prompts/finding-protocol.md` | Protocol appended to Phase 1 analyst prompts (investigate + write findings, NO verification) |
| `prompts/verification-protocol.md` | Protocol appended to Phase 2 Stage B verifier prompts (read findings + MCP interview loop) |
| `../shared-v3/worker-preamble.md` | Common worker preamble template |
| `../shared-v3/prerequisite-gate.md` | Agent Team Mode hard gate |
| `../shared-v3/ontology-scope-mapping.md` | Ontology pool building (Phase 0.7) |
| `../shared-v3/team-teardown.md` | Phase 4 cleanup |
