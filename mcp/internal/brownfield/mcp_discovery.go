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
		Type    string `json:"type"`
		Command string `json:"command"`
	} `json:"transport"`
}

type claudeMCPConfig struct {
	MCPServers map[string]claudeMCPServer `json:"mcpServers"`
}

type claudeMCPServer struct {
	Command string `json:"command"`
}

// DiscoverMCPServers resolves the currently configured MCP servers for the active runtime.
func DiscoverMCPServers(ctx context.Context) ([]MCPServer, error) {
	var (
		servers []MCPServer
		err     error
	)
	if config.ResolveRuntimeBackend() == "codex" {
		servers, err = discoverCodexMCPServers(ctx)
	} else {
		servers, err = discoverClaudeMCPServers()
	}
	if err != nil {
		return nil, err
	}
	return hydrateMCPServerDescriptions(ctx, servers), nil
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
		})
	}
	return servers, nil
}

func discoverClaudeMCPServers() ([]MCPServer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	paths := []string{
		filepath.Join(".claude-plugin", ".mcp.json"),
		".mcp.json",
		filepath.Join(home, ".claude", "mcp.json"),
	}

	// Preserve every visible `/mcp` entry exactly as discovered here. Duplicate
	// names are intentionally collapsed later by the snapshot policy so one
	// documented survivor rule governs persistence across all runtimes.
	var servers []MCPServer
	for _, path := range paths {
		cfg, err := readClaudeMCPConfig(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		names := make([]string, 0, len(cfg.MCPServers))
		for name := range cfg.MCPServers {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			server := cfg.MCPServers[name]
			servers = append(servers, MCPServer{
				Name:         name,
				Path:         deriveCommandPath(server.Command),
				Desc:         "",
				Visible:      true,
				VisibilityOK: true,
				Command:      server.Command,
			})
		}
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
