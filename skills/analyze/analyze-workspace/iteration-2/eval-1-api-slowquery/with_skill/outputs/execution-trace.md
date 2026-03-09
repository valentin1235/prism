# Execution Trace: eval-1-api-slowquery

**Skill version:** 4.1.0
**Task (Korean):** "우리 서비스에서 API 응답이 평균 3초 이상 걸리고, DB 슬로우쿼리 로그가 대량 발생하고 있어. 특히 /api/v1/rooms 엔드포인트가 심각해. 최근 인덱스 관련 마이그레이션을 했는데 그 이후부터 발생한 것 같아. 분석해줘"
**Task (English translation):** "Our service API responses take 3+ seconds on average, and massive DB slow query logs are appearing. The /api/v1/rooms endpoint is particularly bad. We recently ran an index-related migration and the issue seems to have started after that. Analyze this."
**Evaluator:** architect (opus)
**Date:** 2026-03-10

---

## Data Flow Diagram

```
User Input (Korean)
    │
    ▼
[Phase 0: Intake] ──────────────────────────────► description, {short-id}, state dir
    │
    ▼
[Phase 0.5: Seed Analysis] ─────────────────────► seed-analysis.json
    │                                                │
    ▼                                                ▼
[Phase 0.55: Perspective Generation] ◄── reads ── seed-analysis.json
    │                                                │
    ▼                                                ▼
[Phase 0.6: Approval] ──────────────────────────► perspectives.json (+ approved: true)
    │
    ▼
[Phase 0.7: Ontology Scope] ────────────────────► ontology-scope.json
    │
    ▼
[Phase 0.8: Context] ───────────────────────────► context.json
    │
    ▼
[Phase 1: Spawn Analysts] ──── reads ──────────── context.json, ontology-scope.json, perspectives.json
    │                          writes ─────────── perspectives/{perspective-id}/findings.json (per analyst)
    ▼
[Phase 2A: Collect Findings] ── reads ─────────── findings.json (per analyst via SendMessage)
    │                           shuts down ─────── finding analysts
    ▼
[Phase 2B: Verification] ────── reads ─────────── findings.json, context.json, ontology-scope.json
    │                           writes ────────── verified-findings-{perspective-id}.md, analyst-findings.md
    ▼
[Phase 3: Synthesis] ────────── reads ─────────── analyst-findings.md, templates/report.md
    │                           writes ────────── final report
    ▼
[Phase 4: Cleanup] ─────────── TeamDelete
```

---

## Phase-by-Phase Walkthrough

### Prerequisite Gate

**Source:** `skills/shared-v3/prerequisite-gate.md`

**Tool calls:**
1. `Read(~/.claude/settings.json)`
2. Check `env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS === "1"`

**For this scenario:** Assume gate passes. `{PROCEED_TO}` = "Phase 0".

**Workflow correctness:** PASS — Clear binary check with explicit error message and HARD STOP.

**Exit gate completeness:** PASS — Single condition, unambiguous.

---

### Phase 0: Problem Intake

**Source:** `SKILL.md:41-66`

**Step 0.1:** Description provided via `$ARGUMENTS` (the Korean text). No `AskUserQuestion` needed.

**Step 0.2:**
- `Bash(uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8)` → e.g., `a7f3e21b`
- `Bash(mkdir -p ~/.prism/state/analyze-a7f3e21b)`

**Placeholder binding:**
- `{short-id}` = `a7f3e21b` (for paths)
- `{SHORT_ID}` = `a7f3e21b` (for prompt placeholders)
- `{DESCRIPTION}` = the full Korean user input

**Exit gate checks:**
- [x] Description collected (from $ARGUMENTS)
- [x] `{short-id}` generated and state directory created

**Workflow correctness:** PASS
**Exit gate completeness:** PASS — Note at line 64 correctly states severity/status are NOT collected here.

---

### Phase 0.5: Team Creation & Seed Analysis

**Source:** `SKILL.md:70-133`

#### Step 0.5.1: Create Team
```
TeamCreate(team_name: "analyze-a7f3e21b", description: "Analysis: API 응답 3초 이상, DB 슬로우쿼리, /api/v1/rooms")
```

**Note:** `{summary}` in SKILL.md:75 is not formally defined as a placeholder. The orchestrator must infer a summary from the description. This is a **minor ambiguity** — an LLM would naturally summarize, but it is undocumented.

#### Step 0.5.2: Spawn Seed Analyst

**Prompt assembly:**
1. Read `skills/shared-v3/worker-preamble.md`
2. Read `skills/analyze/prompts/seed-analyst.md`
3. Replace in preamble:
   - `{TEAM_NAME}` → `"analyze-a7f3e21b"`
   - `{WORKER_NAME}` → `"seed-analyst"`
   - `{WORK_ACTION}` → `"Actively investigate using available tools (Grep, Read, Bash, MCP). Evaluate dimensions and severity. Write findings to seed-analysis.json. Report via SendMessage."`
4. Replace in seed-analyst prompt:
   - `{DESCRIPTION}` → full Korean user input
   - `{SHORT_ID}` → `a7f3e21b`

**Spawn:**
```
TaskCreate(...)
TaskUpdate(owner="seed-analyst")
Task(
  subagent_type="oh-my-claudecode:architect",
  name="seed-analyst",
  team_name="analyze-a7f3e21b",
  model="opus",
  run_in_background=true,
  prompt="[assembled prompt]"
)
```

#### Step 0.5.3: Expected Seed Analyst Output

For this scenario (API slowquery + DB performance + recent index migration), the seed analyst would likely produce:

```json
{
  "severity": "SEV2",
  "status": "Active",
  "evidence_types": ["code diffs", "git history", "source code"],
  "dimensions": {
    "domain": "data",
    "failure_type": "degradation",
    "evidence_available": ["code diffs", "git history", "source code"],
    "complexity": "single-cause",
    "recurrence": "first-time"
  },
  "research": {
    "findings": [
      {
        "id": 1,
        "finding": "Recent migration altered/dropped indexes on rooms-related tables",
        "source": "migrations/xxx_index_migration.sql:line",
        "tool_used": "Bash",
        "severity": "critical"
      },
      {
        "id": 2,
        "finding": "/api/v1/rooms endpoint handler queries rooms table with joins",
        "source": "src/routes/rooms.ts:42",
        "tool_used": "Grep",
        "severity": "high"
      },
      {
        "id": 3,
        "finding": "git log shows index migration committed N days ago",
        "source": "git log --oneline --since='7 days ago'",
        "tool_used": "Bash",
        "severity": "high"
      }
    ],
    "files_examined": ["migrations/xxx.sql", "src/routes/rooms.ts"],
    "mcp_queries": [],
    "recent_changes": ["abc1234 — index migration"]
  }
}
```

**Severity rationale:** SEV2 because user-facing degradation (3s+ response times) on a specific endpoint, not a full outage (SEV1) but impactful enough to not be SEV3.

**Status rationale:** Active — no mitigation mentioned by user.

**Complexity rationale:** "single-cause" — user explicitly correlates issue with a single migration event. However, a thorough seed analyst might evaluate "multi-factor" if it discovers the migration interacted with existing query patterns. The more likely output is "single-cause" given the clear temporal correlation.

#### Steps 0.5.4-0.5.5: Shutdown + Drain
```
SendMessage(type: "shutdown_request", recipient: "seed-analyst", content: "Seed analysis complete.")
TaskList → TaskOutput (for each completed task)
```

**Exit gate checks:**
- [x] Team created
- [x] Seed-analyst results received via SendMessage
- [x] `seed-analysis.json` written
- [x] Seed-analyst shut down
- [x] All background task outputs drained

**Workflow correctness:** PASS
**Exit gate completeness:** PASS

---

### Phase 0.55: Perspective Generation

**Source:** `SKILL.md:137-191`

#### Step 0.55.1: Spawn Perspective Generator

**Prompt assembly:**
1. Read `skills/shared-v3/worker-preamble.md`
2. Read `skills/analyze/prompts/perspective-generator.md`
3. Replace in preamble:
   - `{TEAM_NAME}` → `"analyze-a7f3e21b"`
   - `{WORKER_NAME}` → `"perspective-generator"`
   - `{WORK_ACTION}` → `"Read seed-analysis.json, apply archetype mapping rules and mandatory rules, generate perspective candidates. Write perspectives.json. Report via SendMessage."`
4. Replace in perspective-generator prompt:
   - `{SHORT_ID}` → `a7f3e21b`
   - `{DESCRIPTION}` → full Korean user input

**Spawn:**
```
TaskCreate(...)
TaskUpdate(owner="perspective-generator")
Task(
  subagent_type="oh-my-claudecode:architect",
  name="perspective-generator",
  team_name="analyze-a7f3e21b",
  model="opus",
  run_in_background=true,
  prompt="[assembled prompt]"
)
```

#### Expected Perspective Selection for This Scenario

**Input characteristics from seed-analysis.json:**
- Domain: data
- Failure type: degradation
- Complexity: single-cause
- Recurrence: first-time
- Key evidence: index migration, slow queries, specific endpoint

**Archetype mapping (perspective-generator.md:42-55):**
Row 3 matches: "Latency spike, OOM, resource exhaustion" → `performance` + `root-cause` + `systems`
Row 4 also partially matches: "Post-deployment failure, config drift" → `deployment` + `timeline` + `root-cause`

**Mandatory rules check (perspective-generator.md:80-89):**

| Rule | Check | Result |
|------|-------|--------|
| Core archetype required | `root-cause` is core | PASS |
| Recurring → systems | recurrence = "first-time" | N/A |
| Evidence-backed only | All have seed research support | PASS |
| Minimum perspectives | ≥2 | PASS (will select 3) |
| Complexity scaling | single-cause → 2-3 perspectives | Constrains to 2-3 |

**Expected perspectives output:**

```json
{
  "perspectives": [
    {
      "id": "root-cause",
      "name": "Root Cause",
      "scope": "Trace the index migration's impact on /api/v1/rooms query execution plans",
      "key_questions": [
        "Which specific indexes were altered/dropped in the migration?",
        "What query patterns in /api/v1/rooms relied on those indexes?",
        "Is the migration reversible without data loss?"
      ],
      "model": "opus",
      "agent_type": "architect",
      "rationale": "Seed analysis found direct temporal correlation between index migration and performance degradation — root cause investigation needed"
    },
    {
      "id": "performance",
      "name": "Performance & Capacity",
      "scope": "Profile DB query execution and API response latency for /api/v1/rooms",
      "key_questions": [
        "What is the query execution plan for the rooms endpoint queries post-migration?",
        "Are there full table scans where index scans were expected?",
        "What is the connection pool utilization under current load?"
      ],
      "model": "sonnet",
      "agent_type": "architect-medium",
      "rationale": "Seed analysis confirmed 3s+ response times and slow query logs — performance profiling needed"
    },
    {
      "id": "deployment",
      "name": "Deployment & Change",
      "scope": "Analyze the index migration change, its review process, and rollback options",
      "key_questions": [
        "What was the migration's before/after diff?",
        "Were there canary or staged rollout steps for the migration?",
        "What is the rollback path and risk?"
      ],
      "model": "sonnet",
      "agent_type": "architect-medium",
      "rationale": "Issue directly correlates with recent migration — deployment change analysis essential"
    }
  ],
  "rules_applied": {
    "core_archetype_included": true,
    "recurring_systems_enforced": "n/a",
    "all_evidence_backed": true,
    "min_perspectives_met": true,
    "complexity_scaling_correct": true
  },
  "selection_summary": "Selected 3 perspectives (within single-cause 2-3 range). root-cause (core) investigates the migration itself, performance profiles the DB impact, deployment examines the change process and rollback options."
}
```

**Why NOT `systems`?** Despite the mapping table suggesting it, the complexity_scaling rule constrains to 2-3 for single-cause. `systems` would overlap with `root-cause` for this specific scenario (both would look at the same migration code path). The perspective quality gate's "orthogonal" check (perspective-generator.md:101) should filter it out.

**Why NOT `timeline`?** The timeline is straightforward (migration → degradation) with no complex multi-event sequence to untangle. Not sufficiently evidence-backed for a separate perspective.

**Archetype mapping table correctness for this scenario:** PASS — The table at perspective-generator.md:42-55 correctly maps this scenario's characteristics to relevant archetypes. The rows are not mutually exclusive, which is correct — the generator must use judgment to combine matches.

#### Steps 0.55.3-0.55.4: Shutdown + Drain

Same pattern as Phase 0.5.

**Exit gate checks:**
- [x] Perspective generator results received
- [x] `perspectives.json` written
- [x] Perspective generator shut down
- [x] All background task outputs drained

**Workflow correctness:** PASS
**Exit gate completeness:** PASS

---

### Phase 0.6: Perspective Approval

**Source:** `SKILL.md:195-225`

**Step 0.6.1:**
```
Read(~/.prism/state/analyze-a7f3e21b/perspectives.json)
AskUserQuestion(
  header: "Perspectives",
  question: "I recommend these 3 perspectives for analysis: root-cause, performance, deployment. How to proceed?",
  options: "Proceed" / "Add perspective" / "Remove perspective" / "Modify perspective"
)
```

**Assumed user action:** "Proceed"

**Step 0.6.3:** Update `perspectives.json` with:
```json
{
  "perspectives": [...],
  "rules_applied": {...},
  "selection_summary": "...",
  "approved": true,
  "user_modifications": []
}
```

**Workflow correctness:** PASS
**Exit gate:** No explicit exit gate for Phase 0.6 — but Phase 0.8 exit gate checks `approved: true`. This is acceptable; the approval loop itself is the gate.

---

### Phase 0.7: Ontology Scope Mapping

**Source:** `SKILL.md:229-237`, `skills/shared-v3/ontology-scope-mapping.md`

**Parameters:**
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"analysis"`
- `{STATE_DIR}` = `~/.prism/state/analyze-a7f3e21b`

**Step 1:** `ToolSearch(query="select:mcp__prism-mcp__prism_docs_roots")` → `mcp__prism-mcp__prism_docs_roots()`

**Scenario branches:**
- If ontology docs configured → present Screen 1 (MCP data sources), Screen 2 (external), Screen 3 (confirm)
- If no ontology docs → `ONTOLOGY_AVAILABLE=false`, still proceed to Screen 1 for MCP data sources

For this DB performance scenario, the user might select Grafana (for Prometheus/Loki metrics) if available, or skip all.

**Output:** `~/.prism/state/analyze-a7f3e21b/ontology-scope.json`

**If no ontology available:** `{ONTOLOGY_SCOPE}` = "N/A — ontology scope not available. Analyze using available evidence only." (per SKILL.md:236)

**Workflow correctness:** PASS — The optional mode gracefully degrades.
**Edge case (ontology unavailable):** Handled explicitly at SKILL.md:236.

---

### Phase 0.8: Context & State Files

**Source:** `SKILL.md:242-268`

**Step 0.8.1:** Write `context.json`:
```json
{
  "summary": "API 응답 3초 이상 지연, DB 슬로우쿼리 대량 발생. /api/v1/rooms 엔드포인트 심각. 인덱스 마이그레이션 이후 발생. Active, SEV2.",
  "research_summary": {
    "key_findings": [
      "Recent index migration altered indexes on rooms-related tables",
      "/api/v1/rooms endpoint queries rooms table with joins",
      "Git log confirms migration committed recently"
    ],
    "files_examined": ["migrations/xxx.sql", "src/routes/rooms.ts"],
    "dimensions": "domain: data, failure_type: degradation, complexity: single-cause, recurrence: first-time"
  },
  "report_language": "ko"
}
```

**Language detection:** Input is Korean → `report_language: "ko"`. This field is consumed by Phase 3 report generation.

**Exit gate checks:**
- [x] `perspectives.json` has `approved: true`
- [x] `context.json` written
- [x] Ontology scope mapping complete or skipped

**Workflow correctness:** PASS
**Exit gate completeness:** PASS

---

### Phase 1: Spawn Analysts (Finding Phase)

**Source:** `SKILL.md:272-330`

#### Step 1.1: Spawn 3 Analysts in Parallel

For each perspective from `perspectives.json`:

**Prompt assembly order (per SKILL.md:280-285):**
1. Read archetype section from prompt file
2. Read `prompts/finding-protocol.md`
3. Concatenate: `[worker preamble] + [archetype prompt] + [finding protocol]`
4. Replace placeholders
5. Spawn

##### Analyst 1: root-cause

- **Archetype:** `prompts/core-archetypes.md` § Root Cause Lens (line 51)
- **Protocol:** `prompts/finding-protocol.md`
- **Placeholders:**
  - `{CONTEXT}` → stringified `context.json` content
  - `{ONTOLOGY_SCOPE}` → text block from `ontology-scope.json` Phase B generation (or "N/A")
  - `{SHORT_ID}` → `a7f3e21b`
  - `{perspective-id}` → `root-cause`
- **Worker preamble:**
  - `{TEAM_NAME}` → `analyze-a7f3e21b`
  - `{WORKER_NAME}` → `root-cause-analyst`
  - `{WORK_ACTION}` → "Investigate from your assigned perspective. Answer ALL key questions with evidence and code references. Write findings to findings.json. Report findings via SendMessage to team-lead. Do NOT run self-verification — that happens in a separate session."

```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="root-cause-analyst",
  team_name="analyze-a7f3e21b",
  model="opus",
  run_in_background=true,
  prompt="[assembled prompt]"
)
```

##### Analyst 2: performance

- **Archetype:** `prompts/extended-archetypes.md` § Performance Lens (line 112)
- **Protocol:** `prompts/finding-protocol.md`
- **Same placeholder replacements** with `{perspective-id}` → `performance`

```
Task(
  subagent_type="oh-my-claudecode:architect-medium",
  name="performance-analyst",
  team_name="analyze-a7f3e21b",
  model="sonnet",
  run_in_background=true,
  prompt="[assembled prompt]"
)
```

##### Analyst 3: deployment

- **Archetype:** `prompts/extended-archetypes.md` § Deployment (line 218)
- **Protocol:** `prompts/finding-protocol.md`
- **Same placeholder replacements** with `{perspective-id}` → `deployment`

```
Task(
  subagent_type="oh-my-claudecode:architect-medium",
  name="deployment-analyst",
  team_name="analyze-a7f3e21b",
  model="sonnet",
  run_in_background=true,
  prompt="[assembled prompt]"
)
```

**Prompt assembly check:**
- Archetype prompts contain `{CONTEXT}` and `{ONTOLOGY_SCOPE}` placeholders: PASS (core-archetypes.md:27,30 / extended-archetypes.md:121,125)
- Finding protocol references `{SHORT_ID}` and `{perspective-id}`: PASS (finding-protocol.md:15,25,29,45,49)
- Concatenation order makes sense: preamble sets the team/role context, archetype sets the analytical lens, protocol sets the investigation workflow: PASS

**Exit gate checks:**
- [x] All 3 analyst tasks created and owners pre-assigned
- [x] All 3 analysts spawned in parallel (run_in_background=true)

**Workflow correctness:** PASS
**Exit gate completeness:** PASS

---

### Phase 2A: Collect Findings

**Source:** `docs/later-phases.md:7-44`

#### Step 2A.1: Wait for Analyst Findings

Each analyst writes:
- `~/.prism/state/analyze-a7f3e21b/perspectives/root-cause/findings.json`
- `~/.prism/state/analyze-a7f3e21b/perspectives/performance/findings.json`
- `~/.prism/state/analyze-a7f3e21b/perspectives/deployment/findings.json`

And sends findings via `SendMessage` to team-lead.

#### Step 2A.2: Drain
```
TaskList → TaskOutput (for each completed task)
```

#### Step 2A.3: Shutdown Finding Analysts
```
SendMessage(type: "shutdown_request", recipient: "root-cause-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "performance-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "deployment-analyst", content: "Finding phase complete.")
```

**Exit gate checks:**
- [x] All 3 analyst findings received via SendMessage
- [x] All 3 `findings.json` files written
- [x] All 3 finding analysts shut down
- [x] All background task outputs drained

**Workflow correctness:** PASS
**Exit gate completeness:** PASS

---

### Phase 2B: Spawn Verification Sessions

**Source:** `docs/later-phases.md:49-137`

#### Step 2B.1: Spawn 3 Verifiers in Parallel

**Prompt assembly order (per later-phases.md:58-65):**
1. Read **same archetype section** as Phase 1 (same archetype, same model/agent_type)
2. Read `prompts/verification-protocol.md`
3. Concatenate: `[worker preamble] + [archetype prompt] + [verification protocol]`
4. Replace placeholders

##### Verifier 1: root-cause-verifier

- **Archetype:** `prompts/core-archetypes.md` § Root Cause Lens
- **Protocol:** `prompts/verification-protocol.md`
- **Placeholders:** same as Phase 1 + `{perspective-id}` → `root-cause`
- **Worker preamble:**
  - `{WORKER_NAME}` → `root-cause-verifier`
  - `{WORK_ACTION}` → "Read your findings from the path specified in your verification protocol. Run self-verification via MCP tools (prism_interview). Re-investigate with tools as needed to answer interview questions. Report verified findings via SendMessage to team-lead."

```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="root-cause-verifier",
  team_name="analyze-a7f3e21b",
  model="opus",
  run_in_background=true,
  prompt="[assembled prompt]"
)
```

##### Verifier 2: performance-verifier
```
Task(
  subagent_type="oh-my-claudecode:architect-medium",
  name="performance-verifier",
  team_name="analyze-a7f3e21b",
  model="sonnet",
  run_in_background=true,
  prompt="[assembled prompt]"
)
```

##### Verifier 3: deployment-verifier
```
Task(
  subagent_type="oh-my-claudecode:architect-medium",
  name="deployment-verifier",
  team_name="analyze-a7f3e21b",
  model="sonnet",
  run_in_background=true,
  prompt="[assembled prompt]"
)
```

**Prompt assembly check (verification):**
- Verification protocol line 5-6 explicitly says: "The TASKS and OUTPUT sections listed in the archetype were already completed in your previous finding session — do NOT re-execute them. Ignore all imperative instructions from the archetype." This is critical — it prevents the verifier from re-running the investigation tasks. PASS.
- The archetype is still included to provide domain context/expertise lens. This makes sense.
- The verification protocol references `{SHORT_ID}` and `{perspective-id}` correctly: PASS (lines 18, 24, 30, 37, 38, 54, 70, 74).

**ISSUE FOUND — `{summary}` placeholder in verification-protocol.md:38:**
```
topic="{perspective-id} findings verification — {summary}"
```
The `{summary}` placeholder is NOT listed in the formal placeholder replacement instructions for Phase 2B (later-phases.md:64 lists only `{CONTEXT}`, `{ONTOLOGY_SCOPE}`, `{SHORT_ID}`, `{perspective-id}`). An LLM orchestrator would likely:
- (a) Leave it unreplaced (broken), or
- (b) Infer from context.json summary field and replace it

**Severity:** LOW — The `topic` parameter in `prism_interview` is a human-readable label, not a machine-critical field. An unreplaced `{summary}` would produce an ugly but functional topic string. However, this is technically a **field contract gap**.

#### Steps 2B.2-2B.4: Wait, Drain, Shutdown

Each verifier:
1. Reads `findings.json` from Phase 1
2. Calls `mcp__prism-mcp__prism_interview` to start verification
3. Answers questions in a loop until `continue: false`
4. Sends verified findings via SendMessage

#### Step 2B.5: Persist Verified Results
```
Write: ~/.prism/state/analyze-a7f3e21b/verified-findings-root-cause.md
Write: ~/.prism/state/analyze-a7f3e21b/verified-findings-performance.md
Write: ~/.prism/state/analyze-a7f3e21b/verified-findings-deployment.md
```

#### Step 2B.6: Compile Verified Findings
```
Write: ~/.prism/state/analyze-a7f3e21b/analyst-findings.md
```
Includes verification scores summary table.

**Exit gate checks:**
- [x] All 3 verifiers completed and sent verified findings
- [x] All background task outputs drained
- [x] All 3 verifiers shut down
- [x] All verified findings persisted
- [x] Compiled findings written to `analyst-findings.md`

**Workflow correctness:** PASS
**Exit gate completeness:** PASS

---

### Phase 3: Synthesis & Report

**Source:** `docs/later-phases.md:141-168`

#### Step 3.1: Read `analyst-findings.md`
#### Step 3.2: Read `templates/report.md`, fill all sections

The report template (`templates/report.md`) contains these sections:
- Executive Summary
- Analysis Overview (with `{SEV}`, `{description}`, etc.)
- Timeline
- Perspective Findings (per lens)
- Integrated Analysis (convergence/divergence/emergent)
- Socratic Verification Summary (per-analyst scores, key Q&A)
- Recommendations (prioritized)
- Prevention Checklist
- Appendix (perspectives rationale, verification details, ontology mapping)

**Note:** The template uses `{description}`, `{SEV}`, `{summary}` etc. as human-readable placeholder hints, not machine-replaceable tokens. The orchestrator fills these from accumulated state. This is clear from context.

**Language:** `report_language: "ko"` from `context.json` — report should be generated in Korean.

**Note on `report_language`:** This field is written in Phase 0.8 (context.json) but there is NO explicit instruction in Phase 3 or the report template to USE this field for language selection. An LLM orchestrator would likely infer it, but it is an **implicit contract** rather than an explicit one.

#### Step 3.3: User Decision
```
AskUserQuestion:
"Is the analysis complete?"
Options: "Complete" / "Need deeper investigation" / "Add recommendations" / "Share with team"
```

**Deeper investigation re-entry:** Bounded to max 2 loops via `investigation_loops` counter in `context.json`.

**Workflow correctness:** PASS
**Exit gate:** No formal exit gate for Phase 3 — user approval serves as the gate.

---

### Phase 4: Cleanup

**Source:** `docs/later-phases.md:174-175`, `skills/shared-v3/team-teardown.md`

1. `TaskList` → find non-completed tasks
2. `SendMessage(type: "shutdown_request")` to each
3. Await `shutdown_response(approve=true)`
4. `TeamDelete`

**Workflow correctness:** PASS

---

## Field Contract Verification

### seed-analysis.json

| Field | Written By | Read By | Contract Status |
|-------|-----------|---------|-----------------|
| `severity` | seed-analyst (seed-analyst.md:83) | Orchestrator (context.json, report) | PASS |
| `status` | seed-analyst (seed-analyst.md:83) | Orchestrator (context.json, report) | PASS |
| `evidence_types` | seed-analyst (seed-analyst.md:86) | Not explicitly consumed downstream | PASS (informational) |
| `dimensions.domain` | seed-analyst (seed-analyst.md:89) | perspective-generator (perspective-generator.md:42) | PASS |
| `dimensions.failure_type` | seed-analyst (seed-analyst.md:90) | perspective-generator (perspective-generator.md:42) | PASS |
| `dimensions.complexity` | seed-analyst (seed-analyst.md:92) | perspective-generator (perspective-generator.md:88 — complexity_scaling rule) | PASS |
| `dimensions.recurrence` | seed-analyst (seed-analyst.md:93) | perspective-generator (perspective-generator.md:85 — recurring → systems rule) | PASS |
| `research.findings` | seed-analyst (seed-analyst.md:95-103) | perspective-generator (perspective-generator.md:86 — evidence-backed rule, :96 — grounding) | PASS |
| `research.files_examined` | seed-analyst (seed-analyst.md:104) | Orchestrator (context.json research_summary) | PASS |
| `research.mcp_queries` | seed-analyst (seed-analyst.md:105) | Not explicitly consumed | PASS (informational) |
| `research.recent_changes` | seed-analyst (seed-analyst.md:106) | Not explicitly consumed | PASS (informational) |

### perspectives.json

| Field | Written By | Read By | Contract Status |
|-------|-----------|---------|-----------------|
| `perspectives[].id` | perspective-generator | Phase 1 spawn (as `{perspective-id}`), Phase 2A/2B (agent naming), report | PASS |
| `perspectives[].name` | perspective-generator | Report template (lens name) | PASS |
| `perspectives[].scope` | perspective-generator | Not injected into analyst prompts | **NOTE** — scope is shown to user in Phase 0.6 but not passed to analysts. Analysts derive scope from archetype + context. Acceptable design. |
| `perspectives[].key_questions` | perspective-generator | Not injected into analyst prompts | **NOTE** — same as scope. Questions guide perspective selection review, not analyst execution. Analysts use archetype TASKS. Acceptable but worth noting — analysts may investigate different questions than those listed. |
| `perspectives[].model` | perspective-generator | Phase 1 + 2B spawn (model parameter) | PASS |
| `perspectives[].agent_type` | perspective-generator | Phase 1 + 2B spawn (subagent_type) | PASS |
| `perspectives[].rationale` | perspective-generator | Phase 0.6 user review, report appendix | PASS |
| `rules_applied` | perspective-generator | Phase 0.6 user display | PASS |
| `approved` | Orchestrator (Phase 0.6) | Phase 0.8 exit gate | PASS |
| `user_modifications` | Orchestrator (Phase 0.6) | Not consumed downstream | PASS (audit trail) |

### context.json

| Field | Written By | Read By | Contract Status |
|-------|-----------|---------|-----------------|
| `summary` | Orchestrator (Phase 0.8) | Phase 1 analysts via `{CONTEXT}`, Phase 3 report | PASS |
| `research_summary` | Orchestrator (Phase 0.8) | Phase 1 analysts via `{CONTEXT}` | PASS |
| `report_language` | Orchestrator (Phase 0.8) | Phase 3 (implicit) | **WEAK** — no explicit instruction to use this field |

### findings.json (per perspective)

| Field | Written By | Read By | Contract Status |
|-------|-----------|---------|-----------------|
| `analyst` | Finding analyst (finding-protocol.md:29) | Verification analyst (verification-protocol.md:30) | PASS |
| `findings[].finding` | Finding analyst | Verification analyst, Phase 2B persist, Phase 3 | PASS |
| `findings[].evidence` | Finding analyst | Verification analyst (re-verification) | PASS |
| `findings[].severity` | Finding analyst | Phase 3 prioritization | PASS |

### ontology-scope.json

| Field | Written By | Read By | Contract Status |
|-------|-----------|---------|-----------------|
| `sources[]` | Orchestrator (Phase 0.7) | Orchestrator (Phase B text block generation → `{ONTOLOGY_SCOPE}`) | PASS |
| `citation_format` | Orchestrator (Phase 0.7) | Text block footer | PASS |

---

## Placeholder Replacement Verification

| Placeholder | Source | Consumers | Status |
|-------------|--------|-----------|--------|
| `{SHORT_ID}` / `{short-id}` | Phase 0.2 uuidgen | All prompts, all file paths | PASS — naming note at SKILL.md:53 clarifies the two forms |
| `{DESCRIPTION}` | Phase 0.1 user input | seed-analyst.md, perspective-generator.md | PASS |
| `{CONTEXT}` | Phase 0.8 context.json | Phase 1 + 2B archetype prompts | PASS |
| `{ONTOLOGY_SCOPE}` | Phase 0.7 → Phase B text block | Phase 1 + 2B archetype prompts | PASS |
| `{perspective-id}` | perspectives.json `id` field | finding-protocol.md, verification-protocol.md, agent names | PASS |
| `{TEAM_NAME}` | Phase 0.5.1 team name | worker-preamble.md | PASS |
| `{WORKER_NAME}` | Per agent spawn | worker-preamble.md | PASS |
| `{WORK_ACTION}` | Per phase instructions | worker-preamble.md | PASS |
| `{summary}` | **UNDOCUMENTED** in replacement lists | verification-protocol.md:38, SKILL.md:75, templates/report.md:14 | **FAIL** — not in any formal replacement list |
| `{PROCEED_TO}` | Caller sets to "Phase 0" | prerequisite-gate.md | PASS |
| `{AVAILABILITY_MODE}` | SKILL.md:232 = "optional" | ontology-scope-mapping.md | PASS |
| `{CALLER_CONTEXT}` | SKILL.md:233 = "analysis" | ontology-scope-mapping.md | PASS |
| `{STATE_DIR}` | SKILL.md:234 | ontology-scope-mapping.md | PASS |

---

## Issues Found

### Issue 1: `{summary}` Placeholder Undocumented (LOW)

**Location:** `verification-protocol.md:38`, `SKILL.md:75`
**Problem:** `{summary}` appears in prompt templates but is never listed in the formal placeholder replacement instructions for any phase.
**Impact:** In `verification-protocol.md:38`, an unreplaced `{summary}` in the `topic` parameter of `prism_interview` would produce a cosmetically ugly but functional topic string. In `SKILL.md:75` (`TeamCreate description`), the orchestrator must infer a summary.
**Risk:** LOW — LLM orchestrators typically infer these from context, but it breaks the explicit contract pattern used for all other placeholders.
**Recommendation:** Either (a) add `{summary}` to the placeholder replacement lists for Phase 0.5 and Phase 2B, or (b) replace the placeholder with a literal instruction like "summarize the description in ~10 words".

### Issue 2: `report_language` Implicit Contract (LOW)

**Location:** `SKILL.md:257` (written), Phase 3 / `templates/report.md` (consumed)
**Problem:** `context.json` includes `report_language` but Phase 3 instructions and the report template never reference this field.
**Impact:** The report may be generated in English instead of Korean despite the field existing.
**Risk:** LOW — An LLM would likely detect Korean input and produce Korean output regardless. But for non-obvious language cases (e.g., English description from a Korean team), the field would go unused.
**Recommendation:** Add explicit instruction in Phase 3 Step 3.2: "Generate the report in the language specified by `context.json.report_language`."

### Issue 3: `key_questions` Not Injected Into Analyst Prompts (INFO)

**Location:** `perspectives.json` → Phase 1 analyst spawn
**Problem:** The perspective generator produces `key_questions` per perspective, but these are never injected into analyst prompts. Analysts use the archetype's hardcoded TASKS instead.
**Impact:** The carefully generated, scenario-specific questions are used only for user review in Phase 0.6. Analysts may investigate different angles.
**Risk:** INFO — This is arguably by design (archetypes provide structured investigation, not ad-hoc questions). But it means the perspective generator's `key_questions` work is partially wasted.
**Recommendation:** Consider injecting `key_questions` as additional focus areas in the analyst prompt, or document that they are review-only.

### Issue 4: `scope` Not Injected Into Analyst Prompts (INFO)

**Location:** Same as Issue 3.
**Problem:** `perspectives[].scope` describes what each perspective should examine for THIS specific case, but it is not passed to the analyst.
**Impact:** Same as Issue 3 — analysts use generic archetype scope.
**Risk:** INFO — Same rationale.

### Issue 5: No `{perspective-id}` in Archetype Prompts Themselves (VERIFIED OK)

**Location:** `prompts/core-archetypes.md`, `prompts/extended-archetypes.md`
**Observation:** The archetype prompts do NOT contain `{perspective-id}` — only `{CONTEXT}`, `{ONTOLOGY_SCOPE}`, `{SHORT_ID}`. The `{perspective-id}` placeholder appears only in `finding-protocol.md` and `verification-protocol.md`, which are concatenated AFTER the archetype.
**Status:** CORRECT — The concatenation order ensures `{perspective-id}` is present in the assembled prompt via the protocol suffix.

### Issue 6: Phase 0.6 Has No Formal Exit Gate (INFO)

**Location:** `SKILL.md:195-225`
**Problem:** Unlike other phases, Phase 0.6 does not have an explicit "Phase 0.6 Exit Gate" section.
**Impact:** Minimal — the user approval loop IS the gate, and Phase 0.8 exit gate checks `approved: true`.
**Risk:** INFO — Structural inconsistency only.

---

## Edge Case Analysis

### Edge Case 1: Seed Analyst Returns Unexpected Data

**Scenario:** seed-analyst returns malformed JSON or missing fields.

**Current handling:** Phase 0.5 exit gate checks "Seed-analyst results received" and "seed-analysis.json written" but does NOT validate the JSON schema. If `dimensions.complexity` is missing, the perspective generator would fail to apply the complexity_scaling rule.

**Risk:** MEDIUM — The perspective generator (an opus-level LLM) would likely handle missing fields gracefully, but the mandatory rules check at perspective-generator.md:88 explicitly references `dimensions.complexity`. A missing field could lead to incorrect perspective count.

**Recommendation:** Add schema validation in Phase 0.5 exit gate: "Verify seed-analysis.json contains required fields: severity, status, dimensions (with all 5 sub-fields), research.findings (non-empty array)."

### Edge Case 2: Ontology Unavailable

**Scenario:** No ontology docs configured, no MCP data sources selected.

**Current handling:** SKILL.md:236 explicitly sets `{ONTOLOGY_SCOPE}` = "N/A — ontology scope not available. Analyze using available evidence only." All archetype prompts use `{ONTOLOGY_SCOPE}` in a "### Reference Documents" section, which would gracefully show the N/A message.

**Risk:** LOW — Well handled.

### Edge Case 3: prism_interview MCP Tool Unavailable

**Scenario:** The `prism-mcp` server is down or `prism_interview` tool fails.

**Current handling:** verification-protocol.md does not specify error handling for MCP tool failure. The verifier would get stuck.

**Risk:** MEDIUM — The verification phase could hang indefinitely.

**Recommendation:** Add a timeout or fallback in verification-protocol.md: "If prism_interview fails after 2 retries, report findings as UNVERIFIED and send to team-lead with a note."

### Edge Case 4: All Perspectives Rejected by User in Phase 0.6

**Scenario:** User removes all perspectives.

**Current handling:** SKILL.md:207 says "Warn if <2 dynamic perspectives" during the iteration loop. But there is no hard block on 0 perspectives.

**Risk:** LOW — The mandatory rules in the perspective generator already enforce ≥2, so the starting point is always ≥2. User would have to actively remove all of them. The Phase 1 spawn would simply spawn 0 analysts and proceed to Phase 2 with nothing to collect.

**Recommendation:** Add a hard gate: "MUST NOT proceed with 0 perspectives. If user removes all, re-prompt."

---

## Archetype Mapping Verification for This Scenario

### Input Characteristics
- Domain: `data`
- Failure type: `degradation`
- Evidence: `code diffs`, `git history`, `source code`
- Complexity: `single-cause`
- Recurrence: `first-time`

### Mapping Table Match (perspective-generator.md:42-55)

| Row | Pattern | Match? | Recommended |
|-----|---------|--------|-------------|
| Latency spike, OOM, resource exhaustion | YES (latency spike) | `performance` + `root-cause` + `systems` |
| Post-deployment failure, config drift | PARTIAL (post-migration) | `deployment` + `timeline` + `root-cause` |
| Data corruption, stale reads, replication lag | NO (not data corruption) | — |

### Expected Selection After Rules

Given `complexity: single-cause` → 2-3 perspectives:
1. `root-cause` (core archetype, appears in both matching rows) — **opus, architect**
2. `performance` (directly matches latency symptoms) — **sonnet, architect-medium**
3. `deployment` (directly matches post-migration trigger) — **sonnet, architect-medium**

### Archetype Table Consistency Check

| perspective-generator.md Table | core-archetypes.md / extended-archetypes.md Spawn Info | Match? |
|------|------|--------|
| `root-cause`: opus, architect | core-archetypes.md:52: `oh-my-claudecode:architect`, opus | PASS |
| `performance`: sonnet, architect-medium | extended-archetypes.md:114: `oh-my-claudecode:architect-medium`, sonnet | PASS |
| `deployment`: sonnet, architect-medium | extended-archetypes.md:220: `oh-my-claudecode:architect-medium`, sonnet | PASS |
| `timeline`: sonnet, architect-medium | core-archetypes.md:20: `oh-my-claudecode:architect-medium`, sonnet | PASS |
| `systems`: opus, architect | core-archetypes.md:100: `oh-my-claudecode:architect`, opus | PASS |
| `impact`: sonnet, architect-medium | core-archetypes.md:147: `oh-my-claudecode:architect-medium`, sonnet | PASS |
| `security`: opus, architect | extended-archetypes.md:27: `oh-my-claudecode:architect`, opus | PASS |
| `data-integrity`: opus, architect | extended-archetypes.md:71: `oh-my-claudecode:architect`, opus | PASS |
| `concurrency`: opus, architect | extended-archetypes.md:304: `oh-my-claudecode:architect`, opus | PASS |
| `dependency`: sonnet, architect-medium | extended-archetypes.md:348: `oh-my-claudecode:architect-medium`, sonnet | PASS |
| `ux`: sonnet, architect-medium | extended-archetypes.md:152: `oh-my-claudecode:architect-medium`, sonnet | PASS |
| `network`: sonnet, architect-medium | extended-archetypes.md:262: `oh-my-claudecode:architect-medium`, sonnet | PASS |
| `financial`: opus, architect | extended-archetypes.md:392: `oh-my-claudecode:architect`, opus | PASS |
| `custom`: Auto, Auto | extended-archetypes.md:443-451: no fixed spawn info (composed at runtime) | PASS |

**All 14 archetypes in the mapping table match their prompt file spawn specifications: PASS.**

---

## Agent Naming Convention Verification

| Phase | Pattern | Example | Source |
|-------|---------|---------|--------|
| Seed Analyst | `seed-analyst` | `seed-analyst` | SKILL.md:82,97 |
| Perspective Generator | `perspective-generator` | `perspective-generator` | SKILL.md:143,157 |
| Finding Analysts | `{perspective-id}-analyst` | `root-cause-analyst` | SKILL.md:309 |
| Verifiers | `{perspective-id}-verifier` | `root-cause-verifier` | later-phases.md:56,74 |

Shutdown recipients match spawn names:
- `seed-analyst` (SKILL.md:117) matches spawn name (SKILL.md:97): PASS
- `perspective-generator` (SKILL.md:176) matches spawn name (SKILL.md:149): PASS
- `{perspective-id}-analyst` (later-phases.md:31) matches spawn name (SKILL.md:309): PASS
- `{perspective-id}-verifier` (later-phases.md:111) matches spawn name (later-phases.md:74): PASS

---

## File Reference Verification

| Referenced Path | Source Location | Exists? | Contains Expected Content? |
|----------------|----------------|---------|---------------------------|
| `prompts/seed-analyst.md` | SKILL.md:80 | YES | Seed analyst prompt with {DESCRIPTION}, {SHORT_ID} |
| `prompts/perspective-generator.md` | SKILL.md:141 | YES | Perspective generator with archetype mapping table |
| `prompts/core-archetypes.md` | SKILL.md:281,289-292 | YES | 4 core archetypes (Timeline, Root Cause, Systems, Impact) |
| `prompts/extended-archetypes.md` | SKILL.md:281,293-303 | YES | 10 extended archetypes (Security through Custom) |
| `prompts/finding-protocol.md` | SKILL.md:282 | YES | Finding phase protocol with JSON schema |
| `prompts/verification-protocol.md` | later-phases.md:62 | YES | Verification phase with prism_interview loop |
| `docs/later-phases.md` | SKILL.md:12,330 | YES | Phase 2-4 instructions |
| `templates/report.md` | later-phases.md:149 | YES | Full report template with all sections |
| `../shared-v3/prerequisite-gate.md` | SKILL.md:31 | YES | Agent team mode hard gate |
| `../shared-v3/worker-preamble.md` | SKILL.md:95 | YES | Worker preamble template |
| `../shared-v3/ontology-scope-mapping.md` | SKILL.md:231 | YES | Ontology scope mapping phases A+B |
| `../shared-v3/team-teardown.md` | later-phases.md:175 | YES | Team cleanup procedure |

**All 12 file references resolve correctly: PASS.**

---

## Overall Verdict

### PASS

The skill design is well-structured with clear phase boundaries, comprehensive exit gates, and consistent field contracts across the multi-phase pipeline. The data flow from seed-analysis.json through perspectives.json through context.json to analyst prompts is traceable and correct.

### Summary of Findings

| ID | Severity | Category | Description |
|----|----------|----------|-------------|
| 1 | LOW | Placeholder | `{summary}` undocumented in formal replacement lists (verification-protocol.md:38, SKILL.md:75) |
| 2 | LOW | Field Contract | `report_language` in context.json has no explicit consumer in Phase 3 |
| 3 | INFO | Design | `key_questions` from perspectives.json not injected into analyst prompts |
| 4 | INFO | Design | `scope` from perspectives.json not injected into analyst prompts |
| 5 | INFO | Structure | Phase 0.6 lacks formal exit gate section (compensated by Phase 0.8 gate) |
| 6 | MEDIUM | Edge Case | No schema validation on seed-analysis.json in Phase 0.5 exit gate |
| 7 | MEDIUM | Edge Case | No error handling for prism_interview MCP tool failure in verification-protocol.md |
| 8 | LOW | Edge Case | No hard block on 0 perspectives after user modification in Phase 0.6 |

### Strengths
- **Clear separation of concerns:** seed-analyst (research) → perspective-generator (strategy) → analysts (investigation) → verifiers (validation) → synthesis
- **Two-session verification model:** Finding and verification are properly separated, preventing self-confirmation bias
- **Comprehensive exit gates:** Every phase (except 0.6) has explicit checklist gates with specific error messages
- **Bug workaround documented:** The `#27431` drain pattern is consistently applied across all phases
- **Archetype table fully consistent:** All 14 archetypes match between perspective-generator.md mapping table and actual prompt file spawn specifications
- **Ontology graceful degradation:** Optional mode handles missing ontology cleanly
- **Re-entry bounded:** Investigation loop capped at 2 iterations in Phase 3

### Weaknesses
- Minor placeholder documentation gaps (`{summary}`)
- `key_questions` and `scope` generated but not utilized by analysts (potential waste or missed opportunity)
- No schema validation at phase boundaries (relies on LLM compliance)
- No MCP tool failure handling in verification protocol
