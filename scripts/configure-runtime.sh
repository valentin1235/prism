#!/usr/bin/env bash
set -euo pipefail

BACKEND="${1:-}"
if [[ "${BACKEND}" == "--backend" ]]; then
  BACKEND="${2:-}"
fi

if [[ -z "${BACKEND}" ]]; then
  echo "Usage: configure-runtime.sh --backend <claude|codex>" >&2
  exit 1
fi

if [[ "${BACKEND}" != "claude" && "${BACKEND}" != "codex" ]]; then
  echo "Unsupported backend: ${BACKEND}" >&2
  exit 1
fi

CONFIG_DIR="${HOME}/.prism"
CONFIG_PATH="${CONFIG_DIR}/config.yaml"
mkdir -p "${CONFIG_DIR}"

CLAUDE_CLI_PATH="${PRISM_CLAUDE_CLI_PATH:-claude}"
CODEX_CLI_PATH="${PRISM_CODEX_CLI_PATH:-${PSM_CODEX_CLI_PATH:-codex}}"

cat > "${CONFIG_PATH}" <<EOF
runtime:
  backend: ${BACKEND}
  claude_cli_path: ${CLAUDE_CLI_PATH}
  codex_cli_path: ${CODEX_CLI_PATH}
  permission_mode: acceptEdits

llm:
  backend: ${BACKEND}
  default_model: default
EOF

echo "Configured Prism runtime backend: ${BACKEND}"
echo "Config saved to ${CONFIG_PATH}"
