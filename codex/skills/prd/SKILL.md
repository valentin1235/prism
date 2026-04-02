---
name: prism-prd
description: PRD policy conflict analysis for Codex. Thin wrapper over the shared Prism PRD skill with Codex command normalization.
version: 1.0.0
---

# psm prd

Run Prism's PRD policy analysis flow from Codex through the shared Codex `psm` integration framework.

## Usage

```text
psm prd /path/to/prd.md
```

## Shared Codex Dispatch

Treat `psm prd` as a command, not as natural language.

1. Resolve the shared Prism asset root deterministically:
   `PRISM_REPO_PATH` when it points to a Prism repo containing `skills/prd/SKILL.md`; otherwise fall back to `Glob(pattern="**/skills/prd/SKILL.md")`.
2. Read the resolved shared Prism PRD skill.
3. Follow that shared skill exactly.
4. Preserve the shared PRD wrapper phases exactly: PRD path intake, shared session/state setup, analyze config creation, analyze delegation, post-processing, report verification, and final report delivery.

## Codex Normalization Rules

- Normalize the command prefix from the Claude Code form to Codex:
  `/prism:prd` -> `psm prd`
- Normalize analyze delegation accordingly:
  `Skill(skill="prism:analyze", args="...")` -> `psm analyze ...`
- Resolve the PRD input path from the user's launch directory, not from the shared Prism repo, and fail if the referenced file does not exist.
- Reuse the shared session id across `~/.prism/state/prd-{short-id}` and `~/.prism/state/analyze-{short-id}` exactly as the shared skill specifies.
- Preserve the shared analyze-config contract exactly, including `topic`, `input_context`, `seed_hints`, and `session_id`, and keep the generated config at `~/.prism/state/prd-{short-id}/analyze-config.json`.
- Preserve the shared analyze handoff format exactly: invoke analyze as `psm analyze --config ~/.prism/state/prd-{short-id}/analyze-config.json` and treat that config file as the handoff artifact consumed by downstream Prism workflows.
- Require the shared analyze output contract before post-processing: `~/.prism/state/analyze-{short-id}/analyst-findings.md` must exist, while a missing `verification-log.json` remains tolerated.
- When the shared PRD skill resolves files relative to its own `SKILL.md`, bind `SKILL_DIR` to `PRISM_REPO_PATH/skills/prd` and keep the post-processor prompt at `PRISM_REPO_PATH/skills/prd/prompts/post-processor.md`.
- Keep the PRD report template at `PRISM_REPO_PATH/skills/prd/templates/report.md` and the analyze handoff assets under `PRISM_REPO_PATH/skills/analyze/...`.
- Preserve the shared post-processing and output gates exactly: wait for the post-processor result, require `~/.prism/state/prd-{short-id}/prd-policy-review-report.md`, require the post-processor to return that same report file path as its handoff result, verify the report contains `PM Decision Checklist`, then copy the final report beside the PRD file as `{PRD_DIR}/prd-policy-review-report.md`.
- Preserve the final delivery format exactly: report the copied PRD-side path first, the state-directory report path second, and the raw analyze artifacts directory as `~/.prism/state/analyze-{short-id}/`.
- Reuse Prism's bundled MCP tools, prompts, and templates from the shared skill directory.
- Do not assume the command was launched from within `~/prism` or from the user's current working directory.
- Do not reimplement or paraphrase the Prism PRD workflow in this Codex wrapper.
