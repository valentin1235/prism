# Ontology Scope Mapping

## Table of Contents

- [Parameters](#parameters)
- [Phase A: Build Ontology Pool](#phase-a-build-ontology-pool)
  - [Pool Source Rules](#pool-source-rules)
  - [Step 1: Discover and Characterize](#step-1-discover-and-characterize-document-sources)
  - [Step 2: Screen 1 — MCP Document Selection](#step-2-screen-1--mcp-document-selection)
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
- ONLY documents registered in the `ontology-docs` global MCP server can enter the pool
- `mcp__ontology-docs__directory_tree` only returns paths within allowed directories — no additional verification needed
- Documents outside allowed paths MUST NOT be included

#### Web Source
- ONLY user-provided URLs (collected in Screen 2) can enter the pool
- Each URL is fetched and summarized at pool build time
- URLs that fail to fetch are marked as `unavailable` in the catalog

#### File Source
- ONLY user-provided file paths (collected in Screen 2) can enter the pool
- Each file is read and summarized at pool build time
- Files that fail to read are marked as `unavailable` in the catalog

### Step 1: Discover and Characterize Document Sources

Call `mcp__ontology-docs__directory_tree` on root to discover top-level structure.

| Result | {AVAILABILITY_MODE}=optional | {AVAILABILITY_MODE}=required |
|--------|------------------------------|------------------------------|
| Success | `ONTOLOGY_AVAILABLE=true`. Proceed to characterize. | Proceed to characterize. |
| Error / MCP not configured | `ONTOLOGY_AVAILABLE=false`. Warn: "ontology-docs MCP not configured. Pool will contain external sources only." Skip to Step 3. | Error: "ontology-docs MCP not configured. See plugin README for setup." **STOP.** |

For each top-level directory discovered:
1. Attempt to read `{dir}/README.md` via `mcp__ontology-docs__read_text_file`
2. If not found, attempt `{dir}/CLAUDE.md`
3. If neither exists, use `mcp__ontology-docs__list_directory` to inspect file listing and infer the domain

Record per directory: path, domain, summary (1-2 lines), key topics (3-5 keywords).

Store as `DISCOVERED_ENTRIES[]`.

### Step 2: Screen 1 — MCP Document Selection

If `ONTOLOGY_AVAILABLE=false` → skip to Step 3 with warning: "No MCP documents available. Only external sources can be added."

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

### Step 3: Screen 2 — External Source Addition

```
AskUserQuestion(
  header: "External Sources",
  question: "Any external sources to include for {CALLER_CONTEXT}?",
  multiSelect: false,
  options: [
    {label: "Add URL", description: "Web docs, API docs, blog posts, etc."},
    {label: "Add file path", description: "Local file or directory path"},
    {label: "None — proceed", description: "Proceed with selected ontology documents only"}
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
| 1 | mcp    | doc  | ...      | ...    | ...     | available |
| 2 | web    | url  | ...      | ...    | ...     | available |
| 3 | file   | file | ...      | ...    | ...     | available |
Total N sources (MCP: n, Web: n, File: n)
```

Then confirm:

```
AskUserQuestion(
  header: "Pool Confirmation",
  question: "Proceed with this ontology pool configuration?",
  multiSelect: false,
  options: [
    {label: "Confirm — proceed", description: "Start {CALLER_CONTEXT} with this configuration"},
    {label: "Reselect documents", description: "Go back to MCP document selection"},
    {label: "Add sources", description: "Go back to external source addition"},
    {label: "Cancel", description: "Proceed without ontology pool"}
  ]
)
```

| Selection | Action |
|-----------|--------|
| Confirm — proceed | Proceed to Step 5 |
| Reselect documents | Return to Step 2 (Screen 1) |
| Add sources | Return to Step 3 (Screen 2) |
| Cancel + `{AVAILABILITY_MODE}`=`required` | Warn: "Ontology pool is required. Add at least one source." Return to Step 3 (Screen 2) to add external sources. If no sources added → return to Step 2 (Screen 1). |
| Cancel + `{AVAILABILITY_MODE}`=`optional` | Warn: "Proceeding without ontology pool." Pool Catalog is empty. Proceed |

### Step 5: Build Final Pool Catalog

Combine all sources into a unified catalog:

```
## Ontology Pool Catalog

| # | Source | Type | Path/URL | Domain | Summary | Key Topics | Status |
|---|--------|------|----------|--------|---------|------------|--------|
| 1 | mcp | doc | {dir}/ | {domain} | {1-2 line summary} | {3-5 keywords} | available |
| 2 | web | url | {url} | {domain} | {1-2 line summary} | {3-5 keywords} | available |
| 3 | file | file | {path} | {domain} | {1-2 line summary} | {3-5 keywords} | available |
| 4 | web | url | {url} | — | — | — | unavailable: {reason} |
```

If catalog is empty after all steps:
- `{AVAILABILITY_MODE}`=`optional` → Pool Catalog is empty. Warn and proceed.
- `{AVAILABILITY_MODE}`=`required` → Error: "No ontology sources available." **STOP.**

**Phase A output:** Pool Catalog (unified table of available document, web, and file sources).

If Pool Catalog is empty and `{AVAILABILITY_MODE}`=`optional` → analysts get `{ONTOLOGY_SCOPE}` = "N/A — no ontology sources available". Skip to Exit Gate.

---

## Phase B: Generate Scoped References

All perspectives (analysts) receive the **full pool** — no per-perspective filtering.

**Note:** The lead MUST generate two `{ONTOLOGY_SCOPE}` variants from this phase — one for analysts (with access instructions) and one for the DA (with verification mission). Inject the correct variant per role at spawn time.

### For all analysts:

Generate `{ONTOLOGY_SCOPE}` block containing all available pool entries:

For **document sources** (mcp):
```
- doc: {path}: {domain} — {summary}
  Access: Use mcp__ontology-docs__ tools (search_files, read_file, read_text_file) to explore.
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
Your reference documents:
{list of ALL available pool entries with access instructions}

Explore these documents through your perspective's lens.
Cite findings as "source:section" (doc sources) or "url:section" (web sources).
```

### For Devil's Advocate:
```
You have access to ALL pool entries. Verify analysts explored thoroughly.

Full Ontology Pool:
{complete pool catalog table — available entries only}

Check: Did each analyst find relevant evidence in the pool documents?
Check: Are there documents or sections that no analyst explored?
```

---

## Exit Gate

- [ ] All ontology-docs MCP directories discovered and characterized (or `ONTOLOGY_AVAILABLE=false`)
- [ ] User selected MCP documents via Screen 1 (or skipped if MCP unavailable)
- [ ] External sources collected via Screen 2 (or explicitly skipped)
- [ ] Pool configuration confirmed via Screen 3
- [ ] Pool Catalog generated with Source, Type, and Status for every entry
- [ ] No MCP document sources included from outside ontology-docs allowed paths (web and file sources are exempt)
- [ ] Web source summaries cached for analyst prompt use
- [ ] File source summaries cached for analyst prompt use
- [ ] Full-pool `{ONTOLOGY_SCOPE}` block generated for all analysts with correct access instructions per source type
- [ ] DA full-scope block generated with verification mission
