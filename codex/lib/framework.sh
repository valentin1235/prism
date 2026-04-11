#!/usr/bin/env bash

set -euo pipefail

PRISM_PSM_FRAMEWORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

prism_psm_die() {
  printf 'psm: %s\n' "$1" >&2
  exit "${2:-1}"
}

prism_psm_resolve_script_dir() {
  local source_path="${BASH_SOURCE[0]}"
  while [ -L "${source_path}" ]; do
    local source_dir
    source_dir="$(cd "$(dirname "${source_path}")" && pwd)"
    source_path="$(readlink "${source_path}")"
    [[ "${source_path}" != /* ]] && source_path="${source_dir}/${source_path}"
  done

  cd "$(dirname "${source_path}")" && pwd
}

prism_psm_resolve_lib_dir() {
  prism_psm_resolve_script_dir
}

prism_psm_load_bridges() {
  local bridges_dir="${PRISM_PSM_FRAMEWORK_DIR}/bridges"

  if [ -d "${bridges_dir}" ]; then
    while IFS= read -r bridge_path; do
      # shellcheck source=/dev/null
      source "${bridge_path}"
    done < <(find "${bridges_dir}" -maxdepth 1 -type f -name '*.sh' | sort)
  fi
}

prism_psm_load_command_configs() {
  local commands_dir="${PRISM_PSM_FRAMEWORK_DIR}/commands"

  if [ -d "${commands_dir}" ]; then
    while IFS= read -r config_path; do
      # shellcheck source=/dev/null
      source "${config_path}"
    done < <(find "${commands_dir}" -maxdepth 1 -type f -name '*.sh' | sort)
  fi
}

prism_psm_command_config_var_name() {
  local command_name="${1:-}"
  local key="${2:-}"
  local normalized_key

  normalized_key="$(printf '%s' "${key}" | tr '[:lower:]-' '[:upper:]_')"
  printf 'PRISM_PSM_COMMAND_%s_%s' "$(printf '%s' "${command_name}" | tr '[:lower:]-' '[:upper:]_')" "${normalized_key}"
}

prism_psm_define_command_config() {
  local command_name="${1:-}"
  local key="${2:-}"
  local value="${3-}"
  local var_name

  [ -n "${command_name}" ] || prism_psm_die "cannot define a command config without a command name."
  [ -n "${key}" ] || prism_psm_die "cannot define a command config without a key."

  var_name="$(prism_psm_command_config_var_name "${command_name}" "${key}")"
  printf -v "${var_name}" '%s' "${value}"
}

prism_psm_command_config() {
  local command_name="${1:-}"
  local key="${2:-}"
  local var_name

  var_name="$(prism_psm_command_config_var_name "${command_name}" "${key}")"
  printf '%s' "${!var_name-}"
}

prism_psm_require_command_config() {
  local command_name="${1:-}"
  local key="${2:-}"
  local value

  value="$(prism_psm_command_config "${command_name}" "${key}")"
  if [ -z "${value}" ]; then
    prism_psm_die "missing '${key}' command config for 'psm ${command_name}'."
  fi

  printf '%s' "${value}"
}

prism_psm_registry_path() {
  local lib_dir
  lib_dir="$(prism_psm_resolve_lib_dir)"
  printf '%s/command-registry.tsv' "${lib_dir}"
}

prism_psm_registry_rows() {
  local registry_path
  registry_path="$(prism_psm_registry_path)"

  if [ ! -f "${registry_path}" ]; then
    prism_psm_die "command registry not found at ${registry_path}."
  fi

  grep -vE '^[[:space:]]*($|#)' "${registry_path}"
}

prism_psm_registry_lookup() {
  local command_name="${1:-}"

  while IFS=$'\t' read -r registered_command skill_dir installed_skill_id; do
    if [ "${registered_command}" = "${command_name}" ]; then
      printf '%s\t%s\t%s\n' "${registered_command}" "${skill_dir}" "${installed_skill_id}"
      return 0
    fi
  done < <(prism_psm_registry_rows)

  return 1
}

prism_psm_supported_commands() {
  while IFS=$'\t' read -r registered_command _; do
    printf '%s\n' "${registered_command}"
  done < <(prism_psm_registry_rows)
}

prism_psm_is_supported_command() {
  local command_name="${1:-}"

  prism_psm_registry_lookup "${command_name}" >/dev/null
}

prism_psm_command_skill_id() {
  local command_name="${1:-}"
  local registry_line

  registry_line="$(prism_psm_registry_lookup "${command_name}")" || return 1
  printf '%s' "${registry_line##*$'\t'}"
}

prism_psm_command_skill_dir() {
  local command_name="${1:-}"
  local registry_line
  local skill_dir

  registry_line="$(prism_psm_registry_lookup "${command_name}")" || return 1
  IFS=$'\t' read -r _ skill_dir _ <<<"${registry_line}"
  printf '%s' "${skill_dir}"
}

prism_psm_usage() {
  local command_name

  cat <<'EOF' >&2
Usage: psm <command> [arguments...]

Supported commands:
EOF

  while IFS= read -r command_name; do
    printf '  psm %s\n' "${command_name}" >&2
  done < <(prism_psm_supported_commands)
}

prism_psm_shell_escape() {
  printf '%q' "$1"
}

prism_psm_join_command() {
  local command_name="$1"
  shift || true

  local prompt="psm ${command_name}"
  local arg
  for arg in "$@"; do
    prompt+=" $(prism_psm_shell_escape "${arg}")"
  done

  printf '%s' "${prompt}"
}

prism_psm_join_command_readable() {
  local command_name="$1"
  shift || true

  local prompt="psm ${command_name}"
  local arg
  for arg in "$@"; do
    prompt+=" ${arg}"
  done

  printf '%s' "${prompt}"
}

prism_psm_assign_array() {
  local array_name="$1"
  shift || true

  if [ "$#" -eq 0 ]; then
    eval "${array_name}=()"
    return 0
  fi

  local quoted_items=()
  local item
  for item in "$@"; do
    quoted_items+=("$(printf '%q' "${item}")")
  done

  eval "${array_name}=(${quoted_items[*]})"
}

prism_psm_command_asset_paths() {
  local command_name="${1:-}"
  local asset_paths_function

  asset_paths_function="$(prism_psm_require_command_config "${command_name}" "asset_paths_function")"
  if ! declare -F "${asset_paths_function}" >/dev/null 2>&1; then
    prism_psm_die "command config for 'psm ${command_name}' references missing asset path function '${asset_paths_function}'."
  fi

  "${asset_paths_function}"
}

prism_psm_canonicalize_dir() {
  local candidate="${1:-}"
  [ -n "${candidate}" ] || return 1
  cd "${candidate}" >/dev/null 2>&1 && pwd
}

prism_psm_repo_root_has_required_assets() {
  local repo_root="${1:-}"
  local command_name="${2:-}"
  [ -n "${repo_root}" ] || return 1

  local relative_path
  while IFS= read -r relative_path; do
    [ -n "${relative_path}" ] || continue
    [ -f "${repo_root}/${relative_path}" ] || return 1
  done < <(prism_psm_command_asset_paths "${command_name}")

  if [ "${command_name}" = "analyze" ] || [ "${command_name}" = "incident" ] || [ "${command_name}" = "prd" ]; then
    [ -f "${repo_root}/agents/devils-advocate.md" ] || return 1
  fi

  return 0
}

prism_psm_resolve_repo_root_candidate() {
  local candidate="${1:-}"
  local command_name="${2:-}"

  candidate="$(prism_psm_canonicalize_dir "${candidate}")" || return 1
  prism_psm_repo_root_has_required_assets "${candidate}" "${command_name}" || return 1
  printf '%s' "${candidate}"
}

prism_psm_resolve_repo_root() {
  local command_name="${1:-}"
  local resolved_root
  local lib_dir
  local repo_root_hint
  local hinted_root
  local inferred_root
  local tried=()

  if [ -n "${PRISM_REPO_PATH:-}" ]; then
    if resolved_root="$(prism_psm_resolve_repo_root_candidate "${PRISM_REPO_PATH}" "${command_name}")"; then
      printf '%s' "${resolved_root}"
      return 0
    fi
    tried+=("PRISM_REPO_PATH=${PRISM_REPO_PATH}")
  fi

  lib_dir="$(prism_psm_resolve_lib_dir)"

  repo_root_hint="${lib_dir}/repo-root"
  if [ -f "${repo_root_hint}" ]; then
    hinted_root="$(<"${repo_root_hint}")"
    if resolved_root="$(prism_psm_resolve_repo_root_candidate "${hinted_root}" "${command_name}")"; then
      printf '%s' "${resolved_root}"
      return 0
    fi
    tried+=("${repo_root_hint}=${hinted_root}")
  fi

  inferred_root="$(cd "${lib_dir}/../.." && pwd)"
  if resolved_root="$(prism_psm_resolve_repo_root_candidate "${inferred_root}" "${command_name}")"; then
    printf '%s' "${resolved_root}"
    return 0
  fi
  tried+=("lib-relative=${inferred_root}")

  prism_psm_die "unable to locate the shared Prism asset root for 'psm ${command_name}'. Tried, in order: ${tried[*]}. Expected a Prism repo containing the required shared assets for that command."
}

prism_psm_resolve_codex_cli() {
  if [ -n "${PSM_CODEX_CLI_PATH:-}" ]; then
    printf '%s' "${PSM_CODEX_CLI_PATH}"
    return 0
  fi

  if command -v codex >/dev/null 2>&1; then
    command -v codex
    return 0
  fi

  prism_psm_die "Codex CLI was not found on PATH. Install Codex CLI or set PSM_CODEX_CLI_PATH."
}

prism_psm_build_prompt() {
  local original_cwd="$1"
  local command_name="$2"
  local skill_id="$3"
  shift 3
  local shared_skill_relative_path
  local shared_skill_path

  shared_skill_relative_path="$(prism_psm_require_command_config "${command_name}" "shared_skill_relative_path")"
  shared_skill_path="${PRISM_REPO_PATH}/${shared_skill_relative_path}"

  local prism_command
  local readable_prism_command
  prism_command="$(prism_psm_join_command "${command_name}" "$@")"
  readable_prism_command="$(prism_psm_join_command_readable "${command_name}" "$@")"

  cat <<EOF
Treat the following as an exact Prism command invocation and execute it via the installed Prism Codex skills and MCP server:

${readable_prism_command}

Exact shell-preserved Prism invocation:
${prism_command}

Registered Prism Codex skill:
${skill_id}

Prism setup may install managed mirror copies under ~/.codex/skills for Codex discovery, but the shared Prism repository is the authored source of truth:
${PRISM_REPO_PATH}

Deterministic Prism asset locator for the shared Prism command workflow:
1. Use PRISM_REPO_PATH when it points to a Prism repo containing ${shared_skill_relative_path}.
2. Otherwise use the installed repo-root pointer shipped with the shared psm integration layer.
3. Otherwise infer the Prism repo root relative to the shared psm library.
The resolved asset root for this invocation is:
${PRISM_REPO_PATH}
Do not resolve Prism assets from the user's working directory.
Do not resolve Prism workflow assets from ~/.codex/skills or from the user's working directory when the shared repo assets are available.

The canonical shared Prism skill for this command is:
${shared_skill_path}

Read and follow that shared Prism skill from the resolved Prism asset root. Treat any installed ~/.codex skill copy as a managed mirror, not as the authored source.

Shared analyze assets that remain available to Codex-side wrappers and adapters when this command delegates into analyze:
- ${PRISM_REPO_PATH}/skills/analyze/SKILL.md
- ${PRISM_REPO_PATH}/skills/analyze/prompts/seed-analyst.md
- ${PRISM_REPO_PATH}/skills/analyze/prompts/perspective-generator.md
- ${PRISM_REPO_PATH}/skills/analyze/prompts/finding-protocol.md
- ${PRISM_REPO_PATH}/skills/analyze/prompts/verification-protocol.md
- ${PRISM_REPO_PATH}/skills/analyze/templates/report.md

The user's original working directory is:
${original_cwd}

Use the original working directory as the project context for repo analysis and file operations. Use the Prism repository only for shared skill, prompt, template, and MCP assets.
EOF
}

prism_psm_command_contract() {
  local command_name="$1"
  local contract_function
  local prompt_function

  contract_function="$(prism_psm_require_command_config "${command_name}" "contract_function")"
  if ! declare -F "${contract_function}" >/dev/null 2>&1; then
    prism_psm_die "command config for 'psm ${command_name}' references missing contract function '${contract_function}'."
  fi
  printf '\n%s\n' "$("${contract_function}")"

  prompt_function="$(prism_psm_command_config "${command_name}" "prompt_function")"
  if [ -n "${prompt_function}" ]; then
    if ! declare -F "${prompt_function}" >/dev/null 2>&1; then
      prism_psm_die "command config for 'psm ${command_name}' references missing prompt function '${prompt_function}'."
    fi
    printf '\n%s\n' "$("${prompt_function}")"
  fi
}

prism_psm_render_markdown_numbered_lines() {
  local function_name="${1:-}"
  local index=1
  local line

  if ! declare -F "${function_name}" >/dev/null 2>&1; then
    prism_psm_die "missing markdown renderer function '${function_name}'."
  fi

  while IFS= read -r line; do
    [ -n "${line}" ] || continue
    if [[ "${line}" == "  "* ]]; then
      printf '   %s\n' "${line#  }"
      continue
    fi
    printf '%s. %s\n' "${index}" "${line}"
    index=$((index + 1))
  done < <("${function_name}")
}

prism_psm_render_markdown_bullets() {
  local function_name="${1:-}"
  local line

  if ! declare -F "${function_name}" >/dev/null 2>&1; then
    prism_psm_die "missing markdown renderer function '${function_name}'."
  fi

  while IFS= read -r line; do
    [ -n "${line}" ] || continue
    if [[ "${line}" == "  "* ]]; then
      printf '  %s\n' "${line#  }"
      continue
    fi
    printf -- '- %s\n' "${line}"
  done < <("${function_name}")
}

prism_psm_render_usage_block() {
  local function_name="${1:-}"
  local line

  if ! declare -F "${function_name}" >/dev/null 2>&1; then
    prism_psm_die "missing usage renderer function '${function_name}'."
  fi

  while IFS= read -r line; do
    [ -n "${line}" ] || continue
    printf '%s\n' "${line}"
  done < <("${function_name}")
}

prism_psm_resolve_skill_version() {
  local command_name="${1:-}"
  local repo_root
  local shared_skill_relative_path
  local shared_skill_path

  repo_root="$(prism_psm_resolve_repo_root "${command_name}")"
  shared_skill_relative_path="$(prism_psm_require_command_config "${command_name}" "shared_skill_relative_path")"
  shared_skill_path="${repo_root}/${shared_skill_relative_path}"

  if [ ! -f "${shared_skill_path}" ]; then
    return 0
  fi

  python3 - "${shared_skill_path}" <<'PY'
from pathlib import Path
import re
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
frontmatter = re.match(r"\A---\n(.*?)\n---\n", text, re.DOTALL)
if not frontmatter:
    raise SystemExit(0)

match = re.search(r"(?m)^version:\s*(.+?)\s*$", frontmatter.group(1))
if match:
    print(match.group(1).strip())
PY
}

prism_psm_render_codex_skill() {
  local command_name="${1:-}"
  local skill_id
  local skill_description
  local skill_version
  local usage_function
  local dispatch_function
  local normalization_function

  if skill_id="$(prism_psm_command_skill_id "${command_name}" 2>/dev/null)"; then
    :
  else
    # Rendering generated Codex skills should remain possible as long as the
    # command config exists, even if the registry lookup is unavailable.
    prism_psm_require_command_config "${command_name}" "shared_skill_relative_path" >/dev/null
    skill_id="prism-${command_name}"
  fi
  skill_description="$(prism_psm_require_command_config "${command_name}" "skill_description")"
  skill_version="$(prism_psm_resolve_skill_version "${command_name}")"
  usage_function="$(prism_psm_require_command_config "${command_name}" "usage_function")"
  dispatch_function="$(prism_psm_require_command_config "${command_name}" "skill_dispatch_function")"
  normalization_function="$(prism_psm_require_command_config "${command_name}" "skill_normalization_function")"

  printf '%s\n' '---'
  printf 'name: %s\n' "${skill_id}"
  printf 'description: %s\n' "${skill_description}"
  if [ -n "${skill_version}" ]; then
    printf 'version: %s\n' "${skill_version}"
  fi
  printf '%s\n\n' '---'
  printf '# psm %s\n\n' "${command_name}"
  printf "Run Prism's %s from Codex through the shared Codex \`psm\` integration framework.\n\n" \
    "$(prism_psm_require_command_config "${command_name}" "skill_title")"
  printf '%s\n\n' '## Usage'
  printf '%s\n' '```text'
  prism_psm_render_usage_block "${usage_function}"
  printf '%s\n\n' '```'
  printf '%s\n\n' '## Shared Codex Dispatch'
  printf 'Treat `psm %s` as a command, not as natural language.\n\n' "${command_name}"
  prism_psm_render_markdown_numbered_lines "${dispatch_function}"
  cat <<'EOF'

## Codex Normalization Rules

EOF
  prism_psm_render_markdown_bullets "${normalization_function}"
}

prism_psm_render_command_markdown() {
  local command_name="${1:-}"
  local description
  local shared_skill_relative_path

  description="$(prism_psm_require_command_config "${command_name}" "command_description")"
  shared_skill_relative_path="$(prism_psm_require_command_config "${command_name}" "shared_skill_relative_path")"

  cat <<EOF
---
description: "${description}"
---

Read the file at \`\${CLAUDE_PLUGIN_ROOT}/${shared_skill_relative_path}\` using the Read tool and follow its instructions exactly.
EOF
}

prism_psm_prepare_command_args() {
  local out_args_name="$1"
  local out_cleanup_name="$2"
  local original_cwd="$3"
  local repo_root="$4"
  local command_name="$5"
  shift 5
  local prepare_function

  eval "${out_args_name}=()"
  eval "${out_cleanup_name}=()"

  prepare_function="$(prism_psm_command_config "${command_name}" "prepare_function")"
  if [ -n "${prepare_function}" ]; then
    if ! declare -F "${prepare_function}" >/dev/null 2>&1; then
      prism_psm_die "command config for 'psm ${command_name}' references missing prepare function '${prepare_function}'."
    fi
    "${prepare_function}" "${out_args_name}" "${out_cleanup_name}" "${original_cwd}" "${repo_root}" "$@"
    return 0
  fi

  prism_psm_assign_array "${out_args_name}" "$@"
}

prism_psm_build_codex_args() {
  local out_args_name="$1"
  local repo_root="$2"
  local original_cwd="$3"
  local built_args=(
    exec
    --skip-git-repo-check
    --dangerously-bypass-approvals-and-sandbox
    -C "${repo_root}"
  )

  if [ "${original_cwd}" != "${repo_root}" ]; then
    built_args+=(--add-dir "${original_cwd}")
  fi

  if [ -n "${PSM_CODEX_MODEL:-}" ]; then
    built_args+=(--model "${PSM_CODEX_MODEL}")
  fi

  prism_psm_assign_array "${out_args_name}" "${built_args[@]}"
}

prism_psm_cleanup_paths() {
  local cleanup_path
  for cleanup_path in "$@"; do
    [ -n "${cleanup_path}" ] || continue
    rm -f "${cleanup_path}"
  done
}

prism_psm_exec() {
  local original_cwd="$1"
  shift

  local command_name="${1:-}"
  shift || true

  local skill_id
  skill_id="$(prism_psm_command_skill_id "${command_name}")" || {
    prism_psm_usage
    prism_psm_die "unsupported command '${command_name}'. The first milestone only supports: $(tr '\n' ' ' < <(prism_psm_supported_commands) | sed 's/ $//')."
  }

  local codex_cli
  codex_cli="$(prism_psm_resolve_codex_cli)"

  local repo_root
  repo_root="$(prism_psm_resolve_repo_root "${command_name}")"
  export PRISM_REPO_PATH="${repo_root}"
  export PRISM_TARGET_CWD="${original_cwd}"

  local command_args=()
  local cleanup_paths=()
  prism_psm_prepare_command_args command_args cleanup_paths "${original_cwd}" "${repo_root}" "${command_name}" "$@"

  local prompt
  if [ "${#command_args[@]}" -gt 0 ]; then
    prompt="$(prism_psm_build_prompt "${original_cwd}" "${command_name}" "${skill_id}" "${command_args[@]}")"
  else
    prompt="$(prism_psm_build_prompt "${original_cwd}" "${command_name}" "${skill_id}")"
  fi
  prompt+=$(prism_psm_command_contract "${command_name}")

  local codex_args=()
  prism_psm_build_codex_args codex_args "${repo_root}" "${original_cwd}"

  local exit_code=0
  set +e
  printf '%s' "${prompt}" | "${codex_cli}" "${codex_args[@]}" -
  exit_code=$?
  set -e

  if [ "${#cleanup_paths[@]}" -gt 0 ]; then
    prism_psm_cleanup_paths "${cleanup_paths[@]}"
  fi

  if [ "${exit_code}" -ne 0 ]; then
    prism_psm_die "Codex execution failed for 'psm ${command_name}' (exit ${exit_code})." "${exit_code}"
  fi
}

prism_psm_run() {
  local original_cwd
  original_cwd="$(pwd)"

  if [ "$#" -lt 1 ]; then
    prism_psm_usage
    prism_psm_die "missing command."
  fi

  prism_psm_exec "${original_cwd}" "$@"
}
