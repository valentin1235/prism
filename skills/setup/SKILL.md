---
name: setup
description: Download and install the prism-mcp binary for your platform and register it as a user-scope MCP server
version: 1.0.0
user-invocable: true
allowed-tools: Bash, Read, Write, AskUserQuestion
---

# Prism MCP Setup

Install the prism-mcp binary and register it as a Claude Code MCP server.

## Step 1: Detect Platform

Run the following to detect OS and architecture:

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
esac
echo "${OS}/${ARCH}"
```

Supported platforms: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`.
If the platform is not supported, inform the user and stop.

## Step 2: Download Binary

Get the latest release tag and download the binary:

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
esac

INSTALL_DIR="$HOME/.prism/mcp"
mkdir -p "$INSTALL_DIR"

# Get latest release tag from public repo
TAG=$(gh release list --repo valentin1235/prism --limit 1 --json tagName -q '.[0].tagName')

if [ -z "$TAG" ]; then
  echo "ERROR: No releases found"
  exit 1
fi

echo "Downloading prism-mcp ${TAG} for ${OS}/${ARCH}..."

gh release download "$TAG" \
  --repo valentin1235/prism \
  --pattern "prism-mcp-${OS}-${ARCH}" \
  --output "$INSTALL_DIR/prism-mcp" \
  --clobber

chmod +x "$INSTALL_DIR/prism-mcp"

# macOS: ad-hoc code sign to prevent SIGKILL (Code Signature Invalid)
if [ "$OS" = "darwin" ]; then
  codesign --sign - --force "$INSTALL_DIR/prism-mcp"
  echo "Code signed (ad-hoc) for macOS"
fi

echo "Installed to $INSTALL_DIR/prism-mcp"
```

If download fails, inform the user about the error and stop.

## Step 3: Register MCP Server

Register prism-mcp as a user-scope MCP server:

```bash
claude mcp add prism-mcp -s user -- "$HOME/.prism/mcp/prism-mcp"
```

## Step 4: Configure Documentation Directories (Optional)

Loop with a single question per iteration. The user can type a path directly or skip:

```
AskUserQuestion(
  header: "Documentation Directories",
  question: "<current_list_or_empty>prism 스킬(incident, prd, plan)이 분석 시 참조할 문서 디렉토리 경로를 입력하세요:",
  options: [
    {label: "Enter path", description: "Type an absolute directory path"},
    {label: "Skip", description: "Save and finish"}
  ]
)
```

- `<current_list_or_empty>`: If directories already added, show `"Current directories:\n- /path/a\n- /path/b\n\n"`. If none yet, omit.
- **"Enter path"**: Extract path → validate with `test -d` → if not found, warn and loop back → if already in list, warn and loop back → if valid, add to list, immediately loop back with updated list.
- **"Skip"**: Exit loop. If any paths were added, write to `~/.prism/ontology-docs.json`. If none, inform user they can configure later.

Write config:
```json
{
  "directories": [
    "/path/to/project-a/docs",
    "/path/to/project-b/docs"
  ]
}
```

## Step 5: Verify

Confirm to the user:
- Binary installed at `~/.prism/mcp/prism-mcp`
- MCP server registered as `prism-mcp` (user scope)
- Documentation directories configured (if any) at `~/.prism/ontology-docs.json`
- Tell the user to restart Claude Code to activate the MCP server
