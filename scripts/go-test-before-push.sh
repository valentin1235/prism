#!/bin/bash
# PreToolUse hook: Run go tests under mcp/ before git push

INPUT=$(cat)

TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')

# Only trigger on Bash tool
if [ "$TOOL_NAME" != "Bash" ]; then
  exit 0
fi

# Check if the command is a git push
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

if ! echo "$COMMAND" | grep -qE '\bgit\s+push\b'; then
  exit 0
fi

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-.}"

# Run go tests under mcp/ (separate Go module)
echo "Running Go tests before push..."
cd "$PROJECT_DIR/mcp" && go test ./... 2>&1
TEST_EXIT=$?

if [ $TEST_EXIT -ne 0 ]; then
  echo ""
  echo "BLOCKED: Go tests failed (exit code $TEST_EXIT). Fix the failing tests before pushing."
  exit 2
fi

echo "All mcp/ tests passed. Proceeding with push."
exit 0
