# Ontology Pool

## Parameters

| Placeholder | Description | Examples |
|-------------|-------------|---------|
| `{AVAILABILITY_MODE}` | Behavior when ontology-docs MCP is not configured | `optional` (warn and proceed) / `required` (error and stop) |
| `{WEB_LINKS}` | User-provided URLs to include in pool | `["https://example.com/guide"]` or empty `[]` |

---

## Pool Source Rules

### Document Source
- ONLY documents registered in the `ontology-docs` global MCP server can enter the pool
- Verify allowed paths via `mcp__ontology-docs__list_allowed_directories`
- Documents outside allowed paths MUST NOT be included

### Web Source
- ONLY user-provided URLs (passed via `{WEB_LINKS}`) can enter the pool
- Each URL is fetched and summarized at pool build time
- URLs that fail to fetch are marked as `unavailable` in the catalog

---

## Step 1: Discover Document Sources

Call `mcp__ontology-docs__directory_tree` on root to discover top-level structure.

| Result | {AVAILABILITY_MODE}=optional | {AVAILABILITY_MODE}=required |
|--------|------------------------------|------------------------------|
| Success | `DOCS_AVAILABLE=true`. Proceed to Step 2. | Proceed to Step 2. |
| Error / MCP not configured | `DOCS_AVAILABLE=false`. Warn: "ontology-docs MCP not configured. Pool will contain web sources only." Skip to Step 3. | Error: "ontology-docs MCP not configured. See plugin README for setup." **STOP.** |

## Step 2: Characterize Document Sources

For each top-level directory discovered in Step 1:
1. Attempt to read `{dir}/README.md` via `mcp__ontology-docs__read_text_file`
2. If not found, attempt `{dir}/CLAUDE.md`
3. If neither exists, use `mcp__ontology-docs__list_directory` to inspect file listing and infer the domain

Record per directory: path, domain, summary (1-2 lines), key topics (3-5 keywords).

## Step 3: Fetch Web Sources

If `{WEB_LINKS}` is empty → skip to Step 4.

For each URL in `{WEB_LINKS}`:
1. Fetch content via `WebFetch`
2. Extract: title, domain/topic, summary (1-2 lines), key topics (3-5 keywords)
3. Cache the extracted summary for analyst prompt injection
4. If fetch fails → mark as `unavailable` with reason

## Step 4: Build Pool Catalog

Combine all sources into a unified catalog:

```
## Ontology Pool Catalog

| # | Source | Type | Path/URL | Domain | Summary | Key Topics | Status |
|---|--------|------|----------|--------|---------|------------|--------|
| 1 | mcp | doc | {dir}/ | {domain} | {1-2 line summary} | {3-5 keywords} | available |
| 2 | web | url | {url} | {domain} | {1-2 line summary} | {3-5 keywords} | available |
| 3 | web | url | {url} | — | — | — | unavailable: {reason} |
```

If both `DOCS_AVAILABLE=false` and no web links available:
- `{AVAILABILITY_MODE}`=`optional` → Pool Catalog is empty. Warn and proceed.
- `{AVAILABILITY_MODE}`=`required` → Error: "No ontology sources available." **STOP.**

## Exit Gate

- [ ] All ontology-docs MCP directories discovered and characterized (or `DOCS_AVAILABLE=false`)
- [ ] All web links fetched and characterized (or marked `unavailable`)
- [ ] Pool Catalog generated with Source, Type, and Status for every entry
- [ ] No sources included from outside ontology-docs MCP allowed paths
- [ ] Web source summaries cached for analyst prompt use
