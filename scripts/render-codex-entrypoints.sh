#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${REPO_ROOT}/codex/lib/psm.sh"

COMMANDS_DIR="${REPO_ROOT}/commands"
GENERATED_SKILLS_DIR="${1:-}"

if [[ -n "${GENERATED_SKILLS_DIR}" ]]; then
  resolved_generated_skills_dir="$(
    python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).expanduser().resolve())' \
      "${GENERATED_SKILLS_DIR}"
  )"
  if [[ "${resolved_generated_skills_dir}" == "${REPO_ROOT}" || "${resolved_generated_skills_dir}" == "${REPO_ROOT}/"* ]]; then
    echo "ERROR: Refusing to generate Codex skill copies inside the Prism repo. Use repo skills/ as the authored source and sync managed installs into ~/.codex/skills only." >&2
    exit 1
  fi
fi

while IFS=$'\t' read -r command_name skill_dir _; do
  [[ -z "${command_name}" || "${command_name}" == \#* ]] && continue
  if [[ "${skill_dir}" != "${command_name}" || "${skill_dir}" == */* ]]; then
    echo "ERROR: Prism shared skill registry drift for '${command_name}'. Expected repo source skills/${command_name}/SKILL.md, got skills/${skill_dir}/SKILL.md." >&2
    exit 1
  fi

  command_path="${COMMANDS_DIR}/${command_name}.md"

  prism_psm_render_command_markdown "${command_name}" > "${command_path}"

  if [[ -n "${GENERATED_SKILLS_DIR}" ]]; then
    skill_path="${GENERATED_SKILLS_DIR}/${skill_dir}/SKILL.md"
    mkdir -p "$(dirname "${skill_path}")"
    prism_psm_render_codex_skill "${command_name}" > "${skill_path}"
  fi
done < "${REPO_ROOT}/codex/lib/command-registry.tsv"
