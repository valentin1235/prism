# Ontology Scope Mapping

**Execution context:** This module is executed by the orchestrator. The `{STATE_DIR}` parameter determines where output files are written.

## Table of Contents

- [Parameters](#parameters)
- [Phase A: Build Ontology Pool](#phase-a-build-ontology-pool)
  - [Pool Source Rules](#pool-source-rules)
  - [Step 1: Check Document Source Availability](#step-1-check-document-source-availability)
  - [Step 2: Source Collection](#step-2-source-collection)
    - [Step 2a: MCP Data Source Selection](#step-2a-mcp-data-source-selection)
    - [Step 2b: External Source Addition](#step-2b-external-source-addition) (includes [Error Handling and Re-prompting](#error-handling-and-re-prompting))
  - [Step 3: Pool Configuration Confirmation](#step-3-pool-configuration-confirmation)
  - [Step 4: Build and Write ontology-scope.json](#step-4-build-and-write-ontology-scopejson)
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
- Discovery via `ToolSearch(query="+mcp__ server", max_results=500)` — uses `+` prefix to require `mcp__` in tool names, then groups by server name
- Selected servers provide their tools to analysts for querying live data (databases, monitoring, error tracking, etc.)
- Tool access instructions are generated per server based on discovered tools
- Analysts MUST call `ToolSearch(query="select:<tool_name>")` to load deferred MCP tools before calling them

#### Web Source
- ONLY user-provided URLs (collected in Step 2b) can enter the pool
- Each URL is **validated for accessibility immediately** upon entry via `WebFetch` — the user gets instant feedback on whether the URL is reachable
- Accessible URLs are fetched, summarized, and cached at entry time (not deferred to pool build)
- Inaccessible URLs are rejected by default; the user may choose to add them as `unavailable` sources

#### File Source
- ONLY user-provided file paths (collected in Step 2b) can enter the pool
- Each file path is **validated for existence immediately** upon entry — non-existent paths are rejected with feedback and never added to the pool
- Existing files are read and summarized at entry time
- Files that exist but fail to read (e.g., permission denied, binary) are marked as `unavailable` in the catalog

### Step 1: Check Document Source Availability

First load the deferred tool: `ToolSearch(query="select:mcp__prism-mcp__prism_docs_roots")`. Then call `mcp__prism-mcp__prism_docs_roots` to get configured documentation directories.

| Result | {AVAILABILITY_MODE}=optional | {AVAILABILITY_MODE}=required |
|--------|------------------------------|------------------------------|
| Returns 1+ paths | `ONTOLOGY_AVAILABLE=true`. Record as `ONTOLOGY_DIRS[]`. Proceed to Step 2. | Record as `ONTOLOGY_DIRS[]`. Proceed to Step 2. |
| Returns 0 paths or "No directories configured" | `ONTOLOGY_AVAILABLE=false`. Warn and proceed to Step 2. | Error: "No documentation directories configured in ~/.prism/ontology-docs.json." **STOP.** |
| Error / tool not found | `ONTOLOGY_AVAILABLE=false`. Warn and proceed to Step 2. | Error and **STOP.** |

### Step 2: Source Collection

Steps 2a and 2b form a single cohesive source-collection phase. The user flows from MCP server selection directly into external source addition without an intermediate confirmation — the combined result is reviewed once in Step 3.

#### Step 2a: MCP Data Source Selection

Discover available MCP servers that can provide queryable data.

##### Discovery

MCP tools follow the naming pattern `mcp__<server_name>__<tool_name>`. Use the `+` prefix query form to require this pattern in tool names — keyword search (e.g., `query="mcp"`) is unreliable and may return unrelated tools or miss MCP tools entirely.

1. **Primary discovery:** Call `ToolSearch(query="+mcp__ server", max_results=500)` to find all MCP tools. The `+mcp__` prefix requires "mcp__" in tool names, ensuring only MCP tools are returned. `max_results=500` accommodates all tools across all servers (typically 15-30 tools per server × 14 servers).
2. **Fallback — empty results:** If step 1 returns 0 tools, try `ToolSearch(query="mcp__", max_results=500)` (keyword search without `+` prefix). If still 0, skip to Step 2b (no MCP servers available).
3. **Completeness check:** If step 1 returns exactly `max_results` tools (500), results may be truncated. Re-run with `max_results=1000`.
4. **Extract server names:** Parse each tool name matching `mcp__<server_name>__<tool_name>` and collect unique `<server_name>` values. Group tools by server.
5. **Exclude builtin servers** — remove these from the discovered list:
   - `prism-mcp` (docs tools handled in Step 1, interview/score tools are internal)
   - Any server name containing `plugin_` (internal plugin tools)
   - Any server name starting with `__` (internal system servers)
6. For each remaining server, compile from already-discovered tools:
   - **Server name**, **Tool count**, **Key tools** (up to 5), **Description** (infer from tool names)

Store as `DISCOVERED_MCP_SERVERS[]`.

**Verification:** Log the count: "Discovered {N} user MCP servers: {comma-separated names}". If the count seems unexpectedly low (<5 servers when many are expected), re-run discovery with `max_results=1000` before proceeding.

If no MCP data sources found → set `SELECTED_MCP_SERVERS[]` to empty, skip to Step 2b.

##### Selection

Present discovered servers via `AskUserQuestion` with `multiSelect` — each option represents a **server** (not an individual tool). The user selects which data source servers analysts can access:

```
AskUserQuestion(
  header: "Live Data Sources ({N} servers discovered)",
  question: "Select live data sources for {CALLER_CONTEXT}. (multiple selection)\nEach item is an MCP server — selecting it grants analysts access to all its read-only tools.",
  multiSelect: true,
  options: [
    {label: "{server_name}", description: "{tool_count} tools — {capability_keywords}"},
    ...one option per DISCOVERED_MCP_SERVERS[] entry (never expand individual tools as separate options),
    {label: "Skip", description: "Proceed without MCP data sources"}
  ]
)
```

Where `{capability_keywords}` is a short phrase (≤10 words) summarizing the server's domain, inferred from its tool names (e.g., "database queries, schema inspection" or "log search, dashboard metrics").

If user selects "Skip", gives empty answer, or selects no servers → proceed to Step 2b with empty `SELECTED_MCP_SERVERS[]`.

For each selected server, record: server name, full tool list (read-only filtered), description, capability summary.

**Safety note:** When compiling tool lists, filter out obvious write/mutation tools (names containing `create`, `update`, `delete`, `patch`, `post`) from the analyst-facing list. For query-execution tools (`run_query`, `run_select_query`), keep them but note: "SELECT queries only."

→ **NEXT ACTION: Continue to Step 2b — external source addition (same phase, no intermediate confirmation).**

#### Step 2b: External Source Addition

A single-question loop that collects URLs and file paths via free-text input with automatic type detection. **Zero external sources is a valid outcome** — the user may select "Done" immediately on the first iteration to proceed with only MCP selections (or no additional sources at all).

##### Loop

Each iteration presents one `AskUserQuestion`. The user either types a source (URL or file path) via the **Other** free-text option, or selects **Done** to finish.

**Every iteration** must display the cumulative list of already-added sources directly in the question text. This ensures the user always has full context of what has been collected before deciding to add more or finish. After each source is processed (added or marked unavailable), rebuild the cumulative list from `WEB_ENTRIES[]` and `FILE_ENTRIES[]` before presenting the next iteration.

```
AskUserQuestion(
  header: "External Sources{cumulative_suffix}",
  question: "{cumulative_list}Paste a URL or file path to add, or select Done.",
  multiSelect: false,
  options: [
    {label: "Done", description: "Proceed with {available_count} available / {total_added} total source(s)"}  // "Proceed without external sources" when total_added == 0
  ]
)
```

Where:
- `{total_added}` = `len(WEB_ENTRIES) + len(FILE_ENTRIES)` (total count including unavailable)
- `{available_count}` = count of entries with `status: "available"` across both arrays
- `{cumulative_suffix}` = ` ({total_added} added)` when total_added > 0, empty string otherwise
- `{cumulative_list}` = when total_added > 0, a numbered list of all collected sources followed by a blank line separator:
  ```
  Added so far:
  1. [web] https://example.com — API documentation ✓
  2. [file] /path/to/config.yaml — Configuration file ✓
  3. [web] https://broken.example.com — ⚠ unavailable (404)

  ```
  Each entry shows: sequential number, type tag (`[web]` or `[file]`), path or URL, summary or description, and status indicator (`✓` for available, `⚠ unavailable ({reason})` for failed fetches/reads).
  When total_added == 0 (first iteration), `{cumulative_list}` is an empty string — the question starts directly with "Paste a URL or file path...".

##### Processing User Input

**Done selected** → Exit loop. Set `WEB_ENTRIES[]` and `FILE_ENTRIES[]` to empty arrays if no sources were added. This is the normal path when the user wants to proceed with MCP data sources only — no warning or confirmation is needed.

**Other / free-text input** → Determine the source type automatically:

1. **Type detection:** If the input starts with `http://` or `https://`, treat as URL. Otherwise, treat as file path.
2. **Empty or whitespace-only input** → Show inline message: "No input detected — please paste a URL or file path." Re-prompt with the same cumulative list.

**URL processing — immediate accessibility validation:**
1. **Format check:** Verify the URL is well-formed (starts with `http://` or `https://`, contains a valid domain). If malformed → show error inline: "⚠ Invalid URL format. Must start with `http://` or `https://`." Do NOT add to any list. Re-prompt with the unchanged cumulative list (see [Error Handling and Re-prompting](#error-handling-and-re-prompting)).
2. **Accessibility check (blocking):** Call `WebFetch(url="{input}", prompt="Return the page title and a 2-sentence summary of the page content.")` immediately — do NOT defer validation to pool build time.
3. **On success:** Extract from WebFetch response: title, domain/topic, summary (1-2 lines), key topics (3-5 keywords). Cache the extracted summary for analyst prompt injection. Add to `WEB_ENTRIES[]` with `status: "available"`. Notify user inline: "✓ Added: {title} ({domain})". Repeat loop.
4. **On failure** (network error, HTTP 4xx/5xx, timeout, empty content, redirect to login page):
   - Notify user with the failure reason and present a choice:
     ```
     AskUserQuestion(
       question: "⚠ Could not access URL: {reason}. What would you like to do?",
       options: [
         {label: "Add anyway", description: "Keep as unavailable source — analysts will see the URL but cannot fetch content"},
         {label: "Skip", description: "Discard this URL and continue"}
       ]
     )
     ```
   - **Add anyway** → Add to `WEB_ENTRIES[]` with `status: "unavailable"`, `reason: "{failure_reason}"`. Notify user: "Added as unavailable." Repeat loop.
   - **Skip** → Discard. Repeat loop.
5. **Authenticated URL hint:** If WebFetch reports an authentication wall or login redirect, append to the failure message: "This URL may require authentication. Consider using an MCP data source with authenticated access instead."

**File path processing:**
1. **Normalize path:** Expand `~` to home directory. Resolve relative paths against the current working directory.
2. **Validate existence immediately:** Use `Glob(pattern="{normalized_path}")` to check whether the path exists.
   - **Path does not exist** → Show error inline: "⚠ File not found: `{normalized_path}`. Please check the path and try again." Do NOT add to `FILE_ENTRIES[]`. Re-prompt with unchanged cumulative list (see [Error Handling and Re-prompting](#error-handling-and-re-prompting)).
   - **Path is a directory** (detected via trailing `/` in glob result or by attempting `Read` which returns a directory error) → Show error inline: "⚠ `{normalized_path}` is a directory, not a file. Provide a file path instead." Re-prompt with unchanged cumulative list.
   - **Path exists** → Continue to step 3.
3. Read content via `Read`
4. Extract: filename, domain/topic, summary (1-2 lines), key topics (3-5 keywords)
5. Cache the extracted summary for analyst prompt injection
6. **On success (steps 3-5 complete):** Add to `FILE_ENTRIES[]` with `status: "available"`. Notify user inline: "✓ Added: {filename}". Repeat loop.
7. **On read failure** despite existence check (e.g., permission denied, binary file) → Add to `FILE_ENTRIES[]` with `status: "unavailable"`, `reason: "{failure_reason}"`. Show error inline: "⚠ Failed to read `{normalized_path}`: {reason}. Added as unavailable." Re-prompt with updated cumulative list showing the new `⚠ unavailable` entry. Repeat loop.

##### Error Handling and Re-prompting

Every validation failure follows the same pattern: **show error inline, then re-prompt with the unchanged cumulative list**. The user never leaves the loop due to an error — they always get another chance to enter a valid source or select Done.

| Failure | Error message | Action |
|---------|--------------|--------|
| Empty / whitespace input | "No input detected — please paste a URL or file path." | Re-prompt immediately (same cumulative list) |
| Malformed URL (no `http://` / `https://`, invalid domain) | "⚠ Invalid URL format. Must start with `http://` or `https://`." | Re-prompt immediately — do NOT add to any list |
| URL inaccessible (4xx, 5xx, timeout, network error) | "⚠ Could not access URL: {reason}." + Add anyway / Skip choice | After choice, re-prompt with updated cumulative list |
| URL authentication wall / login redirect | Above message + "This URL may require authentication. Consider using an MCP data source with authenticated access instead." | Same as inaccessible |
| File path does not exist | "⚠ File not found: `{normalized_path}`. Please check the path and try again." | Re-prompt immediately — do NOT add to `FILE_ENTRIES[]` |
| Path is a directory | "⚠ `{normalized_path}` is a directory, not a file. Provide a file path instead." | Re-prompt immediately — do NOT add to `FILE_ENTRIES[]` |
| File exists but read fails (permission, binary) | "⚠ Failed to read `{normalized_path}`: {reason}. Added as unavailable." | Add to `FILE_ENTRIES[]` with `status: "unavailable"`, re-prompt with updated cumulative list |

**Key invariant:** After every error message, the next `AskUserQuestion` call presents the current cumulative list (unchanged if the source was rejected, updated if it was added as `unavailable`). The user always sees their full context before deciding the next action.

→ **NEXT ACTION: Proceed to Step 3 — confirm pool configuration.**

### Step 3: Pool Configuration Confirmation

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

**Note:** A pool with only MCP sources (Doc + Data) and zero external sources (Web: 0, File: 0) is a valid configuration — do not warn or require the user to add external sources.

Then confirm:

```
AskUserQuestion(
  header: "Pool Confirmation",
  question: "Proceed with this ontology pool configuration?",
  multiSelect: false,
  options: [
    {label: "Confirm — proceed", description: "Start {CALLER_CONTEXT} with this configuration"},
    {label: "Reselect data sources", description: "Go back to MCP data source selection (Step 2a)"},  // ONLY show if Step 2a was presented (MCP data sources were discovered)
    {label: "Add sources", description: "Go back to external source addition (Step 2b)"},
    {label: "Cancel", description: "Proceed without ontology pool"}
  ]
)
```

| Selection | Action |
|-----------|--------|
| Confirm — proceed | Proceed to Step 4 |
| Reselect data sources | Return to Step 2a (MCP selection). **Only show this option if Step 2a was presented (MCP data sources were discovered).** |
| Add sources | Return to Step 2b (external source addition) |
| Cancel + `{AVAILABILITY_MODE}`=`required` | Warn: "Ontology pool is required. Add at least one source." Return to Step 2b to add external sources. Maximum 2 cancel-retry cycles — after 2nd cancel without adding any source, error: "Cannot proceed without at least one ontology source in required mode." **STOP.** |
| Cancel + `{AVAILABILITY_MODE}`=`optional` | Warn: "Proceeding without ontology pool." Pool Catalog is empty. Proceed |

→ **NEXT ACTION: Proceed to Step 4 — build the final catalog.**

### Step 4: Build and Write `ontology-scope.json`

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
