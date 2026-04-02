#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

source "${REPO_ROOT}/codex/lib/psm.sh"

assert_eq() {
  local actual="$1"
  local expected="$2"
  local message="$3"

  if [ "${actual}" != "${expected}" ]; then
    printf 'ASSERTION FAILED: %s\nexpected: %s\nactual: %s\n' "${message}" "${expected}" "${actual}" >&2
    exit 1
  fi
}

assert_file_contains() {
  local path="$1"
  local needle="$2"

  if ! grep -Fq "${needle}" "${path}"; then
    printf 'ASSERTION FAILED: expected %s to contain %s\n' "${path}" "${needle}" >&2
    exit 1
  fi
}

assert_dir_exists() {
  local path="$1"

  if [ ! -d "${path}" ]; then
    printf 'ASSERTION FAILED: expected directory %s to exist\n' "${path}" >&2
    exit 1
  fi
}

assert_not_exists() {
  local path="$1"

  if [ -e "${path}" ]; then
    printf 'ASSERTION FAILED: expected %s to be absent\n' "${path}" >&2
    exit 1
  fi
}

expected_commands=$'analyze\nbrownfield\nincident\nprd\nsetup'
actual_commands="$(prism_psm_supported_commands)"
assert_eq "${actual_commands}" "${expected_commands}" "supported commands should come from the registry"

ontology_path="${REPO_ROOT}/codex/lib/command-ontology.tsv"
assert_file_contains "${ontology_path}" $'analyze\tprism-analyze\tacceptance-bearing\tregistered'
assert_file_contains "${ontology_path}" $'brownfield\tprism-brownfield\tacceptance-bearing\tregistered'
assert_file_contains "${ontology_path}" $'incident\tprism-incident\tacceptance-bearing\tregistered'
assert_file_contains "${ontology_path}" $'prd\tprism-prd\tacceptance-bearing\tregistered'
assert_file_contains "${ontology_path}" $'setup\tprism-setup\tacceptance-bearing\tregistered'
assert_file_contains "${ontology_path}" $'analyze-workspace\tprism-analyze-workspace\tnon-acceptance-bearing\tunregistered'
assert_file_contains "${ontology_path}" $'test-analyze\tprism-test-analyze\tnon-acceptance-bearing\tunregistered'

for command_name in analyze brownfield incident prd setup; do
  prism_psm_is_supported_command "${command_name}"
done

if prism_psm_is_supported_command "analyze-workspace"; then
  printf 'ASSERTION FAILED: analyze-workspace must remain excluded from the milestone registry\n' >&2
  exit 1
fi

if prism_psm_is_supported_command "test-analyze"; then
  printf 'ASSERTION FAILED: test-analyze must remain excluded from the milestone registry\n' >&2
  exit 1
fi

assert_eq "$(prism_psm_command_skill_id analyze)" "prism-analyze" "analyze should resolve to the shared skill id"
assert_eq "$(prism_psm_command_skill_id brownfield)" "prism-brownfield" "brownfield should resolve to the shared skill id"
assert_eq "$(prism_psm_command_skill_id incident)" "prism-incident" "incident should resolve to the shared skill id"
assert_eq "$(prism_psm_command_skill_id prd)" "prism-prd" "prd should resolve to the shared skill id"
assert_eq "$(prism_psm_command_skill_id setup)" "prism-setup" "setup should resolve to the shared skill id"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT
test_tmpdir="${tmpdir}/tmp"
mkdir -p "${test_tmpdir}"

CODEX_HOME="${tmpdir}/codex-home" bash "${REPO_ROOT}/scripts/install-codex.sh" >/dev/null

assert_dir_exists "${tmpdir}/codex-home/bin"
assert_dir_exists "${tmpdir}/codex-home/lib/prism"
assert_file_contains "${tmpdir}/codex-home/lib/prism/psm.sh" "source \"\${PRISM_PSM_LIB_DIR}/framework.sh\""
assert_file_contains "${tmpdir}/codex-home/lib/prism/psm.sh" "prism_psm_load_command_configs"
assert_file_contains "${tmpdir}/codex-home/lib/prism/command-ontology.tsv" $'analyze-workspace\tprism-analyze-workspace\tnon-acceptance-bearing\tunregistered'
assert_file_contains "${tmpdir}/codex-home/lib/prism/framework.sh" "prism_psm_build_codex_args()"
assert_file_contains "${tmpdir}/codex-home/lib/prism/framework.sh" "prism_psm_define_command_config()"
assert_file_contains "${tmpdir}/codex-home/lib/prism/framework.sh" "prism_psm_require_command_config()"
assert_dir_exists "${tmpdir}/codex-home/lib/prism/bridges"
assert_dir_exists "${tmpdir}/codex-home/lib/prism/commands"
assert_file_contains "${tmpdir}/codex-home/lib/prism/commands/analyze.sh" "prism_psm_define_command_config \"analyze\" \"shared_skill_relative_path\""
assert_file_contains "${tmpdir}/codex-home/lib/prism/commands/prd.sh" "prism_psm_define_command_config \"prd\" \"contract_function\""
assert_file_contains "${tmpdir}/codex-home/lib/prism/bridges/incident.sh" "Prism Incident Compatibility Bridge"
assert_file_contains "${tmpdir}/codex-home/lib/prism/bridges/prd.sh" "Prism PRD Compatibility Bridge"
assert_dir_exists "${tmpdir}/codex-home/skills/prism-analyze"
assert_dir_exists "${tmpdir}/codex-home/skills/prism-brownfield"
assert_dir_exists "${tmpdir}/codex-home/skills/prism-incident"
assert_dir_exists "${tmpdir}/codex-home/skills/prism-prd"
assert_dir_exists "${tmpdir}/codex-home/skills/prism-setup"
assert_not_exists "${tmpdir}/codex-home/skills/prism-test-analyze"
assert_not_exists "${tmpdir}/codex-home/skills/prism-analyze-workspace"

assert_eq "$(prism_psm_require_command_config analyze shared_skill_relative_path)" "skills/analyze/SKILL.md" "analyze should expose its shared skill path through the command config contract"
assert_eq "$(prism_psm_require_command_config analyze asset_paths_function)" "prism_psm_analyze_asset_paths" "analyze should expose its asset path resolver through the command config contract"
assert_eq "$(prism_psm_require_command_config analyze contract_function)" "prism_psm_analyze_command_contract" "analyze should expose its command contract through the command config contract"
assert_eq "$(prism_psm_require_command_config analyze prepare_function)" "prism_psm_prepare_analyze_args" "analyze should expose its arg preparation through the command config contract"
assert_eq "$(prism_psm_require_command_config analyze prompt_function)" "prism_psm_analyze_bridge_prompt" "analyze should expose its prompt bridge through the command config contract"
assert_eq "$(prism_psm_require_command_config brownfield shared_skill_relative_path)" "skills/brownfield/SKILL.md" "brownfield should expose its shared skill path through the command config contract"
assert_eq "$(prism_psm_require_command_config brownfield contract_function)" "prism_psm_brownfield_command_contract" "brownfield should expose its command contract through the command config contract"

for command_name in analyze brownfield incident prd setup; do
  skill_dir="$(prism_psm_command_skill_dir "${command_name}")"
  rendered_skill="$(prism_psm_render_codex_skill "${command_name}")"
  actual_skill="$(<"${REPO_ROOT}/codex/skills/${skill_dir}/SKILL.md")"
  assert_eq "${actual_skill}" "${rendered_skill}" "codex skill entrypoint for ${command_name} should be rendered from the shared framework config"

  rendered_command_doc="$(prism_psm_render_command_markdown "${command_name}")"
  actual_command_doc="$(<"${REPO_ROOT}/commands/${command_name}.md")"
  assert_eq "${actual_command_doc}" "${rendered_command_doc}" "command entrypoint for ${command_name} should be rendered from the shared framework config"
done

assert_file_contains "${REPO_ROOT}/commands/brownfield.md" 'Treat `psm brownfield`, `psm brownfield scan`, `psm brownfield defaults`, and `psm brownfield set <indices>` as exact command forms routed through that shared skill.'

config_project="${tmpdir}/config-project"
mkdir -p "${config_project}/configs" "${config_project}/inputs"
cat > "${config_project}/configs/analyze.json" <<'EOF'
{
  "topic": "Adapter contract coverage",
  "input_context": "inputs/request.md",
  "report_template": "__REPORT_TEMPLATE__",
  "seed_hints": "Focus on unchanged artifact contract",
  "session_id": "session-123",
  "model": "claude-sonnet-4-6",
  "ontology_scope": "repo:payments",
  "perspective_injection": "skills/analyze/prompts/finding-protocol.md"
}
EOF
python3 - "${config_project}/configs/analyze.json" "${REPO_ROOT}/skills/analyze/templates/report.md" <<'PY'
from pathlib import Path
import sys

config_path = Path(sys.argv[1])
report_template = sys.argv[2]
config_path.write_text(config_path.read_text().replace("__REPORT_TEMPLATE__", report_template), encoding="utf-8")
PY
printf 'analyze this\n' > "${config_project}/inputs/request.md"

prepared_args=()
cleanup_paths=()
prism_psm_prepare_command_args prepared_args cleanup_paths "${config_project}" "${REPO_ROOT}" analyze --config configs/analyze.json
assert_eq "${prepared_args[0]}" "--config" "analyze adapter should preserve the --config flag"
if [[ "${prepared_args[1]}" != /* ]]; then
  printf 'ASSERTION FAILED: expected normalized analyze config path to be absolute, got %s\n' "${prepared_args[1]}" >&2
  exit 1
fi
assert_file_contains "${prepared_args[1]}" "\"input_context\": \"${config_project}/inputs/request.md\""
assert_file_contains "${prepared_args[1]}" "\"report_template\": \"${REPO_ROOT}/skills/analyze/templates/report.md\""
assert_file_contains "${prepared_args[1]}" "\"seed_hints\": \"Focus on unchanged artifact contract\""
assert_file_contains "${prepared_args[1]}" "\"session_id\": \"session-123\""
assert_file_contains "${prepared_args[1]}" "\"model\": \"claude-sonnet-4-6\""
assert_file_contains "${prepared_args[1]}" "\"ontology_scope\": \"repo:payments\""
assert_file_contains "${prepared_args[1]}" "\"perspective_injection\": \"${REPO_ROOT}/skills/analyze/prompts/finding-protocol.md\""
assert_file_contains "${config_project}/configs/analyze.json" "\"input_context\": \"inputs/request.md\""
assert_file_contains "${config_project}/configs/analyze.json" "\"report_template\": \"${REPO_ROOT}/skills/analyze/templates/report.md\""
assert_file_contains "${config_project}/configs/analyze.json" "\"perspective_injection\": \"skills/analyze/prompts/finding-protocol.md\""

for cleanup_path in "${cleanup_paths[@]}"; do
  rm -f "${cleanup_path}"
done

fake_codex="${tmpdir}/fake-codex.sh"
captured_prompt="${tmpdir}/captured-prompt.txt"
cat > "${fake_codex}" <<EOF
#!/usr/bin/env bash
set -euo pipefail
cat > "${captured_prompt}"
EOF
chmod +x "${fake_codex}"

failing_codex="${tmpdir}/failing-codex.sh"
cat > "${failing_codex}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
exit 23
EOF
chmod +x "${failing_codex}"

invoke_dir="${tmpdir}/invoke-from-here"
mkdir -p "${invoke_dir}"
(
  cd "${invoke_dir}"
  cp "${config_project}/configs/analyze.json" "${invoke_dir}/analyze.json"
  mkdir -p "${invoke_dir}/inputs"
  cp "${config_project}/inputs/request.md" "${invoke_dir}/inputs/request.md"
  TMPDIR="${test_tmpdir}" PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" analyze --config analyze.json
)

failure_stdout="${tmpdir}/failure-stdout.txt"
failure_stderr="${tmpdir}/failure-stderr.txt"
set +e
(
  cd "${invoke_dir}"
  TMPDIR="${test_tmpdir}" PSM_CODEX_CLI_PATH="${failing_codex}" bash "${REPO_ROOT}/codex/bin/psm" analyze --config analyze.json
) >"${failure_stdout}" 2>"${failure_stderr}"
failure_status=$?
set -e
assert_eq "${failure_status}" "23" "psm should preserve codex exit codes on failure"
assert_file_contains "${failure_stderr}" "Codex execution failed for 'psm analyze' (exit 23)."
if find "${test_tmpdir}" -maxdepth 1 -name 'psm-analyze-config.*' | grep -q .; then
  printf 'ASSERTION FAILED: normalized analyze temp configs should be cleaned up after failure\n' >&2
  exit 1
fi

assert_file_contains "${captured_prompt}" "psm analyze"
assert_file_contains "${captured_prompt}" "Prism Analyze Compatibility Bridge"
assert_file_contains "${captured_prompt}" "Preserve the full shared-skill decision flow and exit gates, not just the MCP payload shape."
assert_file_contains "${captured_prompt}" "adapter-generated temporary config path"
assert_file_contains "${captured_prompt}" "Preserve the shared analyze config schema and MCP payload contract exactly."
assert_file_contains "${captured_prompt}" "Path-valued analyze config fields have already been normalized for Codex execution context. Pass them through unchanged once read."
assert_file_contains "${captured_prompt}" "if \`--config <path>\` is present, read that config and use \`config.topic\` as the description when present, otherwise fall back to remaining arguments;"
assert_file_contains "${captured_prompt}" "if no config is present, use the remaining command arguments as the description;"
assert_file_contains "${captured_prompt}" "if the description is still empty, ask the user directly for what to analyze."
assert_file_contains "${captured_prompt}" "Honor the shared Phase 1 exit gate before starting analysis: do not call \`prism_analyze\` until the description has been collected."
assert_file_contains "${captured_prompt}" "Honor the shared Phase 2 exit gate: do not proceed to polling until \`prism_analyze\` returns a \`task_id\`."
assert_file_contains "${captured_prompt}" "During Phase 3, poll \`prism_task_status\` every 30 seconds until the task reaches \`completed\` or \`failed\`"
assert_file_contains "${captured_prompt}" "if the task status is \`failed\`, report the error and stop without calling \`prism_analyze_result\`;"
assert_file_contains "${captured_prompt}" "Honor the shared Phase 4 exit gate: after completion, call \`prism_analyze_result(task_id)\`, present the returned \`summary\`, and communicate the returned \`report_path\`."
assert_file_contains "${captured_prompt}" "The user's original working directory is:"
assert_file_contains "${captured_prompt}" "${invoke_dir}"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/analyze/SKILL.md"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/analyze/templates/report.md"

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" brownfield defaults
)

assert_file_contains "${captured_prompt}" "psm brownfield defaults"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/brownfield/SKILL.md"

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" setup
)

assert_file_contains "${captured_prompt}" "psm setup"
assert_file_contains "${captured_prompt}" "The canonical shared Prism skill for this command is:"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/setup/SKILL.md"
assert_file_contains "${captured_prompt}" "Resolve the shared Prism setup skill deterministically from"
assert_file_contains "${captured_prompt}" "Preserve the shared setup flow exactly, including the brownfield scan, default-selection prompt, MCP tool usage, and final confirmation messaging."
assert_file_contains "${captured_prompt}" "Read and follow that shared Prism skill from the resolved Prism asset root, not from the user's working directory."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" setup defaults
)

assert_file_contains "${captured_prompt}" "psm setup defaults"
assert_file_contains "${captured_prompt}" "Preserve the shared-skill subcommand behavior exactly: \`scan\` means scan only, \`defaults\` means show current defaults, and \`set <indices>\` means update defaults directly with the provided comma-separated indices."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" setup set 6,18,19
)

assert_file_contains "${captured_prompt}" "psm setup set 6\\,18\\,19"
assert_file_contains "${captured_prompt}" "successful default updates should confirm the selected repository names."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" brownfield
)

assert_file_contains "${captured_prompt}" "psm brownfield"
assert_file_contains "${captured_prompt}" "Preserve the default no-argument flow exactly: scan first, render the scan result, then prompt for default selection."
assert_file_contains "${captured_prompt}" "clearing defaults should surface the shared greenfield-mode confirmation"

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" brownfield scan
)

assert_file_contains "${captured_prompt}" "psm brownfield scan"
assert_file_contains "${captured_prompt}" "Preserve the shared-skill subcommand behavior exactly: \`scan\` means scan only, \`defaults\` means show current defaults, and \`set <indices>\` means update defaults directly with the provided comma-separated indices."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" brownfield set 6,18,19
)

assert_file_contains "${captured_prompt}" "psm brownfield set 6\\,18\\,19"
assert_file_contains "${captured_prompt}" "successful default updates should confirm the selected repository names."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" incident checkout outage
)

assert_file_contains "${captured_prompt}" "psm incident checkout outage"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/incident/SKILL.md"
assert_file_contains "${captured_prompt}" "For psm incident, dispatch to the shared Prism incident workflow entrypoint and preserve its full RCA flow:"
assert_file_contains "${captured_prompt}" "Prism Incident Compatibility Bridge"
assert_file_contains "${captured_prompt}" 'Treat `psm incident ...` as the exact Codex equivalent of Claude Code `/prism:incident ...`.'
assert_file_contains "${captured_prompt}" 'Resolve the shared incident workflow entrypoint from `PRISM_REPO_PATH` first: `${PRISM_REPO_PATH}/skills/incident/SKILL.md`.'
assert_file_contains "${captured_prompt}" 'When the shared incident skill dispatches analysis, preserve its asset contract exactly by passing through the shared incident report template and UX perspective injection assets unchanged.'

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" prd "inputs/request.md"
)

assert_file_contains "${captured_prompt}" "psm prd inputs/request.md"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/prd/SKILL.md"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/prd/prompts/post-processor.md"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/prd/templates/report.md"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/analyze/SKILL.md"
assert_file_contains "${captured_prompt}" 'When the shared PRD skill refers to files relative to its own `SKILL.md`, resolve them from `'
assert_file_contains "${captured_prompt}" 'The wrapper has already exported `PRISM_REPO_PATH='
assert_file_contains "${captured_prompt}" "Prism PRD Compatibility Bridge"
assert_file_contains "${captured_prompt}" 'Treat `psm prd ...` as the exact Codex equivalent of Claude Code `/prism:prd ...`.'
assert_file_contains "${captured_prompt}" 'Preserve the full shared PRD decision flow and exit gates, not just the eventual analyze handoff or report artifact.'
assert_file_contains "${captured_prompt}" 'write `~/.prism/state/prd-{short-id}/analyze-config.json` with the shared `topic`, `input_context`, `seed_hints`, and `session_id` contract;'
assert_file_contains "${captured_prompt}" 'keep `~/.prism/state/prd-{short-id}/analyze-config.json` as the concrete handoff artifact and do not substitute a different filename or directory.'
assert_file_contains "${captured_prompt}" 'require `analyst-findings.md` before post-processing;'
assert_file_contains "${captured_prompt}" 'require the post-processor handoff result to be that exact report path.'
assert_file_contains "${captured_prompt}" 'verify the generated report contains `PM Decision Checklist` before presenting success.'
assert_file_contains "${captured_prompt}" 'preserve that three-line handoff format because existing Prism workflows expect those locations and filenames.'

printf 'psm registry test passed\n'
