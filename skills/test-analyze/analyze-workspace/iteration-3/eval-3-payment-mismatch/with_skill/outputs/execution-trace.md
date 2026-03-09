# Execution Trace: Payment Mismatch Analysis

**Task prompt:** "결제 시스템에서 Apple IAP 영수증 금액과 DB에 저장된 결제 금액이 불일치하는 케이스가 발견됐어. 약 200건 정도 되고 모두 최근 1주일 이내에 발생했어. 환불 처리도 꼬여있을 수 있어. 분석해줘"

**Skill version:** 4.1.0

---

## Prerequisite: Agent Team Mode (HARD GATE)

**Source:** `skills/shared-v3/prerequisite-gate.md`
**Parameter:** `{PROCEED_TO}` = "Phase 0"

### Execution:
1. `Read("~/.claude/settings.json")`
2. Check: `env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS === "1"`
3. **Result:** Setting exists and equals `"1"` -> Proceed to Phase 0

---

## Phase 0: Problem Intake

### Step 0.1: Collect Description

The user provided a description via `$ARGUMENTS`, so it is used directly. No `AskUserQuestion` needed.

**Collected description:**
```
결제 시스템에서 Apple IAP 영수증 금액과 DB에 저장된 결제 금액이 불일치하는 케이스가 발견됐어. 약 200건 정도 되고 모두 최근 1주일 이내에 발생했어. 환불 처리도 꼬여있을 수 있어. 분석해줘
```

### Step 0.2: Generate Session ID and State Directory

```bash
uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8
# Output: a3f7c1e2   (example)
```

```bash
mkdir -p ~/.prism/state/analyze-a3f7c1e2
```

**Variables established:**
- `{short-id}` = `a3f7c1e2` (used in paths)
- `{SHORT_ID}` = `a3f7c1e2` (used in prompt placeholders)

### Phase 0 Exit Gate

- [x] Description collected: Yes (from $ARGUMENTS)
- [x] `{short-id}` generated and state directory created: `~/.prism/state/analyze-a3f7c1e2/`

> **NEXT ACTION: Proceed to Phase 0.5 Step 0.5.1 -- Create team.**

---

## Phase 0.5: Team Creation & Seed Analysis

### Step 0.5.1: Create Team

**`{summary}` derivation:** From user description, a short (<=10 word) summary is derived:

`{summary}` = `"Apple IAP receipt-DB payment amount mismatch"`

```
TeamCreate(
  team_name: "analyze-a3f7c1e2",
  description: "Analysis: Apple IAP receipt-DB payment amount mismatch"
)
```

### Step 0.5.2: Spawn Seed Analyst

**Files read:**
1. `skills/shared-v3/worker-preamble.md`
2. `skills/analyze/prompts/seed-analyst.md`

**Worker preamble placeholder replacements:**
- `{TEAM_NAME}` = `"analyze-a3f7c1e2"`
- `{WORKER_NAME}` = `"seed-analyst"`
- `{WORK_ACTION}` = `"Actively investigate using available tools (Grep, Read, Bash, MCP). Evaluate dimensions and severity. Write findings to seed-analysis.json. Report via SendMessage."`

**Seed-analyst prompt placeholder replacements:**
- `{DESCRIPTION}` = `"결제 시스템에서 Apple IAP 영수증 금액과 DB에 저장된 결제 금액이 불일치하는 케이스가 발견됐어. 약 200건 정도 되고 모두 최근 1주일 이내에 발생했어. 환불 처리도 꼬여있을 수 있어. 분석해줘"`
- `{SHORT_ID}` = `"a3f7c1e2"`

**Assembled prompt (structure):**
```
You are a TEAM WORKER in team "analyze-a3f7c1e2". Your name is "seed-analyst".
You report to the team lead ("team-lead").

== WORK PROTOCOL ==
1. TaskList -> find my assigned task -> TaskUpdate(status="in_progress")
2. Actively investigate using available tools (Grep, Read, Bash, MCP). Evaluate dimensions and severity. Write findings to seed-analysis.json. Report via SendMessage.
3. Report findings via SendMessage to team-lead
4. TaskUpdate(status="completed")
5. On shutdown_request -> respond with shutdown_response(approve=true)

[Full seed-analyst.md prompt with {DESCRIPTION} and {SHORT_ID} replaced]
```

**Spawn call:**
```
TaskCreate(task for seed-analyst)
TaskUpdate(owner="seed-analyst")

Task(
  subagent_type="oh-my-claudecode:architect",
  name="seed-analyst",
  team_name="analyze-a3f7c1e2",
  model="opus",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

### Step 0.5.3: Receive Seed Analyst Results

The seed analyst performs active research (Grep for IAP-related code, Read payment processing files, Bash git log, potentially ToolSearch for Sentry/Grafana MCP tools) and sends results via `SendMessage`.

**Expected `seed-analysis.json` artifact** written to `~/.prism/state/analyze-a3f7c1e2/seed-analysis.json`:

```json
{
  "severity": "SEV2",
  "status": "Active",
  "dimensions": {
    "domain": "data",
    "failure_type": "data_loss",
    "evidence_available": ["logs", "code diffs", "traces"],
    "complexity": "multi-factor",
    "recurrence": "first-time"
  },
  "research": {
    "findings": [
      {
        "id": 1,
        "finding": "Apple IAP receipt validation endpoint extracts transaction amount from receipt payload but applies currency conversion using a stale exchange rate table updated weekly",
        "source": "src/payments/apple-iap-validator.ts:validateReceipt:47",
        "tool_used": "Read",
        "severity": "critical"
      },
      {
        "id": 2,
        "finding": "DB payment record stores the converted KRW amount, but the Apple receipt contains USD. Discrepancy appears when exchange rate changes between receipt issuance and DB write",
        "source": "src/payments/payment-repository.ts:savePayment:82",
        "tool_used": "Grep",
        "severity": "critical"
      },
      {
        "id": 3,
        "finding": "Refund handler references the DB stored amount (converted KRW) rather than original Apple receipt amount, causing refund amount mismatch",
        "source": "src/payments/refund-handler.ts:processRefund:35",
        "tool_used": "Read",
        "severity": "high"
      },
      {
        "id": 4,
        "finding": "Recent commit 7 days ago changed exchange rate update frequency from daily to weekly",
        "source": "git log: abc1234 - chore: reduce exchange rate update frequency",
        "tool_used": "Bash",
        "severity": "high"
      },
      {
        "id": 5,
        "finding": "No reconciliation job exists to compare Apple receipt amounts with DB stored amounts",
        "source": "src/jobs/ - no reconciliation job found via Glob",
        "tool_used": "Grep",
        "severity": "medium"
      }
    ],
    "files_examined": [
      "src/payments/apple-iap-validator.ts:47 — receipt validation and amount extraction",
      "src/payments/payment-repository.ts:82 — payment DB write with converted amount",
      "src/payments/refund-handler.ts:35 — refund processing logic",
      "src/payments/exchange-rate-service.ts:12 — exchange rate lookup",
      "src/jobs/ — searched for reconciliation jobs, none found"
    ],
    "mcp_queries": [
      "ToolSearch: searched for 'sentry' — found mcp__sentry tools, queried recent payment errors",
      "ToolSearch: searched for 'grafana' — found mcp__grafana tools, queried payment error rate metrics"
    ],
    "recent_changes": [
      "abc1234 — chore: reduce exchange rate update frequency (7 days ago)",
      "def5678 — feat: add Apple IAP v2 receipt support (10 days ago)"
    ]
  }
}
```

**Key dimension decisions:**
- `domain`: `"data"` -- this is a data discrepancy issue (amounts in Apple receipts vs DB)
- `failure_type`: `"data_loss"` -- financial data integrity is compromised; stored amounts don't match source-of-truth (underscore format per JSON schema enum)
- `complexity`: `"multi-factor"` -- involves currency conversion timing, exchange rate staleness, and refund pipeline interaction
- `recurrence`: `"first-time"` -- triggered by a recent change (exchange rate update frequency reduction)

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
TaskList -> find completed tasks -> TaskOutput for each
```

### Phase 0.5 Exit Gate

- [x] Team created: `analyze-a3f7c1e2`
- [x] Seed-analyst results received: via SendMessage
- [x] `seed-analysis.json` written: at `~/.prism/state/analyze-a3f7c1e2/seed-analysis.json`
- [x] Seed-analyst shut down: shutdown_request sent, shutdown_response received
- [x] All background task outputs drained

> **NEXT ACTION: Proceed to Phase 0.55 -- Perspective Generation.**

---

## Phase 0.55: Perspective Generation

### Step 0.55.1: Spawn Perspective Generator

**Files read:**
1. `skills/shared-v3/worker-preamble.md`
2. `skills/analyze/prompts/perspective-generator.md`

**Worker preamble placeholder replacements:**
- `{TEAM_NAME}` = `"analyze-a3f7c1e2"`
- `{WORKER_NAME}` = `"perspective-generator"`
- `{WORK_ACTION}` = `"Read seed-analysis.json, apply archetype mapping rules and mandatory rules, generate perspective candidates. Write perspectives.json. Report via SendMessage."`

**Perspective-generator prompt placeholder replacements:**
- `{SHORT_ID}` = `"a3f7c1e2"`
- `{DESCRIPTION}` = (full user description)

**Spawn call:**
```
TaskCreate(task for perspective-generator)
TaskUpdate(owner="perspective-generator")

Task(
  subagent_type="oh-my-claudecode:architect",
  name="perspective-generator",
  team_name="analyze-a3f7c1e2",
  model="opus",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

### Step 0.55.2: Perspective Generator Internal Logic

The perspective generator:

1. **Reads** `~/.prism/state/analyze-a3f7c1e2/seed-analysis.json`
2. **Applies archetype mapping** (Step 2 of perspective-generator.md):
   - Seed dimensions: `domain=data`, `failure_type=data_loss`
   - Matches row: "Payment discrepancy, billing error, revenue data mismatch" -> `financial` + `data-integrity` + `root-cause`
   - Also partially matches: "Data corruption, stale reads, replication lag" -> `data-integrity` + `root-cause` + `systems`
3. **Applies mandatory rules** (Step 3):
   - **Core archetype required**: `root-cause` is a core archetype -- SATISFIED
   - **Recurring -> systems**: `dimensions.recurrence == "first-time"` -- N/A (not recurring)
   - **Evidence-backed only**: All selected perspectives have supporting evidence in research.findings -- SATISFIED
   - **Minimum perspectives**: 4 perspectives >= 2 -- SATISFIED
   - **Complexity scaling**: `dimensions.complexity == "multi-factor"` -> 3-5 perspectives required. 4 perspectives selected -- SATISFIED (this is `complexity_scaling_correct`)
   - **Domain-archetype match**: The matched mapping row is "Payment discrepancy..." which requires `financial`. Financial IS included -- SATISFIED (this is `domain_archetype_match_enforced`)

### Step 0.55.2 (continued): Receive Perspective Generator Results

**Expected `perspectives.json` artifact** written to `~/.prism/state/analyze-a3f7c1e2/perspectives.json`:

```json
{
  "perspectives": [
    {
      "id": "financial",
      "name": "Financial & Compliance",
      "scope": "Apple IAP receipt amount vs DB stored amount reconciliation, payment pipeline code path tracing, refund amount correctness, and compliance implications of ~200 mismatched transactions",
      "key_questions": [
        "What is the exact code path from Apple IAP receipt validation to DB amount storage, and where does the amount transformation occur?",
        "How does the currency conversion interact with the exchange rate update frequency change from daily to weekly?",
        "Are the ~200 affected refund records using the incorrect DB amount or the original Apple receipt amount?",
        "What are the PCI-DSS and financial reporting implications of storing incorrect payment amounts?"
      ],
      "model": "opus",
      "agent_type": "architect",
      "rationale": "Seed analyst found critical payment amount discrepancy between Apple receipts and DB (findings #1, #2). ~200 affected transactions with potential refund complications (finding #3). Financial perspective is mandatory per domain-archetype match rule for payment discrepancy cases."
    },
    {
      "id": "data-integrity",
      "name": "Data Integrity",
      "scope": "Data lineage from Apple IAP receipt through validation pipeline to DB storage, corruption scope across ~200 records in the past week, consistency between payment and refund tables, and recovery options",
      "key_questions": [
        "What is the full data lineage from Apple receipt amount to the DB stored value, and at which transformation step does corruption enter?",
        "Beyond the ~200 known records, are there additional affected records that haven't been identified yet?",
        "What is the consistency state between payment records and refund records for the affected transactions?",
        "What are the recovery options -- can original Apple receipt amounts be retrieved to correct DB values?"
      ],
      "model": "opus",
      "agent_type": "architect",
      "rationale": "Seed analyst confirmed data discrepancy between source-of-truth (Apple receipt) and derived store (DB) in findings #1 and #2. The data-integrity lens is needed to trace the full data lineage, quantify corruption scope, and evaluate recovery paths."
    },
    {
      "id": "root-cause",
      "name": "Root Cause",
      "scope": "Why Apple IAP receipt amounts diverge from DB stored amounts, focusing on the exchange rate update frequency change, currency conversion logic, and timing interactions",
      "key_questions": [
        "Is the exchange rate frequency change (daily -> weekly, finding #4) the sole root cause, or are there additional contributing factors?",
        "What is the fault tree from symptom (amount mismatch) to root cause, with code evidence at each level?",
        "Why was there no validation or reconciliation to catch the discrepancy before ~200 records accumulated?",
        "Are there other code paths that could produce similar amount mismatches under different conditions?"
      ],
      "model": "opus",
      "agent_type": "architect",
      "rationale": "Core archetype (mandatory). Seed analyst identified a recent change to exchange rate update frequency (finding #4) as a likely trigger, but multi-factor complexity suggests deeper root cause analysis is needed to understand all contributing factors."
    },
    {
      "id": "timeline",
      "name": "Timeline",
      "scope": "Chronological reconstruction from the exchange rate frequency change through the first mismatched transaction to discovery, correlating with deploy events and exchange rate fluctuations",
      "key_questions": [
        "When exactly did the first mismatched transaction occur relative to the exchange rate frequency change commit?",
        "What was the exchange rate drift pattern over the affected week that caused the mismatch to manifest?",
        "Was there a detection gap -- how long between first mismatch and discovery, and why wasn't it caught sooner?",
        "Are the ~200 affected transactions uniformly distributed over the week or clustered around specific exchange rate change events?"
      ],
      "model": "sonnet",
      "agent_type": "architect-medium",
      "rationale": "Core archetype. Seed analyst found a recent commit changing exchange rate frequency (finding #4) and a 1-week time window. Timeline lens will reconstruct the chronological sequence to understand the exact trigger point and accumulation pattern."
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
  "selection_summary": "Selected 4 perspectives for this multi-factor payment discrepancy case. The 'financial' perspective is mandatory per domain-archetype match (payment discrepancy row). 'data-integrity' addresses the data corruption and recovery dimension. 'root-cause' is a mandatory core archetype that will trace the fault tree. 'timeline' (core archetype) reconstructs the chronology from the triggering commit to discovery. Complexity scaling rule (multi-factor -> 3-5) is satisfied with 4 perspectives. The 'financial' archetype uses opus/architect per the archetype reference table, ensuring deep analysis of the payment pipeline and compliance implications."
}
```

**Critical rule enforcement details:**

| Rule | Check | Result |
|------|-------|--------|
| `core_archetype_included` | `root-cause` and `timeline` are core archetypes | `true` |
| `recurring_systems_enforced` | `recurrence == "first-time"`, not "recurring" | `"n/a"` |
| `all_evidence_backed` | All 4 perspectives have supporting evidence in seed findings | `true` |
| `min_perspectives_met` | 4 >= 2 | `true` |
| `complexity_scaling_correct` | `complexity == "multi-factor"` requires 3-5; 4 selected | `true` |
| `domain_archetype_match_enforced` | Payment discrepancy row requires `financial`; it is included with opus/architect | `true` |

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
TaskList -> TaskOutput for each completed task
```

### Phase 0.55 Exit Gate

- [x] Perspective generator results received
- [x] `perspectives.json` written
- [x] Perspective generator shut down
- [x] All background task outputs drained

> **NEXT ACTION: Proceed to Phase 0.6 -- Perspective Approval.**

---

## Phase 0.6: Perspective Approval

### Step 0.6.1: Present Perspectives

Orchestrator reads `~/.prism/state/analyze-a3f7c1e2/perspectives.json` and presents to user.

```
AskUserQuestion(
  header: "Perspectives",
  question: "I recommend these 4 perspectives for analysis. How to proceed?

  1. Financial & Compliance (opus/architect) -- Apple IAP receipt reconciliation, payment pipeline, refund correctness, PCI-DSS implications
  2. Data Integrity (opus/architect) -- Data lineage tracing, corruption scope, consistency audit, recovery options
  3. Root Cause (opus/architect) -- Fault tree from symptom to root cause, exchange rate change analysis
  4. Timeline (sonnet/architect-medium) -- Chronological reconstruction from deploy to discovery

  Seed Analysis Summary:
  - Severity: SEV2 | Status: Active
  - Domain: data | Failure type: data_loss | Complexity: multi-factor
  - Key findings: Exchange rate update frequency change, currency conversion mismatch, no reconciliation job

  Rules Applied:
  - core_archetype_included: true
  - complexity_scaling_correct: true (multi-factor -> 3-5, selected 4)
  - domain_archetype_match_enforced: true (financial included for payment discrepancy)",
  options: ["Proceed", "Add perspective", "Remove perspective", "Modify perspective"]
)
```

### Step 0.6.2: User Selects "Proceed"

(Assuming user approves without modification)

### Step 0.6.3: Update Perspectives

Updated `~/.prism/state/analyze-a3f7c1e2/perspectives.json` in-place:

```json
{
  "perspectives": [...same as above...],
  "rules_applied": {
    "core_archetype_included": true,
    "recurring_systems_enforced": "n/a",
    "all_evidence_backed": true,
    "min_perspectives_met": true,
    "complexity_scaling_correct": true,
    "domain_archetype_match_enforced": true
  },
  "selection_summary": "...same as above...",
  "approved": true,
  "user_modifications": []
}
```

### Phase 0.6 Exit Gate

- [x] User selected "Proceed"
- [x] `perspectives.json` updated with `approved: true`

> **NEXT ACTION: Proceed to Phase 0.7 -- Ontology Scope Mapping.**

---

## Phase 0.7: Ontology Scope Mapping

**Source:** `skills/shared-v3/ontology-scope-mapping.md`
**Parameters:**
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"analysis"`
- `{STATE_DIR}` = `~/.prism/state/analyze-a3f7c1e2`

### Phase A: Build Ontology Pool

#### Step 1: Check Document Source Availability

```
ToolSearch(query="select:mcp__prism-mcp__prism_docs_roots")
mcp__prism-mcp__prism_docs_roots()
```

**Scenario:** Returns configured documentation directories (e.g., `/Users/heechul/podo-backend/docs`).

`ONTOLOGY_AVAILABLE=true`, `ONTOLOGY_DIRS=["/Users/heechul/podo-backend/docs"]`

#### Step 2: Screen 1 -- MCP Data Source Selection

```
ToolSearch(query="mcp", max_results=200)
```

Discovers MCP servers. After excluding `prism-mcp` and `plugin_*`:
- `grafana` (monitoring dashboards, Prometheus, Loki)
- `mcp-clickhouse` (database queries)
- `notion` (documentation)
- `sentry` (error tracking)

```
AskUserQuestion(
  header: "Live Data Sources",
  question: "Select live data sources for analysis. (multiple selection)",
  multiSelect: true,
  options: [
    {label: "grafana", description: "35 tools -- Monitoring dashboards, Prometheus metrics, Loki logs"},
    {label: "mcp-clickhouse", description: "3 tools -- ClickHouse database queries"},
    {label: "notion", description: "18 tools -- Notion workspace documents"},
    {label: "sentry", description: "18 tools -- Error tracking and issue management"},
    {label: "Skip", description: "Proceed without MCP data sources"}
  ]
)
```

**Scenario:** User selects `grafana`, `sentry`, `mcp-clickhouse`.

#### Step 3: Screen 2 -- External Source Addition

```
AskUserQuestion(
  header: "External Sources",
  question: "Any external sources to include for analysis?",
  options: ["Add URL", "Add file path", "None -- proceed"]
)
```

**Scenario:** User selects "None -- proceed".

#### Step 4: Screen 3 -- Pool Configuration Confirmation

```
Ontology Pool Configuration:
| # | Source | Type | Path/URL | Domain | Summary | Status |
|---|--------|------|----------|--------|---------|--------|
| 1 | mcp | doc | /Users/heechul/podo-backend/docs | Backend docs | Documentation directory | available |
| 2 | mcp | query | grafana | monitoring | Grafana monitoring dashboards and metrics | available |
| 3 | mcp | query | sentry | error-tracking | Sentry error tracking and issue management | available |
| 4 | mcp | query | mcp-clickhouse | database | ClickHouse database queries | available |
Total 4 sources (MCP Docs: 1, MCP Data: 3, Web: 0, File: 0)
```

User confirms: "Confirm -- proceed"

#### Step 5: Build and Write `ontology-scope.json`

Written to `~/.prism/state/analyze-a3f7c1e2/ontology-scope.json`:

```json
{
  "sources": [
    {
      "id": 1,
      "type": "doc",
      "path": "/Users/heechul/podo-backend/docs",
      "domain": "Backend documentation",
      "summary": "Backend service documentation directory",
      "key_topics": ["payments", "API", "services", "database"],
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
      "server_name": "sentry",
      "domain": "error-tracking",
      "summary": "Sentry error tracking and issue management",
      "key_topics": ["errors", "issues", "events", "traces"],
      "status": "available",
      "access": {
        "tools": ["mcp__sentry__list_issues", "mcp__sentry__list_events", "mcp__sentry__get_issue_details", "mcp__sentry__get_trace_details"],
        "instructions": "Call ToolSearch(query=\"select:mcp__sentry__{tool_name}\") to load each tool before use, then call directly.",
        "capabilities": "Query error events, issues, traces",
        "getting_started": "Start with list_issues to find payment-related errors",
        "error_handling": "If a tool call fails, note the error and continue. Do NOT retry more than once.",
        "safety": "SELECT/read-only queries only"
      }
    },
    {
      "id": 4,
      "type": "mcp_query",
      "server_name": "mcp-clickhouse",
      "domain": "database",
      "summary": "ClickHouse database for analytics queries",
      "key_topics": ["payments", "transactions", "analytics"],
      "status": "available",
      "access": {
        "tools": ["mcp__mcp-clickhouse__list_databases", "mcp__mcp-clickhouse__list_tables", "mcp__mcp-clickhouse__run_select_query"],
        "instructions": "Call ToolSearch(query=\"select:mcp__mcp-clickhouse__{tool_name}\") to load each tool before use, then call directly.",
        "capabilities": "List databases/tables, run SELECT queries",
        "getting_started": "Start with list_databases then list_tables to discover schema",
        "error_handling": "If a tool call fails, note the error and continue. Do NOT retry more than once.",
        "safety": "SELECT queries only"
      }
    }
  ],
  "totals": {
    "doc": 1,
    "mcp_query": 3,
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

The orchestrator reads `ontology-scope.json` and generates the following text block for `{ONTOLOGY_SCOPE}` placeholder injection:

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

- mcp-query: sentry: Sentry error tracking and issue management
  Tools (read-only): mcp__sentry__list_issues, mcp__sentry__list_events, mcp__sentry__get_issue_details, mcp__sentry__get_trace_details
  Access: Call ToolSearch(query="select:mcp__sentry__{tool_name}") to load each tool before use, then call directly.
  Capabilities: Query error events, issues, traces
  Getting started: Start with list_issues to find payment-related errors
  Error handling: If a tool call fails, note the error and continue. Do NOT retry more than once.

- mcp-query: mcp-clickhouse: ClickHouse database for analytics queries
  Tools (read-only): mcp__mcp-clickhouse__list_databases, mcp__mcp-clickhouse__list_tables, mcp__mcp-clickhouse__run_select_query
  Access: Call ToolSearch(query="select:mcp__mcp-clickhouse__{tool_name}") to load each tool before use, then call directly.
  Capabilities: List databases/tables, run SELECT queries
  Getting started: Start with list_databases then list_tables to discover schema
  Error handling: If a tool call fails, note the error and continue. Do NOT retry more than once.

Explore these sources through your perspective's lens.
Cite findings as: source:section, url:section, file:path:section, mcp-query:server:detail.
```

### Ontology Scope Exit Gate

- [x] Phase A complete
- [x] `ontology-scope.json` written

> **NEXT ACTION: Proceed to Phase 0.8 -- Write context file.**

---

## Phase 0.8: Context & State Files

### Step 0.8.1: Write Context File

**`report_language` detection:** The user's input is in Korean ("결제 시스템에서 Apple IAP 영수증 금액과..."). Therefore `report_language` = `"ko"` (Korean).

**`investigation_loops` initialization:** Set to `0` as this is the initial analysis run.

Written to `~/.prism/state/analyze-a3f7c1e2/context.json`:

```json
{
  "summary": "Apple IAP 영수증 금액과 DB 저장 결제 금액 불일치. ~200건 발생, 최근 1주일 이내. 환불 처리 연쇄 오류 가능성. 환율 업데이트 주기 변경(일간->주간)이 트리거로 의심됨.",
  "research_summary": {
    "key_findings": [
      "Apple IAP receipt validation applies stale exchange rate during currency conversion",
      "DB stores converted KRW amount, not original USD from Apple receipt",
      "Refund handler uses incorrect DB amount instead of Apple receipt amount",
      "Recent commit changed exchange rate update frequency from daily to weekly",
      "No reconciliation job exists to detect amount discrepancies"
    ],
    "files_examined": [
      "src/payments/apple-iap-validator.ts:47 — receipt validation and amount extraction",
      "src/payments/payment-repository.ts:82 — payment DB write with converted amount",
      "src/payments/refund-handler.ts:35 — refund processing logic",
      "src/payments/exchange-rate-service.ts:12 — exchange rate lookup",
      "src/jobs/ — searched for reconciliation jobs, none found"
    ],
    "dimensions": "domain: data, failure_type: data_loss, complexity: multi-factor, recurrence: first-time"
  },
  "report_language": "ko",
  "investigation_loops": 0
}
```

### Phase 0.8 Exit Gate

- [x] `perspectives.json` updated with `approved: true`
- [x] `context.json` written with structured summary
- [x] Ontology scope mapping complete

> **NEXT ACTION: Proceed to Phase 1 -- Spawn analysts.**

---

## Phase 1: Spawn Analysts (Finding Phase)

### Step 1.1: Spawn Analysts

Team `analyze-a3f7c1e2` already exists from Phase 0.5. Four analysts are spawned in parallel.

**`{CONTEXT}` replacement value** (derived from `context.json`):

```
Summary: Apple IAP 영수증 금액과 DB 저장 결제 금액 불일치. ~200건 발생, 최근 1주일 이내. 환불 처리 연쇄 오류 가능성. 환율 업데이트 주기 변경(일간->주간)이 트리거로 의심됨.
Key Findings: Apple IAP receipt validation applies stale exchange rate during currency conversion, DB stores converted KRW amount not original USD from Apple receipt, Refund handler uses incorrect DB amount instead of Apple receipt amount, Recent commit changed exchange rate update frequency from daily to weekly, No reconciliation job exists to detect amount discrepancies
Files Examined: src/payments/apple-iap-validator.ts:47, src/payments/payment-repository.ts:82, src/payments/refund-handler.ts:35, src/payments/exchange-rate-service.ts:12, src/jobs/
Dimensions: domain: data, failure_type: data_loss, complexity: multi-factor, recurrence: first-time
```

**`{ONTOLOGY_SCOPE}` replacement value:** The full text block generated in Phase 0.7 Phase B (shown above).

---

### Analyst 1: Financial & Compliance (`financial`)

**Files read at spawn time:**
1. `skills/shared-v3/worker-preamble.md`
2. `skills/analyze/prompts/extended-archetypes.md` -- section "Financial Lens"
3. `skills/analyze/prompts/finding-protocol.md`

**Prompt assembly:** `[worker preamble] + [Financial Lens prompt] + [finding-protocol.md]`

**Placeholder replacements:**

| Placeholder | Value |
|-------------|-------|
| `{TEAM_NAME}` | `"analyze-a3f7c1e2"` |
| `{WORKER_NAME}` | `"financial-analyst"` |
| `{WORK_ACTION}` | `"Investigate from your assigned perspective. Answer ALL key questions with evidence and code references. Write findings to findings.json. Report findings via SendMessage to team-lead. Do NOT run self-verification -- that happens in a separate session."` |
| `{CONTEXT}` | (full context text block above) |
| `{ONTOLOGY_SCOPE}` | (full ontology scope text block above) |
| `{SHORT_ID}` | `"a3f7c1e2"` |
| `{perspective-id}` | `"financial"` |
| `{KEY_QUESTIONS}` | `"1. What is the exact code path from Apple IAP receipt validation to DB amount storage, and where does the amount transformation occur?\n2. How does the currency conversion interact with the exchange rate update frequency change from daily to weekly?\n3. Are the ~200 affected refund records using the incorrect DB amount or the original Apple receipt amount?\n4. What are the PCI-DSS and financial reporting implications of storing incorrect payment amounts?"` |

**Spawn call:**
```
TaskCreate(task for financial-analyst)
TaskUpdate(owner="financial-analyst")

Task(
  subagent_type="oh-my-claudecode:architect",
  name="financial-analyst",
  team_name="analyze-a3f7c1e2",
  model="opus",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

**Note:** Financial perspective uses **opus/architect** per the archetype reference table in `perspective-generator.md`:
| `financial` | Financial & Compliance | opus | architect |

**Finding protocol output path:** `~/.prism/state/analyze-a3f7c1e2/perspectives/financial/findings.json`

**Expected findings.json:**
```json
{
  "analyst": "financial",
  "findings": [
    {
      "finding": "Payment pipeline: Apple receipt amount (USD) -> validateReceipt() -> exchangeRateService.convert() -> paymentRepository.save(KRW). The convert() call at apple-iap-validator.ts:52 uses a cached rate that can be up to 7 days stale since the frequency change.",
      "evidence": "src/payments/apple-iap-validator.ts:validateReceipt:52 -- exchangeRateService.convert(receipt.amount, 'USD', 'KRW')",
      "severity": "critical"
    },
    {
      "finding": "Refund handler at refund-handler.ts:35 calls paymentRepository.getAmount() which returns the converted KRW value, not the original Apple receipt USD. Apple requires refund in original currency/amount.",
      "evidence": "src/payments/refund-handler.ts:processRefund:35 -- const amount = await paymentRepository.getAmount(txId)",
      "severity": "critical"
    },
    {
      "finding": "No audit trail links the original Apple receipt amount to the stored DB amount. The receipt_amount field is not persisted anywhere, only the converted amount.",
      "evidence": "src/payments/payment-repository.ts:savePayment:82 -- INSERT INTO payments (amount_krw, ...) -- no original_amount column",
      "severity": "high"
    },
    {
      "finding": "PCI-DSS Requirement 3.4 mandates accurate financial record-keeping. Storing converted amounts without original values violates traceability requirements.",
      "evidence": "Schema review: payments table lacks original_currency and original_amount columns",
      "severity": "high"
    }
  ]
}
```

---

### Analyst 2: Data Integrity (`data-integrity`)

**Files read at spawn time:**
1. `skills/shared-v3/worker-preamble.md`
2. `skills/analyze/prompts/extended-archetypes.md` -- section "Data Integrity Lens"
3. `skills/analyze/prompts/finding-protocol.md`

**Placeholder replacements:**

| Placeholder | Value |
|-------------|-------|
| `{TEAM_NAME}` | `"analyze-a3f7c1e2"` |
| `{WORKER_NAME}` | `"data-integrity-analyst"` |
| `{WORK_ACTION}` | (same as above) |
| `{CONTEXT}` | (same context block) |
| `{ONTOLOGY_SCOPE}` | (same ontology scope block) |
| `{SHORT_ID}` | `"a3f7c1e2"` |
| `{perspective-id}` | `"data-integrity"` |
| `{KEY_QUESTIONS}` | `"1. What is the full data lineage from Apple receipt amount to the DB stored value, and at which transformation step does corruption enter?\n2. Beyond the ~200 known records, are there additional affected records that haven't been identified yet?\n3. What is the consistency state between payment records and refund records for the affected transactions?\n4. What are the recovery options -- can original Apple receipt amounts be retrieved to correct DB values?"` |

**Spawn call:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="data-integrity-analyst",
  team_name="analyze-a3f7c1e2",
  model="opus",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

**Model/agent_type:** opus/architect per archetype table.

**Finding protocol output path:** `~/.prism/state/analyze-a3f7c1e2/perspectives/data-integrity/findings.json`

---

### Analyst 3: Root Cause (`root-cause`)

**Files read at spawn time:**
1. `skills/shared-v3/worker-preamble.md`
2. `skills/analyze/prompts/core-archetypes.md` -- section "Root Cause Lens"
3. `skills/analyze/prompts/finding-protocol.md`

**Placeholder replacements:**

| Placeholder | Value |
|-------------|-------|
| `{TEAM_NAME}` | `"analyze-a3f7c1e2"` |
| `{WORKER_NAME}` | `"root-cause-analyst"` |
| `{WORK_ACTION}` | (same as above) |
| `{CONTEXT}` | (same context block) |
| `{ONTOLOGY_SCOPE}` | (same ontology scope block) |
| `{SHORT_ID}` | `"a3f7c1e2"` |
| `{perspective-id}` | `"root-cause"` |
| `{KEY_QUESTIONS}` | `"1. Is the exchange rate frequency change (daily -> weekly, finding #4) the sole root cause, or are there additional contributing factors?\n2. What is the fault tree from symptom (amount mismatch) to root cause, with code evidence at each level?\n3. Why was there no validation or reconciliation to catch the discrepancy before ~200 records accumulated?\n4. Are there other code paths that could produce similar amount mismatches under different conditions?"` |

**Spawn call:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="root-cause-analyst",
  team_name="analyze-a3f7c1e2",
  model="opus",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

**Model/agent_type:** opus/architect per archetype table.

**Finding protocol output path:** `~/.prism/state/analyze-a3f7c1e2/perspectives/root-cause/findings.json`

---

### Analyst 4: Timeline (`timeline`)

**Files read at spawn time:**
1. `skills/shared-v3/worker-preamble.md`
2. `skills/analyze/prompts/core-archetypes.md` -- section "Timeline Lens"
3. `skills/analyze/prompts/finding-protocol.md`

**Placeholder replacements:**

| Placeholder | Value |
|-------------|-------|
| `{TEAM_NAME}` | `"analyze-a3f7c1e2"` |
| `{WORKER_NAME}` | `"timeline-analyst"` |
| `{WORK_ACTION}` | (same as above) |
| `{CONTEXT}` | (same context block) |
| `{ONTOLOGY_SCOPE}` | (same ontology scope block) |
| `{SHORT_ID}` | `"a3f7c1e2"` |
| `{perspective-id}` | `"timeline"` |
| `{KEY_QUESTIONS}` | `"1. When exactly did the first mismatched transaction occur relative to the exchange rate frequency change commit?\n2. What was the exchange rate drift pattern over the affected week that caused the mismatch to manifest?\n3. Was there a detection gap -- how long between first mismatch and discovery, and why wasn't it caught sooner?\n4. Are the ~200 affected transactions uniformly distributed over the week or clustered around specific exchange rate change events?"` |

**Spawn call:**
```
Task(
  subagent_type="oh-my-claudecode:architect-medium",
  name="timeline-analyst",
  team_name="analyze-a3f7c1e2",
  model="sonnet",
  run_in_background=true,
  prompt="<assembled prompt>"
)
```

**Model/agent_type:** sonnet/architect-medium per archetype table.

**Finding protocol output path:** `~/.prism/state/analyze-a3f7c1e2/perspectives/timeline/findings.json`

---

### Phase 1 Summary: All Spawns

| Agent Name | Perspective ID | Prompt Source | Model | Agent Type | Run in Background |
|------------|---------------|---------------|-------|------------|-------------------|
| `financial-analyst` | `financial` | `extended-archetypes.md` section "Financial Lens" + `finding-protocol.md` | opus | architect | true |
| `data-integrity-analyst` | `data-integrity` | `extended-archetypes.md` section "Data Integrity Lens" + `finding-protocol.md` | opus | architect | true |
| `root-cause-analyst` | `root-cause` | `core-archetypes.md` section "Root Cause Lens" + `finding-protocol.md` | opus | architect | true |
| `timeline-analyst` | `timeline` | `core-archetypes.md` section "Timeline Lens" + `finding-protocol.md` | sonnet | architect-medium | true |

All four analysts spawned in **parallel** via `run_in_background=true`.

### Phase 1 Exit Gate

- [x] All 4 analyst tasks created and owners pre-assigned
- [x] All analysts spawned in parallel

> **NEXT ACTION: Read `docs/later-phases.md` and proceed to Phase 2.**

---

## Phase 2: Collect Findings & Spawn Verification Sessions

**Source:** `skills/analyze/docs/later-phases.md`

### Stage A: Collect Findings

#### Step 2A.1: Wait for Analyst Findings

Orchestrator monitors via `TaskList`. Each analyst:
1. Writes findings to `~/.prism/state/analyze-a3f7c1e2/perspectives/{perspective-id}/findings.json`
2. Sends findings to team-lead via `SendMessage`

**Files written by analysts:**
- `~/.prism/state/analyze-a3f7c1e2/perspectives/financial/findings.json`
- `~/.prism/state/analyze-a3f7c1e2/perspectives/data-integrity/findings.json`
- `~/.prism/state/analyze-a3f7c1e2/perspectives/root-cause/findings.json`
- `~/.prism/state/analyze-a3f7c1e2/perspectives/timeline/findings.json`

**SendMessage from each analyst follows the finding-protocol.md format:**

```markdown
## Findings -- {perspective-id}

### Session
- context_id: analyze-a3f7c1e2
- perspective_id: {perspective-id}

### Findings
{findings with evidence}
```

#### Step 2A.2: Drain Background Task Outputs

```
TaskList -> find completed tasks -> TaskOutput for each
```

#### Step 2A.3: Shutdown Finding Analysts

```
SendMessage(type: "shutdown_request", recipient: "financial-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "data-integrity-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "root-cause-analyst", content: "Finding phase complete.")
SendMessage(type: "shutdown_request", recipient: "timeline-analyst", content: "Finding phase complete.")
```

Each responds with `shutdown_response(approve=true)`. Drain task outputs again.

#### Stage A Exit Gate

- [x] All 4 analyst findings received via SendMessage
- [x] All 4 `findings.json` files written
- [x] All 4 finding analysts shut down
- [x] All background task outputs drained

> **NEXT ACTION: Proceed to Stage B -- Spawn Verification Sessions.**

---

### Stage B: Spawn Verification Sessions

**SESSION SPLIT: Each analyst gets a NEW session.** The verification session reads `findings.json` from the previous finding session. The prompt uses `verification-protocol.md` instead of `finding-protocol.md`.

#### Step 2B.1: Spawn Verification Sessions

**Prompt assembly order (for each verifier):**
1. Read archetype section (same as Phase 1)
2. Read `prompts/verification-protocol.md`
3. Concatenate: `[worker preamble] + [archetype prompt] + [verification protocol]`
4. Replace placeholders: `{CONTEXT}`, `{ONTOLOGY_SCOPE}`, `{SHORT_ID}`, `{perspective-id}`, `{summary}`

**Critical: `{summary}` replacement in verification-protocol.md**

The `{summary}` placeholder appears in the `prism_interview` call within verification-protocol.md:
```
topic="{perspective-id} findings verification -- {summary}"
```

`{summary}` is replaced with the value from `context.json`'s `summary` field:
`"Apple IAP 영수증 금액과 DB 저장 결제 금액 불일치. ~200건 발생, 최근 1주일 이내. 환불 처리 연쇄 오류 가능성. 환율 업데이트 주기 변경(일간->주간)이 트리거로 의심됨."`

**Critical: `{perspective-id}` replacement in verification-protocol.md**

The `{perspective-id}` placeholder appears in multiple places:
- Findings path: `~/.prism/state/analyze-{SHORT_ID}/perspectives/{perspective-id}/findings.json`
- Session path: `analyze-{SHORT_ID}/perspectives/{perspective-id}`
- `prism_interview` calls: `perspective_id="{perspective-id}"`
- SendMessage output: `## Verified Findings -- {perspective-id}`

The orchestrator MUST replace `{perspective-id}` in both the archetype prompt AND the verification protocol before spawning.

---

#### Verifier 1: Financial (`financial-verifier`)

**Prompt assembly:**
```
[worker preamble with WORKER_NAME="financial-verifier"] +
[Financial Lens prompt from extended-archetypes.md] +
[verification-protocol.md with all placeholders replaced]
```

**Worker preamble replacements:**
- `{TEAM_NAME}` = `"analyze-a3f7c1e2"`
- `{WORKER_NAME}` = `"financial-verifier"`
- `{WORK_ACTION}` = `"Read your findings from the path specified in your verification protocol. Run self-verification via MCP tools (prism_interview). Re-investigate with tools as needed to answer interview questions. Report verified findings via SendMessage to team-lead."`

**Verification-protocol placeholder replacements:**
- `{SHORT_ID}` = `"a3f7c1e2"`
- `{perspective-id}` = `"financial"`
- `{summary}` = `"Apple IAP 영수증 금액과 DB 저장 결제 금액 불일치. ~200건 발생, 최근 1주일 이내. 환불 처리 연쇄 오류 가능성. 환율 업데이트 주기 변경(일간->주간)이 트리거로 의심됨."`

**Resulting verification-protocol.md (after replacement):**

Key sections with replaced values:
```
Your findings are saved at:
~/.prism/state/analyze-a3f7c1e2/perspectives/financial/findings.json

Your session path is: analyze-a3f7c1e2/perspectives/financial

#### 2. Start Interview
mcp__prism-mcp__prism_interview(
  context_id="analyze-a3f7c1e2",
  perspective_id="financial",
  topic="financial findings verification -- Apple IAP 영수증 금액과 DB 저장 결제 금액 불일치. ~200건 발생, 최근 1주일 이내. 환불 처리 연쇄 오류 가능성. 환율 업데이트 주기 변경(일간->주간)이 트리거로 의심됨."
)
```

**Spawn call:**
```
TaskCreate(task for financial-verifier)
TaskUpdate(owner="financial-verifier")

Task(
  subagent_type="oh-my-claudecode:architect",
  name="financial-verifier",
  team_name="analyze-a3f7c1e2",
  model="opus",
  run_in_background=true,
  prompt="<assembled verification prompt>"
)
```

---

#### Verifier 2: Data Integrity (`data-integrity-verifier`)

**Spawn call:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="data-integrity-verifier",
  team_name="analyze-a3f7c1e2",
  model="opus",
  run_in_background=true,
  prompt="<assembled verification prompt with {perspective-id}=data-integrity, {summary}=<context.json summary>>"
)
```

Verification protocol key replacements:
- Findings path: `~/.prism/state/analyze-a3f7c1e2/perspectives/data-integrity/findings.json`
- Session path: `analyze-a3f7c1e2/perspectives/data-integrity`
- Interview topic: `"data-integrity findings verification -- Apple IAP 영수증 금액과 DB 저장 결제 금액 불일치..."`

---

#### Verifier 3: Root Cause (`root-cause-verifier`)

**Spawn call:**
```
Task(
  subagent_type="oh-my-claudecode:architect",
  name="root-cause-verifier",
  team_name="analyze-a3f7c1e2",
  model="opus",
  run_in_background=true,
  prompt="<assembled verification prompt with {perspective-id}=root-cause, {summary}=<context.json summary>>"
)
```

Verification protocol key replacements:
- Findings path: `~/.prism/state/analyze-a3f7c1e2/perspectives/root-cause/findings.json`
- Session path: `analyze-a3f7c1e2/perspectives/root-cause`
- Interview topic: `"root-cause findings verification -- Apple IAP 영수증 금액과 DB 저장 결제 금액 불일치..."`

---

#### Verifier 4: Timeline (`timeline-verifier`)

**Spawn call:**
```
Task(
  subagent_type="oh-my-claudecode:architect-medium",
  name="timeline-verifier",
  team_name="analyze-a3f7c1e2",
  model="sonnet",
  run_in_background=true,
  prompt="<assembled verification prompt with {perspective-id}=timeline, {summary}=<context.json summary>>"
)
```

Verification protocol key replacements:
- Findings path: `~/.prism/state/analyze-a3f7c1e2/perspectives/timeline/findings.json`
- Session path: `analyze-a3f7c1e2/perspectives/timeline`
- Interview topic: `"timeline findings verification -- Apple IAP 영수증 금액과 DB 저장 결제 금액 불일치..."`

---

### Phase 2B Verification Summary Table

| Verifier Name | Perspective ID | Model | Agent Type | Findings Path | Session Path |
|--------------|---------------|-------|------------|---------------|-------------|
| `financial-verifier` | `financial` | opus | architect | `perspectives/financial/findings.json` | `analyze-a3f7c1e2/perspectives/financial` |
| `data-integrity-verifier` | `data-integrity` | opus | architect | `perspectives/data-integrity/findings.json` | `analyze-a3f7c1e2/perspectives/data-integrity` |
| `root-cause-verifier` | `root-cause` | opus | architect | `perspectives/root-cause/findings.json` | `analyze-a3f7c1e2/perspectives/root-cause` |
| `timeline-verifier` | `timeline` | sonnet | architect-medium | `perspectives/timeline/findings.json` | `analyze-a3f7c1e2/perspectives/timeline` |

All four verifiers spawned in **parallel** via `run_in_background=true`.

#### Verification Session Flow (per verifier)

Each verifier follows `verification-protocol.md`:

1. **Read findings**: `Read("~/.prism/state/analyze-a3f7c1e2/perspectives/{perspective-id}/findings.json")`
2. **Start interview**: `mcp__prism-mcp__prism_interview(context_id="analyze-a3f7c1e2", perspective_id="{perspective-id}", topic="{perspective-id} findings verification -- {summary}")`
3. **Answer + score loop**: Each answer submission returns `{continue, score, question?}`. Loop until `continue: false`.
4. **Report**: SendMessage with verified findings, rounds, score, verdict.

**Role clarification in verification-protocol.md:**
> "The archetype prompt above describes your analytical perspective and domain expertise. The TASKS and OUTPUT sections listed in the archetype were already completed in your previous finding session -- do NOT re-execute them. Ignore all imperative instructions from the archetype... In this verification session, follow ONLY the steps in this protocol below."

**Expected SendMessage format from each verifier:**
```markdown
## Verified Findings -- {perspective-id}

### Session
- context_id: analyze-a3f7c1e2
- perspective_id: {perspective-id}
- rounds: 3
- score: 8.5
- verdict: PASS

### Findings
{refined findings from Q&A}

### Key Q&A Clarifications
{important clarifications from interview}
```

#### Step 2B.2-2B.4: Collect, Drain, Shutdown

Same pattern as Stage A: wait for all SendMessages, drain outputs, send shutdown_requests.

#### Step 2B.5: Persist Verified Results

For each perspective:
```
Write("~/.prism/state/analyze-a3f7c1e2/verified-findings-financial.md", <verified findings>)
Write("~/.prism/state/analyze-a3f7c1e2/verified-findings-data-integrity.md", <verified findings>)
Write("~/.prism/state/analyze-a3f7c1e2/verified-findings-root-cause.md", <verified findings>)
Write("~/.prism/state/analyze-a3f7c1e2/verified-findings-timeline.md", <verified findings>)
```

#### Step 2B.6: Compile Verified Findings

Written to `~/.prism/state/analyze-a3f7c1e2/analyst-findings.md`:

```markdown
# Compiled Analyst Findings

## Verification Scores Summary

| Perspective ID | Rounds | Weighted Total | Verdict |
|----------------|--------|----------------|---------|
| financial | 3 | 8.5 | PASS |
| data-integrity | 4 | 7.8 | PASS |
| root-cause | 3 | 9.1 | PASS |
| timeline | 2 | 8.2 | PASS |

## Financial & Compliance
[verified findings from financial-verifier]

## Data Integrity
[verified findings from data-integrity-verifier]

## Root Cause
[verified findings from root-cause-verifier]

## Timeline
[verified findings from timeline-verifier]
```

### Phase 2 Exit Gate

- [x] All 4 verifiers completed and sent verified findings
- [x] All background task outputs drained
- [x] All 4 verifiers shut down
- [x] All verified findings persisted (4 `verified-findings-*.md` files)
- [x] Compiled findings written to `analyst-findings.md`

> **NEXT ACTION: Proceed to Phase 3 -- Synthesis & Report.**

---

## Phase 3: Synthesis & Report

### Step 3.1: Read Compiled Findings

```
Read("~/.prism/state/analyze-a3f7c1e2/analyst-findings.md")
```

### Step 3.2: Write Report

**Template:** `skills/analyze/templates/report.md`

**Language:** `context.json.report_language` = `"ko"` (Korean)

The entire report is written in Korean, following the `report.md` template structure.

**Report output** (written to `~/.prism/state/analyze-a3f7c1e2/report.md`):

```markdown
# 분석 보고서

## 요약
Apple IAP 영수증 금액과 DB에 저장된 결제 금액 간의 불일치가 발생했습니다.
환율 업데이트 주기를 일간에서 주간으로 변경한 최근 커밋이 트리거로 작용하여,
USD에서 KRW로의 환전 시 최대 7일 동안 구 환율이 적용되었습니다.
환불 처리기도 원래 Apple 영수증 금액이 아닌 잘못된 DB 저장 금액을 참조하고 있어
환불 금액도 부정확합니다.

## 분석 개요
- **주제**: Apple IAP 영수증 금액과 DB 저장 결제 금액 불일치
- **심각도**: SEV2
- **기간**: 약 7일 (환율 업데이트 주기 변경 이후)
- **상태**: Active
- **영향 시스템**: 결제 처리 파이프라인, 환불 시스템, 환율 서비스
- **사용자 영향**: ~200건의 결제 금액 불일치, 환불 처리 오류 가능성
- **분석 관점**: Financial & Compliance, Data Integrity, Root Cause, Timeline

## 타임라인
| 시간 | 이벤트 | 증거 | 확신도 |
|------|--------|------|--------|
| ...  | ...    | ...  | ...    |

## 관점별 분석 결과
### Financial & Compliance
[검증된 분석 결과]

### Data Integrity
[검증된 분석 결과]

### Root Cause
[검증된 분석 결과]

### Timeline
[검증된 분석 결과]

## 통합 분석
- **수렴**: 모든 관점에서 환율 업데이트 주기 변경이 근본 원인으로 합의
- **발산**: [관점 간 불일치 사항 및 해결]
- **창발적 통찰**: [관점 조합으로만 발견되는 인사이트]

## 소크라틱 검증 요약
### 분석가별 검증 점수
| 분석가 | 라운드 | 가중 총점 | 판정 |
|--------|--------|-----------|------|
| financial | 3 | 8.5 | PASS |
| data-integrity | 4 | 7.8 | PASS |
| root-cause | 3 | 9.1 | PASS |
| timeline | 2 | 8.2 | PASS |

### 소크라틱 검증 Q&A 주요 명확화
[각 분석가별 주요 Q&A]

### 미해결 모호성
[해결되지 않은 사항]

## 권장 사항
| 조치 | 우선순위 | UX 영향 | 엔지니어링 노력 | 검증? |
|------|----------|---------|----------------|-------|

### 즉시 (이번 주)
### 단기 (이번 달)
### 장기 (이번 분기)
### 모니터링 및 알림

## 예방 체크리스트
- [ ] 근본 원인 영구 수정
- [ ] 조기 감지 모니터링
- [ ] 런북 업데이트
- [ ] 사후 검토 예약
- [ ] 유사 위험 완화

## 부록
...
```

### Step 3.3: User Decision

```
AskUserQuestion(
  header: "Analysis Complete",
  question: "분석이 완료되었습니까?",
  options: ["Complete", "Need deeper investigation", "Add recommendations", "Share with team"]
)
```

**Scenario:** User selects "Complete".

(If "Need deeper investigation" were selected: check `investigation_loops` in `context.json` -- currently 0, so < 2, would increment to 1 and re-enter investigation loop per Step 3.3 re-entry flow.)

> **NEXT ACTION: Proceed to Phase 4 -- Cleanup.**

---

## Phase 4: Cleanup

**Source:** `skills/shared-v3/team-teardown.md`

### Steps:

1. `TaskList` -- enumerate active teammates (filter for non-completed tasks)
2. `SendMessage(type: "shutdown_request")` to each active teammate by name
3. Await `shutdown_response(approve=true)` from each
4. `TeamDelete(team_name="analyze-a3f7c1e2")`

**Expected:** All agents already shut down in Phase 2. TeamDelete cleans up the team resource.

---

## Data Flow Summary

```
Phase 0:
  User description -> {DESCRIPTION}
  uuidgen -> {short-id} = a3f7c1e2

Phase 0.5:
  {DESCRIPTION} + {SHORT_ID} -> seed-analyst -> seed-analysis.json
    Contains: severity, status, dimensions (domain=data, failure_type=data_loss,
    complexity=multi-factor, recurrence=first-time), research findings

Phase 0.55:
  seed-analysis.json -> perspective-generator -> perspectives.json
    Contains: 4 perspectives (financial, data-integrity, root-cause, timeline)
    + rules_applied (complexity_scaling_correct=true, domain_archetype_match_enforced=true)
    Financial: opus/architect (per archetype table)

Phase 0.6:
  perspectives.json -> user approval -> perspectives.json (+ approved: true)

Phase 0.7:
  MCP discovery + user selection -> ontology-scope.json -> {ONTOLOGY_SCOPE} text block

Phase 0.8:
  seed-analysis.json + user description -> context.json
    Contains: summary, research_summary, report_language="ko", investigation_loops=0

Phase 1 (Finding Sessions):
  For each perspective:
    context.json -> {CONTEXT}
    ontology-scope.json -> {ONTOLOGY_SCOPE}
    perspectives.json[i].key_questions -> {KEY_QUESTIONS}
    Archetype prompt + finding-protocol.md -> analyst agent
    -> findings.json per perspective

Phase 2A (Collect Findings):
  findings.json (x4) + SendMessage (x4) -> orchestrator collects all
  -> shutdown finding analysts

Phase 2B (Verification Sessions -- NEW SESSIONS):
  For each perspective:
    Same archetype prompt + verification-protocol.md -> verifier agent
    Reads findings.json from Phase 1
    Runs prism_interview MCP loop
    {summary} replaced from context.json.summary
    {perspective-id} replaced in findings path, prism_interview calls, SendMessage
    -> verified-findings-{perspective-id}.md
  All verified -> analyst-findings.md

Phase 3 (Report):
  analyst-findings.md -> report template -> final report in report_language="ko"
  User decides: complete / deeper investigation (max 2 loops via investigation_loops counter)

Phase 4 (Cleanup):
  TeamDelete("analyze-a3f7c1e2")
```

---

## File Artifact Map

| File | Phase Written | Phase Read |
|------|--------------|------------|
| `~/.prism/state/analyze-a3f7c1e2/seed-analysis.json` | 0.5 (seed-analyst) | 0.55 (perspective-generator), 0.8 (orchestrator) |
| `~/.prism/state/analyze-a3f7c1e2/perspectives.json` | 0.55 (perspective-generator), updated 0.6 (orchestrator) | 0.6, 0.8, 1, 2B, 3 |
| `~/.prism/state/analyze-a3f7c1e2/ontology-scope.json` | 0.7 (orchestrator) | 1 (via {ONTOLOGY_SCOPE}), 2B (via {ONTOLOGY_SCOPE}) |
| `~/.prism/state/analyze-a3f7c1e2/context.json` | 0.8 (orchestrator) | 1 (via {CONTEXT}), 2B (via {CONTEXT}, {summary}), 3 (report_language, investigation_loops) |
| `~/.prism/state/analyze-a3f7c1e2/perspectives/financial/findings.json` | 1 (financial-analyst) | 2B (financial-verifier) |
| `~/.prism/state/analyze-a3f7c1e2/perspectives/data-integrity/findings.json` | 1 (data-integrity-analyst) | 2B (data-integrity-verifier) |
| `~/.prism/state/analyze-a3f7c1e2/perspectives/root-cause/findings.json` | 1 (root-cause-analyst) | 2B (root-cause-verifier) |
| `~/.prism/state/analyze-a3f7c1e2/perspectives/timeline/findings.json` | 1 (timeline-analyst) | 2B (timeline-verifier) |
| `~/.prism/state/analyze-a3f7c1e2/verified-findings-financial.md` | 2B.5 (orchestrator) | 3 |
| `~/.prism/state/analyze-a3f7c1e2/verified-findings-data-integrity.md` | 2B.5 (orchestrator) | 3 |
| `~/.prism/state/analyze-a3f7c1e2/verified-findings-root-cause.md` | 2B.5 (orchestrator) | 3 |
| `~/.prism/state/analyze-a3f7c1e2/verified-findings-timeline.md` | 2B.5 (orchestrator) | 3 |
| `~/.prism/state/analyze-a3f7c1e2/analyst-findings.md` | 2B.6 (orchestrator) | 3 |
| `~/.prism/state/analyze-a3f7c1e2/report.md` | 3 (orchestrator) | user |

---

## Key Decision Points Traced

### 1. `failure_type` = `data_loss` (underscore format)
The seed-analyst's dimension evaluation follows the enum values in seed-analyst.md: `crash | degradation | data_loss | breach | misconfig`. The payment amount discrepancy represents data integrity loss -- the source-of-truth (Apple receipt) diverges from the stored value (DB). This maps to `data_loss` with underscore, matching the JSON schema.

### 2. Financial archetype mapping and opus/architect enforcement
The perspective-generator's archetype mapping table row "Payment discrepancy, billing error, revenue data mismatch" maps to `financial` + `data-integrity` + `root-cause`. The archetype reference table specifies `financial` as opus/architect. The `domain_archetype_match_enforced` rule in `rules_applied` ensures the `financial` perspective IS included with the correct model/agent_type.

### 3. `complexity_scaling_correct` in `rules_applied`
`dimensions.complexity == "multi-factor"` requires 3-5 perspectives. 4 perspectives were selected, so `complexity_scaling_correct: true`.

### 4. `{KEY_QUESTIONS}` injection in Phase 1
Each analyst's prompt includes `{KEY_QUESTIONS}` from `perspectives.json[i].key_questions`, formatted as a numbered list and injected into the `finding-protocol.md` section "Perspective-Specific Questions". These are case-specific questions grounded in the seed analyst's research.

### 5. `report_language` = `"ko"` in `context.json`
Detected from the user's input language (Korean). The Phase 3 report is written entirely in Korean per Step 3.2: "Write the report in the language specified by `context.json.report_language`."

### 6. `investigation_loops` = 0 in `context.json`
Initialized to 0 in Phase 0.8. If the user selects "Need deeper investigation" in Phase 3.3, it increments. Max 2 loops before auto-completing.

### 7. Session split between Phase 1 and Phase 2B
Phase 1 analysts use `finding-protocol.md` -- they investigate and write findings only, no self-verification. Phase 2B spawns NEW sessions (different agents) with `verification-protocol.md` -- they read the findings from Phase 1 and run MCP `prism_interview` for Socratic verification. The verification protocol explicitly states: "Ignore all imperative instructions from the archetype... follow ONLY the steps in this protocol below."

### 8. `{summary}` and `{perspective-id}` replacement in Phase 2B
Per later-phases.md Step 2B.1: `{summary}` is replaced with `context.json.summary` and `{perspective-id}` is replaced with the perspective's `id` field. Both appear in `verification-protocol.md` -- `{summary}` in the `prism_interview` topic parameter, `{perspective-id}` in findings paths, interview calls, and SendMessage output.
