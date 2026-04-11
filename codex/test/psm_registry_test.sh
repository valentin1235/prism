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

HOME="${tmpdir}" CODEX_HOME="${tmpdir}/codex-home" bash "${REPO_ROOT}/scripts/install-codex.sh" >/dev/null

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
assert_eq "$(<"${tmpdir}/codex-home/skills/prism-analyze/SKILL.md")" "$(<"${REPO_ROOT}/skills/analyze/SKILL.md")" "installed analyze skill should come from the shared repo skill"
assert_eq "$(<"${tmpdir}/codex-home/skills/prism-brownfield/SKILL.md")" "$(<"${REPO_ROOT}/skills/brownfield/SKILL.md")" "installed brownfield skill should come from the shared repo skill"
assert_eq "$(<"${tmpdir}/codex-home/skills/prism-incident/SKILL.md")" "$(<"${REPO_ROOT}/skills/incident/SKILL.md")" "installed incident skill should come from the shared repo skill"
assert_eq "$(<"${tmpdir}/codex-home/skills/prism-prd/SKILL.md")" "$(<"${REPO_ROOT}/skills/prd/SKILL.md")" "installed prd skill should come from the shared repo skill"
assert_eq "$(<"${tmpdir}/codex-home/skills/prism-setup/SKILL.md")" "$(<"${REPO_ROOT}/skills/setup/SKILL.md")" "installed setup skill should come from the shared repo skill"
shared_analyze_skill_before="$(<"${REPO_ROOT}/skills/analyze/SKILL.md")"
if [ -L "${tmpdir}/codex-home/skills/prism-analyze" ]; then
  printf 'ASSERTION FAILED: expected installed prism-analyze skill to be a managed copy, not a symlink\n' >&2
  exit 1
fi
printf 'mutated installed copy\n' > "${tmpdir}/codex-home/skills/prism-analyze/SKILL.md"
assert_eq "$(<"${REPO_ROOT}/skills/analyze/SKILL.md")" "${shared_analyze_skill_before}" "shared repo analyze skill should remain the authored source"
if [ "$(<"${tmpdir}/codex-home/skills/prism-analyze/SKILL.md")" = "$(<"${REPO_ROOT}/skills/analyze/SKILL.md")" ]; then
  printf 'ASSERTION FAILED: expected installed prism-analyze skill mutation to stay isolated from the repo source\n' >&2
  exit 1
fi
HOME="${tmpdir}" CODEX_HOME="${tmpdir}/codex-home" bash "${REPO_ROOT}/scripts/install-codex.sh" >/dev/null
assert_eq "$(<"${tmpdir}/codex-home/skills/prism-analyze/SKILL.md")" "${shared_analyze_skill_before}" "reinstall should refresh the managed analyze skill copy from the repo source"
resolved_codex_home="$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).resolve())' "${tmpdir}/codex-home")"
assert_file_contains "${tmpdir}/codex-home/config.toml" "# PRISM_SHARED_SKILLS_ROOT points at the canonical shared repo skills/ tree."
assert_file_contains "${tmpdir}/codex-home/config.toml" "# The managed ~/.codex/skills/prism-* entries are setup-refreshed mirrors of PRISM_SHARED_SKILLS_ROOT."
assert_file_contains "${tmpdir}/codex-home/config.toml" "PRISM_SHARED_SKILLS_ROOT = \"${REPO_ROOT}/skills\""
assert_file_contains "${tmpdir}/codex-home/config.toml" "PRISM_CODEX_SKILLS_ROOT = \"${resolved_codex_home}/skills\""
assert_file_contains "${tmpdir}/codex-home/config.toml" "PRISM_CODEX_RULES_ROOT = \"${resolved_codex_home}/rules\""

mkdir -p "${tmpdir}/codex-home/skills/prism-stale-skill"
printf 'stale\n' > "${tmpdir}/codex-home/skills/prism-stale-skill/SKILL.md"
HOME="${tmpdir}" CODEX_HOME="${tmpdir}/codex-home" bash "${REPO_ROOT}/scripts/install-codex.sh" >/dev/null
assert_not_exists "${tmpdir}/codex-home/skills/prism-stale-skill"

sync_target="${tmpdir}/direct-sync-target"
python3 "${REPO_ROOT}/scripts/sync-codex-skills.py" \
  --repo-root "${REPO_ROOT}" \
  --registry-path "${REPO_ROOT}/codex/lib/command-registry.tsv" \
  --shared-skills-root "${REPO_ROOT}/skills" \
  --target-root "${sync_target}" >/dev/null
assert_eq "$(<"${sync_target}/prism-setup/SKILL.md")" "$(<"${REPO_ROOT}/skills/setup/SKILL.md")" "direct skill sync should treat repo skills as the canonical source"

invalid_registry="${tmpdir}/invalid-command-registry.tsv"
cat > "${invalid_registry}" <<'EOF'
# command	skill_dir	installed_skill_id
analyze	prism-analyze	prism-analyze
EOF

invalid_sync_stdout="${tmpdir}/invalid-sync-stdout.txt"
invalid_sync_stderr="${tmpdir}/invalid-sync-stderr.txt"
set +e
python3 "${REPO_ROOT}/scripts/sync-codex-skills.py" \
  --repo-root "${REPO_ROOT}" \
  --registry-path "${invalid_registry}" \
  --shared-skills-root "${REPO_ROOT}/skills" \
  --target-root "${tmpdir}/invalid-sync-target" \
  >"${invalid_sync_stdout}" 2>"${invalid_sync_stderr}"
invalid_sync_status=$?
set -e
assert_eq "${invalid_sync_status}" "1" "direct skill sync should reject non-canonical repo skill registry rows"
assert_file_contains "${invalid_sync_stderr}" "must use repo source 'skills/analyze/'"

repo_sync_stdout="${tmpdir}/repo-sync-stdout.txt"
repo_sync_stderr="${tmpdir}/repo-sync-stderr.txt"
set +e
python3 "${REPO_ROOT}/scripts/sync-codex-skills.py" \
  --repo-root "${REPO_ROOT}" \
  --registry-path "${REPO_ROOT}/codex/lib/command-registry.tsv" \
  --shared-skills-root "${REPO_ROOT}/skills" \
  --target-root "${REPO_ROOT}/codex/skills" \
  >"${repo_sync_stdout}" 2>"${repo_sync_stderr}"
repo_sync_status=$?
set -e
assert_eq "${repo_sync_status}" "1" "direct skill sync should refuse repo-local managed install targets"
assert_file_contains "${repo_sync_stderr}" "Refusing to sync managed Prism skills inside the Prism repo."

claude_home="${tmpdir}/claude-home"
claude_setup_output="${tmpdir}/claude-setup-output.txt"
HOME="${claude_home}" bash "${REPO_ROOT}/scripts/setup.sh" --runtime claude >"${claude_setup_output}"
assert_file_contains "${claude_home}/.prism/config.yaml" "backend: claude"
assert_file_contains "${claude_setup_output}" "Claude uses the checked-in Prism commands/ and skills/ directories directly."
assert_file_contains "${claude_setup_output}" "No duplicate Claude slash-command artifacts were installed or synced."
assert_file_contains "${claude_setup_output}" "Canonical shared skill source remains ${REPO_ROOT}/skills."
assert_file_contains "${claude_setup_output}" "Canonical Claude command source remains ${REPO_ROOT}/commands."
assert_not_exists "${claude_home}/.codex"
assert_not_exists "${REPO_ROOT}/codex/skills/analyze/SKILL.md"
assert_not_exists "${REPO_ROOT}/codex/skills/brownfield/SKILL.md"
assert_not_exists "${REPO_ROOT}/codex/skills/incident/SKILL.md"
assert_not_exists "${REPO_ROOT}/codex/skills/prd/SKILL.md"
assert_not_exists "${REPO_ROOT}/codex/skills/setup/SKILL.md"

codex_setup_home="${tmpdir}/codex-setup-home"
codex_setup_output="${tmpdir}/codex-setup-output.txt"
HOME="${codex_setup_home}" bash "${REPO_ROOT}/scripts/setup.sh" --runtime codex >"${codex_setup_output}"
assert_file_contains "${codex_setup_home}/.prism/config.yaml" "backend: codex"
assert_file_contains "${codex_setup_output}" "Prism Codex setup refreshed the managed ~/.codex Prism skill mirror from the repo skills/ source and updated MCP integration."
assert_file_contains "${codex_setup_output}" "Managed Prism skills in ${codex_setup_home}/.codex/skills were refreshed from the canonical repo source ${REPO_ROOT}/skills"
assert_dir_exists "${codex_setup_home}/.codex/bin"
assert_dir_exists "${codex_setup_home}/.codex/skills/prism-analyze"
assert_dir_exists "${codex_setup_home}/.codex/skills/prism-brownfield"
assert_dir_exists "${codex_setup_home}/.codex/skills/prism-incident"
assert_dir_exists "${codex_setup_home}/.codex/skills/prism-prd"
assert_dir_exists "${codex_setup_home}/.codex/skills/prism-setup"
assert_eq "$(<"${codex_setup_home}/.codex/skills/prism-setup/SKILL.md")" "$(<"${REPO_ROOT}/skills/setup/SKILL.md")" "setup.sh codex flow should install the shared setup skill into ~/.codex"
assert_eq "$(<"${codex_setup_home}/.codex/lib/prism/repo-root")" "${REPO_ROOT}" "setup.sh codex flow should preserve the shared repo-root pointer for installed psm"

render_stdout="${tmpdir}/render-stdout.txt"
render_stderr="${tmpdir}/render-stderr.txt"
set +e
bash "${REPO_ROOT}/scripts/render-codex-entrypoints.sh" "${REPO_ROOT}/codex/skills" \
  >"${render_stdout}" 2>"${render_stderr}"
render_status=$?
set -e
assert_eq "${render_status}" "1" "render script should refuse to recreate duplicate Codex skill sources inside the repo"
assert_file_contains "${render_stderr}" "Refusing to generate Codex skill copies inside the Prism repo."

invalid_render_stdout="${tmpdir}/invalid-render-stdout.txt"
invalid_render_stderr="${tmpdir}/invalid-render-stderr.txt"
registry_backup="${REPO_ROOT}/codex/lib/command-registry.tsv.bak-test"
cp "${REPO_ROOT}/codex/lib/command-registry.tsv" "${registry_backup}"
trap 'rm -rf "${tmpdir}"; if [ -f "${registry_backup}" ]; then mv "${registry_backup}" "${REPO_ROOT}/codex/lib/command-registry.tsv"; fi' EXIT
cp "${invalid_registry}" "${REPO_ROOT}/codex/lib/command-registry.tsv"
set +e
bash "${REPO_ROOT}/scripts/render-codex-entrypoints.sh" "${tmpdir}/generated-skills" \
  >"${invalid_render_stdout}" 2>"${invalid_render_stderr}"
invalid_render_status=$?
set -e
mv "${registry_backup}" "${REPO_ROOT}/codex/lib/command-registry.tsv"
assert_eq "${invalid_render_status}" "1" "render script should reject non-canonical repo skill registry rows"
assert_file_contains "${invalid_render_stderr}" "Expected repo source skills/analyze/SKILL.md"

assert_eq "$(prism_psm_require_command_config analyze shared_skill_relative_path)" "skills/analyze/SKILL.md" "analyze should expose its shared skill path through the command config contract"
assert_eq "$(prism_psm_require_command_config analyze asset_paths_function)" "prism_psm_analyze_asset_paths" "analyze should expose its asset path resolver through the command config contract"
assert_eq "$(prism_psm_require_command_config analyze contract_function)" "prism_psm_analyze_command_contract" "analyze should expose its command contract through the command config contract"
assert_eq "$(prism_psm_require_command_config analyze prepare_function)" "prism_psm_prepare_analyze_args" "analyze should expose its arg preparation through the command config contract"
assert_eq "$(prism_psm_require_command_config analyze prompt_function)" "prism_psm_analyze_bridge_prompt" "analyze should expose its prompt bridge through the command config contract"
assert_eq "$(prism_psm_require_command_config brownfield shared_skill_relative_path)" "skills/brownfield/SKILL.md" "brownfield should expose its shared skill path through the command config contract"
assert_eq "$(prism_psm_require_command_config brownfield contract_function)" "prism_psm_brownfield_command_contract" "brownfield should expose its command contract through the command config contract"

assert_file_contains "${REPO_ROOT}/commands/brownfield.md" 'Read the file at `skills/brownfield/SKILL.md` using the Read tool and follow its instructions exactly.'
assert_file_contains "${REPO_ROOT}/commands/setup.md" 'Read the file at `skills/setup/SKILL.md` using the Read tool and follow its instructions exactly.'
assert_file_contains "${REPO_ROOT}/skills/setup/SKILL.md" "Do not create or sync duplicate Claude slash-command artifacts during this step."
assert_file_contains "${REPO_ROOT}/skills/setup/SKILL.md" 'Validate that the checked-in `commands/` and `skills/` directories still exist in the Prism repo before continuing.'
assert_file_contains "${REPO_ROOT}/skills/brownfield/SKILL.md" 'In Codex, this same shared workflow is invoked through `psm brownfield`.'
assert_file_contains "${REPO_ROOT}/skills/brownfield/SKILL.md" 'any installed `~/.codex/skills/prism-brownfield` copy is just a managed mirror refreshed by setup.'
assert_file_contains "${REPO_ROOT}/skills/brownfield/SKILL.md" "No default repos set. Run '/prism:brownfield' to configure."
assert_file_contains "${REPO_ROOT}/skills/brownfield/SKILL.md" "No default repos set. Interviews will run in greenfield mode."
assert_file_contains "${REPO_ROOT}/skills/brownfield/SKILL.md" "Brownfield defaults updated!"
assert_file_contains "${REPO_ROOT}/CLAUDE.md" "Do not require generated skill copies for slash-command discovery."
assert_file_contains "${REPO_ROOT}/README.md" 'The repo `skills/` tree remains the canonical shared source for both runtimes:'

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
assert_file_contains "${captured_prompt}" "adapter-generated temporary config path"
assert_file_contains "${captured_prompt}" "The shared skill is the only workflow definition."
assert_file_contains "${captured_prompt}" "Path-valued analyze config fields have already been normalized for Codex execution context. Pass them through unchanged once read."
assert_file_contains "${captured_prompt}" 'When the shared skill asks `SELECT who you are: codex | claude`, choose `codex`'
assert_file_contains "${captured_prompt}" "Preserve the shared analyze config schema and MCP payload contract exactly."
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
assert_file_contains "${captured_prompt}" "managed mirror copies under ~/.codex/skills"
assert_file_contains "${captured_prompt}" "Treat any installed ~/.codex skill copy as a managed mirror, not as the authored source."
assert_file_contains "${captured_prompt}" "Resolve the shared Prism setup skill deterministically from"
assert_file_contains "${captured_prompt}" "Treat that shared skill as the only workflow definition; do not duplicate its phase logic, brownfield flow, status text, or stop conditions in the Codex command layer."
assert_file_contains "${captured_prompt}" "Read and follow that shared Prism skill from the resolved Prism asset root."

(
  cd "${invoke_dir}"
  HOME="${codex_setup_home}" PSM_CODEX_CLI_PATH="${fake_codex}" bash "${codex_setup_home}/.codex/bin/psm" setup
)

assert_file_contains "${captured_prompt}" "psm setup"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/setup/SKILL.md"
assert_file_contains "${captured_prompt}" "Treat any installed ~/.codex skill copy as a managed mirror, not as the authored source."
assert_file_contains "${captured_prompt}" "Read and follow that shared Prism skill from the resolved Prism asset root."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" setup defaults
)

assert_file_contains "${captured_prompt}" "psm setup defaults"
assert_file_contains "${captured_prompt}" "Treat that shared skill as the only workflow definition; do not duplicate its phase logic, brownfield flow, status text, or stop conditions in the Codex command layer."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" setup set 6,18,19
)

assert_file_contains "${captured_prompt}" "psm setup set 6\\,18\\,19"
assert_file_contains "${captured_prompt}" "Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" brownfield
)

assert_file_contains "${captured_prompt}" "psm brownfield"
assert_file_contains "${captured_prompt}" "Resolve the shared Prism brownfield skill deterministically from"
assert_file_contains "${captured_prompt}" "Treat installed \`~/.codex/skills/prism-brownfield\` entries as setup-refreshed mirrors of the shared repo skill"
assert_file_contains "${captured_prompt}" "Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" brownfield scan
)

assert_file_contains "${captured_prompt}" "psm brownfield scan"
assert_file_contains "${captured_prompt}" "Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" brownfield set 6,18,19
)

assert_file_contains "${captured_prompt}" "psm brownfield set 6\\,18\\,19"
assert_file_contains "${captured_prompt}" "Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary."

(
  cd "${invoke_dir}"
  PSM_CODEX_CLI_PATH="${fake_codex}" bash "${REPO_ROOT}/codex/bin/psm" incident checkout outage
)

assert_file_contains "${captured_prompt}" "psm incident checkout outage"
assert_file_contains "${captured_prompt}" "${REPO_ROOT}/skills/incident/SKILL.md"
assert_file_contains "${captured_prompt}" "For psm incident, dispatch to the shared Prism incident workflow entrypoint:"
assert_file_contains "${captured_prompt}" "Prism Incident Compatibility Bridge"
assert_file_contains "${captured_prompt}" 'Treat `psm incident ...` as the exact Codex equivalent of Claude Code `/prism:incident ...`.'
assert_file_contains "${captured_prompt}" 'Resolve the shared incident workflow entrypoint from `PRISM_REPO_PATH` first: `${PRISM_REPO_PATH}/skills/incident/SKILL.md`.'
assert_file_contains "${captured_prompt}" 'The shared skill is the only workflow definition.'
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
assert_file_contains "${captured_prompt}" 'The wrapper has already exported `PRISM_REPO_PATH='
assert_file_contains "${captured_prompt}" "Prism PRD Compatibility Bridge"
assert_file_contains "${captured_prompt}" 'Treat `psm prd ...` as the exact Codex equivalent of Claude Code `/prism:prd ...`.'
assert_file_contains "${captured_prompt}" 'The shared skill is the only workflow definition.'

printf 'psm registry test passed\n'
