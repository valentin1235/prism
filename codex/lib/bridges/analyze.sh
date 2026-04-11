#!/usr/bin/env bash

prism_psm_expand_path() {
  local path="${1:-}"

  case "${path}" in
    "~")
      printf '%s' "${HOME}"
      ;;
    "~/"*)
      printf '%s/%s' "${HOME}" "${path#~/}"
      ;;
    *)
      printf '%s' "${path}"
      ;;
  esac
}

prism_psm_absolutize_path() {
  local path="${1:-}"

  path="$(prism_psm_expand_path "${path}")"
  if [[ "${path}" = /* ]]; then
    printf '%s' "${path}"
    return 0
  fi

  local parent
  parent="$(dirname "${path}")"
  local base
  base="$(basename "${path}")"
  printf '%s/%s' "$(cd "${parent}" 2>/dev/null && pwd || printf '%s' "${parent}")" "${base}"
}

prism_psm_resolve_analyze_path() {
  local original_cwd="$1"
  local repo_root="$2"
  local raw_path="$3"
  local allow_repo_fallback="${4:-0}"

  raw_path="$(prism_psm_expand_path "${raw_path}")"
  if [[ "${raw_path}" = /* ]]; then
    printf '%s' "${raw_path}"
    return 0
  fi

  if [ -e "${original_cwd}/${raw_path}" ]; then
    printf '%s' "$(prism_psm_absolutize_path "${original_cwd}/${raw_path}")"
    return 0
  fi

  if [ "${allow_repo_fallback}" = "1" ] && [ -e "${repo_root}/${raw_path}" ]; then
    printf '%s' "$(prism_psm_absolutize_path "${repo_root}/${raw_path}")"
    return 0
  fi

  printf '%s' "$(prism_psm_absolutize_path "${original_cwd}/${raw_path}")"
}

prism_psm_normalize_analyze_config() {
  local config_path="$1"
  local original_cwd="$2"
  local repo_root="$3"
  local temp_path

  temp_path="$(mktemp "${TMPDIR:-/tmp}/psm-analyze-config.XXXXXX")"

  if ! python3 - "${config_path}" "${temp_path}" "${original_cwd}" "${repo_root}" <<'PY'
from __future__ import annotations

import json
import os
import sys

config_path, output_path, original_cwd, repo_root = sys.argv[1:]

with open(config_path, "r", encoding="utf-8") as fh:
    data = json.load(fh)

if not isinstance(data, dict):
    raise SystemExit("analyze config must be a JSON object")

def expand(raw: str) -> str:
    return os.path.expanduser(raw.strip())

def resolve(raw: str, allow_repo_fallback: bool) -> str:
    candidate = expand(raw)
    if os.path.isabs(candidate):
        return candidate

    original = os.path.abspath(os.path.join(original_cwd, candidate))
    if os.path.exists(original):
        return original

    if allow_repo_fallback:
        repo_candidate = os.path.abspath(os.path.join(repo_root, candidate))
        if os.path.exists(repo_candidate):
            return repo_candidate

    return original

for key, allow_repo_fallback in {
    "input_context": False,
    "report_template": True,
    "perspective_injection": True,
}.items():
    value = data.get(key)
    if isinstance(value, str) and value.strip():
        data[key] = resolve(value, allow_repo_fallback)

with open(output_path, "w", encoding="utf-8") as fh:
    json.dump(data, fh, indent=2)
    fh.write("\n")
PY
  then
    rm -f "${temp_path}"
    prism_psm_die "failed to normalize analyze config at ${config_path}."
  fi

  printf '%s' "${temp_path}"
}

prism_psm_prepare_analyze_args() {
  local out_args_name="$1"
  local out_cleanup_name="$2"
  local original_cwd="$3"
  local repo_root="$4"
  shift 4

  local analyze_prepared_args=()
  local analyze_cleanup_paths=()

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --config)
        if [ "$#" -lt 2 ]; then
          prism_psm_die "psm analyze requires a path after --config."
        fi
        local resolved_config
        resolved_config="$(prism_psm_resolve_analyze_path "${original_cwd}" "${repo_root}" "$2" 1)"
        local normalized_config
        normalized_config="$(prism_psm_normalize_analyze_config "${resolved_config}" "${original_cwd}" "${repo_root}")"
        analyze_prepared_args+=("--config" "${normalized_config}")
        analyze_cleanup_paths+=("${normalized_config}")
        shift 2
        ;;
      *)
        analyze_prepared_args+=("$1")
        shift
        ;;
    esac
  done

  if [ "${#analyze_prepared_args[@]}" -gt 0 ]; then
    prism_psm_assign_array "${out_args_name}" "${analyze_prepared_args[@]}"
  else
    prism_psm_assign_array "${out_args_name}"
  fi

  if [ "${#analyze_cleanup_paths[@]}" -gt 0 ]; then
    prism_psm_assign_array "${out_cleanup_name}" "${analyze_cleanup_paths[@]}"
  else
    prism_psm_assign_array "${out_cleanup_name}"
  fi
}

prism_psm_analyze_bridge_prompt() {
  cat <<'EOF'
## Prism Analyze Compatibility Bridge

Follow the shared Prism analyze skill as the source of truth, but apply this Codex adapter contract while doing so:

- Treat `psm analyze ...` as the exact Codex equivalent of Claude Code `/prism:analyze ...`.
- Preserve the full shared-skill decision flow and exit gates, not just the MCP payload shape.
- If the command includes `--config`, the `psm` wrapper may substitute an adapter-generated temporary config path. Treat that file as a compatible copy of the user config with only path normalization applied.
- Preserve the shared analyze config schema and MCP payload contract exactly. Do not rename, drop, or reinterpret fields such as `topic`, `input_context`, `report_template`, `seed_hints`, `session_id`, `model`, `ontology_scope`, or `perspective_injection`.
- Because this bridge is Codex-specific, always include `adaptor: "codex"` when calling `prism_analyze`. Do not rely on server-side runtime inference when the bridge already knows it is running under Codex.
- Path-valued analyze config fields have already been normalized for Codex execution context. Pass them through unchanged once read.
- Use Codex-native equivalents for Claude Code tool names mentioned by the shared skill:
  Read -> inspect files directly
  ToolSearch -> use local search tools such as glob and ripgrep
  AskUserQuestion -> ask the user directly in chat
  mcp__prism__* -> call the same Prism MCP tools unchanged
- Execute the shared intake branch exactly:
  if `--config <path>` is present, read that config and use `config.topic` as the description when present, otherwise fall back to remaining arguments;
  if no config is present, use the remaining command arguments as the description;
  if the description is still empty, ask the user directly for what to analyze.
- Honor the shared Phase 1 exit gate before starting analysis: do not call `prism_analyze` until the description has been collected.
- When calling `prism_analyze`, preserve the shared optional-field behavior exactly:
  omit optional MCP fields when the shared skill says to omit them rather than inventing defaults in the wrapper layer.
- Honor the shared Phase 2 exit gate: do not proceed to polling until `prism_analyze` returns a `task_id`.
- During Phase 3, poll `prism_task_status` every 30 seconds until the task reaches `completed` or `failed`, and surface brief progress updates that include the current stage and progress text.
- Preserve the shared polling branches exactly:
  if the user cancels during polling, call `prism_cancel_task(task_id)` and report the cancellation result;
  if the task status is `failed`, report the error and stop without calling `prism_analyze_result`;
  only continue to result retrieval after a `completed` status.
- Preserve the shared output contract: poll progress via `prism_task_status`, then present the `summary` and `report_path` returned by `prism_analyze_result`.
- Honor the shared Phase 4 exit gate: after completion, call `prism_analyze_result(task_id)`, present the returned `summary`, and communicate the returned `report_path`.
EOF
}
