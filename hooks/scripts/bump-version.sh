#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

bump() {
  local file="$1"
  if [ ! -f "$file" ]; then return; fi
  python3 -c "
import json, sys
path = sys.argv[1]
with open(path) as f:
    data = json.load(f)
v = data.get('version', '0.0.0')
parts = v.split('.')
parts[-1] = str(int(parts[-1]) + 1)
data['version'] = '.'.join(parts)
with open(path, 'w') as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write('\n')
print(f'{path}: {v} -> {data[\"version\"]}', file=sys.stderr)
" "$file"
}

bump "$ROOT/.claude-plugin/plugin.json"
bump "$ROOT/.claude-plugin/marketplace.json"
bump "$ROOT/.codex-plugin/plugin.json"

cd "$ROOT"
git add .claude-plugin/plugin.json .claude-plugin/marketplace.json .codex-plugin/plugin.json
git commit -m "chore: bump Prism plugin version" --allow-empty 2>/dev/null || true
