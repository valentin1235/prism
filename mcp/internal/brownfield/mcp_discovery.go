package brownfield

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/heechul/prism-mcp/internal/config"
)

type codexMCPListEntry struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Transport struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	} `json:"transport"`
}

type claudeMCPConfig struct {
	MCPServers map[string]claudeMCPServer `json:"mcpServers"`
}

type claudeMCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// DiscoverMCPServers resolves the currently configured MCP servers for the active runtime.
// When adaptor is non-empty it overrides the config-file backend selection.
func DiscoverMCPServers(ctx context.Context, adaptor string) ([]MCPServer, error) {
	backend := strings.ToLower(strings.TrimSpace(adaptor))
	if backend == "" {
		backend = config.ResolveRuntimeBackend()
	}
	var (
		servers []MCPServer
		err     error
	)
	if backend == "codex" {
		servers, err = discoverCodexMCPServers(ctx)
	} else {
		servers, err = discoverClaudeMCPServers()
	}
	// Hydrate even on partial success (err != nil but servers non-empty),
	// so valid servers from successful config files are still included.
	hydrated := hydrateMCPServerDescriptions(ctx, servers)
	return hydrated, err
}

func discoverCodexMCPServers(ctx context.Context) ([]MCPServer, error) {
	cmd := exec.CommandContext(ctx, "codex", "mcp", "list", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("codex mcp list failed: %w", err)
	}

	var entries []codexMCPListEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parse codex mcp list: %w", err)
	}

	servers := make([]MCPServer, 0, len(entries))
	for _, entry := range entries {
		if !entry.Enabled {
			continue
		}
		servers = append(servers, MCPServer{
			Name:         entry.Name,
			Path:         deriveCommandPath(entry.Transport.Command),
			Desc:         "",
			Visible:      entry.Enabled,
			VisibilityOK: true,
			Command:      entry.Transport.Command,
			Args:         entry.Transport.Args,
			Env:          entry.Transport.Env,
		})
	}
	return servers, nil
}

func discoverClaudeMCPServers() ([]MCPServer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	// Discover MCPs from official Claude Code config paths:
	// 1. User scope: ~/.claude.json (mcpServers key)
	// 2. Project scope: .mcp.json (handled externally)
	// 3. Installed plugins: ~/.claude/plugins/marketplaces/*/.mcp.json
	var servers []MCPServer

	// User scope MCP config (~/.claude.json)
	userConfig := filepath.Join(home, ".claude.json")
	if s, err := readMCPServersFromConfig(userConfig); err == nil {
		servers = append(servers, s...)
	} else if !os.IsNotExist(err) {
		// non-fatal: ~/.claude.json may contain fields outside mcpServers
	}

	// Plugin MCPs from installed marketplace plugins
	pluginPattern := filepath.Join(home, ".claude", "plugins", "marketplaces", "*", ".mcp.json")
	pluginPaths, _ := filepath.Glob(pluginPattern)
	sort.Strings(pluginPaths)
	var parseErrors []string
	for _, path := range pluginPaths {
		pluginRoot := filepath.Dir(path)
		s, err := readMCPServersFromConfig(path)
		if err != nil {
			if !os.IsNotExist(err) {
				parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", filepath.Base(pluginRoot), err))
			}
			continue
		}
		// Expand ${CLAUDE_PLUGIN_ROOT} in command and args
		for i := range s {
			s[i].Command = expandPluginRoot(s[i].Command, pluginRoot)
			for j := range s[i].Args {
				s[i].Args[j] = expandPluginRoot(s[i].Args[j], pluginRoot)
			}
		}
		servers = append(servers, s...)
	}
	if len(parseErrors) > 0 {
		return servers, fmt.Errorf("plugin mcp config warnings: %s", strings.Join(parseErrors, "; "))
	}

	return servers, nil
}

func readMCPServersFromConfig(path string) ([]MCPServer, error) {
	cfg, err := readClaudeMCPConfig(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(cfg.MCPServers))
	for name := range cfg.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)

	servers := make([]MCPServer, 0, len(names))
	for _, name := range names {
		server := cfg.MCPServers[name]
		servers = append(servers, MCPServer{
			Name:         name,
			Path:         deriveCommandPath(server.Command),
			Desc:         "",
			Visible:      true,
			VisibilityOK: true,
			Command:      server.Command,
			Args:         server.Args,
			Env:          server.Env,
		})
	}
	return servers, nil
}

func readClaudeMCPConfig(path string) (*claudeMCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg claudeMCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

func deriveCommandPath(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if filepath.IsAbs(command) {
		return command
	}
	return ""
}

func expandPluginRoot(s, pluginRoot string) string {
	return strings.ReplaceAll(s, "${CLAUDE_PLUGIN_ROOT}", pluginRoot)
}
