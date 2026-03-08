---
name: add-docs
description: Add documentation directories to ~/.prism/ontology-docs.json for prism_docs tools. Use when the user wants to add docs paths, register documentation directories, or configure ontology docs.
version: 1.0.0
user-invocable: true
allowed-tools: Bash, Read, Write, AskUserQuestion
---

# Add Documentation Directories

Add directory paths to `~/.prism/ontology-docs.json`. These directories are used by prism skills (incident, prd, plan) to reference project docs during analysis via `prism_docs_*` tools.

## Step 1: Load Existing Config

```bash
cat ~/.prism/ontology-docs.json 2>/dev/null || echo '{"directories":[]}'
```

Parse the current `directories` array.

## Step 2: Loop — Collect Paths

Repeat until user selects "Done":

```
AskUserQuestion(
  header: "Add Documentation Directory",
  question: "<current_list_or_empty>\nEnter the absolute path to a documentation directory:",
  options: [
    {label: "Enter path", description: "Type an absolute directory path"},
    {label: "Done", description: "Save and finish"}
  ]
)
```

- `<current_list_or_empty>`: If directories already exist, show `"Current directories:\n- /path/a\n- /path/b\n\n"`. If empty, omit.

**If user selects "Enter path"** (the response will contain the path string):
1. Extract the path from the user's input
2. Validate: `test -d '<path>' && echo OK || echo NOT_FOUND`
3. If not found → warn "Directory not found: <path>" and loop back to the same question
4. If already in list → warn "Already registered: <path>" and loop back
5. If valid and new → add to list, loop back to the same question (the updated current list will show in the next prompt)

**If user selects "Done"** → exit loop, proceed to Step 3.

**Important:** Do NOT add a separate confirmation question after adding a path. After validation, immediately loop back to the same AskUserQuestion with the updated list. This keeps the flow snappy — one question per iteration.

## Step 3: Save

Write the updated config to `~/.prism/ontology-docs.json`:

```json
{
  "directories": [
    "/path/to/dir-a",
    "/path/to/dir-b"
  ]
}
```

Confirm to the user: list of all registered directories. Tell them to restart Claude Code for prism-mcp to pick up changes.
