---
name: remove-docs
description: Remove documentation directories from ~/.prism/ontology-docs.json. Use when the user wants to remove docs paths, unregister documentation directories, or clean up ontology docs config.
version: 1.0.0
user-invocable: true
allowed-tools: Bash, Read, Write, AskUserQuestion
---

# Remove Documentation Directories

Remove directory paths from `~/.prism/ontology-docs.json`.

## Step 1: Load Existing Config

```bash
cat ~/.prism/ontology-docs.json 2>/dev/null || echo '{"directories":[]}'
```

Parse the current `directories` array. If empty, inform the user "No directories configured" and stop.

## Step 2: Select Directories to Remove

Present all registered directories for multi-selection:

```
AskUserQuestion(
  header: "Remove Documentation Directories",
  question: "Select directories to remove:",
  multiSelect: true,
  options: [
    {label: "<path-1>"},
    {label: "<path-2>"},
    ...one per registered directory,
    {label: "Cancel", description: "Keep all directories"}
  ]
)
```

If "Cancel" or no selection → stop.

## Step 3: Save

Remove selected paths from the `directories` array. Write updated config to `~/.prism/ontology-docs.json`.

Confirm to the user: list of remaining directories (or "No directories remaining" if all removed). Tell them to restart Claude Code for prism-mcp to pick up changes.
