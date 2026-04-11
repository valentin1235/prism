#!/usr/bin/env bash

prism_psm_prd_bridge_prompt() {
  cat <<'EOF'
## Prism PRD Compatibility Bridge

Follow the shared Prism PRD skill as the source of truth, but apply this Codex adapter contract while doing so:

- Treat `psm prd ...` as the exact Codex equivalent of Claude Code `/prism:prd ...`.
- Resolve the shared PRD workflow entrypoint from `PRISM_REPO_PATH` first: `${PRISM_REPO_PATH}/skills/prd/SKILL.md`.
- The shared skill is the only workflow definition. Do not restate, paraphrase, or reorder its phases, exit gates, or analyze/post-processing contract in the Codex layer.
- Preserve the PRD input path semantics from the shared skill: resolve user-provided PRD paths from `PRISM_TARGET_CWD`, not from the shared Prism repo.
- When the shared PRD skill delegates to analyze, normalize only the runtime-specific handoff:
  `Skill(skill="prism:analyze", args="--config ...")` -> `psm analyze --config ...`
- Preserve the generated analyze config and report artifact paths exactly as written by the shared skill.
- Use Codex-native equivalents for Claude Code tool names mentioned by the shared skill:
  Read -> inspect files directly
  ToolSearch -> use local search tools such as glob and ripgrep
  AskUserQuestion -> ask the user directly in chat
  Skill/Task delegation -> keep the shared workflow semantics, but dispatch through `psm analyze` and Codex agents
  mcp__prism__* -> call the same Prism MCP tools unchanged when the shared workflow requires them
EOF
}
