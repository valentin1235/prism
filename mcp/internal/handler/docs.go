package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// AllowedDirs holds the list of allowed documentation directories.
var AllowedDirs []string

// InitFilesystem reads ~/.prism/ontology-docs.json and populates AllowedDirs.
func InitFilesystem() error {
	configPath := filepath.Join(os.Getenv("HOME"), ".prism", "ontology-docs.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config = no filesystem tools, not an error
		return nil
	}

	var config struct {
		Directories []string `json:"directories"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("invalid docs.json: %w", err)
	}

	// Resolve and validate paths
	for _, dir := range config.Directories {
		expanded, err := expandHome(dir)
		if err != nil {
			continue
		}
		abs, err := filepath.Abs(expanded)
		if err != nil {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			continue
		}
		AllowedDirs = append(AllowedDirs, abs)
	}
	return nil
}

func expandHome(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// isAllowed resolves the path (including symlinks) and checks it is within allowed directories.
// Returns the resolved absolute path for use in subsequent file operations (avoids TOCTOU).
func isAllowed(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	for _, dir := range AllowedDirs {
		if abs == dir || strings.HasPrefix(abs, dir+string(os.PathSeparator)) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("access denied: %s is outside allowed directories", path)
}

// HandleListRoots is the MCP tool handler for prism_docs_roots.
func HandleListRoots(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if len(AllowedDirs) == 0 {
		return mcp.NewToolResultText("No directories configured. Add paths to ~/.prism/docs.json"), nil
	}
	return mcp.NewToolResultText(strings.Join(AllowedDirs, "\n")), nil
}

// HandleListDir is the MCP tool handler for prism_docs_list.
func HandleListDir(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	path, _ := args["path"].(string)
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	resolved, err := isAllowed(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read directory: %v", err)), nil
	}

	var lines []string
	for _, e := range entries {
		prefix := "[FILE]"
		if e.IsDir() {
			prefix = "[DIR]"
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, e.Name()))
	}
	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

// HandleReadFile is the MCP tool handler for prism_docs_read.
func HandleReadFile(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	path, _ := args["path"].(string)
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	resolved, err := isAllowed(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to stat file: %v", err)), nil
	}
	const maxFileSize = 10 * 1024 * 1024 // 10MB
	if info.Size() > maxFileSize {
		return mcp.NewToolResultError(fmt.Sprintf("file too large: %d bytes (max %d bytes) — use head/tail to read portions", info.Size(), maxFileSize)), nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read file: %v", err)), nil
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	// Apply head/tail if specified
	headN, hasHead := args["head"].(float64)
	tailN, hasTail := args["tail"].(float64)

	if hasHead && int(headN) > 0 && int(headN) < totalLines {
		lines = lines[:int(headN)]
	} else if hasTail && int(tailN) > 0 && int(tailN) < totalLines {
		lines = lines[totalLines-int(tailN):]
	}

	content := strings.Join(lines, "\n")

	// Limit to 500KB
	if len(content) > 500*1024 {
		content = content[:500*1024] + fmt.Sprintf("\n... (truncated at 500KB, total %d lines — use head/tail to paginate)", totalLines)
	}
	return mcp.NewToolResultText(content), nil
}

// HandleSearchFiles is the MCP tool handler for prism_docs_search.
func HandleSearchFiles(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	path, _ := args["path"].(string)
	pattern, _ := args["pattern"].(string)
	if path == "" || pattern == "" {
		return mcp.NewToolResultError("path and pattern are required"), nil
	}

	resolved, err := isAllowed(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var matches []string
	maxResults := 100

	err = filepath.WalkDir(resolved, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if len(matches) >= maxResults {
			return filepath.SkipAll
		}
		// Skip hidden dirs and node_modules
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		matched, _ := filepath.Match(pattern, d.Name())
		if matched {
			matches = append(matches, p)
		}
		return nil
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	if len(matches) == 0 {
		return mcp.NewToolResultText("No matches found"), nil
	}
	return mcp.NewToolResultText(strings.Join(matches, "\n")), nil
}
