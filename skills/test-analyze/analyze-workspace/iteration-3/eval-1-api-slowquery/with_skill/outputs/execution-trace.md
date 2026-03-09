# Execution Trace: analyze skill dry-run

**Task prompt (Korean):** "우리 서비스에서 API 응답이 평균 3초 이상 걸리고, DB 슬로우쿼리 로그가 대량 발생하고 있어. 특히 /api/v1/rooms 엔드포인트가 심각해. 최근 인덱스 관련 마이그레이션을 했는데 그 이후부터 발생한 것 같아. 분석해줘"

**Detected language:** Korean (report_language will be set to Korean)

---

## Prerequisite: Agent Team Mode (HARD GATE)

**Source:** `skills/shared-v3/prerequisite-gate.md` with `{PROCEED_TO}` = "Phase 0"

### Step 1: Check Settings

```
Read("~/.claude/settings.json")
```

Verify: `env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS === "1"`

### Step 2: Decision

| Condition | Result |
|-----------|--------|
| `"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"` exists | **PASS** -> Proceed to Phase 0 |

---

## Phase 0: Problem Intake

### Step 0.1: Collect Description

The user provided a description via `$ARGUMENTS`, so it is used directly. No `AskUserQuestion` needed.

**Collected description:**
```
우리 서비스에서 API 응답이 평균 3초 이상 걸리고, DB 슬로우쿼리 로그가 대량 발생하고 있어. 특히 /api/v1/rooms 엔드포인트가 심각해. 최근 인덱스 관련 마이그레이션을 했는데 그 이후부터 발생한 것 같아. 분석해줘
```

### Step 0.2: Generate Session ID and State Directory

```bash
uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8
# Output (example): a1b2c3d4
```

**`{short-id}`** = `a1b2c3d4` (used in path templates)
**`{SHORT_ID}`** = `a1b2c3d4` (used in prompt placeholders — same value)

```bash
mkdir -p ~/.prism/state/analyze-a1b2c3d4
```

### Phase 0 Exit Gate

- [x] Description collected (from $ARGUMENTS)
- [x] `{short-id}` = `a1b2c3d4` generated, state directory `~/.prism/state/analyze-a1b2c3d4/` created

**NEXT ACTION: Proceed to Phase 0.5 Step 0.5.1**

---

## Phase 0.5: Team Creation & Seed Analysis

### Step 0.5.1: Create Team

**`{summary}` derivation:** Short (<=10 word) summary from user description: "API 응답 지연 및 DB 슬로우쿼리 분석"

```
TeamCreate(
  team_name: "analyze-a1b2c3d4",
  description: "Analysis: API 응답 지연 및 DB 슬로우쿼리 분석"
)
```

### Step 0.5.2: Spawn Seed Analyst

**Files read at spawn time:**
1. `skills/shared-v3/worker-preamble.md`
2. `skills/analyze/prompts/seed-analyst.md`

**Worker preamble placeholder replacements:**
| Placeholder | Value |
|-------------|-------|
| `{TEAM_NAME}` | `"analyze-a1b2c3d4"` |
| `{WORKER_NAME}` | `"seed-analyst"` |
| `{WORK_ACTION}` | `"Actively investigate using available tools (Grep, Read, Bash, MCP). Evaluate dimensions and severity. Write findings to seed-analysis.json. Report via SendMessage."` |

**Seed-analyst prompt placeholder replacements:**
| Placeholder | Replaced With |
|-------------|---------------|
| `{DESCRIPTION}` | `"우리 서비스에서 API 응답이 평균 3초 이상 걸리고, DB 슬로우쿼리 로그가 대량 발생하고 있어. 특히 /api/v1/rooms 엔드포인트가 심각해. 최근 인덱스 관련 마이그레이션을 했는데 그 이후부터 발생한 것 같아. 분석해줘"` |
| `{SHORT_ID}` | `"a1b2c3d4"` |

**Pre-spawn task management:**
```
TaskCreate(team_name="analyze-a1b2c3d4", title="Seed Analysis", ...)
TaskUpdate(task_id=<task-id>, owner="seed-analyst")
```

**Spawn call:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="seed-analyst",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="You are a TEAM WORKER in team \"analyze-a1b2c3d4\". Your name is \"seed-analyst\".
You report to the team lead (\"team-lead\").

== WORK PROTOCOL ==
1. TaskList → find my assigned task → TaskUpdate(status=\"in_progress\")
2. Actively investigate using available tools (Grep, Read, Bash, MCP). Evaluate dimensions and severity. Write findings to seed-analysis.json. Report via SendMessage.
3. Report findings via SendMessage to team-lead
4. TaskUpdate(status=\"completed\")
5. On shutdown_request → respond with shutdown_response(approve=true)

You are the SEED ANALYST for an investigation team.
...
DESCRIPTION:
우리 서비스에서 API 응답이 평균 3초 이상 걸리고, DB 슬로우쿼리 로그가 대량 발생하고 있어. 특히 /api/v1/rooms 엔드포인트가 심각해. 최근 인덱스 관련 마이그레이션을 했는데 그 이후부터 발생한 것 같아. 분석해줘
...
[full seed-analyst.md prompt body with {SHORT_ID} replaced to a1b2c3d4]"
)
```

### Step 0.5.3: Receive Seed Analyst Results

The seed analyst actively investigates (Grep for `/api/v1/rooms`, Read route handlers, Bash `git log --oneline --since="7 days ago"`, ToolSearch for MCP tools like Grafana/Sentry/ClickHouse). It sends results via `SendMessage` and writes to file.

**Expected artifact: `~/.prism/state/analyze-a1b2c3d4/seed-analysis.json`**

```json
{
  "severity": "SEV2",
  "status": "Active",
  "dimensions": {
    "domain": "data",
    "failure_type": "degradation",
    "evidence_available": ["logs", "code diffs"],
    "complexity": "single-cause",
    "recurrence": "first-time"
  },
  "research": {
    "findings": [
      {
        "id": 1,
        "finding": "/api/v1/rooms 엔드포인트의 DB 쿼리가 full table scan 수행 — 최근 마이그레이션에서 인덱스가 삭제되었거나 변경됨",
        "source": "src/routes/rooms.ts:getRooms:45",
        "tool_used": "Grep",
        "severity": "critical"
      },
      {
        "id": 2,
        "finding": "최근 인덱스 마이그레이션 커밋에서 rooms 테이블의 복합 인덱스가 단일 컬럼 인덱스로 변경됨",
        "source": "migrations/20260308_update_room_indexes.sql:15",
        "tool_used": "Read",
        "severity": "critical"
      },
      {
        "id": 3,
        "finding": "git log에서 3일 전 인덱스 관련 마이그레이션 커밋 발견, 이후 슬로우쿼리 증가 시점과 일치",
        "source": "git log: abc1234 — refactor: update room table indexes",
        "tool_used": "Bash",
        "severity": "high"
      },
      {
        "id": 4,
        "finding": "rooms 쿼리에 ORDER BY + WHERE 절 조합이 새 인덱스와 불일치하여 filesort 발생 가능",
        "source": "src/repositories/roomRepository.ts:findRooms:78",
        "tool_used": "Read",
        "severity": "high"
      }
    ],
    "files_examined": [
      "src/routes/rooms.ts:45 — getRooms handler with DB query",
      "src/repositories/roomRepository.ts:78 — findRooms query builder",
      "migrations/20260308_update_room_indexes.sql:15 — index migration"
    ],
    "mcp_queries": [],
    "recent_changes": [
      "abc1234 — refactor: update room table indexes (3 days ago)",
      "def5678 — chore: update ORM dependencies (5 days ago)"
    ]
  }
}
```

### Step 0.5.4: Shutdown Seed Analyst

```
SendMessage(
  type: "shutdown_request",
  recipient: "seed-analyst",
  content: "Seed analysis complete."
)
```

Seed analyst responds with `shutdown_response(approve=true)`.

### Step 0.5.5: Drain Background Task Output

```
TaskList() → find completed tasks → TaskOutput(task_id=<seed-analyst-task-id>)
```

### Phase 0.5 Exit Gate

- [x] Team "analyze-a1b2c3d4" created
- [x] Seed-analyst results received via SendMessage
- [x] `seed-analysis.json` written at `~/.prism/state/analyze-a1b2c3d4/seed-analysis.json`
- [x] Seed-analyst shut down (shutdown_request sent, shutdown_response received)
- [x] All background task outputs drained

**NEXT ACTION: Proceed to Phase 0.55**

---

## Phase 0.55: Perspective Generation

### Step 0.55.1: Spawn Perspective Generator

**Files read at spawn time:**
1. `skills/shared-v3/worker-preamble.md`
2. `skills/analyze/prompts/perspective-generator.md`

**Worker preamble placeholder replacements:**
| Placeholder | Value |
|-------------|-------|
| `{TEAM_NAME}` | `"analyze-a1b2c3d4"` |
| `{WORKER_NAME}` | `"perspective-generator"` |
| `{WORK_ACTION}` | `"Read seed-analysis.json, apply archetype mapping rules and mandatory rules, generate perspective candidates. Write perspectives.json. Report via SendMessage."` |

**Perspective-generator prompt placeholder replacements:**
| Placeholder | Replaced With |
|-------------|---------------|
| `{SHORT_ID}` | `"a1b2c3d4"` |
| `{DESCRIPTION}` | `"우리 서비스에서 API 응답이 평균 3초 이상 걸리고..."` (full user description) |

**Pre-spawn task management:**
```
TaskCreate(team_name="analyze-a1b2c3d4", title="Perspective Generation", ...)
TaskUpdate(task_id=<task-id>, owner="perspective-generator")
```

**Spawn call:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="perspective-generator",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="You are a TEAM WORKER in team \"analyze-a1b2c3d4\". Your name is \"perspective-generator\".
You report to the team lead (\"team-lead\").

== WORK PROTOCOL ==
1. TaskList → find my assigned task → TaskUpdate(status=\"in_progress\")
2. Read seed-analysis.json, apply archetype mapping rules and mandatory rules, generate perspective candidates. Write perspectives.json. Report via SendMessage.
3. Report findings via SendMessage to team-lead
4. TaskUpdate(status=\"completed\")
5. On shutdown_request → respond with shutdown_response(approve=true)

You are the PERSPECTIVE GENERATOR for an investigation team.
...
DESCRIPTION:
우리 서비스에서 API 응답이 평균 3초 이상 걸리고...
...
[full perspective-generator.md prompt body with {SHORT_ID} replaced to a1b2c3d4]"
)
```

### Step 0.55.2: Perspective Generator Internal Logic

The perspective generator reads `~/.prism/state/analyze-a1b2c3d4/seed-analysis.json` and applies:

**Archetype Mapping (Step 2 of perspective-generator.md):**
- Seed dimensions: `domain=data`, `failure_type=degradation`
- Matching row: "Latency spike, OOM, resource exhaustion" -> `performance` + `root-cause` + `systems`

**Mandatory Rules (Step 3 of perspective-generator.md):**

| Rule | Condition | Check | Result |
|------|-----------|-------|--------|
| Core archetype required | Always | `root-cause` is core archetype | **PASS** — `root-cause` already included |
| Recurring -> systems | `recurrence == "first-time"` | N/A | **N/A** |
| Evidence-backed only | Always | All 3 archetypes have supporting findings in seed research | **PASS** |
| Minimum perspectives | Always | 3 >= 2 | **PASS** |
| Complexity scaling | `complexity == "single-cause"` | 3 perspectives for single-cause (range 2-3) | **PASS** |
| Domain-archetype match | "Latency spike" row matched | `performance` included | **PASS** |

**Perspective Quality Gate (Step 4):**
- `performance`: Orthogonal (DB/query perf), evidence-backed (slow queries, full table scan), specific (index migration), actionable (index fix)
- `root-cause`: Orthogonal (why chain), evidence-backed (migration commit, index change), specific (index migration causality), actionable (root fix)
- `systems`: Orthogonal (architecture gaps), evidence-backed (missing safeguards), specific (no query monitoring), actionable (monitoring/guardrails)

### Step 0.55.2 (cont'd): Receive Results

**Expected artifact: `~/.prism/state/analyze-a1b2c3d4/perspectives.json`**

```json
{
  "perspectives": [
    {
      "id": "performance",
      "name": "Performance & Capacity",
      "scope": "/api/v1/rooms 엔드포인트의 DB 쿼리 성능 분석 — 인덱스 마이그레이션 전후 쿼리 실행 계획 비교, full table scan 원인 식별, 슬로우쿼리 패턴 분석",
      "key_questions": [
        "인덱스 마이그레이션 전후로 /api/v1/rooms 쿼리의 실행 계획(EXPLAIN)이 어떻게 변경되었는가?",
        "현재 rooms 테이블의 쿼리가 어떤 인덱스를 사용하고 있으며, filesort가 발생하는 조건은 무엇인가?",
        "슬로우쿼리가 발생하는 구체적인 쿼리 패턴과 그 빈도는 어떠한가?"
      ],
      "model": "sonnet",
      "agent_type": "architect-medium",
      "rationale": "Seed analyst가 발견한 full table scan과 filesort 발생 가능성(finding #1, #4)을 DB 성능 관점에서 심층 분석 필요. 인덱스 마이그레이션이 직접적 원인인지 쿼리 실행 계획 비교를 통해 확인."
    },
    {
      "id": "root-cause",
      "name": "Root Cause",
      "scope": "인덱스 마이그레이션이 성능 저하의 근본 원인인지 5 Whys 및 가설 검증 — 복합 인덱스→단일 인덱스 변경의 영향, ORM 쿼리 생성 패턴과 인덱스 불일치 분석",
      "key_questions": [
        "migrations/20260308_update_room_indexes.sql에서 정확히 어떤 인덱스가 삭제/변경되었으며, 이전 복합 인덱스와 현재 단일 인덱스의 차이는 무엇인가?",
        "roomRepository.ts의 findRooms 쿼리 빌더가 생성하는 SQL과 새 인덱스 구조 간의 불일치 지점은 어디인가?",
        "인덱스 변경 외에 동시에 발생한 다른 변경(ORM 의존성 업데이트 등)이 성능에 기여했을 가능성은 있는가?"
      ],
      "model": "opus",
      "agent_type": "architect",
      "rationale": "Seed analyst의 finding #2에서 복합→단일 인덱스 변경이 발견되었고, finding #3에서 시점 일치가 확인됨. 근본 원인이 인덱스 변경 단독인지, ORM 업데이트(def5678)와의 복합 요인인지 가설 검증이 필요."
    },
    {
      "id": "systems",
      "name": "Systems & Architecture",
      "scope": "DB 쿼리 성능 모니터링 및 방어 체계 분석 — 슬로우쿼리 감지 메커니즘, 인덱스 마이그레이션 검증 프로세스, 쿼리 성능 회귀 방지 아키텍처",
      "key_questions": [
        "현재 슬로우쿼리 모니터링 및 알림 체계가 있는가? 있다면 왜 조기 감지에 실패했는가?",
        "인덱스 마이그레이션 시 쿼리 성능 영향을 사전 검증하는 프로세스(EXPLAIN 테스트, 스테이징 벤치마크 등)가 존재하는가?",
        "DB 쿼리 타임아웃, 커넥션 풀 제한 등 성능 저하 시 blast radius를 제한하는 방어 메커니즘이 있는가?"
      ],
      "model": "opus",
      "agent_type": "architect",
      "rationale": "인덱스 마이그레이션이 성능 저하를 유발했다면, 이를 사전에 감지/방지하는 아키텍처 수준의 방어 체계가 부재했다는 의미. Seed analyst의 연구에서 모니터링/알림 관련 언급이 없어 시스템 차원의 갭 분석 필요."
    }
  ],
  "rules_applied": {
    "core_archetype_included": true,
    "recurring_systems_enforced": "n/a",
    "all_evidence_backed": true,
    "min_perspectives_met": true,
    "complexity_scaling_correct": true,
    "domain_archetype_match_enforced": true
  },
  "selection_summary": "Latency spike/degradation 시나리오에 매핑하여 performance + root-cause + systems 3개 관점 선정. complexity=single-cause이므로 2-3개 범위 내 3개 선택. core archetype인 root-cause 포함으로 core_archetype_included 규칙 충족. recurrence=first-time이므로 recurring_systems_enforced는 N/A. 모든 관점이 seed analyst의 구체적 발견(full table scan, 인덱스 변경, 커밋 시점 일치)에 근거하여 evidence-backed."
}
```

**Key field completeness check:**
- `perspectives[]`: 3 entries, each with `id`, `name`, `scope`, `key_questions`, `model`, `agent_type`, `rationale` -- **all required fields present**
- `rules_applied`: all 6 mandatory rule fields present (`core_archetype_included`, `recurring_systems_enforced`, `all_evidence_backed`, `min_perspectives_met`, `complexity_scaling_correct`, `domain_archetype_match_enforced`)
- `selection_summary`: present with reasoning

**Complexity-based perspective count:**
- `dimensions.complexity == "single-cause"` -> 2-3 perspectives
- 3 perspectives selected -> within range

### Step 0.55.3: Shutdown Perspective Generator

```
SendMessage(
  type: "shutdown_request",
  recipient: "perspective-generator",
  content: "Perspective generation complete."
)
```

### Step 0.55.4: Drain Background Task Output

```
TaskList() → TaskOutput(task_id=<perspective-generator-task-id>)
```

### Phase 0.55 Exit Gate

- [x] Perspective generator results received via SendMessage
- [x] `perspectives.json` written at `~/.prism/state/analyze-a1b2c3d4/perspectives.json`
- [x] Perspective generator shut down
- [x] All background task outputs drained

**NEXT ACTION: Proceed to Phase 0.6**

---

## Phase 0.6: Perspective Approval

### Step 0.6.1: Present Perspectives

Orchestrator reads `~/.prism/state/analyze-a1b2c3d4/perspectives.json` and `seed-analysis.json`.

```
AskUserQuestion(
  header: "Perspectives",
  question: "I recommend these 3 perspectives for analysis. How to proceed?

**Seed Analysis Summary:**
- Severity: SEV2 | Status: Active
- Domain: data | Failure type: degradation | Complexity: single-cause
- Key findings: full table scan on rooms query, 복합→단일 인덱스 변경, 커밋 시점 일치

**Perspectives:**
1. **Performance & Capacity** (sonnet/architect-medium) — DB 쿼리 성능 분석, 인덱스 전후 실행 계획 비교
2. **Root Cause** (opus/architect) — 인덱스 마이그레이션 근본 원인 가설 검증
3. **Systems & Architecture** (opus/architect) — 모니터링 갭, 방어 체계 분석

**Rules Applied:**
- core_archetype_included: true (root-cause)
- complexity_scaling_correct: true (single-cause -> 3 perspectives)
- domain_archetype_match_enforced: true (performance)",
  options: ["Proceed", "Add perspective", "Remove perspective", "Modify perspective"]
)
```

### Step 0.6.2: User Selects "Proceed"

User selects **"Proceed"** (simulated for dry-run).

### Step 0.6.3: Update Perspectives

Orchestrator updates `~/.prism/state/analyze-a1b2c3d4/perspectives.json` in-place, adding approval metadata:

```json
{
  "perspectives": [
    { "id": "performance", "name": "Performance & Capacity", "scope": "...", "key_questions": [...], "model": "sonnet", "agent_type": "architect-medium", "rationale": "..." },
    { "id": "root-cause", "name": "Root Cause", "scope": "...", "key_questions": [...], "model": "opus", "agent_type": "architect", "rationale": "..." },
    { "id": "systems", "name": "Systems & Architecture", "scope": "...", "key_questions": [...], "model": "opus", "agent_type": "architect", "rationale": "..." }
  ],
  "rules_applied": {
    "core_archetype_included": true,
    "recurring_systems_enforced": "n/a",
    "all_evidence_backed": true,
    "min_perspectives_met": true,
    "complexity_scaling_correct": true,
    "domain_archetype_match_enforced": true
  },
  "selection_summary": "Latency spike/degradation 시나리오에 매핑하여...",
  "approved": true,
  "user_modifications": []
}
```

**Fields added by orchestrator:** `approved: true`, `user_modifications: []` (no changes)
**Fields preserved from Phase 0.55:** `perspectives`, `rules_applied`, `selection_summary`

### Phase 0.6 Exit Gate

- [x] User selected "Proceed"
- [x] `perspectives.json` updated with `approved: true`

**NEXT ACTION: Proceed to Phase 0.7**

---

## Phase 0.7: Ontology Scope Mapping

**Source:** `skills/shared-v3/ontology-scope-mapping.md`

**Parameters:**
| Placeholder | Value |
|-------------|-------|
| `{AVAILABILITY_MODE}` | `optional` |
| `{CALLER_CONTEXT}` | `"analysis"` |
| `{STATE_DIR}` | `~/.prism/state/analyze-a1b2c3d4` |

### Phase A: Build Ontology Pool

#### Step 1: Check Document Source Availability

```
ToolSearch(query="select:mcp__prism-mcp__prism_docs_roots")
mcp__prism-mcp__prism_docs_roots()
```

**Scenario:** Returns configured documentation directories (e.g., `["/Users/heechul/podo-backend/docs"]`).

`ONTOLOGY_AVAILABLE=true`, `ONTOLOGY_DIRS=["/Users/heechul/podo-backend/docs"]`

#### Step 2: Screen 1 -- MCP Data Source Selection

```
ToolSearch(query="mcp", max_results=200)
```

Discovers MCP servers. After excluding `prism-mcp` and `plugin_*`:
- `grafana` (monitoring -- Prometheus, Loki, dashboards)
- `mcp-clickhouse` (database queries)
- `sentry` (error tracking)
- `notion` (documentation/wiki)

```
AskUserQuestion(
  header: "Live Data Sources",
  question: "Select live data sources for analysis. (multiple selection)",
  multiSelect: true,
  options: [
    {label: "grafana", description: "35+ tools — Prometheus metrics, Loki logs, dashboards, alerts"},
    {label: "mcp-clickhouse", description: "3 tools — ClickHouse database queries"},
    {label: "sentry", description: "15+ tools — Error tracking, issue management"},
    {label: "notion", description: "15+ tools — Documentation, pages, databases"},
    {label: "Skip", description: "Proceed without MCP data sources"}
  ]
)
```

**Simulated user selection:** `["grafana", "mcp-clickhouse"]` (relevant to DB performance analysis)

#### Step 3: Screen 2 -- External Source Addition

```
AskUserQuestion(
  header: "External Sources",
  question: "Any external sources to include for analysis?",
  options: ["Add URL", "Add file path", "None — proceed"]
)
```

**Simulated user selection:** "None -- proceed"

#### Step 4: Screen 3 -- Pool Configuration Confirmation

```
Ontology Pool Configuration:
| # | Source | Type      | Path/URL                        | Domain     | Summary                   | Status    |
|---|--------|-----------|---------------------------------|------------|---------------------------|-----------|
| 1 | mcp    | doc       | /Users/heechul/podo-backend/docs| backend    | Documentation directory   | available |
| 2 | mcp    | mcp_query | grafana                         | monitoring | Grafana monitoring        | available |
| 3 | mcp    | mcp_query | mcp-clickhouse                  | database   | ClickHouse database       | available |
Total 3 sources (MCP Docs: 1, MCP Data: 2, Web: 0, File: 0)
```

```
AskUserQuestion(
  header: "Pool Confirmation",
  question: "Proceed with this ontology pool configuration?",
  options: [
    {label: "Confirm — proceed", description: "Start analysis with this configuration"},
    {label: "Reselect data sources", description: "Go back to data source selection (Screen 1)"},
    {label: "Add sources", description: "Go back to external source addition (Screen 2)"},
    {label: "Cancel", description: "Proceed without ontology pool"}
  ]
)
```

**Simulated user selection:** "Confirm -- proceed"

#### Step 5: Build and Write `ontology-scope.json`

**Written to:** `~/.prism/state/analyze-a1b2c3d4/ontology-scope.json`

```json
{
  "sources": [
    {
      "id": 1,
      "type": "doc",
      "path": "/Users/heechul/podo-backend/docs",
      "domain": "backend documentation",
      "summary": "Backend service documentation directory",
      "key_topics": ["API", "database", "architecture", "deployment"],
      "status": "available",
      "access": {
        "tools": ["prism_docs_list", "prism_docs_read", "prism_docs_search"],
        "instructions": "Use prism_docs_* tools. Pass directory path as argument."
      }
    },
    {
      "id": 2,
      "type": "mcp_query",
      "server_name": "grafana",
      "domain": "monitoring",
      "summary": "Grafana monitoring dashboards and metrics",
      "key_topics": ["prometheus", "loki", "dashboards", "alerts"],
      "status": "available",
      "access": {
        "tools": ["mcp__grafana__query_prometheus", "mcp__grafana__query_loki_logs", "mcp__grafana__list_datasources", "mcp__grafana__search_dashboards"],
        "instructions": "Call ToolSearch(query=\"select:mcp__grafana__{tool_name}\") to load each tool before use, then call directly.",
        "capabilities": "Query Prometheus metrics, Loki logs, dashboards",
        "getting_started": "Start with list_datasources to discover available data",
        "error_handling": "If a tool call fails, note the error and continue. Do NOT retry more than once.",
        "safety": "SELECT/read-only queries only"
      }
    },
    {
      "id": 3,
      "type": "mcp_query",
      "server_name": "mcp-clickhouse",
      "domain": "database",
      "summary": "ClickHouse database for analytics queries",
      "key_topics": ["clickhouse", "analytics", "queries", "slow-query-logs"],
      "status": "available",
      "access": {
        "tools": ["mcp__mcp-clickhouse__list_databases", "mcp__mcp-clickhouse__list_tables", "mcp__mcp-clickhouse__run_select_query"],
        "instructions": "Call ToolSearch(query=\"select:mcp__mcp-clickhouse__{tool_name}\") to load each tool before use, then call directly.",
        "capabilities": "List databases/tables, run SELECT queries",
        "getting_started": "Start with list_databases to discover available data",
        "error_handling": "If a tool call fails, note the error and continue. Do NOT retry more than once.",
        "safety": "SELECT/read-only queries only"
      }
    }
  ],
  "totals": {
    "doc": 1,
    "mcp_query": 2,
    "web": 0,
    "file": 0,
    "unavailable": 0
  },
  "citation_format": {
    "doc": "source:section",
    "web": "url:section",
    "file": "file:path:section",
    "mcp_query": "mcp-query:server:detail"
  }
}
```

### Phase B: Generate `{ONTOLOGY_SCOPE}` Text Block

Orchestrator reads `ontology-scope.json` and generates:

```
Your reference documents and data sources:

- doc: Backend service documentation directory (available)
  Directories: /Users/heechul/podo-backend/docs
  Access: Use prism_docs_* tools. Pass directory path as argument.
    prism_docs_list
    prism_docs_read
    prism_docs_search

- mcp-query: grafana: Grafana monitoring dashboards and metrics
  Tools (read-only): mcp__grafana__query_prometheus, mcp__grafana__query_loki_logs, mcp__grafana__list_datasources, mcp__grafana__search_dashboards
  Access: Call ToolSearch(query="select:mcp__grafana__{tool_name}") to load each tool before use, then call directly.
  Capabilities: Query Prometheus metrics, Loki logs, dashboards
  Getting started: Start with list_datasources to discover available data
  Error handling: If a tool call fails, note the error and continue. Do NOT retry more than once.

- mcp-query: mcp-clickhouse: ClickHouse database for analytics queries
  Tools (read-only): mcp__mcp-clickhouse__list_databases, mcp__mcp-clickhouse__list_tables, mcp__mcp-clickhouse__run_select_query
  Access: Call ToolSearch(query="select:mcp__mcp-clickhouse__{tool_name}") to load each tool before use, then call directly.
  Capabilities: List databases/tables, run SELECT queries
  Getting started: Start with list_databases to discover available data
  Error handling: If a tool call fails, note the error and continue. Do NOT retry more than once.

Explore these sources through your perspective's lens.
Cite findings as: source:section, url:section, file:path:section, mcp-query:server:detail.
```

This text block is stored for injection into `{ONTOLOGY_SCOPE}` in Phase 1 and Phase 2B.

### Ontology Scope Mapping Exit Gate

- [x] Phase A complete: document source checked, MCP data sources selected, external sources skipped, pool confirmed
- [x] `~/.prism/state/analyze-a1b2c3d4/ontology-scope.json` written

**NEXT ACTION: Proceed to Phase 0.8**

---

## Phase 0.8: Context & State Files

### Step 0.8.1: Write Context File

**Written to:** `~/.prism/state/analyze-a1b2c3d4/context.json`

**Data flow:** Fields sourced from `seed-analysis.json`:
- `summary` — synthesized from user description + seed analyst findings
- `research_summary.key_findings` — from `seed-analysis.json.research.findings[].finding`
- `research_summary.files_examined` — from `seed-analysis.json.research.files_examined`
- `research_summary.dimensions` — from `seed-analysis.json.dimensions`
- `report_language` — detected from user input language (Korean)
- `investigation_loops` — initialized to 0

```json
{
  "summary": "API 응답 3초 이상 지연, DB 슬로우쿼리 대량 발생. /api/v1/rooms 엔드포인트가 주요 영향. 최근 인덱스 마이그레이션(3일 전) 이후 시작. rooms 테이블 full table scan 및 filesort 발생 의심.",
  "research_summary": {
    "key_findings": [
      "/api/v1/rooms 엔드포인트의 DB 쿼리가 full table scan 수행",
      "최근 인덱스 마이그레이션에서 rooms 테이블의 복합 인덱스가 단일 컬럼 인덱스로 변경됨",
      "git log에서 3일 전 인덱스 관련 마이그레이션 커밋 발견, 슬로우쿼리 증가 시점과 일치",
      "rooms 쿼리에 ORDER BY + WHERE 절 조합이 새 인덱스와 불일치하여 filesort 발생 가능"
    ],
    "files_examined": [
      "src/routes/rooms.ts:45 — getRooms handler with DB query",
      "src/repositories/roomRepository.ts:78 — findRooms query builder",
      "migrations/20260308_update_room_indexes.sql:15 — index migration"
    ],
    "dimensions": "domain: data, failure_type: degradation, complexity: single-cause, recurrence: first-time"
  },
  "report_language": "Korean",
  "investigation_loops": 0
}
```

### Phase 0.8 Exit Gate

- [x] `perspectives.json` updated with `approved: true`
- [x] `context.json` written with structured summary
- [x] Ontology scope mapping complete (`ontology-scope.json` exists)

**NEXT ACTION: Proceed to Phase 1**

---

## Phase 1: Spawn Analysts (Finding Phase)

Team `analyze-a1b2c3d4` already exists. 3 analysts spawned in parallel.

### Step 1.1: `{CONTEXT}` Replacement Value

Derived from `context.json` per SKILL.md instructions:

```
Summary: API 응답 3초 이상 지연, DB 슬로우쿼리 대량 발생. /api/v1/rooms 엔드포인트가 주요 영향. 최근 인덱스 마이그레이션(3일 전) 이후 시작. rooms 테이블 full table scan 및 filesort 발생 의심.
Key Findings: /api/v1/rooms 엔드포인트의 DB 쿼리가 full table scan 수행, 최근 인덱스 마이그레이션에서 rooms 테이블의 복합 인덱스가 단일 컬럼 인덱스로 변경됨, git log에서 3일 전 인덱스 관련 마이그레이션 커밋 발견 슬로우쿼리 증가 시점과 일치, rooms 쿼리에 ORDER BY + WHERE 절 조합이 새 인덱스와 불일치하여 filesort 발생 가능
Files Examined: src/routes/rooms.ts:45 — getRooms handler with DB query, src/repositories/roomRepository.ts:78 — findRooms query builder, migrations/20260308_update_room_indexes.sql:15 — index migration
Dimensions: domain: data, failure_type: degradation, complexity: single-cause, recurrence: first-time
```

### Step 1.1: `{ONTOLOGY_SCOPE}` Replacement Value

The full text block generated in Phase 0.7 Phase B (see above).

### Analyst 1: Performance (performance-analyst)

**Source prompt:** `prompts/extended-archetypes.md` § Performance Lens + `prompts/finding-protocol.md`

**Pre-spawn:**
```
TaskCreate(team_name="analyze-a1b2c3d4", title="Performance Analysis", ...)
TaskUpdate(task_id=<task-id>, owner="performance-analyst")
```

**Prompt assembly order:**
1. Worker preamble (with replacements)
2. Performance Lens archetype prompt (from `extended-archetypes.md`)
3. Finding protocol (`finding-protocol.md`)

**Worker preamble replacements:**
| Placeholder | Value |
|-------------|-------|
| `{TEAM_NAME}` | `"analyze-a1b2c3d4"` |
| `{WORKER_NAME}` | `"performance-analyst"` |
| `{WORK_ACTION}` | `"Investigate from your assigned perspective. Answer ALL key questions with evidence and code references. Write findings to findings.json. Report findings via SendMessage to team-lead. Do NOT run self-verification — that happens in a separate session."` |

**Archetype + finding protocol placeholder replacements:**
| Placeholder | Replaced With | Source |
|-------------|---------------|--------|
| `{CONTEXT}` | (text block above) | `context.json` formatted |
| `{ONTOLOGY_SCOPE}` | (text block from Phase 0.7 Phase B) | `ontology-scope.json` |
| `{SHORT_ID}` | `"a1b2c3d4"` | Phase 0 |
| `{perspective-id}` | `"performance"` | `perspectives.json[0].id` |
| `{KEY_QUESTIONS}` | `"1. 인덱스 마이그레이션 전후로 /api/v1/rooms 쿼리의 실행 계획(EXPLAIN)이 어떻게 변경되었는가?\n2. 현재 rooms 테이블의 쿼리가 어떤 인덱스를 사용하고 있으며, filesort가 발생하는 조건은 무엇인가?\n3. 슬로우쿼리가 발생하는 구체적인 쿼리 패턴과 그 빈도는 어떠한가?"` | `perspectives.json[0].key_questions` |

**Spawn:**
```
Task(
  subagent_type="oh-my-claudecode:architect-medium",
  name="performance-analyst",
  team_name="analyze-a1b2c3d4",
  model="sonnet",
  run_in_background=true,
  prompt="[worker preamble]

You are the PERFORMANCE & CAPACITY ANALYST.

CONTEXT:
Summary: API 응답 3초 이상 지연...
Key Findings: ...
Files Examined: ...
Dimensions: ...

### Reference Documents
Your reference documents and data sources:
- doc: Backend service documentation directory (available)
  ...
- mcp-query: grafana: ...
- mcp-query: mcp-clickhouse: ...
Explore these sources through your perspective's lens.
...

TASKS:
1. Resource profiling: CPU/memory/IO/network...
2. Bottleneck: critical path slowest segment, DB query performance...
3. Queuing: queue depths...
4. Capacity vs. demand...

OUTPUT:
...

# Analyst Protocol — Finding Phase

## Perspective-Specific Questions

1. 인덱스 마이그레이션 전후로 /api/v1/rooms 쿼리의 실행 계획(EXPLAIN)이 어떻게 변경되었는가?
2. 현재 rooms 테이블의 쿼리가 어떤 인덱스를 사용하고 있으며, filesort가 발생하는 조건은 무엇인가?
3. 슬로우쿼리가 발생하는 구체적인 쿼리 패턴과 그 빈도는 어떠한가?

## Data Source Constraint
...

Your findings path is: ~/.prism/state/analyze-a1b2c3d4/perspectives/performance/findings.json
..."
)
```

**Findings written to:** `~/.prism/state/analyze-a1b2c3d4/perspectives/performance/findings.json`

```json
{
  "analyst": "performance",
  "findings": [
    {
      "finding": "rooms 테이블 SELECT 쿼리에서 idx_rooms_status_created_at 복합 인덱스가 삭제되고 idx_rooms_status 단일 인덱스만 남아, ORDER BY created_at 시 filesort 발생",
      "evidence": "migrations/20260308_update_room_indexes.sql:15 — DROP INDEX idx_rooms_status_created_at",
      "severity": "critical"
    },
    {
      "finding": "findRooms 쿼리의 WHERE status = ? ORDER BY created_at DESC LIMIT ? 패턴이 현재 인덱스로 커버되지 않음",
      "evidence": "src/repositories/roomRepository.ts:findRooms:78-92",
      "severity": "critical"
    },
    {
      "finding": "rooms 테이블 row count 약 50만건으로, full table scan 시 3초+ 소요 예상",
      "evidence": "mcp-query:mcp-clickhouse:SELECT count() FROM rooms",
      "severity": "high"
    }
  ]
}
```

### Analyst 2: Root Cause (root-cause-analyst)

**Source prompt:** `prompts/core-archetypes.md` § Root Cause Lens + `prompts/finding-protocol.md`

**Pre-spawn:**
```
TaskCreate(team_name="analyze-a1b2c3d4", title="Root Cause Analysis", ...)
TaskUpdate(task_id=<task-id>, owner="root-cause-analyst")
```

**Placeholder replacements:**
| Placeholder | Replaced With |
|-------------|---------------|
| `{CONTEXT}` | (same text block as above) |
| `{ONTOLOGY_SCOPE}` | (same text block as above) |
| `{SHORT_ID}` | `"a1b2c3d4"` |
| `{perspective-id}` | `"root-cause"` |
| `{KEY_QUESTIONS}` | `"1. migrations/20260308_update_room_indexes.sql에서 정확히 어떤 인덱스가 삭제/변경되었으며...\n2. roomRepository.ts의 findRooms 쿼리 빌더가 생성하는 SQL과 새 인덱스 구조 간의 불일치 지점은...\n3. 인덱스 변경 외에 동시에 발생한 다른 변경(ORM 의존성 업데이트 등)이 성능에 기여했을 가능성은..."` |

**Spawn:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="root-cause-analyst",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="[worker preamble + Root Cause Lens archetype + finding-protocol with all placeholders replaced]"
)
```

**Findings written to:** `~/.prism/state/analyze-a1b2c3d4/perspectives/root-cause/findings.json`

```json
{
  "analyst": "root-cause",
  "findings": [
    {
      "finding": "근본 원인: 마이그레이션에서 복합 인덱스(status, created_at)를 단일 인덱스(status)로 교체하면서 ORDER BY created_at 쿼리가 인덱스를 활용하지 못하게 됨",
      "evidence": "migrations/20260308_update_room_indexes.sql:12-18 — DROP INDEX idx_rooms_status_created_at, ADD INDEX idx_rooms_status(status)",
      "severity": "critical"
    },
    {
      "finding": "ORM 의존성 업데이트(def5678)는 쿼리 생성 패턴에 영향 없음 확인 — 단순 버전 범프",
      "evidence": "git diff def5678 — package.json only, no query builder changes",
      "severity": "low"
    },
    {
      "finding": "마이그레이션 PR 리뷰에서 EXPLAIN 검증 절차 없이 승인됨",
      "evidence": "git log --format='%H %s' abc1234 — no EXPLAIN test evidence in commit",
      "severity": "high"
    }
  ]
}
```

### Analyst 3: Systems (systems-analyst)

**Source prompt:** `prompts/core-archetypes.md` § Systems Lens + `prompts/finding-protocol.md`

**Pre-spawn:**
```
TaskCreate(team_name="analyze-a1b2c3d4", title="Systems Analysis", ...)
TaskUpdate(task_id=<task-id>, owner="systems-analyst")
```

**Placeholder replacements:**
| Placeholder | Replaced With |
|-------------|---------------|
| `{CONTEXT}` | (same text block) |
| `{ONTOLOGY_SCOPE}` | (same text block) |
| `{SHORT_ID}` | `"a1b2c3d4"` |
| `{perspective-id}` | `"systems"` |
| `{KEY_QUESTIONS}` | `"1. 현재 슬로우쿼리 모니터링 및 알림 체계가 있는가?...\n2. 인덱스 마이그레이션 시 쿼리 성능 영향을 사전 검증하는 프로세스가...\n3. DB 쿼리 타임아웃, 커넥션 풀 제한 등 성능 저하 시 blast radius를 제한하는..."` |

**Spawn:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="systems-analyst",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="[worker preamble + Systems Lens archetype + finding-protocol with all placeholders replaced]"
)
```

**Findings written to:** `~/.prism/state/analyze-a1b2c3d4/perspectives/systems/findings.json`

```json
{
  "analyst": "systems",
  "findings": [
    {
      "finding": "슬로우쿼리 모니터링 알림 체계 부재 — Grafana 대시보드에 DB 쿼리 latency 패널이 없고 알림 규칙도 미설정",
      "evidence": "mcp-query:grafana:search_dashboards — no slow query dashboard found",
      "severity": "high"
    },
    {
      "finding": "DB 쿼리 타임아웃이 30초로 설정되어 있어 3초 슬로우쿼리가 커넥션 풀을 점유하며 cascade 가능",
      "evidence": "src/config/database.ts:queryTimeout:12 — timeout: 30000",
      "severity": "high"
    },
    {
      "finding": "마이그레이션 CI 파이프라인에 EXPLAIN 자동 검증 스텝 없음",
      "evidence": ".github/workflows/migrate.yml — no EXPLAIN step",
      "severity": "medium"
    }
  ]
}
```

### Phase 1 Exit Gate

- [x] All 3 analyst tasks created and owners pre-assigned
- [x] All 3 analysts spawned in parallel (run_in_background=true)

**NEXT ACTION: Read `docs/later-phases.md` and proceed to Phase 2**

---

## Phase 2: Collect Findings & Spawn Verification Sessions

**Source:** `skills/analyze/docs/later-phases.md`

### Stage A: Collect Findings

#### Step 2A.1: Wait for Analyst Findings

```
TaskList() → monitor for all 3 analysts to complete
```

Each analyst sends findings via `SendMessage` in this format:
```markdown
## Findings — {perspective-id}

### Session
- context_id: analyze-a1b2c3d4
- perspective_id: {perspective-id}

### Findings
{findings with evidence}
```

#### Step 2A.2: Drain Background Task Outputs

```
TaskList() → find completed tasks → TaskOutput() for each
```

Done for all 3 analysts: `performance-analyst`, `root-cause-analyst`, `systems-analyst`.

#### Step 2A.3: Shutdown Finding Analysts

```
SendMessage(type: "shutdown_request", recipient: "performance-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "root-cause-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "systems-analyst", content: "Finding phase complete.")
```

Wait for `shutdown_response(approve=true)` from each, then drain task outputs again.

#### Stage A Exit Gate

- [x] All 3 analyst findings received via SendMessage
- [x] All 3 `findings.json` files written:
  - `~/.prism/state/analyze-a1b2c3d4/perspectives/performance/findings.json`
  - `~/.prism/state/analyze-a1b2c3d4/perspectives/root-cause/findings.json`
  - `~/.prism/state/analyze-a1b2c3d4/perspectives/systems/findings.json`
- [x] All 3 finding analysts shut down
- [x] All background task outputs drained

**NEXT ACTION: Proceed to Stage B**

---

### Stage B: Spawn Verification Sessions

**Key difference from Phase 1:** NEW sessions using `verification-protocol.md` instead of `finding-protocol.md`. Each verifier reads its perspective's `findings.json` and runs `prism_interview` for Socratic verification.

#### Step 2B.1: `{summary}` Replacement Value

Derived from `context.json.summary`:
```
API 응답 3초 이상 지연, DB 슬로우쿼리 대량 발생. /api/v1/rooms 엔드포인트가 주요 영향. 최근 인덱스 마이그레이션(3일 전) 이후 시작.
```

This is used in the `prism_interview` topic field: `"{perspective-id} findings verification — {summary}"`

#### Verifier 1: Performance (performance-verifier)

**Source prompt:** `prompts/extended-archetypes.md` § Performance Lens + `prompts/verification-protocol.md`

**Pre-spawn:**
```
TaskCreate(team_name="analyze-a1b2c3d4", title="Performance Verification", ...)
TaskUpdate(task_id=<task-id>, owner="performance-verifier")
```

**Worker preamble replacements:**
| Placeholder | Value |
|-------------|-------|
| `{TEAM_NAME}` | `"analyze-a1b2c3d4"` |
| `{WORKER_NAME}` | `"performance-verifier"` |
| `{WORK_ACTION}` | `"Read your findings from the path specified in your verification protocol. Run self-verification via MCP tools (prism_interview). Re-investigate with tools as needed to answer interview questions. Report verified findings via SendMessage to team-lead."` |

**Archetype + verification protocol placeholder replacements:**
| Placeholder | Replaced With | Source |
|-------------|---------------|--------|
| `{CONTEXT}` | (same text block from Phase 1) | `context.json` formatted |
| `{ONTOLOGY_SCOPE}` | (same text block from Phase 0.7) | `ontology-scope.json` |
| `{SHORT_ID}` | `"a1b2c3d4"` | Phase 0 |
| `{perspective-id}` | `"performance"` | `perspectives.json[0].id` |
| `{summary}` | `"API 응답 3초 이상 지연, DB 슬로우쿼리 대량 발생. /api/v1/rooms 엔드포인트가 주요 영향. 최근 인덱스 마이그레이션(3일 전) 이후 시작."` | `context.json.summary` |

**Spawn:**
```
Task(
  subagent_type="oh-my-claudecode:architect-medium",
  name="performance-verifier",
  team_name="analyze-a1b2c3d4",
  model="sonnet",
  run_in_background=true,
  prompt="[worker preamble]

You are the PERFORMANCE & CAPACITY ANALYST.

CONTEXT:
Summary: API 응답 3초 이상 지연...
...

### Reference Documents
[ontology scope text block]

TASKS: [archetype tasks — but verification-protocol says to IGNORE these imperative instructions]

# Analyst Protocol — Verification Phase

## Role Clarification — READ THIS FIRST
The archetype prompt above describes your analytical perspective and domain expertise. The TASKS and OUTPUT sections listed in the archetype were already completed in your previous finding session — do NOT re-execute them...

## Context
You are the same analyst who produced findings in a previous session. Your findings are saved at:
~/.prism/state/analyze-a1b2c3d4/perspectives/performance/findings.json

## Self-Verification (MCP)
Your session path is: analyze-a1b2c3d4/perspectives/performance

### Steps
#### 1. Read Your Findings
Read ~/.prism/state/analyze-a1b2c3d4/perspectives/performance/findings.json

#### 2. Start Interview
mcp__prism-mcp__prism_interview(
  context_id=\"analyze-a1b2c3d4\",
  perspective_id=\"performance\",
  topic=\"performance findings verification — API 응답 3초 이상 지연, DB 슬로우쿼리 대량 발생. /api/v1/rooms 엔드포인트가 주요 영향. 최근 인덱스 마이그레이션(3일 전) 이후 시작.\"
)
..."
)
```

**Verifier's `prism_interview` call:**
```
mcp__prism-mcp__prism_interview(
  context_id="analyze-a1b2c3d4",
  perspective_id="performance",
  topic="performance findings verification — API 응답 3초 이상 지연, DB 슬로우쿼리 대량 발생. /api/v1/rooms 엔드포인트가 주요 영향. 최근 인덱스 마이그레이션(3일 전) 이후 시작."
)
→ returns { context_id: "analyze-a1b2c3d4", perspective_id: "performance", round: 1, question: "..." }
```

The verifier answers questions in a loop, submitting responses:
```
mcp__prism-mcp__prism_interview(
  context_id="analyze-a1b2c3d4",
  perspective_id="performance",
  response="{answer with evidence}"
)
→ returns { ..., round: 2, continue: true/false, score: N, question?: "...", reason?: "..." }
```

Loop continues until `continue: false`.

**Verified findings sent via SendMessage:**
```markdown
## Verified Findings — performance

### Session
- context_id: analyze-a1b2c3d4
- perspective_id: performance
- rounds: 3
- score: 8.5
- verdict: PASS

### Findings
[refined findings from Q&A]

### Key Q&A Clarifications
[important clarifications that strengthened analysis]
```

#### Verifier 2: Root Cause (root-cause-verifier)

**Source prompt:** `prompts/core-archetypes.md` § Root Cause Lens + `prompts/verification-protocol.md`

**Placeholder replacements:**
| Placeholder | Replaced With |
|-------------|---------------|
| `{CONTEXT}` | (same text block) |
| `{ONTOLOGY_SCOPE}` | (same text block) |
| `{SHORT_ID}` | `"a1b2c3d4"` |
| `{perspective-id}` | `"root-cause"` |
| `{summary}` | `"API 응답 3초 이상 지연..."` |

**Spawn:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="root-cause-verifier",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="[worker preamble + Root Cause Lens + verification-protocol with all placeholders replaced]"
)
```

**prism_interview call:**
```
mcp__prism-mcp__prism_interview(
  context_id="analyze-a1b2c3d4",
  perspective_id="root-cause",
  topic="root-cause findings verification — API 응답 3초 이상 지연..."
)
```

#### Verifier 3: Systems (systems-verifier)

**Source prompt:** `prompts/core-archetypes.md` § Systems Lens + `prompts/verification-protocol.md`

**Placeholder replacements:**
| Placeholder | Replaced With |
|-------------|---------------|
| `{CONTEXT}` | (same text block) |
| `{ONTOLOGY_SCOPE}` | (same text block) |
| `{SHORT_ID}` | `"a1b2c3d4"` |
| `{perspective-id}` | `"systems"` |
| `{summary}` | `"API 응답 3초 이상 지연..."` |

**Spawn:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="systems-verifier",
  team_name="analyze-a1b2c3d4",
  model="opus",
  run_in_background=true,
  prompt="[worker preamble + Systems Lens + verification-protocol with all placeholders replaced]"
)
```

**prism_interview call:**
```
mcp__prism-mcp__prism_interview(
  context_id="analyze-a1b2c3d4",
  perspective_id="systems",
  topic="systems findings verification — API 응답 3초 이상 지연..."
)
```

#### Step 2B.2: Wait for Verified Findings

All 3 verifiers complete their `prism_interview` loops and send verified findings via `SendMessage`.

#### Step 2B.3: Drain Background Task Outputs

```
TaskList() → TaskOutput() for each completed verifier
```

#### Step 2B.4: Shutdown Verifiers

```
SendMessage(type: "shutdown_request", recipient: "performance-verifier", content: "Verification complete.")
SendMessage(type: "shutdown_request", recipient: "root-cause-verifier", content: "Verification complete.")
SendMessage(type: "shutdown_request", recipient: "systems-verifier", content: "Verification complete.")
```

Wait for shutdown acknowledgment, drain task outputs again.

#### Step 2B.5: Persist Verified Results

Write verified findings files:
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-performance.md`
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-root-cause.md`
- `~/.prism/state/analyze-a1b2c3d4/verified-findings-systems.md`

MCP session artifacts at:
- `~/.prism/state/analyze-a1b2c3d4/perspectives/performance/interview.json` + `findings.json`
- `~/.prism/state/analyze-a1b2c3d4/perspectives/root-cause/interview.json` + `findings.json`
- `~/.prism/state/analyze-a1b2c3d4/perspectives/systems/interview.json` + `findings.json`

#### Step 2B.6: Compile Verified Findings

**Written to:** `~/.prism/state/analyze-a1b2c3d4/analyst-findings.md`

Contents include all verified findings from all 3 perspectives plus verification scores summary:

```markdown
# Compiled Analyst Findings

## Verification Scores Summary
| Perspective | Rounds | Weighted Total | Verdict |
|-------------|--------|---------------|---------|
| performance | 3 | 8.5 | PASS |
| root-cause | 4 | 9.0 | PASS |
| systems | 3 | 8.0 | PASS |

## Performance & Capacity
[verified findings from performance-verifier]

## Root Cause
[verified findings from root-cause-verifier]

## Systems & Architecture
[verified findings from systems-verifier]
```

### Phase 2 Exit Gate

- [x] All 3 verifiers completed and sent verified findings
- [x] All background task outputs drained
- [x] All 3 verifiers shut down
- [x] All verified findings persisted (3 `verified-findings-*.md` files)
- [x] Compiled findings written to `analyst-findings.md`

**NEXT ACTION: Proceed to Phase 3**

---

## Phase 3: Synthesis & Report

### Step 3.1: Integrate Findings

Read `~/.prism/state/analyze-a1b2c3d4/analyst-findings.md` to get all verified findings.

### Step 3.2: Fill Report Template

Read `templates/report.md` and fill all sections.

**Critical:** Report written in **Korean** (`context.json.report_language = "Korean"`).

**Report output (abbreviated structure):**

```markdown
# Analysis Report

## Executive Summary
최근 인덱스 마이그레이션(2026-03-08)에서 rooms 테이블의 복합 인덱스(status, created_at)가
단일 인덱스(status)로 변경되면서 /api/v1/rooms 엔드포인트의 DB 쿼리가 full table scan 및
filesort를 수행하게 되어 API 응답이 평균 3초 이상 지연되고 있음. 복합 인덱스 복원이 즉시 필요.

## Analysis Overview
- **Subject**: API 응답 지연 및 DB 슬로우쿼리 — /api/v1/rooms 엔드포인트
- **Severity**: SEV2
- **Duration**: 2026-03-08 ~ 진행 중
- **Status**: Active
- **Affected Systems**: rooms API, database
- **User Impact**: /api/v1/rooms 호출 시 3초+ 응답 지연
- **Perspectives Used**: Performance & Capacity, Root Cause, Systems & Architecture

## Timeline
| Time | Event | Evidence | Confidence |
|------|-------|----------|------------|
| 2026-03-08 | 인덱스 마이그레이션 배포 | git commit abc1234 | High |
| 2026-03-08~ | 슬로우쿼리 로그 급증 시작 | DB slow query logs | High |
| 2026-03-10 | 사용자 보고 — API 응답 지연 | User report | High |

## Perspective Findings

### Performance & Capacity
[full verified findings]

### Root Cause
[full verified findings]

### Systems & Architecture
[full verified findings]

## Integrated Analysis
- **Convergence**: 3개 관점 모두 인덱스 마이그레이션이 직접 원인임에 동의
- **Divergence**: 없음 — 단일 원인 사건
- **Emergent Insights**: 마이그레이션 검증 프로세스와 모니터링 부재가 복합적으로 작용하여 조기 감지 실패

## Socratic Verification Summary

### Per-Analyst Verification Scores
| Analyst | Rounds | Weighted Total | Verdict |
|---------|--------|---------------|---------|
| performance | 3 | 8.5 | PASS |
| root-cause | 4 | 9.0 | PASS |
| systems | 3 | 8.0 | PASS |

### Key Clarifications from Socratic Verification Q&A
#### Performance
| Round | Question | Answer | Impact on Findings |
|-------|----------|--------|--------------------|
...

### Unresolved Ambiguities
| Analyst | Ambiguity | Reason Unresolved | Impact on Conclusions |
|---------|-----------|-------------------|-----------------------|
| (none) | N/A | N/A | N/A |

## Recommendations
| Action | Priority | UX Impact | Eng Effort | Verified? |
|--------|----------|-----------|------------|-----------|
| 복합 인덱스 복원 | P0 | High | Low | Yes |
| 슬로우쿼리 모니터링 알림 추가 | P1 | Medium | Low | Yes |
| 마이그레이션 CI에 EXPLAIN 검증 추가 | P1 | N/A | Medium | Yes |
| DB 쿼리 타임아웃 30초→5초 조정 | P2 | Medium | Low | Yes |

### Immediate (This Week)
- rooms 테이블에 복합 인덱스 (status, created_at) 복원
- DB 쿼리 타임아웃 30초→5초 조정

### Short-Term (This Month)
- 슬로우쿼리 모니터링 Grafana 대시보드 및 알림 설정
- 마이그레이션 CI 파이프라인에 EXPLAIN 자동 검증 스텝 추가

### Long-Term (This Quarter)
- DB 스키마 변경 시 자동 성능 회귀 테스트 체계 구축
- 쿼리 성능 기준선(baseline) 모니터링 도입

### Monitoring & Alerting
- 슬로우쿼리 3초 이상 알림
- /api/v1/rooms p95 latency 대시보드

## Prevention Checklist
- [ ] Root cause permanently fixed (복합 인덱스 복원)
- [ ] Monitoring for early detection (슬로우쿼리 알림)
- [ ] Runbook updated
- [ ] Post-review scheduled
- [ ] Similar risks elsewhere mitigated

## Appendix
...
```

### Step 3.3: User Decision

```
AskUserQuestion(
  header: "Analysis Complete",
  question: "Is the analysis complete?",
  options: ["Complete", "Need deeper investigation", "Add recommendations", "Share with team"]
)
```

**Simulated user selection:** "Complete"

(If "Need deeper investigation" were selected: check `investigation_loops` in `context.json` (currently 0, max 2), increment, append iteration summary to `prior-iterations.md`, ask for gap focus, spawn new analysts through Phase 1 -> Phase 2 flow, return to Phase 3.)

**NEXT ACTION: Proceed to Phase 4**

---

## Phase 4: Cleanup

**Source:** `skills/shared-v3/team-teardown.md`

### Steps

1. **Enumerate active teammates:**
```
TaskList() → filter for non-completed tasks in team "analyze-a1b2c3d4"
```
(Expected: all tasks completed by this point)

2. **Send shutdown requests** to any remaining active teammates:
```
SendMessage(type: "shutdown_request", recipient: "{name}", ...)
```
(Expected: none remaining)

3. **Await shutdown responses:**
Each responds with `shutdown_response(approve=true)`.

4. **Delete team:**
```
TeamDelete(team_name: "analyze-a1b2c3d4")
```

---

## Data Flow Summary

### Artifact Chain

```
Phase 0    → {short-id} = a1b2c3d4
             {description} = user prompt (Korean)

Phase 0.5  → seed-analysis.json
             ├── severity, status, dimensions, research
             └── Written by: seed-analyst (architect/opus)

Phase 0.55 → perspectives.json
             ├── perspectives[] (id, name, scope, key_questions, model, agent_type, rationale)
             ├── rules_applied (6 mandatory rule checks)
             ├── selection_summary
             ├── Read from: seed-analysis.json (dimensions → archetype mapping)
             └── Written by: perspective-generator (architect/opus)

Phase 0.6  → perspectives.json (updated in-place)
             ├── + approved: true
             ├── + user_modifications: []
             └── Updated by: orchestrator

Phase 0.7  → ontology-scope.json
             ├── sources[] (doc, mcp_query entries)
             ├── totals, citation_format
             └── Written by: orchestrator (via ontology-scope-mapping.md)

Phase 0.8  → context.json
             ├── summary (from description + seed findings)
             ├── research_summary (from seed-analysis.json)
             ├── report_language: "Korean" (detected from input)
             ├── investigation_loops: 0
             └── Written by: orchestrator

Phase 1    → perspectives/{perspective-id}/findings.json (x3)
             ├── Each reads: context.json, ontology-scope.json, perspectives.json
             ├── Prompt uses: finding-protocol.md
             └── Written by: each analyst agent

Phase 2A   → [shutdown finding analysts, drain outputs]

Phase 2B   → perspectives/{perspective-id}/interview.json (x3) — MCP artifacts
             → verified-findings-{perspective-id}.md (x3)
             → analyst-findings.md (compiled)
             ├── Each reads: findings.json (from Phase 1)
             ├── Prompt uses: verification-protocol.md (NEW sessions)
             ├── MCP tool: prism_interview (with {summary} in topic)
             └── Written by: each verifier agent + orchestrator

Phase 3    → Final report (from report.md template)
             ├── Reads: analyst-findings.md, context.json (for report_language)
             ├── Language: Korean
             └── Written by: orchestrator

Phase 4    → TeamDelete("analyze-a1b2c3d4")
```

### Placeholder Replacement Map

| Placeholder | Set In | Value | Used In |
|-------------|--------|-------|---------|
| `{short-id}` | Phase 0.2 | `a1b2c3d4` | All path templates |
| `{SHORT_ID}` | Phase 0.2 | `a1b2c3d4` | All prompt placeholders |
| `{DESCRIPTION}` | Phase 0.1 | User's Korean prompt | seed-analyst, perspective-generator |
| `{summary}` | Phase 0.5.1 / Phase 2B | Short summary / context.json.summary | TeamCreate description / verification-protocol prism_interview topic |
| `{TEAM_NAME}` | Phase 0.5.1 | `"analyze-a1b2c3d4"` | All worker preambles |
| `{WORKER_NAME}` | Per spawn | Agent name | Worker preambles |
| `{WORK_ACTION}` | Per spawn | Role-specific action | Worker preambles |
| `{CONTEXT}` | Phase 0.8 | Formatted text from context.json | Phase 1 analysts, Phase 2B verifiers |
| `{ONTOLOGY_SCOPE}` | Phase 0.7 | Text block from ontology-scope.json | Phase 1 analysts, Phase 2B verifiers |
| `{KEY_QUESTIONS}` | Phase 0.55 | From perspectives.json per perspective | Phase 1 analysts only |
| `{perspective-id}` | Phase 0.55 | `"performance"`, `"root-cause"`, `"systems"` | Phase 1 + Phase 2B |

### Agent Spawn Summary

| Phase | Agent Name | subagent_type | model | Protocol |
|-------|-----------|---------------|-------|----------|
| 0.5 | seed-analyst | oh-my-claudecode:architect | opus | seed-analyst.md |
| 0.55 | perspective-generator | oh-my-claudecode:architect | opus | perspective-generator.md |
| 1 | performance-analyst | oh-my-claudecode:architect-medium | sonnet | extended-archetypes.md § Performance + finding-protocol.md |
| 1 | root-cause-analyst | oh-my-claudecode:architect | opus | core-archetypes.md § Root Cause + finding-protocol.md |
| 1 | systems-analyst | oh-my-claudecode:architect | opus | core-archetypes.md § Systems + finding-protocol.md |
| 2B | performance-verifier | oh-my-claudecode:architect-medium | sonnet | extended-archetypes.md § Performance + verification-protocol.md |
| 2B | root-cause-verifier | oh-my-claudecode:architect | opus | core-archetypes.md § Root Cause + verification-protocol.md |
| 2B | systems-verifier | oh-my-claudecode:architect | opus | core-archetypes.md § Systems + verification-protocol.md |

### Session Split: Finding vs. Verification

| Aspect | Phase 1 (Finding) | Phase 2B (Verification) |
|--------|-------------------|------------------------|
| Protocol file | `finding-protocol.md` | `verification-protocol.md` |
| Session type | NEW session per analyst | NEW session per verifier (separate from finding) |
| Agent name | `{perspective-id}-analyst` | `{perspective-id}-verifier` |
| Task owner | `{perspective-id}-analyst` | `{perspective-id}-verifier` |
| Primary action | Investigate + write findings.json | Read findings.json + prism_interview loop |
| MCP tools | Ontology tools only | Ontology tools + prism_interview |
| Output | findings.json + SendMessage | verified findings SendMessage |
| Archetype tasks | EXECUTE (investigate) | IGNORE (already completed) |
| `{KEY_QUESTIONS}` | Injected | NOT injected (not in verification-protocol) |
| `{summary}` | NOT used | Injected into prism_interview topic |

### File System State at Completion

```
~/.prism/state/analyze-a1b2c3d4/
├── seed-analysis.json            (Phase 0.5)
├── perspectives.json             (Phase 0.55, updated 0.6)
├── ontology-scope.json           (Phase 0.7)
├── context.json                  (Phase 0.8)
├── perspectives/
│   ├── performance/
│   │   ├── findings.json         (Phase 1)
│   │   └── interview.json        (Phase 2B — MCP artifact)
│   ├── root-cause/
│   │   ├── findings.json         (Phase 1)
│   │   └── interview.json        (Phase 2B)
│   └── systems/
│       ├── findings.json         (Phase 1)
│       └── interview.json        (Phase 2B)
├── verified-findings-performance.md   (Phase 2B.5)
├── verified-findings-root-cause.md    (Phase 2B.5)
├── verified-findings-systems.md       (Phase 2B.5)
└── analyst-findings.md           (Phase 2B.6)
```
