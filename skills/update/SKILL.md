---
name: update
description: Update Prism plugin to the latest version and reload
version: 1.0.0
user-invocable: true
allowed-tools: Bash, Skill
---

# Prism Update

Update the Prism plugin to the latest version from the marketplace and reload it.

## Steps

1. Run `claude plugin update prism@prism 2>&1` via Bash and show the output to the user.
2. Delete the existing MCP binary so `run.sh` rebuilds it on next session start:
   ```bash
   rm -f ~/.claude/plugins/marketplaces/prism/mcp/prism-mcp 2>/dev/null
   rm -f ~/.claude/plugins/cache/prism/prism/*/mcp/prism-mcp 2>/dev/null
   ```
3. Tell the user: "Prism 플러그인이 업데이트되었습니다. 세션을 재시작하면 MCP 바이너리가 자동으로 새 소스에서 빌드됩니다."
