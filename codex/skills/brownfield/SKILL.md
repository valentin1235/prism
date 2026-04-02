---
name: prism-brownfield
description: Scan and manage brownfield repository defaults for interviews through the shared Prism brownfield skill.
---

# psm brownfield

Run Prism's brownfield repository scan and default-management workflow from Codex through the shared Codex `psm` integration framework.

## Usage

```text
psm brownfield
psm brownfield scan
psm brownfield defaults
psm brownfield set 6,18,19
```

## Shared Codex Dispatch

Treat `psm brownfield` as a command, not as natural language.

1. Use `Glob(pattern="**/skills/brownfield/SKILL.md")` to locate the shared Prism brownfield skill.
2. Read the first match.
3. Follow that shared skill exactly.

## Codex Normalization Rules

- Normalize the command prefix from the Claude Code form to Codex:
  `/prism:brownfield` -> `psm brownfield`
- Reuse Prism's bundled MCP tools and shared skill assets.
- Do not reimplement the workflow locally in this Codex wrapper.
