#!/bin/bash
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$DIR/prism-mcp"

# Build if binary doesn't exist or any Go source changed
NEEDS_BUILD=false
if [ ! -f "$BIN" ]; then
  NEEDS_BUILD=true
else
  for f in "$DIR"/*.go; do
    if [ "$f" -nt "$BIN" ]; then
      NEEDS_BUILD=true
      break
    fi
  done
fi
if [ "$NEEDS_BUILD" = true ]; then
  (cd "$DIR" && go build -o "$BIN" .) >&2
  if [ "$(uname -s)" = "Darwin" ]; then
    codesign --sign - --force "$BIN" 2>/dev/null >&2 || true
  fi
fi

exec "$BIN" "$@"
