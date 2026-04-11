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
SYNC_SKILLS_SCRIPT="${REPO_ROOT}/scripts/sync-codex-skills.py"

resolve_shared_skills_root() {
  local candidate_root
  candidate_root="${REPO_ROOT}/skills"

  if [[ ! -d "${candidate_root}" ]]; then
    echo "ERROR: Missing canonical shared Prism skills directory: ${candidate_root}" >&2
    exit 1
  fi

  if [[ ! -f "${candidate_root}/setup/SKILL.md" ]]; then
    echo "ERROR: Missing canonical shared Prism setup skill: ${candidate_root}/setup/SKILL.md" >&2
    exit 1
  fi

  printf '%s\n' "${candidate_root}"
}

SHARED_SKILLS_ROOT="$(resolve_shared_skills_root)"

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
python3 "${SYNC_SKILLS_SCRIPT}" \
  --repo-root "${REPO_ROOT}" \
  --registry-path "${REGISTRY_PATH}" \
  --shared-skills-root "${SHARED_SKILLS_ROOT}" \
  --target-root "${SKILLS_DIR}" \
  --namespace "prism-" >/dev/null

python3 - "${CONFIG_PATH}" "${REPO_ROOT}" "${CODEX_DIR}" <<'PY'
from __future__ import annotations

from pathlib import Path
import sys

config_path = Path(sys.argv[1])
repo_root = Path(sys.argv[2]).resolve()
codex_dir = Path(sys.argv[3]).resolve()
run_script = (repo_root / "mcp" / "run.sh").resolve()
shared_skills_root = (repo_root / "skills").resolve()
codex_skills_root = (codex_dir / "skills").resolve()
codex_rules_root = (codex_dir / "rules").resolve()

managed_block = f"""# Prism MCP hookup for Codex CLI.
# PRISM_REPO_PATH is the source of truth for shared Prism skill, prompt, template, and MCP assets.
# PRISM_SHARED_SKILLS_ROOT points at the canonical shared repo skills/ tree.
# PRISM_CODEX_SKILLS_ROOT and PRISM_CODEX_RULES_ROOT point at the managed Codex install roots under ~/.codex.
# The managed ~/.codex/skills/prism-* entries are setup-refreshed mirrors of PRISM_SHARED_SKILLS_ROOT.
[mcp_servers.prism]
command = "{run_script}"

[mcp_servers.prism.env]
PRISM_AGENT_RUNTIME = "codex"
PRISM_LLM_BACKEND = "codex"
PRISM_REPO_PATH = "{repo_root}"
PRISM_SHARED_SKILLS_ROOT = "{shared_skills_root}"
PRISM_CODEX_SKILLS_ROOT = "{codex_skills_root}"
PRISM_CODEX_RULES_ROOT = "{codex_rules_root}"
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
echo "Managed Prism skills in ${SKILLS_DIR} were refreshed from the canonical repo source ${SHARED_SKILLS_ROOT}"
echo "Add ${BIN_DIR} to PATH to use 'psm' from any working directory."
