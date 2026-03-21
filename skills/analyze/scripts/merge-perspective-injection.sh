#!/bin/bash
# DEPRECATED (v6.0): This script is no longer used. Perspective injection is handled
# internally by the Go MCP server. Retained as design reference only.
#
# Merges perspective_injection.json into perspectives.json
# Usage: merge-perspective-injection.sh <state-dir>
# Example: merge-perspective-injection.sh ~/.prism/state/analyze-abc12345
#
# If perspective_injection.json does not exist, exits silently (no-op).
# If it exists, appends its array elements to perspectives.json's "perspectives" array.

set -euo pipefail

STATE_DIR="$1"
INJECTION_FILE="$STATE_DIR/perspective_injection.json"
PERSPECTIVES_FILE="$STATE_DIR/perspectives.json"

if [ ! -f "$INJECTION_FILE" ]; then
  exit 0
fi

if [ ! -f "$PERSPECTIVES_FILE" ]; then
  echo "ERROR: perspectives.json not found at $PERSPECTIVES_FILE" >&2
  exit 1
fi

# Merge: append injection array into perspectives.perspectives array
python3 -c "
import json, sys

perspectives_file = sys.argv[1]
injection_file = sys.argv[2]

with open(perspectives_file, 'r') as f:
    data = json.load(f)

with open(injection_file, 'r') as f:
    injection = json.load(f)

if not isinstance(injection, list):
    print('ERROR: perspective_injection.json must be a JSON array', file=sys.stderr)
    sys.exit(1)

data['perspectives'].extend(injection)

with open(perspectives_file, 'w') as f:
    json.dump(data, f, indent=2, ensure_ascii=False)

print(f'Merged {len(injection)} injected perspective(s) into perspectives.json')
" "$PERSPECTIVES_FILE" "$INJECTION_FILE"
