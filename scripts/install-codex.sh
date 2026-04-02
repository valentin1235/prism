#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CODEX_DIR="${CODEX_HOME:-${HOME}/.codex}"
RULES_DIR="${CODEX_DIR}/rules"
SKILLS_DIR="${CODEX_DIR}/skills"
BIN_DIR="${CODEX_DIR}/bin"
LIB_DIR="${CODEX_DIR}/lib/prism"
CONFIG_PATH="${CODEX_DIR}/config.toml"
REGISTRY_PATH="${REPO_ROOT}/codex/lib/command-registry.tsv"

mkdir -p "${RULES_DIR}" "${SKILLS_DIR}" "${BIN_DIR}" "${LIB_DIR}"

cp "${REPO_ROOT}/codex/rules/prism.md" "${RULES_DIR}/prism.md"
cp "${REPO_ROOT}/codex/bin/psm" "${BIN_DIR}/psm"
cp "${REPO_ROOT}/codex/lib/framework.sh" "${LIB_DIR}/framework.sh"
cp "${REPO_ROOT}/codex/lib/psm.sh" "${LIB_DIR}/psm.sh"
cp "${REGISTRY_PATH}" "${LIB_DIR}/command-registry.tsv"
cp "${REPO_ROOT}/codex/lib/command-ontology.tsv" "${LIB_DIR}/command-ontology.tsv"
rm -rf "${LIB_DIR}/bridges"
cp -R "${REPO_ROOT}/codex/lib/bridges" "${LIB_DIR}/bridges"
rm -rf "${LIB_DIR}/commands"
cp -R "${REPO_ROOT}/codex/lib/commands" "${LIB_DIR}/commands"
printf '%s\n' "${REPO_ROOT}" > "${LIB_DIR}/repo-root"
chmod +x "${BIN_DIR}/psm" "${LIB_DIR}/framework.sh" "${LIB_DIR}/psm.sh" "${REPO_ROOT}/scripts/configure-runtime.sh" "${REPO_ROOT}/scripts/setup.sh"

while IFS=$'\t' read -r command_name skill_dir installed_skill_id; do
  [[ -z "${command_name}" || "${command_name}" == \#* ]] && continue
  skill_dir="${REPO_ROOT}/codex/skills/${skill_dir}"
  if [[ ! -d "${skill_dir}" ]]; then
    echo "ERROR: Missing Codex skill directory: ${skill_dir}" >&2
    exit 1
  fi

  target_dir="${SKILLS_DIR}/${installed_skill_id}"
  rm -rf "${target_dir}"
  cp -R "${skill_dir}" "${target_dir}"
done < "${REGISTRY_PATH}"

python3 - "${CONFIG_PATH}" "${REPO_ROOT}" <<'PY'
from __future__ import annotations

from pathlib import Path
import sys

config_path = Path(sys.argv[1])
repo_root = Path(sys.argv[2]).resolve()
run_script = (repo_root / "mcp" / "run.sh").resolve()

managed_block = f"""# Prism MCP hookup for Codex CLI.
# PRISM_REPO_PATH is the source of truth for shared Prism skill, prompt, template, and MCP assets.
[mcp_servers.prism]
command = "{run_script}"

[mcp_servers.prism.env]
PRISM_REPO_PATH = "{repo_root}"
"""

raw = config_path.read_text(encoding="utf-8") if config_path.exists() else ""
lines = raw.splitlines()
output: list[str] = []
skip = False

for line in lines:
    stripped = line.strip()
    if stripped in {"[mcp_servers.prism]", "[mcp_servers.prism.env]"}:
        skip = True
        continue
    if skip and stripped.startswith("[") and stripped not in {
        "[mcp_servers.prism]",
        "[mcp_servers.prism.env]",
    }:
        skip = False
    if skip:
        continue
    output.append(line)

if output and output[-1].strip():
    output.append("")

output.extend(managed_block.strip().splitlines())
config_path.write_text("\n".join(output).rstrip() + "\n", encoding="utf-8")
PY

PRISM_CODEX_CLI_PATH="${PSM_CODEX_CLI_PATH:-${PRISM_CODEX_CLI_PATH:-codex}}" \
  bash "${REPO_ROOT}/scripts/configure-runtime.sh" --backend codex >/dev/null

echo "Installed Prism Codex artifacts into ${CODEX_DIR}"
echo "Add ${BIN_DIR} to PATH to use 'psm' from any working directory."
