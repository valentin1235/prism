---
name: prism-setup
description: Run Prism setup from Codex through the shared brownfield scan and default-selection workflow.
version: 2.0.0
---

# psm setup

Run Prism's setup flow from Codex through the shared Codex `psm` integration framework.

## Usage

```text
psm setup
psm setup scan
psm setup defaults
psm setup set 6,18,19
```

## Shared Codex Dispatch

Treat `psm setup` as a command, not as natural language.

1. Resolve the shared Prism asset root deterministically:
   `PRISM_REPO_PATH` when it points to a Prism repo containing `skills/setup/SKILL.md`; otherwise fall back to `Glob(pattern="**/skills/setup/SKILL.md")`.
2. Read the resolved shared Prism setup skill.
3. Follow that shared skill exactly.

## Codex Normalization Rules

- Normalize the command prefix from the Claude Code form to Codex where referenced:
  `/prism:setup` -> `psm setup`
  `/prism:brownfield` -> `psm brownfield`
- Reuse Prism's bundled MCP tools and shared skill content exactly.
- When the shared setup skill configures Prism runtime, `psm setup` must run the shared `scripts/setup.sh --runtime codex` flow so the managed Codex installation artifacts and `~/.prism/config.yaml` stay aligned.
- Do not assume the command was launched from within `~/prism` or from the user's current working directory.
- Do not reimplement or paraphrase the Prism setup workflow in this Codex wrapper.
