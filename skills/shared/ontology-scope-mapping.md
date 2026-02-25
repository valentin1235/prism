# Ontology Scope Mapping

Pre-determine which ontology docs each perspective should explore during analysis, preventing analysts from wasting time searching irrelevant documents.

## Parameters

| Placeholder | Description | Examples |
|-------------|-------------|---------|
| `{AVAILABILITY_MODE}` | Behavior when MCP is not configured | `optional` (warn and proceed) / `required` (error and stop) |
| `{UNMAPPED_POLICY}` | Whether unmapped perspectives are allowed | `allowed` (acceptable for non-ontology domains) / `forbidden` (perspective must be reassessed) |

---

## Step 1: Check Ontology Availability

Call `mcp__ontology-docs__directory_tree` on root to discover top-level structure.

| Result | {AVAILABILITY_MODE}=optional | {AVAILABILITY_MODE}=required |
|--------|------------------------------|------------------------------|
| Success | `ONTOLOGY_AVAILABLE=true`. Proceed. | Proceed. |
| Error / MCP not configured | `ONTOLOGY_AVAILABLE=false`. Warn: "ontology-docs MCP not configured. Analysis will proceed without reference docs." **Skip remaining steps.** | Error: "ontology-docs MCP not configured. See plugin README for setup." **STOP.** |

## Step 2: Characterize Ontologies

For each top-level directory:
1. Attempt to read `{dir}/README.md` via `mcp__ontology-docs__read_text_file`
2. If not found, attempt `{dir}/CLAUDE.md`
3. If neither exists, use `mcp__ontology-docs__list_directory` to inspect file listing and infer the domain

Build ontology catalog:

```
| # | Path | Domain | Summary | Key Topics |
|---|------|--------|---------|------------|
| 1 | {dir}/ | {domain} | {1-2 line summary} | {3-5 keywords} |
```

## Step 3: Map Perspectives to Ontologies

Match each locked perspective's scope against catalog entries:

```
| Perspective | Relevant Ontologies | Reasoning |
|-------------|-------------------|-----------|
| {perspective-name} | {catalog #1}, {catalog #3} | {why these docs are relevant} |
| {perspective-name} | (none) | {domain outside ontology coverage} |
```

Mapping rules:
- Map ONLY when perspective scope and ontology domain are directly related
- One ontology MAY be mapped to multiple perspectives
- Do NOT force-map irrelevant ontologies

**If `{UNMAPPED_POLICY}`=`allowed`**: Unmapped perspectives are acceptable (analysis domain outside ontology coverage).
**If `{UNMAPPED_POLICY}`=`forbidden`**: Unmapped perspectives are NOT allowed — analysis requires reference docs. Reassess the perspective if no relevant ontology exists.

## Step 4: Generate Scoped References

Generate per-perspective `{ONTOLOGY_SCOPE}` blocks:

### When ontologies are mapped:
```
Your assigned ontology docs for this perspective:
- {path}: {domain} — {summary}
- {path}: {domain} — {summary}

Use mcp__ontology-docs__ tools (search_files, read_file, read_text_file, etc.) to explore THESE docs first.
You MAY search other ontology docs if needed, but prioritize your assigned scope.
Cite findings as "filename:section".
```

### When no ontologies are mapped (allowed only):
```
No ontology docs mapped to this perspective.
You MAY search ontology-docs if you discover relevant documentation during analysis,
but this is not expected for your scope.
```

### For Devil's Advocate (always full scope):
```
You have access to ALL ontology docs. Verify analysts explored the right scope.

Available ontology docs:
{full catalog table}

Check: Did each analyst find relevant evidence in their assigned docs?
Check: Are there relevant docs in UNASSIGNED ontology areas that analysts missed?
```

## Exit Gate

- [ ] Catalog complete (domain/summary confirmed for each top-level directory)
- [ ] Mapping decision made for every perspective (unmapped = intentional decision, per `{UNMAPPED_POLICY}`)
- [ ] Per-perspective `{ONTOLOGY_SCOPE}` blocks generated
- [ ] DA full-scope block generated
