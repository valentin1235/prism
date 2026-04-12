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

	scanHomeForRepos   = ScanHomeForRepos
	discoverMCPServers = DiscoverMCPServers
)

// InitStore opens the brownfield database at dbPath. Safe to call multiple
// times (sync.Once).
func InitStore(dbPath string) error {
	storeOnce.Do(func() {
		s, err := NewStoreAt(dbPath)
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

// ResetInitStoreForTest resets the sync.Once guard so InitStore can be called
// again in tests. Must only be used from test code.
func ResetInitStoreForTest() {
	storeOnce = sync.Once{}
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
	repos, err := scanHomeForRepos(scanRoot)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
	}

	// Register repos first so MCP failures don't block repo registration.
	_, bulkErr := store.BulkRegister(repos)

	var mcpErr error
	servers, discoverErr := discoverMCPServers(ctx)
	if discoverErr != nil {
		mcpErr = fmt.Errorf("mcp discovery: %w", discoverErr)
	}
	// On complete failure (error + no servers), preserve existing entries.
	// On partial success or full success, sync entries with current results.
	if discoverErr == nil || len(servers) > 0 {
		if _, err := store.SyncMCPEntries(servers); err != nil {
			mcpErr = fmt.Errorf("mcp sync: %w", err)
		}
	}

	// Fetch all entries (repo + mcp) with unified numbering
	allEntries, _, listErr := store.ListEntries(0, 0, false)
	if listErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list after scan failed: %v", listErr)), nil
	}

	// Count by type
	var repoCount, mcpCount int
	for _, e := range allEntries {
		switch e.Type {
		case "repo":
			repoCount++
		case "mcp":
			mcpCount++
		}
	}

	// Build unified list with shared rowid numbering
	var lines []string
	lines = append(lines, fmt.Sprintf("Scan complete. %d repositories, %d MCP servers registered.", repoCount, mcpCount), "")

	for _, e := range allEntries {
		marker := ""
		if e.IsDefault {
			marker = " *"
		}
		lines = append(lines, fmt.Sprintf("%2d. (%s) %s%s", e.RowID, e.Type, e.Name, marker))
	}

	lines = append(lines, "")

	// Collect defaults (any type)
	var defaultNames []string
	for _, e := range allEntries {
		if e.IsDefault {
			defaultNames = append(defaultNames, e.Name)
		}
	}
	if len(defaultNames) > 0 {
		lines = append(lines, fmt.Sprintf("Defaults (* marked): %s", strings.Join(defaultNames, ", ")))
	} else {
		lines = append(lines, "No defaults set.")
	}
	summary := strings.Join(lines, "\n")

	if bulkErr != nil {
		summary += fmt.Sprintf("\n\nWarning: %s", bulkErr.Error())
	}
	if mcpErr != nil {
		summary += fmt.Sprintf("\n\nWarning (MCP): %s", mcpErr.Error())
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

	defaultEntries, _, _ := store.ListEntries(0, 0, true)

	result := map[string]interface{}{
		"action":   "query",
		"repos":    repos,
		"total":    total,
		"offset":   offset,
		"defaults": defaultEntries,
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
	defaults, _, _ := store.ListEntries(0, 0, true)
	var names []string
	for _, entry := range defaults {
		names = append(names, entry.Name)
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
