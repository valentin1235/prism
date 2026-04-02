---
name: prism-analyze
description: Runs multi-perspective analysis via MCP server orchestration. Thin Codex wrapper over the shared Prism analyze skill.
version: 7.1.0
---

# psm analyze

Run Prism's general-purpose multi-perspective analysis flow from Codex through the shared Codex `psm` integration framework.

## Usage

```text
psm analyze
psm analyze <topic>
psm analyze --config /path/to/config.json
```

## Shared Codex Dispatch

Treat `psm analyze` as a command, not as natural language.

1. Use `Glob(pattern="**/skills/analyze/SKILL.md")` to locate the shared Prism analyze skill.
2. Treat `PRISM_REPO_PATH/skills/analyze/SKILL.md` as the canonical shared skill path when it is available.
3. If multiple matches exist, prefer the Prism-owned path under `PRISM_REPO_PATH` over matches from the user's target repository or working directory.
4. Read the first match.
5. Follow that shared skill exactly.

## Codex Normalization Rules

- Normalize the command prefix from the Claude Code form to Codex:
  `/prism:analyze` -> `psm analyze`
- Normalize shared-skill references accordingly:
  `Use \`prism:brownfield\`` -> `Use \`psm brownfield\``
  `Wrapper skills (e.g., \`/prd\`)` -> `Wrapper skills (e.g., \`psm prd\`)`
- Treat the installed Prism MCP server plus `PRISM_REPO_PATH` as the source of truth for shared analyze assets.
- Resolve the shared Prism asset root deterministically in this order:
  `PRISM_REPO_PATH` when it points to a Prism repo containing the required shared analyze assets.
  The installed `repo-root` pointer shipped with the shared `psm` integration layer.
  A Prism repo root inferred relative to the shared `psm` library.
- Never resolve shared Prism analyze assets from the user's working directory.
- Reuse these shared analyze assets directly from the Prism repository whenever Codex can consume them without translation:
  `PRISM_REPO_PATH/skills/analyze/SKILL.md`
  `PRISM_REPO_PATH/skills/analyze/prompts/seed-analyst.md`
  `PRISM_REPO_PATH/skills/analyze/prompts/perspective-generator.md`
  `PRISM_REPO_PATH/skills/analyze/prompts/finding-protocol.md`
  `PRISM_REPO_PATH/skills/analyze/prompts/verification-protocol.md`
  `PRISM_REPO_PATH/skills/analyze/templates/report.md`
- Pass shared analyze config paths such as `report_template` through unchanged when they already point at Prism assets.
- Do not reimplement or paraphrase the Prism analyze workflow in this Codex wrapper.
