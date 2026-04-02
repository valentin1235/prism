package brownfield

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// store is the package-level singleton, initialized by InitStore.
var (
	store     *Store
	storeOnce sync.Once
	storeErr  error
)

// InitStore opens the brownfield database. Safe to call multiple times (sync.Once).
func InitStore() error {
	storeOnce.Do(func() {
		s, err := NewStore()
		if err != nil {
			storeErr = err
			return
		}
		store = s
	})
	return storeErr
}

// SetStoreForTest replaces the package-level store (testing only).
func SetStoreForTest(s *Store) {
	store = s
}

// HandleBrownfield is the MCP tool handler for prism_brownfield.
func HandleBrownfield(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if store == nil {
		return mcp.NewToolResultError("brownfield store not initialized"), nil
	}

	args := request.Params.Arguments
	action := inferAction(args)

	switch action {
	case "scan":
		return handleScan(ctx, args)
	case "register":
		return handleRegister(ctx, args)
	case "query":
		return handleQuery(args)
	case "set_default":
		return handleSetDefault(args)
	case "set_defaults":
		return handleSetDefaults(args)
	case "generate_desc":
		return handleGenerateDesc(ctx, args)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown action: %s", action)), nil
	}
}

// inferAction auto-detects the action from parameters when action is omitted.
func inferAction(args map[string]interface{}) string {
	if a, ok := args["action"].(string); ok && a != "" {
		return a
	}
	if _, ok := args["indices"]; ok {
		return "set_defaults"
	}
	if _, ok := args["is_default"]; ok {
		return "set_default"
	}
	if _, ok := args["path"]; ok {
		return "register"
	}
	return "query"
}

// --- Action handlers ---

func handleScan(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	scanRoot, _ := args["scan_root"].(string)
	if scanRoot != "" {
		abs, err := filepath.Abs(scanRoot)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid scan_root: %v", err)), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("cannot resolve home directory: %v", err)), nil
		}
		homePrefix := home + string(filepath.Separator)
		if abs != home && !strings.HasPrefix(abs, homePrefix) {
			return mcp.NewToolResultError("scan_root must be within home directory"), nil
		}
		scanRoot = abs
	}
	repos, err := ScanHomeForRepos(scanRoot)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
	}

	if len(repos) == 0 {
		return mcp.NewToolResultText("No GitHub repositories found in your home directory."), nil
	}

	_, bulkErr := store.BulkRegister(repos)

	// Fetch all repos with defaults to build formatted list (matches ouroboros format)
	allRepos, total, listErr := store.List(0, 0, false)
	if listErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list after scan failed: %v", listErr)), nil
	}
	defaults, _, _ := store.List(0, 0, true)

	// Build compact numbered list: "{rowid}. {name} *"
	var lines []string
	lines = append(lines, fmt.Sprintf("Scan complete. %d repositories registered.", total), "")
	for _, r := range allRepos {
		marker := ""
		if r.IsDefault {
			marker = " *"
		}
		lines = append(lines, fmt.Sprintf("%2d. %s%s", r.RowID, r.Name, marker))
	}
	lines = append(lines, "")
	if len(defaults) > 0 {
		var ids, names []string
		for _, d := range defaults {
			ids = append(ids, fmt.Sprintf("%d", d.RowID))
			names = append(names, d.Name)
		}
		lines = append(lines, fmt.Sprintf("Defaults (* marked): %s (%s)", strings.Join(ids, ", "), strings.Join(names, ", ")))
	} else {
		lines = append(lines, "No defaults set.")
	}
	summary := strings.Join(lines, "\n")

	if bulkErr != nil {
		summary += fmt.Sprintf("\n\nWarning: %s", bulkErr.Error())
	}

	return mcp.NewToolResultText(summary), nil
}

func handleRegister(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return mcp.NewToolResultError("path is required for register action"), nil
	}

	// Resolve path
	abs, err := filepath.Abs(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid path: %v", err)), nil
	}
	if evalPath, err := filepath.EvalSymlinks(abs); err == nil {
		abs = evalPath
	}

	// Verify directory exists
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return mcp.NewToolResultError(fmt.Sprintf("directory not found: %s", abs)), nil
	}

	name, _ := args["name"].(string)
	if name == "" {
		name = filepath.Base(abs)
	}

	desc, _ := args["desc"].(string)
	if desc == "" {
		// Auto-generate from README
		generated, err := GenerateDesc(ctx, abs)
		if err == nil && generated != "" {
			desc = generated
		}
	}

	if err := store.Register(abs, name, desc); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("register failed: %v", err)), nil
	}

	result := map[string]interface{}{
		"action": "register",
		"path":   abs,
		"name":   name,
		"desc":   desc,
	}
	return jsonResult(result)
}

func handleQuery(args map[string]interface{}) (*mcp.CallToolResult, error) {
	offset := intArg(args, "offset", 0)
	limit := intArg(args, "limit", 0)
	defaultOnly, _ := args["default_only"].(bool)

	repos, total, err := store.List(offset, limit, defaultOnly)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}

	// Collect defaults
	var defaults []Repo
	for _, r := range repos {
		if r.IsDefault {
			defaults = append(defaults, r)
		}
	}
	// If querying all, also get full default list
	if !defaultOnly {
		allDefaults, _, _ := store.List(0, 0, true)
		defaults = allDefaults
	}

	result := map[string]interface{}{
		"action":   "query",
		"repos":    repos,
		"total":    total,
		"offset":   offset,
		"defaults": defaults,
		"count":    len(repos),
	}
	return jsonResult(result)
}

func handleSetDefault(args map[string]interface{}) (*mcp.CallToolResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return mcp.NewToolResultError("path is required for set_default action"), nil
	}

	isDefault := true
	if v, ok := args["is_default"].(bool); ok {
		isDefault = v
	}

	if err := store.UpdateDefault(path, isDefault); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set_default failed: %v", err)), nil
	}

	result := map[string]interface{}{
		"action":     "set_default",
		"path":       path,
		"is_default": isDefault,
	}
	return jsonResult(result)
}

func handleSetDefaults(args map[string]interface{}) (*mcp.CallToolResult, error) {
	indicesStr, _ := args["indices"].(string)

	// Empty string means "clear all defaults" (indices="" for "none")
	var ids []int64
	if indicesStr != "" {
		parts := strings.Split(indicesStr, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			id, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid id: %s", p)), nil
			}
			ids = append(ids, id)
		}
	}

	if err := store.SetDefaultsByRowIDs(ids); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set_defaults failed: %v", err)), nil
	}

	if len(ids) == 0 {
		return mcp.NewToolResultText("No default repos set. Interviews will run in greenfield mode.\nYou can set defaults anytime with: /prism:brownfield"), nil
	}

	// Return updated defaults
	defaults, _, _ := store.List(0, 0, true)
	var names []string
	for _, repo := range defaults {
		names = append(names, repo.Name)
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		"Brownfield defaults updated!\nDefaults: %s\n\nThese repos will be used as context in interviews.",
		strings.Join(names, ", "),
	)), nil
}

func handleGenerateDesc(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return mcp.NewToolResultError("path is required for generate_desc action"), nil
	}

	// Resolve path consistently with handleRegister
	abs, err := filepath.Abs(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid path: %v", err)), nil
	}
	if evalPath, err := filepath.EvalSymlinks(abs); err == nil {
		abs = evalPath
	}
	path = abs

	desc, err := GenerateDesc(ctx, path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("description generation failed: %v", err)), nil
	}
	if desc == "" {
		return mcp.NewToolResultError("no README found in repository"), nil
	}

	// Update in database
	if err := store.UpdateDesc(path, desc); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update description: %v", err)), nil
	}

	result := map[string]interface{}{
		"action": "generate_desc",
		"path":   path,
		"desc":   desc,
	}
	return jsonResult(result)
}

// --- Helpers ---

func jsonResult(v interface{}) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal failed: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func intArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}
