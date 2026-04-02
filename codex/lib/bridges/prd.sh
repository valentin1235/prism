#!/usr/bin/env bash

prism_psm_prd_bridge_prompt() {
  cat <<'EOF'
## Prism PRD Compatibility Bridge

Follow the shared Prism PRD skill as the source of truth, but apply this Codex adapter contract while doing so:

- Treat `psm prd ...` as the exact Codex equivalent of Claude Code `/prism:prd ...`.
- Resolve the shared PRD workflow entrypoint from `PRISM_REPO_PATH` first: `${PRISM_REPO_PATH}/skills/prd/SKILL.md`.
- Preserve the full shared PRD decision flow and exit gates, not just the eventual analyze handoff or report artifact.
- Preserve the shared PRD intake behavior exactly:
  use the command argument as the PRD file path when provided, otherwise ask the user directly;
  verify that the PRD file exists before proceeding;
  if the PRD file path is relative, resolve it from `PRISM_TARGET_CWD`, not from the shared Prism repo.
- Preserve the shared session setup contract exactly:
  generate the short session id once and reuse it for both `prd-<short-id>` and `analyze-<short-id>` state directories under `~/.prism/state/`;
  determine `REPORT_LANGUAGE` from `CLAUDE.md` when available, otherwise infer it from the user's language in the session.
- Preserve the shared Phase 0 exit gate before analyze config creation: do not proceed until the PRD path is confirmed, the state directories exist, and `REPORT_LANGUAGE` is known.
- Preserve the shared config-generation behavior exactly:
  read the PRD and any relevant companion files in the same directory;
  write `~/.prism/state/prd-{short-id}/analyze-config.json` with the shared `topic`, `input_context`, `seed_hints`, and `session_id` contract;
  bind `SKILL_DIR` to `${PRISM_REPO_PATH}/skills/prd` so the shared post-processor prompt and report template resolve from the Prism asset root.
- When the shared PRD skill delegates to analyze, normalize the delegation exactly:
  `Skill(skill="prism:analyze", args="--config ...")` -> `psm analyze --config ...`;
  preserve the generated analyze config contents and pass shared asset paths through unchanged once written;
  keep `~/.prism/state/prd-{short-id}/analyze-config.json` as the concrete handoff artifact and do not substitute a different filename or directory.
- Preserve the shared analyze-output verification exactly:
  use the shared session id to locate `~/.prism/state/analyze-{short-id}`;
  require `analyst-findings.md` before post-processing;
  tolerate a missing `verification-log.json` because the shared post-processor defines fallback confidence handling.
- Preserve the shared post-processing behavior exactly:
  read `${PRISM_REPO_PATH}/skills/prd/prompts/post-processor.md`;
  keep `{REPORT_TEMPLATE_PATH}` bound to `${PRISM_REPO_PATH}/skills/prd/templates/report.md`;
  wait for the post-processor result instead of backgrounding it;
  require `~/.prism/state/prd-{short-id}/prd-policy-review-report.md` to exist after post-processing;
  require the post-processor handoff result to be that exact report path.
- Preserve the shared Phase 2 exit gate: verify the generated report contains `PM Decision Checklist` before presenting success.
- Preserve the shared delivery contract exactly:
  copy the final report into the PRD file's directory as `prd-policy-review-report.md`;
  report both the copied report path and `~/.prism/state/prd-{short-id}/prd-policy-review-report.md`;
  mention the raw analyze artifacts directory under `~/.prism/state/analyze-{short-id}/`;
  preserve that three-line handoff format because existing Prism workflows expect those locations and filenames.
- Use Codex-native equivalents for Claude Code tool names mentioned by the shared skill:
  Read -> inspect files directly
  ToolSearch -> use local search tools such as glob and ripgrep
  AskUserQuestion -> ask the user directly in chat
  Skill/Task delegation -> keep the shared workflow semantics, but dispatch through `psm analyze` and Codex agents
  mcp__prism__* -> call the same Prism MCP tools unchanged when the shared workflow requires them
EOF
}
