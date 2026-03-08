---
name: setup
description: Download and install the prism-mcp binary for your platform and register it as a user-scope MCP server
version: 1.0.0
user-invocable: true
allowed-tools: Bash, Read, Write
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
echo "Installed to $INSTALL_DIR/prism-mcp"
```

If download fails, inform the user about the error and stop.

## Step 3: Register MCP Server

Register prism-mcp as a user-scope MCP server:

```bash
claude mcp add prism-mcp -s user -- "$HOME/.prism/mcp/prism-mcp"
```

## Step 4: Verify

Confirm to the user:
- Binary installed at `~/.prism/mcp/prism-mcp`
- MCP server registered as `prism-mcp` (user scope)
- Tell the user to restart Claude Code to activate the MCP server
