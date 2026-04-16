package brownfield

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestDiscoverClaudeMCPServersPreservesDuplicateVisibleNamesUntilSnapshotNormalization(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), "home")

	writeJSON := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	// User scope config (~/.claude.json) with "dup" and "home-only"
	writeJSON(filepath.Join(homeDir, ".claude.json"), `{
  "mcpServers": {
    "dup": {"command": "/tmp/zeta"},
    "home-only": {"command": "/tmp/home-only"}
  }
}`)
	// Plugin config with "dup" and "plugin-only"
	writeJSON(filepath.Join(homeDir, ".claude", "plugins", "marketplaces", "test-plugin", ".mcp.json"), `{
  "mcpServers": {
    "dup": {"command": "/tmp/alpha"},
    "plugin-only": {"command": "/tmp/plugin-only"}
  }
}`)

	t.Setenv("HOME", homeDir)

	servers, err := discoverClaudeMCPServers()
	if err != nil {
		t.Fatalf("discoverClaudeMCPServers(): %v", err)
	}
	if len(servers) != 4 {
		t.Fatalf("len(servers) = %d, want 4 visible entries before snapshot normalization", len(servers))
	}

	var dupPaths []string
	for _, server := range servers {
		if server.Name == "dup" {
			dupPaths = append(dupPaths, server.Path)
		}
	}
	if len(dupPaths) != 2 {
		t.Fatalf("duplicate visible entries = %v, want both dup candidates preserved", dupPaths)
	}
	// User config comes first, then plugin
	if dupPaths[0] != "/tmp/zeta" || dupPaths[1] != "/tmp/alpha" {
		t.Fatalf("dup paths = %v, want [/tmp/zeta /tmp/alpha] in discovery order (user then plugin)", dupPaths)
	}

	snapshots := snapshotRowsForMCPServers(servers, testRegisteredAt())
	if len(snapshots) != 3 {
		t.Fatalf("len(snapshots) = %d, want 3 canonical names after normalization", len(snapshots))
	}
	if snapshots[0].Name != "dup" {
		t.Fatalf("snapshots[0].Name = %q, want dup", snapshots[0].Name)
	}
	if snapshots[0].Path == nil || *snapshots[0].Path != "/tmp/alpha" {
		t.Fatalf("dup survivor path = %v, want /tmp/alpha from %q", snapshots[0].Path, mcpSnapshotNameCollisionPolicy.description)
	}
}

func TestNormalizeVisibleMCPServersForSnapshotNameCollisionPolicyIsPermutationStable(t *testing.T) {
	duplicates := []MCPServer{
		{Name: "alpha", Path: "", Desc: "resolved but no approved path"},
		{Name: " alpha ", Path: "/tmp/alpha-z", Desc: ""},
		{Name: "alpha", Path: "/tmp/alpha-b", Desc: "desc b"},
		{Name: "alpha", Path: "/tmp/alpha-a", Desc: "desc a"},
	}

	expected := MCPServer{Name: "alpha", Path: "/tmp/alpha-a", Desc: "desc a"}
	expectedSnapshot := snapshotRowsForMCPServers([]MCPServer{expected}, testRegisteredAt())[0]

	permutations := permuteMCPServers(duplicates)
	if len(permutations) == 0 {
		t.Fatal("expected duplicate permutations")
	}

	for i, permutation := range permutations {
		servers := append(append([]MCPServer{}, permutation...), MCPServer{Name: "beta", Path: "/tmp/beta", Desc: "beta desc"})
		normalized := normalizeVisibleMCPServersForSnapshot(servers)
		if len(normalized) != 2 {
			t.Fatalf("permutation %d normalized len = %d, want 2", i, len(normalized))
		}

		alpha := normalized[0]
		if alpha.Name != expected.Name {
			t.Fatalf("permutation %d alpha name = %q, want %q", i, alpha.Name, expected.Name)
		}
		if alpha.Path != expected.Path {
			t.Fatalf("permutation %d alpha path = %q, want %q from %q", i, alpha.Path, expected.Path, mcpSnapshotNameCollisionPolicy.id)
		}
		if alpha.Desc != expected.Desc {
			t.Fatalf("permutation %d alpha desc = %q, want %q from %q", i, alpha.Desc, expected.Desc, mcpSnapshotNameCollisionPolicy.id)
		}

		snapshots := snapshotRowsForMCPServers(servers, testRegisteredAt())
		if len(snapshots) != 2 {
			t.Fatalf("permutation %d snapshot len = %d, want 2", i, len(snapshots))
		}
		if snapshots[0].Name != expectedSnapshot.Name {
			t.Fatalf("permutation %d alpha snapshot name = %q, want %q", i, snapshots[0].Name, expectedSnapshot.Name)
		}
		if snapshots[0].Path == nil || expectedSnapshot.Path == nil || *snapshots[0].Path != *expectedSnapshot.Path {
			t.Fatalf("permutation %d alpha snapshot path = %v, want %v", i, snapshots[0].Path, expectedSnapshot.Path)
		}
		if snapshots[0].Desc != expectedSnapshot.Desc {
			t.Fatalf("permutation %d alpha snapshot desc = %q, want %q", i, snapshots[0].Desc, expectedSnapshot.Desc)
		}
		if snapshots[0].IsDefault != expectedSnapshot.IsDefault {
			t.Fatalf("permutation %d alpha snapshot is_default = %t, want %t", i, snapshots[0].IsDefault, expectedSnapshot.IsDefault)
		}
		if snapshots[0].RegisteredAt != expectedSnapshot.RegisteredAt {
			t.Fatalf("permutation %d alpha snapshot registered_at = %q, want %q", i, snapshots[0].RegisteredAt, expectedSnapshot.RegisteredAt)
		}
	}
}

func permuteMCPServers(servers []MCPServer) [][]MCPServer {
	if len(servers) == 0 {
		return nil
	}

	items := append([]MCPServer{}, servers...)
	var out [][]MCPServer
	var visit func(int)
	visit = func(index int) {
		if index == len(items) {
			out = append(out, append([]MCPServer{}, items...))
			return
		}
		for i := index; i < len(items); i++ {
			items[index], items[i] = items[i], items[index]
			visit(index + 1)
			items[index], items[i] = items[i], items[index]
		}
	}
	visit(0)
	return out
}

type fakeMCPToolListingClient struct {
	tools []mcp.Tool
	err   error
}

func (f *fakeMCPToolListingClient) Initialize(context.Context, mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &mcp.InitializeResult{}, nil
}

func (f *fakeMCPToolListingClient) ListTools(context.Context, mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &mcp.ListToolsResult{Tools: f.tools}, nil
}

func (f *fakeMCPToolListingClient) Close() error {
	return nil
}

func TestHydrateMCPServerDescriptionsUsesResolvedToolMetadata(t *testing.T) {
	originalFactory := newMCPToolListingClient
	newMCPToolListingClient = func(command string, args ...string) (mcpToolListingClient, error) {
		return &fakeMCPToolListingClient{
			tools: []mcp.Tool{
				{Name: "get_thread_replies", Description: "Fetch Slack thread replies."},
				{Name: "search_messages", Description: "Search Slack messages."},
			},
		}, nil
	}
	t.Cleanup(func() { newMCPToolListingClient = originalFactory })

	hydrated := hydrateMCPServerDescriptions(context.Background(), []MCPServer{
		{Name: "slack", Command: "mock-slack-mcp"},
	})
	if len(hydrated) != 1 {
		t.Fatalf("len(hydrated) = %d, want 1", len(hydrated))
	}

	want := "Fetch Slack thread replies; Search Slack messages."
	if hydrated[0].Desc != want {
		t.Fatalf("hydrated desc = %q, want %q", hydrated[0].Desc, want)
	}

	snapshots := snapshotRowsForMCPServers(hydrated, testRegisteredAt())
	if len(snapshots) != 1 {
		t.Fatalf("len(snapshots) = %d, want 1", len(snapshots))
	}
	if snapshots[0].Desc != want {
		t.Fatalf("snapshot desc = %q, want resolved tool summary %q", snapshots[0].Desc, want)
	}
}

func TestHydrateMCPServerDescriptionsLeavesFallbackWhenToolMetadataFails(t *testing.T) {
	originalFactory := newMCPToolListingClient
	newMCPToolListingClient = func(command string, args ...string) (mcpToolListingClient, error) {
		return &fakeMCPToolListingClient{err: context.DeadlineExceeded}, nil
	}
	t.Cleanup(func() { newMCPToolListingClient = originalFactory })

	hydrated := hydrateMCPServerDescriptions(context.Background(), []MCPServer{
		{Name: "slack", Command: "mock-slack-mcp"},
	})
	if len(hydrated) != 1 {
		t.Fatalf("len(hydrated) = %d, want 1", len(hydrated))
	}
	if hydrated[0].Desc != "" {
		t.Fatalf("hydrated desc = %q, want unresolved description to remain blank before snapshot fallback", hydrated[0].Desc)
	}

	snapshots := snapshotRowsForMCPServers(hydrated, testRegisteredAt())
	if len(snapshots) != 1 {
		t.Fatalf("len(snapshots) = %d, want 1", len(snapshots))
	}

	want := "MCP server slack (tool metadata unavailable at scan time)"
	if snapshots[0].Desc != want {
		t.Fatalf("snapshot desc = %q, want deterministic fallback %q", snapshots[0].Desc, want)
	}
}
