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
- Preserve the full shared incident decision flow and exit gates, not just the eventual `prism_analyze` payload.
- Preserve the shared incident intake behavior exactly:
  collect the incident description from command arguments first, otherwise ask the user directly;
  if screenshot paths are referenced, verify and read them, then inline their contents into the incident description before analysis starts;
  determine the report language from `CLAUDE.md` when available, otherwise infer it from the user's input.
- Preserve the shared session setup contract: resolve the shared incident skill directory, generate the short session id, and create the corresponding `~/.prism/state/analyze-<short-id>/` workspace before calling MCP.
- When the shared incident skill dispatches analysis, preserve its asset contract exactly by passing through the shared incident report template and UX perspective injection assets unchanged.
- Use Codex-native equivalents for Claude Code tool names mentioned by the shared skill:
  Read -> inspect files directly
  ToolSearch -> use local search tools such as glob and ripgrep
  AskUserQuestion -> ask the user directly in chat
  mcp__prism__* -> call the same Prism MCP tools unchanged
- Honor the shared Phase 2 exit gate: do not proceed to polling until `prism_analyze` returns a `task_id`.
- During Phase 3, poll `prism_task_status` every 30 seconds until the task reaches `completed` or `failed`, surfacing brief progress updates with the current stage and progress text.
- Preserve the shared failure and cancellation branches exactly: if the user cancels, call `prism_cancel_task(task_id)`; if the task fails, report the error and stop without calling `prism_analyze_result`.
- Honor the shared Phase 4 exit gate: after completion, call `prism_analyze_result(task_id)`, present the returned `summary`, communicate the returned `report_path`, and mention the raw artifacts directory under `~/.prism/state/analyze-<short-id>/`.
EOF
}
