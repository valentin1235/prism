#!/usr/bin/env bash

prism_psm_prepare_incident_args() {
  local out_args_name="$1"
  local out_cleanup_name="$2"

  shift 4

  local incident_description="${*:-}"
  if [ -n "${incident_description}" ]; then
    prism_psm_assign_array "${out_args_name}" "${incident_description}"
  else
    prism_psm_assign_array "${out_args_name}"
  fi

  prism_psm_assign_array "${out_cleanup_name}"
}

prism_psm_incident_bridge_prompt() {
  cat <<'EOF'
## Prism Incident Compatibility Bridge

Follow the shared Prism incident skill as the source of truth, but apply this Codex adapter contract while doing so:

- Treat `psm incident ...` as the exact Codex equivalent of Claude Code `/prism:incident ...`.
- Resolve the shared incident workflow entrypoint from `PRISM_REPO_PATH` first: `${PRISM_REPO_PATH}/skills/incident/SKILL.md`.
- The shared skill is the only workflow definition. Do not restate, paraphrase, or reorder its phases, exit gates, or MCP contract in the Codex layer.
- When the shared skill asks `SELECT who you are: codex | claude`, choose `codex`, store it as `{ADAPTOR}`, and pass `adaptor: "{ADAPTOR}"` to `prism_analyze`.
- When the shared incident skill dispatches analysis, preserve its asset contract exactly by passing through the shared incident report template and UX perspective injection assets unchanged.
- Use Codex-native equivalents for Claude Code tool names mentioned by the shared skill:
  Read -> inspect files directly
  ToolSearch -> use local search tools such as glob and ripgrep
  AskUserQuestion -> ask the user directly in chat
  mcp__prism__* -> call the same Prism MCP tools unchanged
EOF
}
