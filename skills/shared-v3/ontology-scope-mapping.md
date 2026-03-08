# Ontology Scope Mapping

**Execution context:** This module can be executed either by the orchestrator directly OR by a setup agent (see `setup-agent.md`). The `{STATE_DIR}` parameter determines where output files are written. All AskUserQuestion interactions work in both contexts.

## Table of Contents

- [Parameters](#parameters)
- [Phase A: Build Ontology Pool](#phase-a-build-ontology-pool)
  - [Pool Source Rules](#pool-source-rules)
  - [Step 1: Check Document Source Availability](#step-1-check-document-source-availability)
  - [Step 2: Screen 1 — MCP Data Source Selection](#step-2-screen-1--mcp-data-source-selection)
  - [Step 3: Screen 2 — External Source Addition](#step-3-screen-2--external-source-addition)
  - [Step 4: Screen 3 — Pool Configuration Confirmation](#step-4-screen-3--pool-configuration-confirmation)
  - [Step 5: Build Final Pool Catalog](#step-5-build-final-pool-catalog)
- [Phase B: Generate Scoped References](#phase-b-generate-scoped-references)
- [Exit Gate](#exit-gate)

---

## Parameters

| Placeholder | Description | Examples |
|-------------|-------------|---------|
| `{AVAILABILITY_MODE}` | Behavior when document source is not configured | `optional` (warn and proceed) / `required` (error and stop) |
| `{CALLER_CONTEXT}` | Context label for screen prompt customization | `"analysis"` / `"PRD analysis"` / `"incident analysis"` |
| `{STATE_DIR}` | Absolute path to the skill's state directory for file persistence | `.omc/state/plan-abc123/` |

---

## Phase A: Build Ontology Pool

### Pool Source Rules

#### Document Source
- The `prism-mcp` server exposes documentation directories via `prism_docs_*` tools (configured in `~/.prism/ontology-docs.json`)
- The lead calls `prism_docs_roots` to get `ONTOLOGY_DIRS[]`
- Each directory is a separate pool entry; analysts search only within these paths using `prism_docs_list`, `prism_docs_read`, `prism_docs_search`

#### MCP Data Source
- Any registered MCP server (excluding `prism-mcp` and internal plugin tools) can be added as a queryable data source
- Discovery via `ToolSearch` to find available MCP server tools grouped by server name
- Selected servers provide their tools to analysts for querying live data (databases, monitoring, error tracking, etc.)
- Tool access instructions are generated per server based on discovered tools
- Analysts MUST call `ToolSearch(query="select:<tool_name>")` to load deferred MCP tools before calling them

#### Web Source
- ONLY user-provided URLs (collected in Screen 2) can enter the pool
- Each URL is fetched and summarized at pool build time
- URLs that fail to fetch are marked as `unavailable` in the catalog

#### File Source
- ONLY user-provided file paths (collected in Screen 2) can enter the pool
- Each file is read and summarized at pool build time
- Files that fail to read are marked as `unavailable` in the catalog

### Step 1: Check Document Source Availability

First load the deferred tool: `ToolSearch(query="select:mcp__prism-mcp__prism_docs_roots")`. Then call `mcp__prism-mcp__prism_docs_roots` to get configured documentation directories.

| Result | {AVAILABILITY_MODE}=optional | {AVAILABILITY_MODE}=required |
|--------|------------------------------|------------------------------|
| Returns 1+ paths | `ONTOLOGY_AVAILABLE=true`. Record as `ONTOLOGY_DIRS[]`. Proceed to Step 2. | Record as `ONTOLOGY_DIRS[]`. Proceed to Step 2. |
| Returns 0 paths or "No directories configured" | `ONTOLOGY_AVAILABLE=false`. Warn and proceed to Step 2. | Error: "No documentation directories configured in ~/.prism/ontology-docs.json." **STOP.** |
| Error / tool not found | `ONTOLOGY_AVAILABLE=false`. Warn and proceed to Step 2. | Error and **STOP.** |

### Step 2: Screen 1 — MCP Data Source Selection

Discover available MCP servers that can provide queryable data.

#### Discovery

1. Call `ToolSearch(query="mcp", max_results=200)` to discover MCP servers. Extract unique server names from tool name patterns: `mcp__<server_name>__<tool_name>`.
2. **Exclude** these servers:
   - `prism-mcp` (docs tools handled in Step 1, interview/score tools are internal)
   - Any server name containing `plugin_` (internal plugin tools)
3. For each remaining server, compile from already-discovered tools:
   - **Server name**, **Tool count**, **Key tools** (up to 5), **Description** (infer from tool names)

Store as `DISCOVERED_MCP_SERVERS[]`.

If no MCP data sources found → set `SELECTED_MCP_SERVERS[]` to empty, skip to Step 3.

#### Selection

Present discovered servers via `AskUserQuestion` — the user decides which data sources analysts can access:

```
AskUserQuestion(
  header: "Live Data Sources",
  question: "Select live data sources for {CALLER_CONTEXT}. (multiple selection)",
  multiSelect: true,
  options: [
    {label: "{server_name}", description: "{tool_count} tools — {description}"},
    ...per discovered server,
    {label: "Skip", description: "Proceed without MCP data sources"}
  ]
)
```

If user selects "Skip", gives empty answer, or selects no servers → proceed to Step 3 with empty `SELECTED_MCP_SERVERS[]`.

For each selected server, record: server name, tool list, description, capability summary.

**Safety note:** When compiling tool lists, filter out obvious write/mutation tools (names containing `create`, `update`, `delete`, `patch`, `post`) from the analyst-facing list. For query-execution tools (`run_query`, `run_select_query`), keep them but note: "SELECT queries only."

→ **NEXT ACTION: Proceed to Step 3 — ask about external sources.**

### Step 3: Screen 2 — External Source Addition

```
AskUserQuestion(
  header: "External Sources",
  question: "Any external sources to include for {CALLER_CONTEXT}?",
  multiSelect: false,
  options: [
    {label: "Add URL", description: "Web docs, API docs, blog posts, etc."},
    {label: "Add file path", description: "Local file or directory path"},
    {label: "None — proceed", description: "Proceed with selected sources only"}
  ]
)
```

#### Add URL
1. Collect URL from user input
2. Fetch content via `WebFetch`
3. Extract: title, domain/topic, summary (1-2 lines), key topics (3-5 keywords)
4. Cache the extracted summary for analyst prompt injection
5. If fetch fails → mark as `unavailable` with reason
6. Add to `WEB_ENTRIES[]`
7. Return to Screen 2 (repeat loop)

#### Add file path
1. Collect file path from user input
2. Read content via `Read`
3. Extract: filename, domain/topic, summary (1-2 lines), key topics (3-5 keywords)
4. Cache the extracted summary for analyst prompt injection
5. If read fails → mark as `unavailable` with reason
6. Add to `FILE_ENTRIES[]`
7. Return to Screen 2 (repeat loop)

#### None — proceed
Exit loop.

→ **NEXT ACTION: Proceed to Step 4 — confirm pool configuration.**

### Step 4: Screen 3 — Pool Configuration Confirmation

Output the assembled catalog as text:

```
Ontology Pool Configuration:
| # | Source | Type | Path/URL | Domain | Summary | Status |
|---|--------|------|----------|--------|---------|--------|
| 1..N | mcp | doc  | {ONTOLOGY_DIRS[i]} | {domain} | Documentation directory | available |
| N+1  | mcp | query| mysql    | ...    | ...     | available |
| 3 | web    | url  | ...      | ...    | ...     | available |
| 4 | file   | file | ...      | ...    | ...     | available |
Total N sources (MCP Docs: {0 or 1}, MCP Data: n, Web: n, File: n)
```

Then confirm:

```
AskUserQuestion(
  header: "Pool Confirmation",
  question: "Proceed with this ontology pool configuration?",
  multiSelect: false,
  options: [
    {label: "Confirm — proceed", description: "Start {CALLER_CONTEXT} with this configuration"},
    {label: "Reselect data sources", description: "Go back to data source selection (Screen 1)"},  // ONLY show if Screen 1 was presented (MCP data sources were discovered)
    {label: "Add sources", description: "Go back to external source addition (Screen 2)"},
    {label: "Cancel", description: "Proceed without ontology pool"}
  ]
)
```

| Selection | Action |
|-----------|--------|
| Confirm — proceed | Proceed to Step 5 |
| Reselect data sources | Return to Step 2 (Screen 1). **Only show this option if Screen 1 was presented (MCP data sources were discovered).** |
| Add sources | Return to Step 3 (Screen 2) |
| Cancel + `{AVAILABILITY_MODE}`=`required` | Warn: "Ontology pool is required. Add at least one source." Return to Step 3 (Screen 2) to add external sources. Maximum 2 cancel-retry cycles — after 2nd cancel without adding any source, error: "Cannot proceed without at least one ontology source in required mode." **STOP.** |
| Cancel + `{AVAILABILITY_MODE}`=`optional` | Warn: "Proceeding without ontology pool." Pool Catalog is empty. Proceed |

→ **NEXT ACTION: Proceed to Step 5 — build the final catalog.**

### Step 5: Build and Write `ontology-scope.json`

Combine all sources into a single JSON file that contains both catalog metadata and access instructions.

If no sources after all steps:
- `{AVAILABILITY_MODE}`=`optional` → Warn and proceed. Analysts get `{ONTOLOGY_SCOPE}` = "N/A — no ontology sources available". Skip to Exit Gate.
- `{AVAILABILITY_MODE}`=`required` → Error: "No ontology sources available." **STOP.**

Write to `{STATE_DIR}/ontology-scope.json`:

```json
{
  "sources": [
    {
      "id": 1,
      "type": "doc",
      "path": "/path/to/docs",
      "domain": "inferred domain",
      "summary": "Documentation directory",
      "key_topics": ["topic1", "topic2"],
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
      "key_topics": ["prometheus", "loki", "dashboards"],
      "status": "available",
      "access": {
        "tools": ["mcp__grafana__query_prometheus", "mcp__grafana__query_loki_logs"],
        "instructions": "Call ToolSearch(query=\"select:mcp__{server_name}__{tool_name}\") to load each tool before use, then call directly.",
        "capabilities": "Query Prometheus metrics, Loki logs, dashboards",
        "getting_started": "Start with list_datasources to discover available data",
        "error_handling": "If a tool call fails, note the error and continue. Do NOT retry more than once.",
        "safety": "SELECT/read-only queries only"
      }
    },
    {
      "id": 3,
      "type": "web",
      "url": "https://example.com/docs",
      "domain": "API documentation",
      "summary": "1-2 line summary",
      "key_topics": ["keyword1", "keyword2", "keyword3"],
      "status": "available",
      "access": {
        "instructions": "Content summary provided. Use WebFetch for deeper exploration.",
        "cached_summary": "Fetched content summary from pool build time"
      }
    },
    {
      "id": 4,
      "type": "file",
      "path": "/path/to/file",
      "domain": "domain",
      "summary": "1-2 line summary",
      "key_topics": ["keyword1", "keyword2"],
      "status": "available",
      "access": {
        "instructions": "Content summary provided. Original file at path.",
        "cached_summary": "Read content summary from pool build time"
      }
    },
    {
      "id": 5,
      "type": "web",
      "url": "https://failed.example.com",
      "status": "unavailable",
      "reason": "fetch failed: 404"
    }
  ],
  "totals": {
    "doc": 1,
    "mcp_query": 1,
    "web": 1,
    "file": 1,
    "unavailable": 1
  },
  "citation_format": {
    "doc": "source:section",
    "web": "url:section",
    "file": "file:path:section",
    "mcp_query": "mcp-query:server:detail"
  }
}
```

### Field Rules

- `sources[].id`: sequential integer, starts at 1
- `sources[].type`: one of `doc`, `mcp_query`, `web`, `file`
- `sources[].status`: `available` or `unavailable`
- `sources[].key_topics`: 3-5 keywords (inferred or extracted at pool build time)
- `sources[].access`: present only when `status == "available"` — contains type-specific access instructions
- `sources[].reason`: present only when `status == "unavailable"`
- `totals`: count per type. `unavailable` counts all failed sources regardless of type

---

## Phase B: Generate `{ONTOLOGY_SCOPE}` Text Block

The orchestrator reads `{STATE_DIR}/ontology-scope.json` and generates a text block for analyst prompt injection. Only `available` sources are included in the text block.

### Text block format

```
Your reference documents and data sources:

- doc: {summary} ({status})
  Directories: {path}
  Access: {access.instructions}
    {access.tools — one per line}

- mcp-query: {server_name}: {summary}
  Tools (read-only): {access.tools}
  Access: {access.instructions}
  Capabilities: {access.capabilities}
  Getting started: {access.getting_started}
  Error handling: {access.error_handling}

- web: {url}: {domain} — {summary}
  Access: {access.instructions}
  {access.cached_summary}

- file: {path}: {domain} — {summary}
  Access: {access.instructions}
  {access.cached_summary}

Explore these sources through your perspective's lens.
Cite findings as: {citation_format values}.
```

The orchestrator constructs this text block from the JSON and injects it into the `{ONTOLOGY_SCOPE}` placeholder before spawning analysts.

**Backward compatibility:** If `ontology-scope.json` does not exist at read time (e.g., older session, fast-track skip), the orchestrator injects: `{ONTOLOGY_SCOPE}` = "N/A — ontology scope file not found. Analyze using available evidence only."

---

## Exit Gate

**Empty pool skip:** If sources array is empty and `{AVAILABILITY_MODE}`=`optional`, file-write item below is N/A.

- [ ] Phase A complete: document source checked, MCP data sources selected (or skipped), external sources collected (or skipped), pool confirmed
- [ ] `{STATE_DIR}/ontology-scope.json` written with all source metadata and access instructions
