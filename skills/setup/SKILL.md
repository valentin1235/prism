---
name: setup
description: Scan brownfield repositories and configure defaults for prism skills
version: 2.0.0
user-invocable: true
allowed-tools: Bash, Write, AskUserQuestion, ToolSearch, mcp__prism__prism_brownfield, mcp__plugin_prism_prism__prism_brownfield
---

# Prism Setup

Scan and register brownfield repositories so prism skills (analyze, incident, prd) can use existing codebase context during analysis.

> **Note**: prism is now a built-in MCP server bundled with this plugin. No separate binary installation or MCP registration is needed.

## Step 1: Brownfield Repository Scan

Scan the user's home directory for existing git repositories and register them in the prism DB. This enables interviews to use brownfield context for existing projects.

**Show scanning indicator:**
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

**Display the repos in a plain-text 2-column grid** (NOT a markdown table). Use a code block so columns align. Example:

```
Scan complete. 8 repositories registered.

 1. repo-alpha                   5. repo-epsilon
 2. repo-bravo *                 6. repo-foxtrot
 3. repo-charlie                 7. repo-golf *
 4. repo-delta                   8. repo-hotel
```

Include `*` markers for defaults exactly as they appear in the scan response. Do not summarize or truncate the list. The user needs to see all repo numbers to pick defaults.

**If no repos found**, skip the default selection prompt and proceed to Step 2.

**Default repo selection — IMMEDIATELY after showing the list:**

Use `AskUserQuestion` with the current default numbers from the scan response.

**If defaults exist**, show them as the recommended option:

```json
{
  "questions": [{
    "question": "Which repos to set as default for interviews? Enter numbers like '6, 18, 19'.",
    "header": "Default Repos",
    "options": [
      {"label": "<current default numbers> (Recommended)", "description": "<current default names>"},
      {"label": "None", "description": "Clear all defaults — interviews will run in greenfield mode"},
      {"label": "Select repos", "description": "Type repo numbers to set as default"}
    ],
    "multiSelect": false
  }]
}
```

**If no defaults exist**, do NOT show a "(Recommended)" option — offer "None" and "Select repos" instead:

```json
{
  "questions": [{
    "question": "Which repos to set as default for interviews? Enter numbers like '6, 18, 19'.",
    "header": "Default Repos",
    "options": [
      {"label": "None", "description": "No default repos — interviews will run in greenfield mode"},
      {"label": "Select repos", "description": "Type repo numbers to set as default"}
    ],
    "multiSelect": false
  }]
}
```

The user can select the recommended defaults (if any), choose "None", or type custom numbers.

After the user responds, use ONE MCP call to update all defaults at once:

```
Tool: prism_brownfield
Arguments: { "action": "set_defaults", "indices": "<comma-separated IDs>" }
```

Example: if the user picks IDs 6, 18, 19 → `{ "action": "set_defaults", "indices": "6,18,19" }`

This clears all existing defaults and sets the selected repos as default in one call.

If "None" → `{ "action": "set_defaults", "indices": "" }` to clear all defaults.

**Confirmation:**
```
Brownfield defaults updated!
Defaults: podo-app, podo-backend, grape

These repos will be used as context in interviews.
```

Or if "None" selected:
```
No default repos set. Interviews will run in greenfield mode.
You can set defaults anytime with: /prism:brownfield
```

## Step 2: Verify

Confirm to the user:
- Brownfield repositories scanned and defaults configured (if any)
- prism is bundled as a built-in MCP server — no restart needed
