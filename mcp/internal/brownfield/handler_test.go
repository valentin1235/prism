package brownfield

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	prismconfig "github.com/heechul/prism-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

func newTestHandlerStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{db: db}
	if err := s.initialize(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	SetStoreForTest(s)
	t.Cleanup(func() { SetStoreForTest(nil) })
	origScan := scanHomeForRepos
	origDiscover := discoverMCPServers
	scanHomeForRepos = ScanHomeForRepos
	discoverMCPServers = func(context.Context) ([]MCPServer, error) { return nil, nil }
	t.Cleanup(func() {
		scanHomeForRepos = origScan
		discoverMCPServers = origDiscover
	})
	return s
}

func makeRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Arguments: args,
		},
	}
}

func TestHandlerScanFormat(t *testing.T) {
	s := newTestHandlerStore(t)
	s.Register("/tmp/alpha", "alpha", "")
	s.Register("/tmp/beta", "beta", "")
	s.UpdateDefault("/tmp/beta", true)

	// Use home dir as scan_root (allowed), but it won't find new repos — that's fine.
	// We're testing the output format with pre-registered repos.
	home, _ := os.UserHomeDir()
	req := makeRequest(map[string]any{
		"action":    "scan",
		"scan_root": home,
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	text := extractText(result)
	if !strings.Contains(text, "Scan complete.") {
		t.Errorf("expected 'Scan complete.' header, got: %s", text)
	}
	if !strings.Contains(text, "alpha") {
		t.Errorf("expected 'alpha' in list, got: %s", text)
	}
	if !strings.Contains(text, "beta *") {
		t.Errorf("expected 'beta *' (default marker), got: %s", text)
	}
	if !strings.Contains(text, "Defaults (* marked):") {
		t.Errorf("expected defaults summary, got: %s", text)
	}
}

func TestHandlerScanNoReposFound(t *testing.T) {
	s := newTestHandlerStore(t)
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "alpha", Desc: ""},
			{Name: "beta", Desc: ""},
		}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	emptyDir := filepath.Join(home, "prism", ".tmp-handler-test-empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(emptyDir) })
	req := makeRequest(map[string]any{
		"action":    "scan",
		"scan_root": emptyDir,
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	text := extractText(result)
	if !strings.Contains(text, "0 repositories") {
		t.Fatalf("expected 0 repositories in message, got: %q", text)
	}
	if !strings.Contains(text, "MCP servers registered") {
		t.Fatalf("expected MCP servers in message, got: %q", text)
	}

	total, err := s.CountMCPs()
	if err != nil {
		t.Fatalf("count mcps: %v", err)
	}
	if total != 2 {
		t.Fatalf("stored mcps = %d, want 2", total)
	}
}

func TestHandlerScanNoDefaults(t *testing.T) {
	newTestHandlerStore(t)

	home, _ := os.UserHomeDir()
	req := makeRequest(map[string]any{
		"action":    "scan",
		"scan_root": home,
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(result)
	if !strings.Contains(text, "No defaults set.") {
		t.Errorf("expected 'No defaults set.', got: %s", text)
	}
}

func TestHandlerScanStoresOneRowPerDiscoveredMCP(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "alpha", Desc: ""},
			{Name: "beta", Desc: ""},
			{Name: "gamma", Desc: ""},
		}, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	total, err := s.CountMCPs()
	if err != nil {
		t.Fatalf("count mcps: %v", err)
	}
	if total != 3 {
		t.Fatalf("stored mcps = %d, want 3", total)
	}
}

func TestHandlerScanAppliesDeterministicNameCollisionPolicy(t *testing.T) {
	testCases := []struct {
		name    string
		servers []MCPServer
	}{
		{
			name: "preferred candidate first",
			servers: []MCPServer{
				{Name: "alpha", Path: "/tmp/alpha-a", Desc: "desc z"},
				{Name: "beta", Path: "/tmp/beta", Desc: "beta desc"},
				{Name: " alpha ", Path: "/tmp/alpha-b", Desc: "desc a"},
			},
		},
		{
			name: "preferred candidate last",
			servers: []MCPServer{
				{Name: " alpha ", Path: "/tmp/alpha-b", Desc: "desc a"},
				{Name: "beta", Path: "/tmp/beta", Desc: "beta desc"},
				{Name: "alpha", Path: "/tmp/alpha-a", Desc: "desc z"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestHandlerStore(t)
			alphaWinner := mcpSnapshotNameCollisionPolicy.choose(tc.servers[0], tc.servers[2])
			expectedAlpha := snapshotRowsForMCPServers([]MCPServer{alphaWinner}, testRegisteredAt())[0]
			scanHomeForRepos = func(string) ([]Repo, error) {
				return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
			}
			discoverMCPServers = func(context.Context) ([]MCPServer, error) {
				return tc.servers, nil
			}

			req := makeRequest(map[string]any{
				"action": "scan",
			})
			if _, err := HandleBrownfield(context.Background(), req); err != nil {
				t.Fatal(err)
			}

			total, err := s.CountMCPs()
			if err != nil {
				t.Fatalf("count mcps: %v", err)
			}
			if total != 2 {
				t.Fatalf("stored mcps = %d, want 2", total)
			}

			assertRuntimeSQLiteSnapshotNames(t, s, snapshotNamesForMCPServers(tc.servers))

			alphaRow := runtimeSQLiteSnapshotRowByName(t, s, "alpha")
			if alphaRow.Name != expectedAlpha.Name {
				t.Fatalf("runtime alpha name = %q, want %q", alphaRow.Name, expectedAlpha.Name)
			}
			if expectedAlpha.Path == nil {
				if alphaRow.Path != nil {
					t.Fatalf("runtime alpha path = %v, want NULL", alphaRow.Path)
				}
			} else if alphaRow.Path == nil || *alphaRow.Path != *expectedAlpha.Path {
				t.Fatalf("runtime alpha path = %v, want %q from %q", alphaRow.Path, *expectedAlpha.Path, mcpSnapshotNameCollisionPolicy.id)
			}
			if alphaRow.Desc != expectedAlpha.Desc {
				t.Fatalf("runtime alpha desc = %q, want %q from %q", alphaRow.Desc, expectedAlpha.Desc, mcpSnapshotNameCollisionPolicy.id)
			}
		})
	}
}

func TestHandlerScanStoresOnlyVisibleDeduplicatedMCPServers(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "alpha", Path: "/tmp/alpha-a", Desc: "first desc", Visible: true, VisibilityOK: true},
			{Name: "beta", Path: "/tmp/beta-hidden", Desc: "hidden desc", Visible: false, VisibilityOK: true},
			{Name: " alpha ", Path: "/tmp/alpha-b", Desc: "second desc", Visible: true, VisibilityOK: true},
			{Name: "gamma", Path: "/tmp/gamma", Desc: "gamma desc"},
		}, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after visibility-filtered scan: %v", err)
	}
	if len(mcps) != 2 {
		t.Fatalf("len(mcps) = %d, want 2", len(mcps))
	}
	if mcps[0].Name != "alpha" {
		t.Fatalf("mcps[0].Name = %q, want alpha", mcps[0].Name)
	}
	if mcps[0].Path == nil || *mcps[0].Path != "/tmp/alpha-a" {
		t.Fatalf("alpha path = %v, want /tmp/alpha-a", mcps[0].Path)
	}
	if mcps[1].Name != "gamma" {
		t.Fatalf("mcps[1].Name = %q, want gamma", mcps[1].Name)
	}
}

func TestHandlerScanUsesSharedRegisteredAtAcrossSnapshotRows(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "alpha", Desc: ""},
			{Name: "beta", Desc: ""},
			{Name: "gamma", Desc: ""},
		}, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after scan: %v", err)
	}
	if len(mcps) != 3 {
		t.Fatalf("len(mcps) = %d, want 3", len(mcps))
	}

	registeredAt := strings.TrimSpace(mcps[0].RegisteredAt)
	if registeredAt == "" {
		t.Fatal("expected registered_at to be populated for scanned MCP rows")
	}

	for _, m := range mcps[1:] {
		if got := strings.TrimSpace(m.RegisteredAt); got != registeredAt {
			t.Fatalf("mcp %q registered_at = %q, want shared scan timestamp %q", m.Name, got, registeredAt)
		}
	}
}

func TestHandlerScanRecomputesMCPDefaultsAsFalseWithoutDefaultSource(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "alpha", Desc: "Alpha tools", IsDefault: true},
			{Name: "beta", Desc: "Beta tools"},
			{Name: "gamma", Desc: "Gamma tools", IsDefault: true},
		}, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after scan: %v", err)
	}
	if len(mcps) != 3 {
		t.Fatalf("len(mcps) = %d, want 3", len(mcps))
	}

	for _, m := range mcps {
		if m.IsDefault {
			t.Fatalf("mcp %q is_default = true, want false when no MCP default source is configured", m.Name)
		}
	}

	assertRuntimeSQLiteSnapshotDefaultsAllFalse(t, s)
}

func TestHandlerScanStoresUnknownMCPPathAsNull(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "alpha", Path: "   ", Desc: ""},
		}, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	var storedPath sql.NullString
	if err := s.db.QueryRow(`
		SELECT path
		FROM mcp_server_snapshot
		WHERE name = ?
	`, "alpha").Scan(&storedPath); err != nil {
		t.Fatalf("select stored mcp path: %v", err)
	}
	if storedPath.Valid {
		t.Fatalf("stored path = %#v, want SQL NULL", storedPath)
	}
}

func TestHandlerScanKeepsVisibleMCPWhenToolMetadataUnavailable(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{
				Name: "slack",
				Path: "   ",
				Desc: "",
			},
		}, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after scan: %v", err)
	}
	if len(mcps) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(mcps))
	}
	if mcps[0].Name != "slack" {
		t.Fatalf("stored mcp name = %q, want slack", mcps[0].Name)
	}

	wantDesc := "MCP server slack (tool metadata unavailable at scan time)"
	if mcps[0].Desc != wantDesc {
		t.Fatalf("stored mcp desc = %q, want %q", mcps[0].Desc, wantDesc)
	}
}

func TestHandlerScanUsesVisibilityOnlyForSnapshotMembership(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "filesystem", Desc: "Reads and writes local files.", Visible: true, VisibilityOK: true},
			{Name: "github", Desc: "", Visible: true, VisibilityOK: true},
			{Name: "slack", Desc: "   ", Visible: true, VisibilityOK: true},
			{Name: "hidden", Desc: "", Visible: false, VisibilityOK: true},
		}, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after scan: %v", err)
	}
	if len(mcps) != 3 {
		t.Fatalf("len(mcps) = %d, want 3 visible rows only", len(mcps))
	}

	if mcps[0].Name != "filesystem" || mcps[0].Desc != "Reads and writes local files." {
		t.Fatalf("filesystem row = %#v, want resolved description to survive", mcps[0])
	}
	if mcps[1].Name != "github" || mcps[1].Desc != "MCP server github (tool metadata unavailable at scan time)" {
		t.Fatalf("github row = %#v, want visible server with fallback desc", mcps[1])
	}
	if mcps[2].Name != "slack" || mcps[2].Desc != "MCP server slack (tool metadata unavailable at scan time)" {
		t.Fatalf("slack row = %#v, want visible server with fallback desc", mcps[2])
	}
}

func TestHandlerScanRescanRemovesStaleMCPRows(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}

	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "alpha", Desc: ""},
			{Name: "beta", Desc: ""},
			{Name: "gamma", Desc: ""},
		}, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "beta", Desc: ""},
		}, nil
	}

	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	assertRuntimeSQLiteSnapshotNames(t, s, []string{"beta"})

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after rescan: %v", err)
	}
	if len(mcps) != 1 {
		t.Fatalf("len(mcps) after rescan = %d, want 1", len(mcps))
	}
	if mcps[0].Name != "beta" {
		t.Fatalf("remaining mcp after rescan = %q, want beta", mcps[0].Name)
	}

	var staleCount int
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM mcp_server_snapshot
		WHERE name IN ('alpha', 'gamma')
	`).Scan(&staleCount); err != nil {
		t.Fatalf("count stale mcps after rescan: %v", err)
	}
	if staleCount != 0 {
		t.Fatalf("stale mcps after rescan = %d, want 0", staleCount)
	}
}

func TestHandlerScanRescanToEmptyRemovesAllMCPRows(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}

	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "alpha", Desc: ""},
			{Name: "beta", Desc: ""},
		}, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return nil, nil
	}

	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	total, err := s.CountMCPs()
	if err != nil {
		t.Fatalf("count mcps after empty rescan: %v", err)
	}
	if total != 0 {
		t.Fatalf("stored mcps after empty rescan = %d, want 0", total)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after empty rescan: %v", err)
	}
	if len(mcps) != 0 {
		t.Fatalf("len(mcps) after empty rescan = %d, want 0", len(mcps))
	}

	var snapshotRows int
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM mcp_server_snapshot
	`).Scan(&snapshotRows); err != nil {
		t.Fatalf("count raw snapshot rows after empty rescan: %v", err)
	}
	if snapshotRows != 0 {
		t.Fatalf("raw snapshot rows after empty rescan = %d, want 0", snapshotRows)
	}
}

func TestHandlerScanSnapshotMatchesDeduplicatedVisibleNameSet(t *testing.T) {
	s := newTestHandlerStore(t)
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}

	if _, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "stale-a", Desc: ""},
		{Name: "stale-b", Desc: ""},
	}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	servers := []MCPServer{
		{Name: "alpha", Path: "/tmp/alpha-a", Desc: "alpha tools", Visible: true, VisibilityOK: true},
		{Name: " alpha ", Path: "/tmp/alpha-b", Desc: "older alpha", Visible: true, VisibilityOK: true},
		{Name: "beta", Desc: "", Visible: true, VisibilityOK: true},
		{Name: "hidden", Desc: "hidden tools", Visible: false, VisibilityOK: true},
		{Name: "gamma", Desc: "gamma tools"},
	}
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return servers, nil
	}

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	assertRuntimeSQLiteSnapshotNames(t, s, snapshotNamesForMCPServers(servers))
	assertRuntimeSQLiteSnapshotDefaultsAllFalse(t, s)
}

func TestHandlerScanMigratesLegacyMCPSnapshotSchemaBeforeSnapshotWrite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy-handler.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE brownfield_repos (
			path TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			desc TEXT,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE brownfield_mcps (
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			desc TEXT,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	s := &Store{db: db}
	SetStoreForTest(s)
	t.Cleanup(func() { SetStoreForTest(nil) })

	origScan := scanHomeForRepos
	origDiscover := discoverMCPServers
	scanHomeForRepos = func(string) ([]Repo, error) {
		return []Repo{{Path: "/tmp/repo1", Name: "repo1"}}, nil
	}
	discoverMCPServers = func(context.Context) ([]MCPServer, error) {
		return []MCPServer{
			{Name: "alpha", Path: "   ", Desc: ""},
		}, nil
	}
	t.Cleanup(func() {
		scanHomeForRepos = origScan
		discoverMCPServers = origDiscover
	})

	req := makeRequest(map[string]any{
		"action": "scan",
	})
	if _, err := HandleBrownfield(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	check := runtimeSQLiteMCPSnapshotCheck(t, s)
	if !check.SchemaOK {
		t.Fatal("expected RuntimeSQLiteMCPSnapshotCheck().SchemaOK after migrated scan")
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after migrated scan: %v", err)
	}
	if len(mcps) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(mcps))
	}
	if mcps[0].Name != "alpha" {
		t.Fatalf("mcps[0].Name = %q, want alpha", mcps[0].Name)
	}
	if mcps[0].Path != nil {
		t.Fatalf("mcps[0].Path = %v, want nil after NULL-path migration write", mcps[0].Path)
	}
}

func TestHandlerQuery(t *testing.T) {
	s := newTestHandlerStore(t)
	s.Register("/tmp/repo1", "repo1", "desc1")
	s.Register("/tmp/repo2", "repo2", "desc2")

	req := makeRequest(map[string]any{
		"action": "query",
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(result)
	if !strings.Contains(text, "repo1") || !strings.Contains(text, "repo2") {
		t.Errorf("expected repos in query result, got: %s", text)
	}
}

func TestHandlerSetDefaults(t *testing.T) {
	s := newTestHandlerStore(t)
	s.Register("/tmp/a", "a", "")
	s.Register("/tmp/b", "b", "")
	repos, _, _ := s.List(0, 0, false)

	indices := fmt.Sprintf("%d,%d", repos[0].RowID, repos[1].RowID)
	req := makeRequest(map[string]any{
		"action":  "set_defaults",
		"indices": indices,
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(result)
	if !strings.Contains(text, "Brownfield defaults updated!") {
		t.Errorf("expected confirmation header, got: %s", text)
	}
	if !strings.Contains(text, "Defaults: a, b") {
		t.Errorf("expected updated default names, got: %s", text)
	}
	if !strings.Contains(text, "These repos will be used as context in interviews.") {
		t.Errorf("expected usage note, got: %s", text)
	}

	defaults, _, _ := s.List(0, 0, true)
	if len(defaults) != 2 {
		t.Errorf("expected 2 defaults, got %d", len(defaults))
	}
}

func TestHandlerSetDefaultsEmpty(t *testing.T) {
	newTestHandlerStore(t)

	req := makeRequest(map[string]any{
		"action":  "set_defaults",
		"indices": "  , , ",
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(result)
	if !strings.Contains(text, "No default repos set. Interviews will run in greenfield mode.") {
		t.Errorf("expected greenfield mode message, got: %s", text)
	}
	if !strings.Contains(text, "You can set defaults anytime with: /prism:brownfield") {
		t.Errorf("expected defaults cleared message, got: %s", text)
	}
}

func TestHandlerSetDefaultsEmptyString(t *testing.T) {
	newTestHandlerStore(t)

	req := makeRequest(map[string]any{
		"action":  "set_defaults",
		"indices": "",
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(result)
	if !strings.Contains(text, "No default repos set. Interviews will run in greenfield mode.") {
		t.Errorf("expected greenfield mode message, got: %s", text)
	}
	if !strings.Contains(text, "You can set defaults anytime with: /prism:brownfield") {
		t.Errorf("expected defaults cleared message, got: %s", text)
	}
}

func TestHandlerInferAction(t *testing.T) {
	tests := []struct {
		args     map[string]any
		expected string
	}{
		{map[string]any{"action": "scan"}, "scan"},
		{map[string]any{"indices": "1,2"}, "set_defaults"},
		{map[string]any{"is_default": true}, "set_default"},
		{map[string]any{"path": "/tmp"}, "register"},
		{map[string]any{}, "query"},
	}
	for _, tt := range tests {
		got := inferAction(tt.args)
		if got != tt.expected {
			t.Errorf("inferAction(%v) = %q, want %q", tt.args, got, tt.expected)
		}
	}
}

func TestHandlerRegister(t *testing.T) {
	newTestHandlerStore(t)

	dir := t.TempDir()
	repoDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(repoDir, 0755)
	cmds := [][]string{
		{"git", "init"},
		{"git", "remote", "add", "origin", "https://github.com/test/myrepo.git"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v: %v\n%s", args, err, out)
		}
	}

	req := makeRequest(map[string]any{
		"action": "register",
		"path":   repoDir,
		"name":   "myrepo",
		"desc":   "test repo",
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(result)
	if !strings.Contains(text, "myrepo") {
		t.Errorf("expected 'myrepo' in result, got: %s", text)
	}

	repos, _, _ := store.List(0, 0, false)
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].Desc != "test repo" {
		t.Errorf("desc = %q, want 'test repo'", repos[0].Desc)
	}
}

func TestHandlerScanRootRestriction(t *testing.T) {
	newTestHandlerStore(t)

	req := makeRequest(map[string]any{
		"action":    "scan",
		"scan_root": "/etc",
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(result)
	if !strings.Contains(text, "must be within home directory") {
		t.Errorf("expected home dir restriction error, got: %s", text)
	}
}

func TestHandlerUnknownAction(t *testing.T) {
	newTestHandlerStore(t)

	req := makeRequest(map[string]any{
		"action": "invalid_action",
	})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(result)
	if !strings.Contains(text, "unknown action") {
		t.Errorf("expected unknown action error, got: %s", text)
	}
}

func TestHandlerStoreNotInitialized(t *testing.T) {
	SetStoreForTest(nil)
	req := makeRequest(map[string]any{"action": "query"})
	result, err := HandleBrownfield(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(result)
	if !strings.Contains(text, "not initialized") {
		t.Errorf("expected 'not initialized' error, got: %s", text)
	}
}

func TestInitStoreCreatesRuntimeMCPSnapshotSchema(t *testing.T) {
	resetStoreSingletonForTest(t)

	home := t.TempDir()
	t.Setenv("HOME", home)

	runtimeDBPath := prismconfig.RuntimeSQLitePath()
	if err := InitStore(runtimeDBPath); err != nil {
		t.Fatalf("InitStore(%q): %v", runtimeDBPath, err)
	}
	if store == nil {
		t.Fatal("expected InitStore to populate package store")
	}
	runtimeDBInfo, err := os.Stat(runtimeDBPath)
	if err != nil {
		t.Fatalf("stat runtime sqlite path %q: %v", runtimeDBPath, err)
	}
	if got := store.dbPathForTest(); got == "" {
		t.Fatal("expected InitStore runtime db path to resolve")
	} else {
		gotInfo, err := os.Stat(got)
		if err != nil {
			t.Fatalf("stat InitStore runtime db path %q: %v", got, err)
		}
		if !os.SameFile(gotInfo, runtimeDBInfo) {
			t.Fatalf("InitStore runtime db path = %q, want same file as %q", got, runtimeDBPath)
		}
	}

	runtimeStore, err := OpenRuntimeSQLiteStore()
	if err != nil {
		t.Fatalf("OpenRuntimeSQLiteStore(): %v", err)
	}
	defer runtimeStore.Close()

	check := runtimeSQLiteMCPSnapshotCheck(t, runtimeStore)
	if !check.SchemaOK {
		t.Fatal("expected RuntimeSQLiteMCPSnapshotCheck().SchemaOK for runtime sqlite store")
	}

	metadata := check.Metadata
	assertRuntimeSQLiteMetadataUsesDatabasePath(t, metadata, runtimeDBPath)
	assertRuntimeSQLiteTableExists(t, runtimeStore, "brownfield_repos")
	assertRuntimeSQLiteTableExists(t, runtimeStore, mcpSnapshotTableName)
	shape := check.Shape
	if !shape.TableExists {
		t.Fatalf("expected detailed runtime schema check to see %q", mcpSnapshotTableName)
	}
	if !shape.NameColumnPrimaryKey {
		t.Fatal("expected detailed runtime schema check to keep name as PRIMARY KEY")
	}
	if !shape.PathColumnNullable {
		t.Fatal("expected detailed runtime schema check to keep path nullable")
	}
	if !shape.MatchesExpectedSchema {
		t.Fatal("expected detailed runtime schema shape verdict to match expected schema")
	}

	createSQL := metadata.Table.CreateSQL
	if !strings.Contains(strings.ToLower(createSQL), "name text not null primary key") {
		t.Fatalf("expected runtime create SQL to contain name primary key, got %s", createSQL)
	}
	if strings.Contains(strings.ToLower(createSQL), "path text not null") {
		t.Fatalf("expected runtime create SQL to keep path nullable, got %s", createSQL)
	}
}

// --- helpers ---

func resetStoreSingletonForTest(t *testing.T) {
	t.Helper()

	previousStore := store
	previousErr := storeErr
	previousOnce := storeOnce

	store = nil
	storeErr = nil
	storeOnce = sync.Once{}

	t.Cleanup(func() {
		if store != nil {
			_ = store.Close()
		}
		store = previousStore
		storeErr = previousErr
		storeOnce = previousOnce
	})
}

func (s *Store) dbPathForTest() string {
	if s == nil || s.db == nil {
		return ""
	}

	var path string
	if err := s.db.QueryRow(`PRAGMA database_list`).Scan(new(int), new(string), &path); err != nil {
		return ""
	}
	return path
}

func extractText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	text, ok := r.Content[0].(mcp.TextContent)
	if !ok {
		return ""
	}
	return text.Text
}
