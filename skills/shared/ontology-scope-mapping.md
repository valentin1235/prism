# Ontology Scope Mapping

Orchestrate the ontology workflow: build a pool of reference documents, map them to perspectives, and generate scoped references for analysts.

## Parameters

| Placeholder | Description | Examples |
|-------------|-------------|---------|
| `{AVAILABILITY_MODE}` | Behavior when ontology-docs MCP is not configured | `optional` (warn and proceed) / `required` (error and stop) |
| `{UNMAPPED_POLICY}` | Whether unmapped perspectives are allowed | `allowed` (acceptable for non-ontology domains) / `forbidden` (perspective must be reassessed) |
| `{WEB_LINKS}` | User-provided URLs to include in pool | `["https://example.com/guide"]` or empty `[]` (default: `[]`) |

---

## Phase A: Build Ontology Pool

→ Read and execute `ontology-pool.md` with:
- `{AVAILABILITY_MODE}` = (pass through from caller)
- `{WEB_LINKS}` = (pass through from caller)

**Output:** Pool Catalog (unified table of available document and web sources).

If Pool Catalog is empty and `{AVAILABILITY_MODE}`=`optional` → analysts get `{ONTOLOGY_SCOPE}` = "N/A — no ontology sources available". Skip to Exit Gate.

## Phase B: Map Perspectives to Pool Entries

With the Pool Catalog from Phase A, evaluate each locked perspective against available pool entries.

### Step B.1: Select Relevant Entries

For each locked perspective, select relevant pool entries:

```
| Perspective | Selected Ontologies | Reasoning |
|-------------|---------------------|-----------|
| {perspective-name} | Pool #{1}, #{3} | {why these are relevant to this perspective's scope} |
| {perspective-name} | (none) | {domain outside pool coverage} |
```

Mapping rules:
- Map ONLY when perspective scope and pool entry domain are directly related
- One pool entry MAY be mapped to multiple perspectives
- Do NOT force-map irrelevant entries
- `unavailable` entries MUST NOT be mapped

**If `{UNMAPPED_POLICY}`=`allowed`**: Unmapped perspectives are acceptable (analysis domain outside pool coverage).
**If `{UNMAPPED_POLICY}`=`forbidden`**: Unmapped perspectives are NOT allowed — reassess the perspective if no relevant pool entry exists.

## Phase C: Generate Scoped References

Generate per-perspective `{ONTOLOGY_SCOPE}` blocks:

### When pool entries are mapped:

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

Combined per-perspective block:
```
Your assigned reference documents for this perspective:
{list of mapped entries with access instructions}

Analyze ONLY these documents through your perspective's lens.
Cite findings as "source:section" (doc sources) or "url:section" (web sources).
```

### When no entries are mapped (allowed only):
```
No reference documents mapped to this perspective.
Proceed with analysis based on available evidence and expertise.
```

### For Devil's Advocate (always full scope):
```
You have access to ALL pool entries. Verify analysts explored the right scope.

Full Ontology Pool:
{complete pool catalog table — available entries only}

Check: Did each analyst find relevant evidence in their assigned documents?
Check: Are there relevant documents in UNASSIGNED pool entries that analysts missed?
```

## Exit Gate

- [ ] Pool Catalog built (all MCP docs + web links characterized)
- [ ] Mapping decision made for every perspective (unmapped = intentional, per `{UNMAPPED_POLICY}`)
- [ ] Per-perspective `{ONTOLOGY_SCOPE}` blocks generated with correct access instructions per source type
- [ ] DA full-scope block generated with complete available pool
