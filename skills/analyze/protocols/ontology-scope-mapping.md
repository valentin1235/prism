# Ontology Scope Mapping

**Execution context:** Executed by the orchestrator. `{STATE_DIR}` determines where output files are written.

**Execution style:** Complete each step fully, then move to the next. Do ONE tool call at a time — do not plan ahead.

## Parameters

| Placeholder | Description | Examples |
|-------------|-------------|---------|
| `{AVAILABILITY_MODE}` | Behavior when document source is not configured | `optional` / `required` |
| `{CALLER_CONTEXT}` | Context label for prompts | `"analysis"` / `"PRD analysis"` |
| `{STATE_DIR}` | State directory path | `~/.prism/state/analyze-abc123/` |

---

## Step 1: Check Document Source Availability

First load the deferred tool: `ToolSearch(query="select:mcp__prism__prism_docs_roots")`. Then call `mcp__prism__prism_docs_roots`.

| Result | optional | required |
|--------|----------|----------|
| 1+ paths | `ONTOLOGY_AVAILABLE=true`. Record as `ONTOLOGY_DIRS[]`. | `ONTOLOGY_AVAILABLE=true`. Record as `ONTOLOGY_DIRS[]`. |
| 0 paths / error | `ONTOLOGY_AVAILABLE=false`. Warn, continue. | **STOP.** |

→ **NOW do Step 2a. Nothing else.**

---

## Step 2a: MCP Data Source Selection

Call `ToolSearch(query="mcp", max_results=200)`. Keep only tools matching `mcp__<server>__<tool>` pattern. Extract unique server names, **exclude**: `prism`, names containing `plugin_`, names starting with `__`.

If no servers found → skip to Step 2b.

Present servers via `AskUserQuestion`:

```
AskUserQuestion(
  header: "Live Data Sources",
  question: "Select live data sources for {CALLER_CONTEXT}. (multiple selection)\nEach item is an MCP server.",
  multiSelect: true,
  options: [
    {label: "{server_name}", description: "{tool_count} tools — {capability_keywords}"},
    {label: "Skip", description: "Proceed without MCP data sources"}
  ]
)
```

For selected servers: record server name, read-only tool list (filter out create/update/delete/patch/post tools), capability summary.

→ **NOW do Step 2b. Nothing else.**

---

## Step 2b: External Source Addition

Loop collecting URLs/file paths. Zero sources is valid.

```
AskUserQuestion(
  header: "External Sources",
  question: "{cumulative_list}Paste a URL or file path to add, or select Done.",
  multiSelect: false,
  options: [{label: "Done", description: "Proceed with {count} source(s)"}]
)
```

- **Done** → exit loop
- **URL** (http/https): validate via `WebFetch`, add if accessible
- **File path**: validate via `Read`, add if exists

→ **NOW do Step 3. Nothing else.**

---

## Step 3: Pool Confirmation

Display assembled catalog as a table, then confirm:

```
AskUserQuestion(
  header: "Pool Confirmation",
  question: "Proceed with this ontology pool configuration?",
  multiSelect: false,
  options: [
    {label: "Confirm", description: "Start {CALLER_CONTEXT}"},
    {label: "Reselect data sources", description: "Back to Step 2a"},
    {label: "Add sources", description: "Back to Step 2b"},
    {label: "Cancel", description: "Proceed without ontology pool"}
  ]
)
```

- **Confirm** → proceed to Step 4
- **Reselect** → return to Step 2a (only if MCP servers were discovered)
- **Add sources** → return to Step 2b
- **Cancel + required** → warn, return to Step 2b (max 2 retries, then STOP)
- **Cancel + optional** → proceed without pool

→ **NOW do Step 4. Read `protocols/ontology-scope-schema.md` for the JSON schema.**

---

## Step 4: Write `ontology-scope.json`

Read `protocols/ontology-scope-schema.md` (relative to SKILL.md) for the full JSON schema and field rules. Write the file to `{STATE_DIR}/ontology-scope.json`.

---

## Exit Gate

- [ ] Document source checked
- [ ] MCP data sources selected or skipped
- [ ] External sources collected or skipped
- [ ] Pool confirmed
- [ ] `{STATE_DIR}/ontology-scope.json` written (or skipped if empty pool + optional mode)
