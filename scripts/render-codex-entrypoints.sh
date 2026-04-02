#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${REPO_ROOT}/codex/lib/psm.sh"

while IFS=$'\t' read -r command_name skill_dir _; do
  [[ -z "${command_name}" || "${command_name}" == \#* ]] && continue

  skill_path="${REPO_ROOT}/codex/skills/${skill_dir}/SKILL.md"
  command_path="${REPO_ROOT}/commands/${command_name}.md"

  prism_psm_render_codex_skill "${command_name}" > "${skill_path}"
  prism_psm_render_command_markdown "${command_name}" > "${command_path}"
done < "${REPO_ROOT}/codex/lib/command-registry.tsv"
