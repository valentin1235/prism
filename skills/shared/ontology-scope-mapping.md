# Ontology Scope Mapping

## Table of Contents

- [Parameters](#parameters)
- [Phase A: Build Ontology Pool](#phase-a-build-ontology-pool)
  - [Pool Source Rules](#pool-source-rules)
  - [Step 1: Discover and Characterize](#step-1-discover-and-characterize-document-sources)
    - [1A. Probe MCP Availability](#1a-probe-mcp-availability)
    - [1B. Enumerate Top-Level Entries](#1b-enumerate-top-level-entries)
    - [1C. Classify Entries via 3-Category Heuristic](#1c-classify-entries-via-3-category-heuristic)
    - [1D. Depth-2 Probe for UNKNOWN Entries](#1d-depth-2-probe-for-unknown-entries)
    - [1E. Characterize KEEP Entries](#1e-characterize-keep-entries)
    - [1F. Scoped search_files Fallback](#1f-scoped-search_files-fallback-rare--last-resort)
  - [Step 2: Screen 1 — MCP Document Selection](#step-2-screen-1--mcp-document-selection)
  - [Step 3: Screen 2 — MCP Data Source Selection](#step-3-screen-2--mcp-data-source-selection)
  - [Step 4: Screen 3 — External Source Addition](#step-4-screen-3--external-source-addition)
  - [Step 5: Screen 4 — Pool Configuration Confirmation](#step-5-screen-4--pool-configuration-confirmation)
  - [Step 6: Build Final Pool Catalog](#step-6-build-final-pool-catalog)
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
- ONLY documents registered in the `ontology-docs` global MCP server can enter the pool
- Discovery uses `list_allowed_directories` + `list_directory` with 3-category heuristic (KEEP/SKIP/UNKNOWN) — see Step 1 for details
- `search_files` is reserved as a last-resort fallback within already-scoped directories only
- Documents outside allowed paths MUST NOT be included

#### MCP Data Source
- Any registered MCP server (excluding `ontology-docs` and internal plugin tools) can be added as a queryable data source
- Discovery via `ToolSearch` to find available MCP server tools grouped by server name
- Selected servers provide their tools to analysts for querying live data (databases, monitoring, error tracking, etc.)
- Tool access instructions are generated per server based on discovered tools
- Analysts MUST call `ToolSearch(query="select:<tool_name>")` to load deferred MCP tools before calling them

#### Web Source
- ONLY user-provided URLs (collected in Screen 3) can enter the pool
- Each URL is fetched and summarized at pool build time
- URLs that fail to fetch are marked as `unavailable` in the catalog

#### File Source
- ONLY user-provided file paths (collected in Screen 3) can enter the pool
- Each file is read and summarized at pool build time
- Files that fail to read are marked as `unavailable` in the catalog

### Step 1: Discover and Characterize Document Sources

#### 1A. Probe MCP Availability

Call `mcp__ontology-docs__list_allowed_directories` to discover the root paths registered in ontology-docs.

| Result | {AVAILABILITY_MODE}=optional | {AVAILABILITY_MODE}=required |
|--------|------------------------------|------------------------------|
| Success (returns 1+ paths) | `ONTOLOGY_AVAILABLE=true`. Record paths as `ALLOWED_ROOTS[]`. Proceed to 1B. | Record paths as `ALLOWED_ROOTS[]`. Proceed to 1B. |
| Error / MCP not configured | `ONTOLOGY_AVAILABLE=false`. Warn: "ontology-docs MCP not configured. Pool will contain MCP data sources and external sources only." Skip to Step 3. | Error: "ontology-docs MCP not configured. See plugin README for setup." **STOP.** |

#### 1B. Enumerate Top-Level Entries

For each path in `ALLOWED_ROOTS[]`, call `mcp__ontology-docs__list_directory(path)`.

Collect every returned entry (directories and files) into `RAW_ENTRIES[]`. Each entry has: name, full path, type (directory or file).

**Circuit Breaker:** Track cumulative MCP calls in a counter `MCP_CALL_COUNT`. Initialize to the calls already made (list_allowed_directories + one list_directory per root). Hard ceiling: **100 MCP calls total** for the entire Step 1. If the ceiling is reached at any point, stop discovery, emit warning: "Discovery budget reached ({MCP_CALL_COUNT}/100 calls). Proceeding with {N} entries discovered so far." Skip to 1E with whatever entries have been classified.

#### 1C. Classify Entries via 3-Category Heuristic

Classify each directory entry in `RAW_ENTRIES[]` into exactly one category: **KEEP**, **SKIP**, or **UNKNOWN**.

File entries (non-directories) at the top level are collected separately as `TOP_LEVEL_FILES[]` and are not classified (they are included as-is in discovery results if relevant, e.g., a standalone README.md).

##### Classification Table

| Category | Rule | Examples |
|----------|------|---------|
| **KEEP** | Name matches a doc keyword at a **word boundary**. A word boundary is defined as: start/end of string, or adjacent to `-`, `_`, `/`, or `.` (i.e., the keyword appears as a complete segment in a hyphen-, underscore-, dot-, or slash-delimited name). The doc keywords are: `docs`, `doc`, `documentation`, `wiki`, `knowledge`, `kb`, `handbook`, `manual`, `guide`, `guides`, `reference`, `references`, `specs`, `spec`, `runbook`, `runbooks`, `playbook`, `playbooks`, `notes`, `ontology`, `glossary`, `faq`, `howto`, `tutorials`, `tutorial`, `articles`, `blog` | `podo-docs`, `api-docs`, `docs`, `my_wiki`, `knowledge-base`, `user-guide`, `project.docs` |
| **SKIP** | Name starts with `.` (hidden), OR name exactly matches a known non-doc directory. Known non-doc list: `node_modules`, `vendor`, `dist`, `build`, `out`, `target`, `.git`, `.svn`, `.hg`, `__pycache__`, `.cache`, `.bun`, `.terraform`, `.next`, `.nuxt`, `coverage`, `tmp`, `temp`, `log`, `logs`, `bin`, `obj`, `pkg`, `.idea`, `.vscode`, `.settings`, `bower_components`, `jspm_packages`, `.gradle`, `.mvn`, `.cargo`, `.rustup`, `venv`, `.venv`, `env`, `.env`, `site-packages`, `Pods`, `DerivedData`, `.DS_Store`, `Thumbs.db` | `.git`, `node_modules`, `.terraform`, `.bun`, `dist`, `.cache` |
| **UNKNOWN** | Everything else — does not match KEEP or SKIP | `podo-backend`, `infra`, `services`, `src`, `packages`, `apps`, `lib`, `core` |

##### Doc Keyword False-Positive Prevention

The following names are **explicitly excluded** from KEEP despite containing doc substrings, because they are not documentation directories:

| Name Pattern | Why Excluded |
|--------------|-------------|
| `docker`, `dockerfile`, `docker-compose` | Contains `doc` but is container configuration |
| `docusaurus` | Build tool, not a doc root itself (its content dirs like `docs/` will be caught separately) |
| `docket` | Contains `doc` but is unrelated |

Implementation: Before applying KEEP rules, check if the name (lowercased) starts with `docker` or exactly matches `docusaurus` or `docket`. If so, classify as UNKNOWN (not KEEP).

Store classified entries as `KEEP_ENTRIES[]`, `SKIP_ENTRIES[]`, `UNKNOWN_ENTRIES[]`.

#### 1D. Depth-2 Probe for UNKNOWN Entries

For each directory in `UNKNOWN_ENTRIES[]`, call `mcp__ontology-docs__list_directory(path)` (increment `MCP_CALL_COUNT`; respect the 100-call ceiling).

For each child directory returned:
1. Apply the same KEEP/SKIP classification from the table in 1C.
2. If a child matches **KEEP**, record the **child path** (not the parent) as a doc root. Add it to `KEEP_ENTRIES[]` with a note: `"discovered via depth-2 probe of {parent}"`.
3. If no children match KEEP, the UNKNOWN parent is discarded (not included in discovery results).

**Key rule:** The doc root is the most specific matching directory. If `podo-backend/podo-docs` matches KEEP, the doc root is `podo-backend/podo-docs`, NOT `podo-backend`.

Do NOT recurse beyond depth 2 (i.e., do not probe children of children).

#### 1E. Characterize KEEP Entries

For each directory in `KEEP_ENTRIES[]` (both directly matched and depth-2 discovered):

1. Attempt to read `{dir}/README.md` via `mcp__ontology-docs__read_text_file` (increment `MCP_CALL_COUNT`).
2. If not found, attempt `{dir}/CLAUDE.md`.
3. If neither exists, call `mcp__ontology-docs__list_directory(dir)` to inspect file listing and infer the domain from filenames and directory structure.

Record per entry: path, domain, summary (1-2 lines), key topics (3-5 keywords), discovery method (`direct` or `depth-2:{parent}`).

Store as `DISCOVERED_ENTRIES[]`.

#### 1F. Scoped search_files Fallback (Rare — Last Resort)

**Condition:** Only execute this sub-step if `DISCOVERED_ENTRIES[]` is empty after 1E AND there are UNKNOWN entries that were discarded (had no KEEP children at depth-2). This is a fallback to catch unconventionally named doc directories.

For each ALLOWED_ROOT where no KEEP entries were found, call `mcp__ontology-docs__search_files(path={ALLOWED_ROOT}, pattern="README.md")` (increment `MCP_CALL_COUNT`; respect ceiling).

**Result size guard:** If the search returns more than **200 results**, discard the results and emit warning: "search_files returned {N} results for {ALLOWED_ROOT} — too broad to process. Skipping automated refinement." Proceed without these results.

If results are within the 200 limit, extract unique parent directories from the matched file paths. For each unique parent:
1. Apply KEEP/SKIP/UNKNOWN classification from 1C.
2. KEEP matches are added to `KEEP_ENTRIES[]` and characterized per 1E.
3. SKIP and UNKNOWN matches are discarded.

**This sub-step is NOT the primary discovery mechanism.** It only runs as a last-resort fallback when the heuristic in 1C/1D found nothing.

#### Step 1 Cost Summary

| Sub-step | Purpose | MCP Calls (typical) |
|----------|---------|-------------------|
| 1A | Discover allowed roots | 1 |
| 1B | List top-level entries per root | 1 per root |
| 1C | Classify KEEP/SKIP/UNKNOWN | 0 (pure logic) |
| 1D | Probe UNKNOWN children | 1 per UNKNOWN dir |
| 1E | Characterize KEEP entries | 1-2 per KEEP entry |
| 1F | Fallback search (rare) | 1 per root (if triggered) |
| **Total** | | **Typically 10-30, hard max 100** |

### Step 2: Screen 1 — MCP Document Selection

If `ONTOLOGY_AVAILABLE=false` → skip to Step 3 with warning: "No MCP documents available. MCP data sources and external sources can still be added."

Present discovered entries for user selection via `AskUserQuestion` with `multiSelect: true`.

#### Screen 1A — Single page (entries ≤ 3)

```
AskUserQuestion(
  header: "Ontology Pool",
  question: "Select ontology documents for {CALLER_CONTEXT}. (multiple selection)",
  multiSelect: true,
  options: [
    {label: "{domain}", description: "{path} — {summary}"},
    ...per discovered entry
  ]
)
```

#### Screen 1B — Pagination (entries > 3)

Present in batches of 3, with a 4th option to include all on the page:

```
AskUserQuestion(
  header: "Ontology Pool (page/total)",
  question: "Include these documents in the ontology pool? (select to include)",
  multiSelect: true,
  options: [
    {label: "{domain_1}", description: "{path_1} — {summary_1}"},
    {label: "{domain_2}", description: "{path_2} — {summary_2}"},
    {label: "{domain_3}", description: "{path_3} — {summary_3}"},
    {label: "Include all", description: "Include all documents on this page"}
  ]
)
```

Repeat for each page. Collect all selected entries into `SELECTED_ENTRIES[]`.

### Step 3: Screen 2 — MCP Data Source Selection

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

If no MCP data sources are discovered → skip to Step 4 with note: "No queryable MCP data sources found."

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

If user selects "Skip" or no servers → proceed to Step 4 with empty `SELECTED_MCP_SERVERS[]`.

Otherwise, for each selected server:
1. Record full tool list (call additional `ToolSearch(query="+<server_name>")` if the initial discovery was incomplete)
2. Generate a brief capability summary describing what analysts can query
3. Add to `SELECTED_MCP_SERVERS[]` with: server name, full tool list, description, capability summary

### Step 4: Screen 3 — External Source Addition

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
7. Return to Screen 3 (repeat loop)

#### Add file path
1. Collect file path from user input
2. Read content via `Read`
3. Extract: filename, domain/topic, summary (1-2 lines), key topics (3-5 keywords)
4. Cache the extracted summary for analyst prompt injection
5. If read fails → mark as `unavailable` with reason
6. Add to `FILE_ENTRIES[]`
7. Return to Screen 3 (repeat loop)

#### None — proceed
Exit loop, proceed to Step 5.

### Step 5: Screen 4 — Pool Configuration Confirmation

Output the assembled catalog as text:

```
Ontology Pool Configuration:
| # | Source | Type | Path/URL | Domain | Summary | Status |
|---|--------|------|----------|--------|---------|--------|
| 1 | mcp    | doc  | ...      | ...    | ...     | available |
| 2 | mcp    | query| mysql    | ...    | ...     | available |
| 3 | web    | url  | ...      | ...    | ...     | available |
| 4 | file   | file | ...      | ...    | ...     | available |
Total N sources (MCP Docs: n, MCP Data: n, Web: n, File: n)
```

Then confirm:

```
AskUserQuestion(
  header: "Pool Confirmation",
  question: "Proceed with this ontology pool configuration?",
  multiSelect: false,
  options: [
    {label: "Confirm — proceed", description: "Start {CALLER_CONTEXT} with this configuration"},
    {label: "Reselect documents", description: "Go back to MCP document selection (Screen 1)"},
    {label: "Reselect data sources", description: "Go back to data source selection (Screen 2)"},  // ONLY show if Screen 2 was presented (MCP data sources were discovered)
    {label: "Add sources", description: "Go back to external source addition (Screen 3)"},
    {label: "Cancel", description: "Proceed without ontology pool"}
  ]
)
```

| Selection | Action |
|-----------|--------|
| Confirm — proceed | Proceed to Step 6 |
| Reselect documents | Return to Step 2 (Screen 1) |
| Reselect data sources | Return to Step 3 (Screen 2). **Only show this option if Screen 2 was presented (MCP data sources were discovered).** |
| Add sources | Return to Step 4 (Screen 3) |
| Cancel + `{AVAILABILITY_MODE}`=`required` | Warn: "Ontology pool is required. Add at least one source." Return to Step 4 (Screen 3) to add external sources. If no sources added → return to Step 2 (Screen 1). Maximum 2 cancel-retry cycles — after 2nd cancel without adding any source, error: "Cannot proceed without at least one ontology source in required mode." **STOP.** |
| Cancel + `{AVAILABILITY_MODE}`=`optional` | Warn: "Proceeding without ontology pool." Pool Catalog is empty. Proceed |

### Step 6: Build Final Pool Catalog

Combine all sources into a unified catalog:

```
## Ontology Pool Catalog

| # | Source | Type | Path/URL | Domain | Summary | Key Topics | Status |
|---|--------|------|----------|--------|---------|------------|--------|
| 1 | mcp | doc | {dir}/ | {domain} | {1-2 line summary} | {3-5 keywords} | available |
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

For **document sources** (mcp doc):
```
- doc: {path}: {domain} — {summary}
  Access: Use mcp__ontology-docs__ tools (search_files, read_file, read_text_file) to explore.
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

Check: Did each analyst find relevant evidence in the pool documents?
Check: Are there documents or sections that no analyst explored?
Check: Did analysts effectively query the available MCP data sources?
Check: Are there MCP data sources that could have provided additional evidence but were not queried?
Check: Did analysts reference relevant web sources from the pool?
Check: Did analysts reference relevant file sources from the pool?
```

---

## Exit Gate

- [ ] Allowed roots discovered via `list_allowed_directories` (or `ONTOLOGY_AVAILABLE=false`)
- [ ] Top-level entries classified via 3-category heuristic (KEEP/SKIP/UNKNOWN)
- [ ] UNKNOWN entries probed at depth-2; doc roots recorded at most-specific matching path
- [ ] All KEEP entries characterized with domain, summary, and key topics
- [ ] MCP call count stayed within 100-call ceiling
- [ ] `search_files` only used as last-resort fallback (1F), not as primary discovery
- [ ] User selected MCP documents via Screen 1 (or skipped if MCP unavailable)
- [ ] MCP data sources discovered via `ToolSearch` and presented to user via Screen 2 (or none available)
- [ ] Selected MCP servers recorded with full tool lists and capability summaries
- [ ] External sources collected via Screen 3 (or explicitly skipped)
- [ ] Pool configuration confirmed via Screen 4
- [ ] Pool Catalog generated with Source, Type, and Status for every entry
- [ ] No MCP document sources included from outside ontology-docs allowed paths (web, file, and MCP data sources are exempt)
- [ ] Web source summaries cached for analyst prompt use
- [ ] File source summaries cached for analyst prompt use
- [ ] MCP data source access instructions generated with tool loading steps (`ToolSearch` → direct call)
- [ ] Full-pool `{ONTOLOGY_SCOPE}` block generated for all analysts with correct access instructions per source type
- [ ] DA full-scope block generated with verification mission (including MCP data source utilization check)
