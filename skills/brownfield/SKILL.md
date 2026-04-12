---
name: brownfield
description: "Scan and manage brownfield repository defaults for interviews"
version: 2.0.0
user-invocable: true
allowed-tools: ToolSearch, AskUserQuestion, mcp__prism__prism_brownfield, mcp__plugin_prism_prism__prism_brownfield
---

# /prism:brownfield

Scan your home directory for existing git repositories and manage default repos used as context in interviews.

In Codex, this same shared workflow is invoked through `psm brownfield`. The repo `skills/brownfield/SKILL.md` file remains the canonical source; any installed `~/.codex/skills/prism-brownfield` copy is just a managed mirror refreshed by setup.

## Usage

```
/prism:brownfield                # Scan repos and set defaults
/prism:brownfield scan           # Scan only (no default selection)
/prism:brownfield defaults       # Show current defaults
/prism:brownfield set 6,18,19   # Set defaults by repo numbers
```

**Trigger keywords:** "brownfield", "scan repos", "default repos", "brownfield scan"

---

## How It Works

### MCP Snapshot Ontology

When a brownfield scan refreshes the MCP snapshot, duplicate visible `/mcp` entries with the same server name must be resolved before SQLite insertion using this documented policy:

```yaml
name_collision_policy:
  id: prefer_approved_path_then_resolved_description_then_lexicographically_smallest_normalized_snapshot_fingerprint
  rule: prefer approved path, then resolved description, then lexicographically smallest normalized snapshot fingerprint
```

This survivor rule is authoritative for the scan workflow. It must not depend on SQLite primary-key conflict side effects or discovery insertion order.

### Default flow (no args)

**Step 1: Scan**

Show scanning indicator:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Scanning for Existing Projects...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Looking for git repositories in your home directory.
Only GitHub-hosted repos will be registered.
This may take a moment...
```

**Implementation — use MCP tools only, do NOT use CLI or Python scripts:**

1. Load the brownfield MCP tool: `ToolSearch query: "+prism brownfield"`
2. Call scan+register:
   ```
   Tool: prism_brownfield
   Arguments: { "action": "scan" }
   ```
   This scans `~/` for GitHub repos and registers them in DB. Existing defaults are preserved.

The scan response `text` already contains a pre-formatted numbered list with `[default]` markers. **Do NOT make any additional MCP calls to list or query repos.**

**Display the results in a plain-text 2-column grid** (NOT a markdown table). Use a code block so columns align. Repos always appear first, MCP servers after. Example:

```
Scan complete. 5 repositories, 3 MCP servers registered.

 1. (repo) repo-alpha *          5. (repo) repo-epsilon
 2. (repo) repo-bravo *          6. (mcp) plugin:ouroboros
 3. (repo) repo-charlie          7. (mcp) mcp-clickhouse
 4. (repo) repo-delta            8. (mcp) sentry
```

Include `*` markers for defaults exactly as they appear in the scan response.

**If no repos and no MCP servers found**, show:
```
No GitHub repositories or MCP servers found.
```
Then stop.

**Step 2: Default Selection**

**IMMEDIATELY after showing the list**, use `AskUserQuestion` with the current default numbers from the scan response:

```json
{
  "questions": [{
    "question": "Which repos to set as default for interviews? Enter numbers like '6, 18, 19'.",
    "header": "Default Repos",
    "options": [
      {"label": "<current default numbers> (Recommended)", "description": "<current default names>"},
      {"label": "None", "description": "No default repos — interviews will run in greenfield mode"}
    ],
    "multiSelect": false
  }]
}
```

The user can select the recommended defaults, choose "None", or type custom numbers.

After the user responds, use ONE MCP call to update all defaults at once:

```
Tool: prism_brownfield
Arguments: { "action": "set_defaults", "indices": "<comma-separated IDs>" }
```

Example: if the user picks IDs 6, 18, 19 → `{ "action": "set_defaults", "indices": "6,18,19" }`

This clears all existing defaults and sets the selected repos as default in one call.

If "None" → `{ "action": "set_defaults", "indices": "" }` to clear all defaults.

**Step 3: Confirmation**

```
Brownfield defaults updated!
Defaults: grape, podo-app, podo-backend

These repos will be used as context in interviews.
```

Or if "None" selected:
```
No default repos set. Interviews will run in greenfield mode.
You can set defaults anytime with: /prism:brownfield
```

---

### Subcommand: `scan`

Scan only, no default selection prompt. Show the numbered list and stop.

---

### Subcommand: `defaults`

Load the brownfield MCP tool and call:
```
Tool: prism_brownfield
Arguments: { "action": "scan" }
```

Display only the repos marked with `*` (defaults). If none, show:
```
No default repos set. Run '/prism:brownfield' to configure.
```

---

### Subcommand: `set <indices>`

Directly set defaults without scanning. Parse the comma-separated indices from the user's input and call:

```
Tool: prism_brownfield
Arguments: { "action": "set_defaults", "indices": "<indices>" }
```

Show confirmation with updated defaults.
