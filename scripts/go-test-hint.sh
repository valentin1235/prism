#!/bin/bash
# PostToolUse hook: When a .go file under mcp/ is edited/written, hint to generate test code

INPUT=$(cat)

TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')

# Only trigger on Edit or Write
case "$TOOL_NAME" in
  Edit|Write) ;;
  *) exit 0 ;;
esac

# Extract file_path from tool_input
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Must be under mcp/, must be .go, must NOT be _test.go
if [[ "$FILE_PATH" != *"/mcp/"* ]]; then
  exit 0
fi
if [[ "$FILE_PATH" != *.go ]]; then
  exit 0
fi
if [[ "$FILE_PATH" == *_test.go ]]; then
  exit 0
fi

# Derive expected test file path
TEST_FILE="${FILE_PATH%.go}_test.go"
BASENAME=$(basename "$FILE_PATH")
TEST_BASENAME=$(basename "$TEST_FILE")

# Check if test file already exists
if [ -f "$TEST_FILE" ]; then
  echo "Go source modified: $BASENAME → test file exists ($TEST_BASENAME). Review and update the test if the changes affect exported functions or behavior."
else
  echo "Go source created/modified: $BASENAME → no test file found. Generate $TEST_BASENAME with table-driven tests covering the exported functions."
fi
