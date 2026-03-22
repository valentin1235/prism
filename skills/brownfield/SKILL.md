---
name: brownfield
description: Scan home directory for GitHub repos, register them, and manage default selections for analysis context
version: 1.0.0
user-invocable: true
allowed-tools: AskUserQuestion, mcp__prism__prism_brownfield
---

# Brownfield Repository Registry

Scan the user's home directory for existing GitHub repositories, register them in the brownfield registry, and let the user select defaults for analysis context.

## Phase 1: Scan

1. Call `prism_brownfield` with `action: "scan"` to discover GitHub repos under `~/`.
2. Report scan results: `"Found {found} GitHub repositories, {registered} registered."`

## Phase 2: Display & Page Through Results

3. Call `prism_brownfield` with `action: "query"`, `offset: 0`, `limit: 30` to fetch the first page.
4. Present results in a formatted table via `AskUserQuestion`:

```
AskUserQuestion(
  header: "Brownfield Repositories ({offset+1}–{offset+count} of {total})",
  question: "<table>\n\nSelect repos to mark as default, or navigate pages:",
  options: [
    {label: "Select defaults", description: "Enter rowid numbers to set as defaults"},
    {label: "Next page", description: "Show next 30 repos"},
    {label: "Previous page", description: "Show previous 30 repos"},
    {label: "Done", description: "Finish browsing"}
  ]
)
```

**Table format** (include in the question text):
```
#    | ★ | Name              | Path                          | Description
-----|---|-------------------|-------------------------------|------------------
  6  |   | my-api            | /Users/me/my-api              | REST API for ...
 18  | ★ | frontend          | /Users/me/frontend            | React dashboard
 19  | ★ | infra             | /Users/me/infra               | Terraform configs
```

- `★` marks repos currently set as default.
- `#` is the rowid from the query result.

### Navigation Logic

- **"Select defaults"**: Ask the user for a comma-separated list of rowid numbers. Call `prism_brownfield` with `action: "set_defaults"`, `indices: "<user_input>"`. Then re-display the current page with updated defaults.
- **"Next page"**: Increment offset by 30, fetch and display next page. Hide if already at last page.
- **"Previous page"**: Decrement offset by 30, fetch and display previous page. Hide if offset is 0.
- **"Done"**: Exit the browsing loop.

## Phase 3: Summary

5. Call `prism_brownfield` with `action: "query"`, `default_only: true` to get final defaults.
6. Display summary:

```
Default repositories:
- {name} — {path} — {desc}
- ...

Use `/brownfield` again to rescan or change defaults.
```

## Edge Cases

- If scan finds 0 repos: inform user no GitHub repos found, suggest checking git remote configuration.
- If no defaults are set after "Done": warn that no default repos are selected.
- Description generation: when a repo is set as default and has no description, the MCP server auto-generates one from README/CLAUDE.md.
