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

if [[ -z "${RUNTIME}" ]]; then
  RUNTIME="$(detect_runtime)"
fi

case "${RUNTIME}" in
  codex)
    bash "${REPO_ROOT}/scripts/install-codex.sh"
    echo ""
    echo "Next:"
    echo "  export PATH=\"\$HOME/.codex/bin:\$PATH\""
    echo "  psm setup"
    ;;
  claude)
    bash "${REPO_ROOT}/scripts/configure-runtime.sh" --backend claude
    echo ""
    echo "Prism runtime configured for Claude Code."
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
