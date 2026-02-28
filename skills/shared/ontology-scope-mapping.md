# Ontology Scope Mapping

## Table of Contents

- [Parameters](#parameters)
- [Phase A: Build Ontology Pool](#phase-a-build-ontology-pool)
  - [Pool Source Rules](#pool-source-rules)
  - [Step 1: Check ontology-docs MCP Availability](#step-1-check-ontology-docs-mcp-availability)
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
| `{AVAILABILITY_MODE}` | Behavior when ontology-docs MCP is not configured | `optional` (warn and proceed) / `required` (error and stop) |
| `{CALLER_CONTEXT}` | Context label for screen prompt customization | `"analysis"` / `"PRD analysis"` / `"incident analysis"` |

---

## Phase A: Build Ontology Pool

### Pool Source Rules

#### Document Source
- The `ontology-docs` global MCP server is registered as a **single pool entry** when available
- No pre-selection of individual directories — analysts explore the MCP directly at reasoning time using `search_files`, `read_file`, `list_directory`, etc.
- The lead captures `ALLOWED_ROOTS[]` from `list_allowed_directories` and passes them to analysts as exploration starting points

#### MCP Data Source
- Any registered MCP server (excluding `ontology-docs` and internal plugin tools) can be added as a queryable data source
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

### Step 1: Check ontology-docs MCP Availability

Call `mcp__ontology-docs__list_allowed_directories` to check if the ontology-docs MCP server is reachable.

| Result | {AVAILABILITY_MODE}=optional | {AVAILABILITY_MODE}=required |
|--------|------------------------------|------------------------------|
| Success (returns 1+ paths) | `ONTOLOGY_AVAILABLE=true`. Record returned paths as `ALLOWED_ROOTS[]`. Proceed to Step 2. | Record returned paths as `ALLOWED_ROOTS[]`. Proceed to Step 2. |
| Error / MCP not configured | `ONTOLOGY_AVAILABLE=false`. Warn: "ontology-docs MCP not configured. Pool will contain MCP data sources and external sources only." Proceed to Step 2. | Error: "ontology-docs MCP not configured. See plugin README for setup." **STOP.** |

**That's it.** No directory enumeration, classification, or characterization. Analysts will explore ontology-docs directly at reasoning time using the MCP tools available to them.

### Step 2: Screen 1 — MCP Data Source Selection

Discover available MCP servers that can provide queryable data (databases, monitoring, error tracking, project management, etc.).

#### Discovery

Two-pronged discovery to ensure all MCP servers are found:

1. Call `ListMcpResourcesTool()` (no server filter) to discover all resource-based MCP servers. Extract unique server names from the `server` field in results.
2. Call `ToolSearch(query="mcp", max_results=50)` to discover tool-based MCP servers. Extract unique server names from tool name patterns: `mcp__<server_name>__<tool_name>`.
3. Combine server names from steps 1 and 2. Deduplicate.
4. **Exclude** these servers from the data source list:
   - `ontology-docs` (handled as Document Source in Step 1)
   - Any server name containing `plugin_` (internal plugin tools, not data sources)
5. For each remaining server, call `ToolSearch(query="+<server_name>", max_results=10)` to list its tools. Compile:
   - **Server name**: the `<server_name>` from the tool prefix
   - **Tool count**: number of tools discovered for this server
   - **Key tools**: up to 5 most relevant tool names (without the `mcp__<server>__` prefix for readability)
   - **Description**: infer purpose from tool names (e.g., `run_query` + `get_table_schema` → "Database queries and schema inspection"). If purpose cannot be confidently inferred, use: "Query {server_name} via {tool_count} available tools"
   - **Read-only tools**: filter out obvious write/mutation tools (names containing `create`, `update`, `delete`, `patch`, `post`) for the analyst tool list. For query-execution tools like `run_query` or `run_select_query`, keep them but add a safety note: "Use SELECT queries only. Do NOT execute INSERT, UPDATE, DELETE, or DDL statements." Record full list separately for reference.

Store as `DISCOVERED_MCP_SERVERS[]`.

If no MCP data sources are discovered → skip to Step 3 with note: "No queryable MCP data sources found."

#### Selection

Present discovered MCP servers for user selection via `AskUserQuestion` with `multiSelect: true`:

```
AskUserQuestion(
  header: "Live Data Sources",
  question: "Select live data sources to enable for {CALLER_CONTEXT}. These are connected services (databases, monitoring, error tracking) that analysts can query in real-time during analysis. (multiple selection)",
  multiSelect: true,
  options: [
    {label: "{server_name}", description: "{tool_count} tools — {description}. Key: {key_tool_1}, {key_tool_2}, ..."},
    ...per discovered server,
    {label: "Skip", description: "Proceed without MCP data sources"}
  ]
)
```

If user selects "Skip" or no servers → proceed to Step 3 with empty `SELECTED_MCP_SERVERS[]`.

Otherwise, for each selected server:
1. Record full tool list (call additional `ToolSearch(query="+<server_name>")` if the initial discovery was incomplete)
2. Generate a brief capability summary describing what analysts can query
3. Add to `SELECTED_MCP_SERVERS[]` with: server name, full tool list, description, capability summary

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
Exit loop, proceed to Step 4.

### Step 4: Screen 3 — Pool Configuration Confirmation

Output the assembled catalog as text:

```
Ontology Pool Configuration:
| # | Source | Type | Path/URL | Domain | Summary | Status |
|---|--------|------|----------|--------|---------|--------|
| 1 | mcp    | doc  | ontology-docs MCP | (analyst-explored) | Analysts explore directly | available |
| 2 | mcp    | query| mysql    | ...    | ...     | available |
| 3 | web    | url  | ...      | ...    | ...     | available |
| 4 | file   | file | ...      | ...    | ...     | available |
Total N sources (MCP Docs: 1, MCP Data: n, Web: n, File: n)
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

### Step 5: Build Final Pool Catalog

Combine all sources into a unified catalog:

```
## Ontology Pool Catalog

| # | Source | Type | Path/URL | Domain | Summary | Key Topics | Status |
|---|--------|------|----------|--------|---------|------------|--------|
| 1 | mcp | doc | ontology-docs | (analyst-explored) | Full MCP access | search_files, read_file, list_directory | available |
| 2 | mcp | query | {server_name} | {domain} | {description} | {key tools} | available |
| 3 | web | url | {url} | {domain} | {1-2 line summary} | {3-5 keywords} | available |
| 4 | file | file | {path} | {domain} | {1-2 line summary} | {3-5 keywords} | available |
| 5 | web | url | {url} | — | — | — | unavailable: {reason} |
```

If catalog is empty after all steps:
- `{AVAILABILITY_MODE}`=`optional` → Pool Catalog is empty. Warn and proceed.
- `{AVAILABILITY_MODE}`=`required` → Error: "No ontology sources available." **STOP.**

**Phase A output:** Pool Catalog (unified table of available document, MCP data, web, and file sources).

If Pool Catalog is empty and `{AVAILABILITY_MODE}`=`optional` → analysts get `{ONTOLOGY_SCOPE}` = "N/A — no ontology sources available". Skip to Exit Gate.

---

## Phase B: Generate Scoped References

All perspectives (analysts) receive the **full pool** — no per-perspective filtering.

**Note:** The lead MUST generate two `{ONTOLOGY_SCOPE}` variants from this phase — one for analysts (with access instructions) and one for the DA (with verification mission). Inject the correct variant per role at spawn time.

### For all analysts:

Generate `{ONTOLOGY_SCOPE}` block containing all available pool entries:

For **document sources** (ontology-docs MCP):
```
- doc: ontology-docs MCP (available)
  Allowed roots: {ALLOWED_ROOTS[] joined by ", "}
  Access: Use mcp__ontology-docs__ tools to explore documentation at reasoning time.
  Discovery: Call list_directory on the allowed roots above to browse structure,
             search_files to find relevant documents by keyword,
             read_file or read_text_file to read content.
  Note: No pre-selected directories — explore the full MCP scope through your perspective's lens.
```

For **MCP data sources** (mcp query):
```
- mcp-query: {server_name}: {description}
  Tools (read-only): {tool_1}, {tool_2}, {tool_3}, ...
  Access: Call `ToolSearch(query="select:mcp__{server_name}__{tool_name}")` to load each tool before use, then call it directly.
  Capabilities: {what can be queried}
  Getting started: {discovery pattern — e.g., "Call get_table_schema first to discover tables, then run_query for data"}
  Error handling: If a tool call fails (auth error, timeout, permission denied), note the error and continue analysis with other available sources. Do NOT retry more than once.
```

For **web sources**:
```
- web: {url}: {domain} — {summary}
  Access: Content summary provided below. Use WebFetch if deeper exploration needed.
  {cached summary from pool build}
```

For **file sources**:
```
- file: {path}: {domain} — {summary}
  Access: Content summary provided below. Original file at {path}.
  {cached summary from pool build}
```

Combined per-analyst block:
```
Your reference documents and data sources:
{list of ALL available pool entries with access instructions}

Explore these sources through your perspective's lens.
Cite findings as "source:section" (doc sources), "url:section" (web sources), "file:path:section" (file sources), or "mcp-query:server:detail" (MCP data sources).
```

### For Devil's Advocate:
```
You have access to ALL pool entries. Verify analysts explored thoroughly.

Full Ontology Pool:
{complete pool catalog table — available entries only}

Check: Did each analyst find relevant evidence in the ontology-docs MCP?
Check: Are there documents or sections within ontology-docs that no analyst explored?
Check: Did analysts effectively query the available MCP data sources?
Check: Are there MCP data sources that could have provided additional evidence but were not queried?
Check: Did analysts reference relevant web sources from the pool?
Check: Did analysts reference relevant file sources from the pool?
```

---

## Exit Gate

- [ ] ontology-docs MCP availability checked via `list_allowed_directories` (or `ONTOLOGY_AVAILABLE=false`)
- [ ] `ALLOWED_ROOTS[]` captured and included in `{ONTOLOGY_SCOPE}` doc source block (if available)
- [ ] MCP data sources discovered via `ToolSearch` and presented to user via Screen 1 (or none available)
- [ ] Selected MCP servers recorded with full tool lists and capability summaries
- [ ] External sources collected via Screen 2 (or explicitly skipped)
- [ ] Pool configuration confirmed via Screen 3
- [ ] Pool Catalog generated with Source, Type, and Status for every entry
- [ ] Web source summaries cached for analyst prompt use
- [ ] File source summaries cached for analyst prompt use
- [ ] MCP data source access instructions generated with tool loading steps (`ToolSearch` → direct call)
- [ ] Full-pool `{ONTOLOGY_SCOPE}` block generated for all analysts with correct access instructions per source type
- [ ] DA full-scope block generated with verification mission (including ontology-docs and MCP data source utilization check)
