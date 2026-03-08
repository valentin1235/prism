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

Repeat until user says "Done":

1. Ask for a directory path:
   ```
   AskUserQuestion(
     header: "Add Documentation Directory",
     question: "Enter the absolute path to a documentation directory:"
   )
   ```

2. Validate the path exists:
   ```bash
   test -d '<path>' && echo OK || echo NOT_FOUND
   ```
   If not found, warn and ask again.

3. If the path is already in the list, warn "Already registered" and skip.

4. Add to list, then ask:
   ```
   AskUserQuestion(
     header: "Add More?",
     question: "Added: <path>\n\nCurrent list:\n<all paths>\n\nAdd another?",
     options: [
       {label: "Add another", description: "Add one more directory"},
       {label: "Done", description: "Save and finish"}
     ]
   )
   ```

5. If "Add another" → go to step 1. If "Done" → exit loop.

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
