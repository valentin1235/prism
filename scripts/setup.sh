#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

RUNTIME="${1:-}"
if [[ "${RUNTIME}" == "--runtime" ]]; then
  RUNTIME="${2:-}"
fi

detect_runtime() {
  if command -v codex >/dev/null 2>&1; then
    echo "codex"
    return 0
  fi
  if command -v claude >/dev/null 2>&1; then
    echo "claude"
    return 0
  fi
  echo ""
}

require_claude_shared_sources() {
  local required_paths=(
    "${REPO_ROOT}/CLAUDE.md"
    "${REPO_ROOT}/commands"
    "${REPO_ROOT}/commands/setup.md"
    "${REPO_ROOT}/skills"
    "${REPO_ROOT}/skills/setup/SKILL.md"
  )
  local missing_paths=()
  local path

  for path in "${required_paths[@]}"; do
    if [[ ! -e "${path}" ]]; then
      missing_paths+=("${path}")
    fi
  done

  if [[ ${#missing_paths[@]} -gt 0 ]]; then
    echo "ERROR: Prism Claude setup requires the checked-in repo commands/ and skills/ directories." >&2
    echo "Claude uses those repo assets directly as the canonical shared source; setup must not replace them with generated copies." >&2
    printf 'Missing path: %s\n' "${missing_paths[@]}" >&2
    exit 1
  fi
}

if [[ -z "${RUNTIME}" ]]; then
  RUNTIME="$(detect_runtime)"
fi

case "${RUNTIME}" in
  codex)
    bash "${REPO_ROOT}/scripts/install-codex.sh"
    echo ""
    echo "Prism Codex setup refreshed the managed ~/.codex Prism skill mirror from the repo skills/ source and updated MCP integration."
    echo "Next:"
    echo "  export PATH=\"\$HOME/.codex/bin:\$PATH\""
    echo "  psm setup"
    ;;
  claude)
    require_claude_shared_sources
    bash "${REPO_ROOT}/scripts/configure-runtime.sh" --backend claude
    echo ""
    echo "Prism runtime configured for Claude Code."
    echo "Canonical shared skill source remains ${REPO_ROOT}/skills."
    echo "Canonical Claude command source remains ${REPO_ROOT}/commands."
    echo "Claude uses the checked-in Prism commands/ and skills/ directories directly."
    echo "No duplicate Claude slash-command artifacts were installed or synced."
    echo "Next:"
    echo "  Start a Claude Code session"
    echo "  Run /prism:setup"
    ;;
  *)
    echo "Usage: setup.sh --runtime <claude|codex>" >&2
    echo "Could not detect a supported runtime automatically." >&2
    exit 1
    ;;
esac
