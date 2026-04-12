package brownfield

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	s := &Store{db: db}
	if err := s.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testRegisteredAt() string {
	return time.Date(2026, time.April, 12, 3, 4, 5, 0, time.UTC).Format(time.RFC3339Nano)
}

func TestRegisterAndList(t *testing.T) {
	s := newTestStore(t)

	if err := s.Register("/tmp/repo-a", "repo-a", "desc A"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.Register("/tmp/repo-b", "repo-b", ""); err != nil {
		t.Fatalf("register: %v", err)
	}

	repos, total, err := s.List(0, 0, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(repos) != 2 {
		t.Errorf("repos = %d, want 2", len(repos))
	}
	if repos[0].Name != "repo-a" {
		t.Errorf("repos[0].Name = %q, want repo-a", repos[0].Name)
	}
}

func TestRegisterUpsert(t *testing.T) {
	s := newTestStore(t)

	s.Register("/tmp/repo-a", "repo-a", "original desc")
	// Upsert with empty desc should preserve existing desc
	s.Register("/tmp/repo-a", "repo-a", "")

	repos, _, _ := s.List(0, 0, false)
	if repos[0].Desc != "original desc" {
		t.Errorf("desc = %q, want 'original desc'", repos[0].Desc)
	}

	// Upsert with new desc should update
	s.Register("/tmp/repo-a", "repo-a", "new desc")
	repos, _, _ = s.List(0, 0, false)
	if repos[0].Desc != "new desc" {
		t.Errorf("desc = %q, want 'new desc'", repos[0].Desc)
	}
}

func TestBulkRegister(t *testing.T) {
	s := newTestStore(t)

	repos := []Repo{
		{Path: "/tmp/r1", Name: "r1"},
		{Path: "/tmp/r2", Name: "r2"},
		{Path: "/tmp/r3", Name: "r3"},
	}
	count, err := s.BulkRegister(repos)
	if err != nil {
		t.Fatalf("bulk register: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	all, total, _ := s.List(0, 0, false)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(all) != 3 {
		t.Errorf("all = %d, want 3", len(all))
	}
}

func TestUpdateDefault(t *testing.T) {
	s := newTestStore(t)

	s.Register("/tmp/repo-a", "repo-a", "")
	s.Register("/tmp/repo-b", "repo-b", "")

	// Set repo-a as default
	if err := s.UpdateDefault("/tmp/repo-a", true); err != nil {
		t.Fatalf("update default: %v", err)
	}

	defaults, _, _ := s.List(0, 0, true)
	if len(defaults) != 1 || defaults[0].Path != "/tmp/repo-a" {
		t.Errorf("expected repo-a as default, got %v", defaults)
	}

	// Set repo-b as default too (multi-default)
	s.UpdateDefault("/tmp/repo-b", true)
	defaults, _, _ = s.List(0, 0, true)
	if len(defaults) != 2 {
		t.Errorf("expected 2 defaults, got %d", len(defaults))
	}

	// Unset repo-a
	s.UpdateDefault("/tmp/repo-a", false)
	defaults, _, _ = s.List(0, 0, true)
	if len(defaults) != 1 || defaults[0].Path != "/tmp/repo-b" {
		t.Errorf("expected only repo-b as default, got %v", defaults)
	}
}

func TestUpdateDefaultNotFound(t *testing.T) {
	s := newTestStore(t)

	err := s.UpdateDefault("/nonexistent", true)
	if err == nil {
		t.Error("expected error for nonexistent repo")
	}
}

func TestSetDefaultsByRowIDs(t *testing.T) {
	s := newTestStore(t)

	s.Register("/tmp/r1", "r1", "")
	s.Register("/tmp/r2", "r2", "")
	s.Register("/tmp/r3", "r3", "")

	// Get rowids
	repos, _, _ := s.List(0, 0, false)
	ids := []int64{repos[0].RowID, repos[2].RowID}

	if err := s.SetDefaultsByRowIDs(ids); err != nil {
		t.Fatalf("set defaults by ids: %v", err)
	}

	defaults, _, _ := s.List(0, 0, true)
	if len(defaults) != 2 {
		t.Errorf("expected 2 defaults, got %d", len(defaults))
	}

	// Replace with just one
	s.SetDefaultsByRowIDs([]int64{repos[1].RowID})
	defaults, _, _ = s.List(0, 0, true)
	if len(defaults) != 1 || defaults[0].Path != "/tmp/r2" {
		t.Errorf("expected only r2 as default, got %v", defaults)
	}
}

func TestListPagination(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 5; i++ {
		s.Register("/tmp/r"+string(rune('a'+i)), "r"+string(rune('a'+i)), "")
	}

	repos, total, _ := s.List(0, 2, false)
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(repos) != 2 {
		t.Errorf("repos = %d, want 2", len(repos))
	}

	repos, _, _ = s.List(2, 2, false)
	if len(repos) != 2 {
		t.Errorf("repos = %d, want 2", len(repos))
	}

	repos, _, _ = s.List(4, 2, false)
	if len(repos) != 1 {
		t.Errorf("repos = %d, want 1", len(repos))
	}
}

func TestUpdateDesc(t *testing.T) {
	s := newTestStore(t)

	s.Register("/tmp/repo-a", "repo-a", "")
	s.UpdateDesc("/tmp/repo-a", "updated description")

	repos, _, _ := s.List(0, 0, false)
	if repos[0].Desc != "updated description" {
		t.Errorf("desc = %q, want 'updated description'", repos[0].Desc)
	}
}

func TestReplaceMCPsSnapshotAppliesDeterministicNameCollisionPolicy(t *testing.T) {
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
			s := newTestStore(t)
			alphaWinner := mcpSnapshotNameCollisionPolicy.choose(tc.servers[0], tc.servers[2])
			expectedAlpha := snapshotRowsForMCPServers([]MCPServer{alphaWinner}, testRegisteredAt())[0]

			count, err := s.SyncMCPEntries(tc.servers)
			if err != nil {
				t.Fatalf("replace mcps snapshot: %v", err)
			}
			if count != 2 {
				t.Fatalf("snapshot count = %d, want 2", count)
			}

			mcps, err := s.ListMCPs()
			if err != nil {
				t.Fatalf("list mcps: %v", err)
			}
			if len(mcps) != 2 {
				t.Fatalf("len(mcps) = %d, want 2", len(mcps))
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

func TestMCPSnapshotNameCollisionPolicyIsSingleSelectionSource(t *testing.T) {
	current := MCPServer{Name: "alpha", Path: "/tmp/alpha-z", Desc: "desc z"}
	candidate := MCPServer{Name: " alpha ", Path: "/tmp/alpha-a", Desc: "desc a"}

	got := mcpSnapshotNameCollisionPolicy.choose(current, candidate)
	if got.Path != "/tmp/alpha-a" {
		t.Fatalf("policy survivor path = %q, want /tmp/alpha-a", got.Path)
	}
	if mcpSnapshotNameCollisionPolicy.id == "" {
		t.Fatal("expected non-empty collision policy id")
	}
	if mcpSnapshotNameCollisionPolicy.description == "" {
		t.Fatal("expected non-empty collision policy description")
	}
	if mcpSnapshotOntology.nameCollisionPolicyID != mcpSnapshotNameCollisionPolicy.id {
		t.Fatalf("ontology collision policy id = %q, want %q", mcpSnapshotOntology.nameCollisionPolicyID, mcpSnapshotNameCollisionPolicy.id)
	}
	if mcpSnapshotOntology.nameCollisionPolicyDescription != mcpSnapshotNameCollisionPolicy.description {
		t.Fatalf("ontology collision policy description = %q, want %q", mcpSnapshotOntology.nameCollisionPolicyDescription, mcpSnapshotNameCollisionPolicy.description)
	}
}

func TestReplaceMCPsSnapshotNameCollisionPolicyIsPermutationStableBeforeSQLiteInsert(t *testing.T) {
	duplicates := []MCPServer{
		{Name: "alpha", Path: "", Desc: "resolved but no approved path"},
		{Name: " alpha ", Path: "/tmp/alpha-z", Desc: ""},
		{Name: "alpha", Path: "/tmp/alpha-b", Desc: "desc b"},
		{Name: "alpha", Path: "/tmp/alpha-a", Desc: "desc a"},
	}
	expectedAlpha := snapshotRowsForMCPServers([]MCPServer{{Name: "alpha", Path: "/tmp/alpha-a", Desc: "desc a"}}, testRegisteredAt())[0]

	permutations := permuteMCPServers(duplicates)
	if len(permutations) == 0 {
		t.Fatal("expected duplicate permutations")
	}

	for i, permutation := range permutations {
		s := newTestStore(t)
		servers := append(append([]MCPServer{}, permutation...), MCPServer{Name: "beta", Path: "/tmp/beta", Desc: "beta desc"})

		count, err := s.SyncMCPEntries(servers)
		if err != nil {
			t.Fatalf("permutation %d replace mcps snapshot: %v", i, err)
		}
		if count != 2 {
			t.Fatalf("permutation %d snapshot count = %d, want 2", i, count)
		}

		assertRuntimeSQLiteSnapshotNames(t, s, []string{"alpha", "beta"})

		alphaRow := runtimeSQLiteSnapshotRowByName(t, s, "alpha")
		if alphaRow.Path == nil || *alphaRow.Path != *expectedAlpha.Path {
			t.Fatalf("permutation %d runtime alpha path = %v, want %q from %q", i, alphaRow.Path, *expectedAlpha.Path, mcpSnapshotNameCollisionPolicy.id)
		}
		if alphaRow.Desc != expectedAlpha.Desc {
			t.Fatalf("permutation %d runtime alpha desc = %q, want %q from %q", i, alphaRow.Desc, expectedAlpha.Desc, mcpSnapshotNameCollisionPolicy.id)
		}
	}
}

func TestScanSkipDirs(t *testing.T) {
	// Verify skip dirs are properly configured
	expectedSkips := []string{
		"node_modules", ".venv", "__pycache__", ".cache",
		"Library", ".Trash", "vendor", ".gradle",
		"build", "dist", "target", ".tox",
		".mypy_cache", ".pytest_cache", ".cargo",
		"Pods", ".npm", ".nvm", ".local",
		".docker", ".rustup", "go",
	}
	for _, d := range expectedSkips {
		if !skipDirs[d] {
			t.Errorf("expected %q in skipDirs", d)
		}
	}
}

func TestScanEmptyDir(t *testing.T) {
	dir := t.TempDir()
	repos, err := ScanHomeForRepos(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos from empty dir, got %d", len(repos))
	}
}

func TestScanWithGitRepo(t *testing.T) {
	dir := t.TempDir()

	// Create a real git repo with GitHub origin using git commands
	repoDir := filepath.Join(dir, "my-repo")
	os.MkdirAll(repoDir, 0755)

	cmds := [][]string{
		{"git", "init"},
		{"git", "remote", "add", "origin", "https://github.com/user/my-repo.git"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %v\n%s", args, err, out)
		}
	}

	repos, err := ScanHomeForRepos(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(repos))
		return
	}
	if repos[0].Name != "my-repo" {
		t.Errorf("name = %q, want my-repo", repos[0].Name)
	}
}

func TestReadReadme(t *testing.T) {
	dir := t.TempDir()

	// No readme
	if content := readReadme(dir); content != "" {
		t.Errorf("expected empty, got %q", content)
	}

	// Create README.md
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Project\nA test project"), 0644)
	content := readReadme(dir)
	if content == "" {
		t.Error("expected README content")
	}
	if content != "# My Project\nA test project" {
		t.Errorf("unexpected content: %q", content)
	}

	// CLAUDE.md takes priority
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("Claude project info"), 0644)
	content = readReadme(dir)
	if content != "Claude project info" {
		t.Errorf("CLAUDE.md should take priority, got: %q", content)
	}
}

func TestReadReadmeTruncation(t *testing.T) {
	dir := t.TempDir()

	// Create a large README
	largeContent := make([]byte, 5000)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	os.WriteFile(filepath.Join(dir, "README.md"), largeContent, 0644)

	content := readReadme(dir)
	if len(content) != maxReadmeChars {
		t.Errorf("content length = %d, want %d", len(content), maxReadmeChars)
	}
}

func TestSetDefaultsByRowIDsInvalidID(t *testing.T) {
	s := newTestStore(t)
	s.Register("/tmp/r1", "r1", "")
	repos, _, _ := s.List(0, 0, false)
	// Set r1 as default first
	if err := s.SetDefaultsByRowIDs([]int64{repos[0].RowID}); err != nil {
		t.Fatalf("set defaults: %v", err)
	}

	// Try to set a non-existent rowid
	err := s.SetDefaultsByRowIDs([]int64{99999})
	if err == nil {
		t.Error("expected error for invalid rowid")
	}

	// Verify existing defaults are preserved (tx rolled back)
	defaults, _, _ := s.List(0, 0, true)
	if len(defaults) != 1 {
		t.Errorf("existing defaults should be preserved, got %d", len(defaults))
	}
}

func TestUpdateDescNotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdateDesc("/nonexistent/path", "some desc")
	if err == nil {
		t.Error("expected error for nonexistent repo")
	}
}

func TestInitializeCreatesEntriesTable(t *testing.T) {
	s := newTestStore(t)

	entriesExists, err := s.SQLiteTableExists(entriesTableName)
	if err != nil {
		t.Fatalf("SQLiteTableExists(%q): %v", entriesTableName, err)
	}
	if !entriesExists {
		t.Fatalf("expected table %q to exist", entriesTableName)
	}

	// Legacy tables should not exist on fresh init
	for _, legacy := range []string{legacyRepoTableName, mcpSnapshotTableName, legacyMCPSnapshotTableName} {
		exists, err := s.SQLiteTableExists(legacy)
		if err != nil {
			t.Fatalf("SQLiteTableExists(%q): %v", legacy, err)
		}
		if exists {
			t.Fatalf("expected legacy table %q to be absent", legacy)
		}
	}

	schema := runtimeSQLiteTableSchema(t, s, entriesTableName)
	if !schema.Exists {
		t.Fatalf("expected table %q to exist in schema", entriesTableName)
	}

	createSQL := schema.CreateSQL
	if createSQL == "" {
		t.Fatalf("expected %s create SQL", entriesTableName)
	}
	if want := "unique(type, key)"; !strings.Contains(strings.ToLower(createSQL), want) {
		t.Fatalf("expected create SQL to contain %q, got %s", want, createSQL)
	}
}

func TestInitializeMigratesLegacyTables(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Create legacy tables with data
	if _, err := db.Exec(`
		CREATE TABLE brownfield_repos (
			path TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			desc TEXT,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO brownfield_repos (path, name, desc, is_default) VALUES ('/tmp/repo1', 'repo1', 'desc1', 1);
		INSERT INTO brownfield_repos (path, name, desc) VALUES ('/tmp/repo2', 'repo2', 'desc2');
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
	if err := s.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// Entries table should exist
	entriesExists, err := s.SQLiteTableExists(entriesTableName)
	if err != nil {
		t.Fatalf("SQLiteTableExists(%q): %v", entriesTableName, err)
	}
	if !entriesExists {
		t.Fatalf("expected %q to exist after migration", entriesTableName)
	}

	// Legacy tables should be dropped
	for _, legacy := range []string{legacyRepoTableName, legacyMCPSnapshotTableName} {
		exists, err := s.SQLiteTableExists(legacy)
		if err != nil {
			t.Fatalf("SQLiteTableExists(%q): %v", legacy, err)
		}
		if exists {
			t.Fatalf("expected legacy table %q to be dropped", legacy)
		}
	}

	// Verify migrated repo data
	repos, total, err := s.List(0, 0, false)
	if err != nil {
		t.Fatalf("list repos: %v", err)
	}
	if total != 2 {
		t.Fatalf("total repos = %d, want 2", total)
	}
	if repos[0].Name != "repo1" {
		t.Fatalf("repos[0].Name = %q, want repo1", repos[0].Name)
	}

	// Verify defaults preserved
	defaults, _, _ := s.List(0, 0, true)
	if len(defaults) != 1 || defaults[0].Name != "repo1" {
		t.Fatalf("expected repo1 as default, got %v", defaults)
	}
}

func TestInitializeMigratesLegacyMCPData(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy-mcp.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE mcp_server_snapshot (
			name TEXT NOT NULL PRIMARY KEY,
			path TEXT,
			desc TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			registered_at TIMESTAMP NOT NULL
		);
		INSERT INTO mcp_server_snapshot (name, desc, is_default, registered_at) VALUES ('alpha', 'alpha desc', 0, '2026-01-01T00:00:00Z');
		INSERT INTO mcp_server_snapshot (name, path, desc, is_default, registered_at) VALUES ('beta', '/tmp/beta', 'beta desc', 0, '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("create legacy mcp schema: %v", err)
	}

	s := &Store{db: db}
	if err := s.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 2 {
		t.Fatalf("len(mcps) = %d, want 2", len(mcps))
	}
	if mcps[0].Name != "alpha" {
		t.Fatalf("mcps[0].Name = %q, want alpha", mcps[0].Name)
	}
	if mcps[1].Name != "beta" || mcps[1].Path == nil || *mcps[1].Path != "/tmp/beta" {
		t.Fatalf("mcps[1] = %#v, want beta with path /tmp/beta", mcps[1])
	}

	// Legacy table should be dropped
	exists, _ := s.SQLiteTableExists(mcpSnapshotTableName)
	if exists {
		t.Fatalf("expected legacy table %q to be dropped", mcpSnapshotTableName)
	}
}

func TestSyncMCPEntriesRemovesStaleMCPs(t *testing.T) {
	s := newTestStore(t)

	count, err := s.SyncMCPEntries([]MCPServer{
		{Name: "alpha", Desc: ""},
		{Name: "beta", Desc: ""},
		{Name: "gamma", Desc: ""},
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if count != 3 {
		t.Fatalf("sync count = %d, want 3", count)
	}

	total, err := s.CountMCPs()
	if err != nil {
		t.Fatalf("count mcps: %v", err)
	}
	if total != 3 {
		t.Fatalf("stored mcps = %d, want 3", total)
	}

	// Rescan with only delta
	if _, err := s.SyncMCPEntries([]MCPServer{
		{Name: "delta", Desc: ""},
	}); err != nil {
		t.Fatalf("sync second pass: %v", err)
	}

	total, err = s.CountMCPs()
	if err != nil {
		t.Fatalf("count mcps after rescan: %v", err)
	}
	if total != 1 {
		t.Fatalf("stored mcps after rescan = %d, want 1", total)
	}

	assertRuntimeSQLiteMCPEntryNames(t, s, []string{"delta"})

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after rescan: %v", err)
	}
	if len(mcps) != 1 || mcps[0].Name != "delta" {
		t.Fatalf("remaining mcp after rescan = %v, want [delta]", mcps)
	}

	var staleCount int
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM brownfield_entries
		WHERE type = 'mcp' AND key IN ('alpha', 'beta', 'gamma')
	`).Scan(&staleCount); err != nil {
		t.Fatalf("count stale mcps after rescan: %v", err)
	}
	if staleCount != 0 {
		t.Fatalf("stale mcps after rescan = %d, want 0", staleCount)
	}
}

func TestSyncMCPEntriesPreservesRowID(t *testing.T) {
	s := newTestStore(t)

	// First sync
	s.SyncMCPEntries([]MCPServer{
		{Name: "alpha", Desc: "alpha tools"},
		{Name: "beta", Desc: "beta tools"},
	})

	// Get rowids
	entries, _, _ := s.ListEntries(0, 0, false)
	mcpRowIDs := make(map[string]int64)
	for _, e := range entries {
		if e.Type == "mcp" {
			mcpRowIDs[e.Name] = e.RowID
		}
	}

	// Second sync with same servers + one new
	s.SyncMCPEntries([]MCPServer{
		{Name: "alpha", Desc: "alpha tools v2"},
		{Name: "beta", Desc: "beta tools v2"},
		{Name: "gamma", Desc: "gamma tools"},
	})

	// Verify existing rowids preserved
	entries, _, _ = s.ListEntries(0, 0, false)
	for _, e := range entries {
		if e.Type == "mcp" {
			if oldID, ok := mcpRowIDs[e.Name]; ok {
				if e.RowID != oldID {
					t.Fatalf("mcp %q rowid changed from %d to %d, want preserved", e.Name, oldID, e.RowID)
				}
			}
		}
	}
}

func TestSyncMCPEntriesUpdatePreservesIsDefaultAndRefreshesDescPath(t *testing.T) {
	s := newTestStore(t)

	// Initial sync
	s.SyncMCPEntries([]MCPServer{{Name: "alpha", Desc: "v1"}})

	// Set alpha as default
	entries, _, _ := s.ListEntries(0, 0, false)
	var alphaID int64
	for _, e := range entries {
		if e.Type == "mcp" {
			alphaID = e.RowID
		}
	}
	s.SetDefaultsByRowIDs([]int64{alphaID})

	// Rescan: alpha goes through UPDATE branch with new desc/path
	s.SyncMCPEntries([]MCPServer{{Name: "alpha", Desc: "v2", Path: "/new/path"}})

	// Verify is_default preserved, desc/path updated
	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(mcps))
	}
	if !mcps[0].IsDefault {
		t.Fatal("is_default should be preserved across rescan")
	}
	if mcps[0].Desc != "v2" {
		t.Fatalf("desc not updated: got %q, want v2", mcps[0].Desc)
	}
	if mcps[0].Path == nil || *mcps[0].Path != "/new/path" {
		t.Fatalf("path not updated: got %v, want /new/path", mcps[0].Path)
	}

	// Verify rowid preserved
	entries, _, _ = s.ListEntries(0, 0, false)
	for _, e := range entries {
		if e.Type == "mcp" && e.RowID != alphaID {
			t.Fatalf("rowid changed from %d to %d", alphaID, e.RowID)
		}
	}
}

func TestSyncMCPEntriesDeduplicatesVisibleNames(t *testing.T) {
	s := newTestStore(t)

	// Seed stale entries
	s.SyncMCPEntries([]MCPServer{
		{Name: "stale-a", Desc: "stale"},
		{Name: "stale-b", Desc: "stale"},
	})

	servers := []MCPServer{
		{Name: "alpha", Path: "/tmp/alpha-a", Desc: "alpha tools", Visible: true, VisibilityOK: true},
		{Name: " alpha ", Path: "/tmp/alpha-b", Desc: "older alpha", Visible: true, VisibilityOK: true},
		{Name: "beta", Desc: "", Visible: true, VisibilityOK: true},
		{Name: "hidden", Desc: "hidden tools", Visible: false, VisibilityOK: true},
		{Name: "gamma", Desc: "gamma tools"},
	}

	count, err := s.SyncMCPEntries(servers)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	expectedNames := snapshotNamesForMCPServers(servers)
	if count != len(expectedNames) {
		t.Fatalf("sync count = %d, want %d", count, len(expectedNames))
	}

	assertRuntimeSQLiteMCPEntryNames(t, s, expectedNames)
	assertRuntimeSQLiteMCPDefaultsAllFalse(t, s)

	var staleRows int
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM brownfield_entries
		WHERE type = 'mcp' AND key IN ('stale-a', 'stale-b', 'hidden')
	`).Scan(&staleRows); err != nil {
		t.Fatalf("count stale rows: %v", err)
	}
	if staleRows != 0 {
		t.Fatalf("stale or hidden rows persisted = %d, want 0", staleRows)
	}
}

func TestSyncMCPEntriesUsesProvidedDescription(t *testing.T) {
	s := newTestStore(t)

	wantDesc := "Filesystem, shell, and search tools for local project inspection."
	if _, err := s.SyncMCPEntries([]MCPServer{
		{Name: "workspace", Desc: "  " + wantDesc + "  "},
	}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 1 || mcps[0].Desc != wantDesc {
		t.Fatalf("desc = %q, want %q", mcps[0].Desc, wantDesc)
	}
}

func TestNormalizeMCPDescriptionUsesTrimmedResolvedMetadata(t *testing.T) {
	want := "Filesystem, shell, and search tools for local project inspection."
	if got := normalizeMCPDescription("workspace", "  "+want+"  "); got != want {
		t.Fatalf("normalizeMCPDescription() = %q, want %q", got, want)
	}
}

func TestNormalizeMCPDescriptionUsesDeterministicFallbackFormat(t *testing.T) {
	want := "MCP server slack (tool metadata unavailable at scan time)"
	if got := normalizeMCPDescription(" slack ", "   "); got != want {
		t.Fatalf("normalizeMCPDescription() = %q, want %q", got, want)
	}
}

func TestSyncMCPEntriesFallsBackToPlaceholderDesc(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.SyncMCPEntries([]MCPServer{
		{Name: "slack", Desc: "   "},
	}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(mcps))
	}

	want := "MCP server slack (tool metadata unavailable at scan time)"
	if mcps[0].Desc != want {
		t.Fatalf("desc = %q, want %q", mcps[0].Desc, want)
	}
}

func TestSyncMCPEntriesIncludesVisibleServers(t *testing.T) {
	s := newTestStore(t)

	count, err := s.SyncMCPEntries([]MCPServer{
		{Name: "filesystem", Desc: "Reads and writes local files."},
		{Name: "slack", Desc: "   "},
		{Name: "github", Desc: ""},
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 3 {
		t.Fatalf("len(mcps) = %d, want 3", len(mcps))
	}

	if mcps[0].Name != "filesystem" || mcps[0].Desc != "Reads and writes local files." {
		t.Fatalf("filesystem row = %#v, want resolved description", mcps[0])
	}
	if mcps[1].Name != "github" || mcps[1].Desc != "MCP server github (tool metadata unavailable at scan time)" {
		t.Fatalf("github row = %#v, want fallback desc", mcps[1])
	}
	if mcps[2].Name != "slack" || mcps[2].Desc != "MCP server slack (tool metadata unavailable at scan time)" {
		t.Fatalf("slack row = %#v, want fallback desc", mcps[2])
	}
}

func TestSyncMCPEntriesNewEntriesDefaultFalse(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.SyncMCPEntries([]MCPServer{
		{Name: "alpha", Desc: "", IsDefault: true},
		{Name: "beta", Desc: ""},
		{Name: "gamma", Desc: "", IsDefault: true},
	}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 3 {
		t.Fatalf("len(mcps) = %d, want 3", len(mcps))
	}

	for _, m := range mcps {
		if m.IsDefault {
			t.Fatalf("mcp %q is_default = true, want false for new entries", m.Name)
		}
	}

	assertRuntimeSQLiteMCPDefaultsAllFalse(t, s)
}

func TestSyncMCPEntriesStoresNilPathWhenApprovedMetadataIsUnavailable(t *testing.T) {
	s := newTestStore(t)

	count, err := s.SyncMCPEntries([]MCPServer{
		{Name: "absolute-command", Path: " /tmp/prism-mcp ", Desc: "Uses an approved absolute command path."},
		{Name: "missing-path", Path: "   ", Desc: "Resolved tools but no approved path metadata."},
		{Name: "unset-path", Desc: "Resolved tools with no path field."},
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 3 {
		t.Fatalf("len(mcps) = %d, want 3", len(mcps))
	}

	if mcps[0].Name != "absolute-command" || mcps[0].Path == nil || *mcps[0].Path != "/tmp/prism-mcp" {
		t.Fatalf("absolute-command path = %v, want /tmp/prism-mcp", mcps[0].Path)
	}
	if mcps[1].Name != "missing-path" || mcps[1].Path != nil {
		t.Fatalf("missing-path path = %v, want nil", mcps[1].Path)
	}
	if mcps[2].Name != "unset-path" || mcps[2].Path != nil {
		t.Fatalf("unset-path path = %v, want nil", mcps[2].Path)
	}
}

func TestListEntriesUnifiedNumbering(t *testing.T) {
	s := newTestStore(t)

	s.Register("/tmp/repo1", "repo1", "desc1")
	s.Register("/tmp/repo2", "repo2", "desc2")
	s.SyncMCPEntries([]MCPServer{
		{Name: "alpha", Desc: "alpha tools"},
		{Name: "beta", Desc: "beta tools"},
	})

	entries, total, err := s.ListEntries(0, 0, false)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4", len(entries))
	}

	// All should have unique rowids
	seen := make(map[int64]bool)
	for _, e := range entries {
		if seen[e.RowID] {
			t.Fatalf("duplicate rowid %d", e.RowID)
		}
		seen[e.RowID] = true
	}
}

func TestSetDefaultsByRowIDsMixedTypes(t *testing.T) {
	s := newTestStore(t)

	s.Register("/tmp/repo1", "repo1", "")
	s.SyncMCPEntries([]MCPServer{
		{Name: "alpha", Desc: "alpha tools"},
	})

	entries, _, _ := s.ListEntries(0, 0, false)
	var repoID, mcpID int64
	for _, e := range entries {
		switch e.Type {
		case "repo":
			repoID = e.RowID
		case "mcp":
			mcpID = e.RowID
		}
	}

	// Set both as default
	if err := s.SetDefaultsByRowIDs([]int64{repoID, mcpID}); err != nil {
		t.Fatalf("set defaults: %v", err)
	}

	defaults, err := s.DefaultEntries()
	if err != nil {
		t.Fatalf("default entries: %v", err)
	}
	if len(defaults) != 2 {
		t.Fatalf("defaults = %d, want 2", len(defaults))
	}
}

func TestListMCPsReturnsNilPathWhenUnknown(t *testing.T) {
	s := newTestStore(t)

	s.SyncMCPEntries([]MCPServer{
		{Name: "unknown-server", Desc: "Unknown path"},
	})

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("ListMCPs: %v", err)
	}
	if len(mcps) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(mcps))
	}
	if mcps[0].Path != nil {
		t.Fatalf("mcps[0].Path = %v, want nil", *mcps[0].Path)
	}
}
