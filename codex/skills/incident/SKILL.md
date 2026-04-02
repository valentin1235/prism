---
name: prism-incident
description: Run Prism incident RCA analysis through the shared incident skill, MCP flow, and report assets.
version: 2.1.0
---

# psm incident

Run Prism's incident root cause analysis flow from Codex through the shared Codex `psm` integration framework.

## Usage

```text
psm incident
psm incident <incident description>
```

## Shared Codex Dispatch

Treat `psm incident` as a command, not as natural language.

1. Use `Glob(pattern="**/skills/incident/SKILL.md")` to locate the shared Prism incident skill.
2. Treat `PRISM_REPO_PATH/skills/incident/SKILL.md` as the canonical shared skill path when it is available.
3. If multiple matches exist, prefer the Prism-owned path under `PRISM_REPO_PATH` over matches from the user's target repository or working directory.
4. Read the first match.
5. Follow that shared skill exactly.

## Codex Normalization Rules

- Normalize the command prefix from the Claude Code form to Codex:
  `/prism:incident` -> `psm incident`
- Normalize analyze delegation accordingly:
  any Codex-side analyze invocation must use `psm analyze`
- Treat the installed Prism MCP server plus `PRISM_REPO_PATH` as the source of truth for the shared incident workflow and assets.
- Resolve the shared Prism asset root deterministically in this order:
  `PRISM_REPO_PATH` when it points to a Prism repo containing `skills/incident/SKILL.md`.
  The installed `repo-root` pointer shipped with the shared `psm` integration layer.
  A Prism repo root inferred relative to the shared `psm` library.
- Never resolve shared incident skill assets from the user's working directory.
- Do not assume the command was launched from within `~/prism` or from the user's current working directory.
- Preserve the full shared incident workflow and exit gates, including:
  incident intake from command arguments or direct user prompt;
  screenshot-path verification plus inlined image-content extraction;
  report-language selection from `CLAUDE.md` when available, otherwise from user language;
  session-id generation and `~/.prism/state/analyze-<short-id>/` workspace creation before MCP execution;
  `prism_analyze` dispatch with the shared incident `report_template`, `perspective_injection`, and language contract;
  polling via `prism_task_status` until `completed` or `failed`;
  cancellation via `prism_cancel_task(task_id)`;
  final retrieval through `prism_analyze_result(task_id)` with summary, report path, and raw-artifact location.
- Reuse Prism's bundled MCP tools, prompts, perspectives, and templates from the shared skill directory.
- Reuse these shared incident assets directly from the Prism repository whenever Codex can consume them without translation:
  `PRISM_REPO_PATH/skills/incident/SKILL.md`
  `PRISM_REPO_PATH/skills/incident/templates/report.md`
  `PRISM_REPO_PATH/skills/incident/perspectives/ux-impact.json`
- Do not reimplement or paraphrase the Prism incident workflow in this Codex wrapper.
