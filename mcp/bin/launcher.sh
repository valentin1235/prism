#!/usr/bin/env bash
# Platform-detecting launcher for prism-mcp
# Automatically selects the correct binary for the current OS/architecture.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Normalize architecture names to Go-style identifiers
# - x86_64 (Linux/macOS Intel) → amd64
# - i686/i386 (32-bit x86, best-effort) → amd64
# - aarch64 (Linux ARM64) → arm64
# - arm64 (macOS Apple Silicon) → arm64
case "$ARCH" in
  x86_64|i686|i386) ARCH="amd64" ;;
  aarch64|arm64)    ARCH="arm64" ;;
  *)
    echo "ERROR: Unsupported architecture: $ARCH" >&2
    echo "Supported architectures: x86_64/amd64, aarch64/arm64" >&2
    exit 1
    ;;
esac

BINARY="${SCRIPT_DIR}/prism-mcp-${OS}-${ARCH}"

if [ ! -f "$BINARY" ]; then
  echo "ERROR: Unsupported platform — ${OS}/${ARCH}" >&2
  echo "" >&2
  echo "prism-mcp supports the following platforms:" >&2
  echo "  • darwin/arm64  (macOS Apple Silicon)" >&2
  echo "  • darwin/amd64  (macOS Intel)" >&2
  echo "  • linux/amd64   (Linux x86_64)" >&2
  echo "  • linux/arm64   (Linux ARM64)" >&2
  echo "" >&2
  echo "To build from source: cd mcp && make build" >&2
  exit 1
fi

if [ ! -x "$BINARY" ]; then
  chmod +x "$BINARY" 2>/dev/null || {
    echo "ERROR: Cannot make ${BINARY} executable." >&2
    echo "Try: chmod +x ${BINARY}" >&2
    exit 1
  }
fi

exec "$BINARY" "$@"
