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

			count, err := s.ReplaceMCPsSnapshot(tc.servers)
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

		count, err := s.ReplaceMCPsSnapshot(servers)
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

func TestInitializeCreatesMCPTable(t *testing.T) {
	s := newTestStore(t)

	check := runtimeSQLiteMCPSnapshotCheck(t, s)
	if !check.SchemaOK {
		t.Fatalf("RuntimeSQLiteMCPSnapshotCheck().SchemaOK = false, want true")
	}
	legacyExists, err := s.SQLiteTableExists(legacyMCPSnapshotTableName)
	if err != nil {
		t.Fatalf("SQLiteTableExists(%q): %v", legacyMCPSnapshotTableName, err)
	}
	if legacyExists {
		t.Fatalf("expected legacy table %q to be absent", legacyMCPSnapshotTableName)
	}

	repoExists, err := s.SQLiteTableExists("brownfield_repos")
	if err != nil {
		t.Fatalf("SQLiteTableExists(%q): %v", "brownfield_repos", err)
	}
	if !repoExists {
		t.Fatalf("expected table %q to exist", "brownfield_repos")
	}
	schema := check.Metadata.Table
	if !schema.Exists {
		t.Fatalf("expected table %q to exist", mcpSnapshotTableName)
	}
	shape := check.Shape
	if !shape.TableExists {
		t.Fatalf("expected detailed runtime schema check to mark %q as existing", mcpSnapshotTableName)
	}
	if got := shape.ColumnCount; got != 5 {
		t.Fatalf("shape column count = %d, want 5", got)
	}
	if !shape.NameColumnNotNull {
		t.Error("expected detailed runtime schema check to require name NOT NULL")
	}
	if !shape.NameColumnPrimaryKey {
		t.Error("expected detailed runtime schema check to require name PRIMARY KEY")
	}
	if !shape.PathColumnPresent {
		t.Error("expected detailed runtime schema check to include path column")
	}
	if !shape.PathColumnNullable {
		t.Error("expected detailed runtime schema check to keep path nullable")
	}
	if !shape.DescColumnNotNull {
		t.Error("expected detailed runtime schema check to require desc NOT NULL")
	}
	if !shape.IsDefaultColumnNotNull {
		t.Error("expected detailed runtime schema check to require is_default NOT NULL")
	}
	if !shape.RegisteredAtColumnNotNull {
		t.Error("expected detailed runtime schema check to require registered_at NOT NULL")
	}
	if !shape.NameNonEmptyConstraint {
		t.Error("expected detailed runtime schema check to require non-empty name")
	}
	if !shape.PathDeclaredAsText || !shape.PathExplicitlyNotNotNull {
		t.Error("expected detailed runtime schema check to record nullable TEXT path")
	}
	if !shape.DescNonEmptyConstraint {
		t.Error("expected detailed runtime schema check to require non-empty desc")
	}
	if !shape.IsDefaultBooleanConstraint {
		t.Error("expected detailed runtime schema check to constrain is_default to boolean values")
	}
	if !shape.RegisteredAtNonEmptyConstraint {
		t.Error("expected detailed runtime schema check to require non-empty registered_at")
	}
	if shape.RegisteredAtHasDefault {
		t.Error("expected detailed runtime schema check to reject registered_at default timestamp")
	}
	if !shape.MatchesExpectedSchema {
		t.Error("expected detailed runtime schema check to match expected schema")
	}

	createSQL := schema.CreateSQL
	if createSQL == "" {
		t.Fatalf("expected %s create SQL", mcpSnapshotTableName)
	}
	if want := "name text not null primary key"; !strings.Contains(strings.ToLower(createSQL), want) {
		t.Fatalf("expected create SQL to contain %q, got %s", want, createSQL)
	}
	if want := "check (length(trim(desc)) > 0)"; !strings.Contains(strings.ToLower(createSQL), want) {
		t.Fatalf("expected create SQL to contain %q, got %s", want, createSQL)
	}
	if want := "check (is_default in (0, 1))"; !strings.Contains(strings.ToLower(createSQL), want) {
		t.Fatalf("expected create SQL to contain %q, got %s", want, createSQL)
	}
	if strings.Contains(strings.ToLower(createSQL), "registered_at timestamp not null default current_timestamp") {
		t.Fatalf("registered_at should not use a default timestamp, got %s", createSQL)
	}

	if _, err := s.db.Exec(`
		INSERT INTO mcp_server_snapshot (name, path, desc, is_default, registered_at)
		VALUES (?, NULL, ?, ?, ?)
	`, "null-path-server", "nullable path schema check", false, testRegisteredAt()); err != nil {
		t.Fatalf("insert NULL path into %s: %v", mcpSnapshotTableName, err)
	}

	var storedPath sql.NullString
	if err := s.db.QueryRow(`
		SELECT path
		FROM mcp_server_snapshot
		WHERE name = ?
	`, "null-path-server").Scan(&storedPath); err != nil {
		t.Fatalf("select nullable path row: %v", err)
	}
	if storedPath.Valid {
		t.Fatalf("stored path = %#v, want SQL NULL", storedPath)
	}
}

func TestInitializeMigratesLegacyMCPTableSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

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
	if err := s.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	check := runtimeSQLiteMCPSnapshotCheck(t, s)
	if !check.SchemaOK {
		t.Fatalf("RuntimeSQLiteMCPSnapshotCheck().SchemaOK = false, want true")
	}
	schema := check.Metadata.Table
	if !schema.Exists {
		t.Fatalf("expected migrated table %q to exist", mcpSnapshotTableName)
	}
	shape := check.Shape
	if !shape.NameColumnPrimaryKey {
		t.Fatal("expected migrated schema check to keep name as PRIMARY KEY")
	}
	if !shape.PathColumnNullable {
		t.Fatal("expected migrated schema check to keep path nullable")
	}
	if !shape.MatchesExpectedSchema {
		t.Fatal("expected migrated schema shape verdict to match runtime schema")
	}

	createSQL := schema.CreateSQL
	if !strings.Contains(strings.ToLower(createSQL), "name text not null primary key") {
		t.Fatalf("expected migrated create SQL to contain primary key definition, got %s", createSQL)
	}
	if !strings.Contains(strings.ToLower(createSQL), "check (length(trim(desc)) > 0)") {
		t.Fatalf("expected migrated create SQL to require non-empty desc, got %s", createSQL)
	}
	if !strings.Contains(strings.ToLower(createSQL), "check (is_default in (0, 1))") {
		t.Fatalf("expected migrated create SQL to constrain is_default boolean values, got %s", createSQL)
	}
	if _, err := db.Exec(`
		INSERT INTO mcp_server_snapshot (name, path, desc, is_default, registered_at)
		VALUES (?, NULL, ?, ?, ?)
	`, "migrated-null-path", "desc", false, testRegisteredAt()); err != nil {
		t.Fatalf("insert NULL path after migration: %v", err)
	}
}

func TestReplaceMCPsSnapshotAuthoritativelyReplacesPriorRuntimeRows(t *testing.T) {
	s := newTestStore(t)

	count, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "alpha", Desc: ""},
		{Name: "beta", Desc: ""},
		{Name: "gamma", Desc: ""},
	})
	if err != nil {
		t.Fatalf("replace snapshot: %v", err)
	}
	if count != 3 {
		t.Fatalf("replace count = %d, want 3", count)
	}

	total, err := s.CountMCPs()
	if err != nil {
		t.Fatalf("count mcps: %v", err)
	}
	if total != 3 {
		t.Fatalf("stored mcps = %d, want 3", total)
	}

	if _, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "delta", Desc: ""},
	}); err != nil {
		t.Fatalf("replace snapshot second pass: %v", err)
	}

	total, err = s.CountMCPs()
	if err != nil {
		t.Fatalf("count mcps after rescan: %v", err)
	}
	if total != 1 {
		t.Fatalf("stored mcps after rescan = %d, want 1", total)
	}

	assertRuntimeSQLiteSnapshotNames(t, s, []string{"delta"})

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps after rescan: %v", err)
	}
	if len(mcps) != 1 {
		t.Fatalf("len(mcps) after rescan = %d, want 1", len(mcps))
	}
	if mcps[0].Name != "delta" {
		t.Fatalf("remaining mcp after rescan = %q, want delta", mcps[0].Name)
	}
	if strings.TrimSpace(mcps[0].Desc) == "" {
		t.Fatal("expected ReplaceMCPsSnapshot to persist a non-empty desc")
	}

	var staleCount int
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM mcp_server_snapshot
		WHERE name IN ('alpha', 'beta', 'gamma')
	`).Scan(&staleCount); err != nil {
		t.Fatalf("count stale mcps after rescan: %v", err)
	}
	if staleCount != 0 {
		t.Fatalf("stale mcps after rescan = %d, want 0", staleCount)
	}
}

func TestReplaceMCPsSnapshotPersistsExactlyOneRowPerDeduplicatedVisibleName(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "stale-a", Desc: "stale"},
		{Name: "stale-b", Desc: "stale"},
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

	count, err := s.ReplaceMCPsSnapshot(servers)
	if err != nil {
		t.Fatalf("replace snapshot: %v", err)
	}

	expectedNames := snapshotNamesForMCPServers(servers)
	if count != len(expectedNames) {
		t.Fatalf("replace count = %d, want %d", count, len(expectedNames))
	}

	assertRuntimeSQLiteSnapshotNames(t, s, expectedNames)
	assertRuntimeSQLiteSnapshotDefaultsAllFalse(t, s)

	var staleRows int
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM mcp_server_snapshot
		WHERE name IN ('stale-a', 'stale-b', 'hidden')
	`).Scan(&staleRows); err != nil {
		t.Fatalf("count stale rows: %v", err)
	}
	if staleRows != 0 {
		t.Fatalf("stale or hidden rows persisted = %d, want 0", staleRows)
	}
}

func TestReplaceMCPsSnapshotUsesProvidedDescriptionWhenAvailable(t *testing.T) {
	s := newTestStore(t)

	wantDesc := "Filesystem, shell, and search tools for local project inspection."
	if _, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "workspace", Desc: "  " + wantDesc + "  "},
	}); err != nil {
		t.Fatalf("replace snapshot: %v", err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(mcps))
	}
	if mcps[0].Desc != wantDesc {
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

func TestReplaceMCPsSnapshotFallsBackToDeterministicPlaceholderWhenMetadataUnavailable(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "slack", Desc: "   "},
	}); err != nil {
		t.Fatalf("replace snapshot: %v", err)
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

func TestReplaceMCPsSnapshotIncludesVisibleServersWhenMetadataResolutionFails(t *testing.T) {
	s := newTestStore(t)

	count, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "filesystem", Desc: "Reads and writes local files."},
		{Name: "slack", Desc: "   "},
		{Name: "github", Desc: ""},
	})
	if err != nil {
		t.Fatalf("replace snapshot: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3 visible servers persisted", count)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 3 {
		t.Fatalf("len(mcps) = %d, want 3", len(mcps))
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

func TestReplaceMCPsSnapshotSetsDeterministicScanFields(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "alpha", Desc: "", IsDefault: true},
		{Name: "beta", Desc: ""},
		{Name: "gamma", Desc: "", IsDefault: true},
	}); err != nil {
		t.Fatalf("replace snapshot: %v", err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 3 {
		t.Fatalf("len(mcps) = %d, want 3", len(mcps))
	}

	registeredAt := mcps[0].RegisteredAt
	if strings.TrimSpace(registeredAt) == "" {
		t.Fatal("expected scan registered_at to be populated")
	}

	for _, m := range mcps {
		if m.IsDefault {
			t.Fatalf("mcp %q is_default = true, want false", m.Name)
		}
		if m.RegisteredAt != registeredAt {
			t.Fatalf("mcp %q registered_at = %q, want shared scan timestamp %q", m.Name, m.RegisteredAt, registeredAt)
		}
	}

	assertRuntimeSQLiteSnapshotDefaultsAllFalse(t, s)
}

func TestReplaceMCPsSnapshotPersistsRequiredFieldsToRuntimeSQLite(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: " filesystem ", Desc: "Reads and writes local files.", IsDefault: true},
		{Name: "slack", Desc: "   "},
	}); err != nil {
		t.Fatalf("replace snapshot: %v", err)
	}

	rows, err := s.db.Query(`
		SELECT name, desc, is_default, registered_at
		FROM mcp_server_snapshot
		ORDER BY name ASC
	`)
	if err != nil {
		t.Fatalf("query persisted snapshot rows: %v", err)
	}
	defer rows.Close()

	type row struct {
		name         sql.NullString
		desc         sql.NullString
		isDefault    sql.NullBool
		registeredAt sql.NullString
	}

	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.name, &r.desc, &r.isDefault, &r.registeredAt); err != nil {
			t.Fatalf("scan persisted snapshot row: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate persisted snapshot rows: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("row count = %d, want 2", len(got))
	}

	for _, r := range got {
		if !r.name.Valid || strings.TrimSpace(r.name.String) == "" {
			t.Fatalf("persisted row missing name: %#v", r)
		}
		if !r.desc.Valid || strings.TrimSpace(r.desc.String) == "" {
			t.Fatalf("persisted row %q missing desc: %#v", r.name.String, r)
		}
		if !r.isDefault.Valid {
			t.Fatalf("persisted row %q missing is_default: %#v", r.name.String, r)
		}
		if !r.registeredAt.Valid || strings.TrimSpace(r.registeredAt.String) == "" {
			t.Fatalf("persisted row %q missing registered_at: %#v", r.name.String, r)
		}
	}

	if got[0].name.String != "filesystem" {
		t.Fatalf("first persisted name = %q, want filesystem", got[0].name.String)
	}
	if got[0].isDefault.Bool {
		t.Fatalf("filesystem is_default = true, want false")
	}
	if got[1].desc.String != "MCP server slack (tool metadata unavailable at scan time)" {
		t.Fatalf("slack desc = %q, want deterministic fallback", got[1].desc.String)
	}
}

func TestReplaceMCPsSnapshotUsesSingleCaptureTimestampForEntireScan(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "alpha", Desc: "Alpha tools", RegisteredAt: "2020-01-01T00:00:00Z"},
		{Name: "beta", Desc: "Beta tools", RegisteredAt: "2030-01-01T00:00:00Z"},
		{Name: "gamma", Desc: "Gamma tools", RegisteredAt: "2040-01-01T00:00:00Z"},
	}); err != nil {
		t.Fatalf("replace snapshot: %v", err)
	}
	sharedRegisteredAt := assertRuntimeSQLiteSnapshotSharedRegisteredAt(t, s, 3)

	disallowed := map[string]struct{}{
		"2020-01-01T00:00:00Z": {},
		"2030-01-01T00:00:00Z": {},
		"2040-01-01T00:00:00Z": {},
	}
	if _, exists := disallowed[sharedRegisteredAt]; exists {
		t.Fatalf("shared scan timestamp = %q, want scan capture timestamp instead of per-server input value", sharedRegisteredAt)
	}
}

func TestReplaceMCPsSnapshotStoresNilPathWhenApprovedMetadataIsUnavailable(t *testing.T) {
	s := newTestStore(t)

	count, err := s.ReplaceMCPsSnapshot([]MCPServer{
		{Name: "absolute-command", Path: " /tmp/prism-mcp ", Desc: "Uses an approved absolute command path."},
		{Name: "missing-path", Path: "   ", Desc: "Resolved tools but no approved path metadata."},
		{Name: "unset-path", Desc: "Resolved tools with no path field."},
	})
	if err != nil {
		t.Fatalf("replace snapshot: %v", err)
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

func TestReplaceMCPsPopulatesRequiredFields(t *testing.T) {
	s := newTestStore(t)

	path := "/tmp/prism-mcp"
	registeredAt := testRegisteredAt()
	if err := s.ReplaceMCPs([]MCPServerSnapshot{
		{Name: "filesystem", Desc: "Reads and inspects local files.", IsDefault: true, Path: &path, RegisteredAt: registeredAt},
		{Name: "slack", Desc: "Searches Slack messages and thread replies.", RegisteredAt: registeredAt},
	}); err != nil {
		t.Fatalf("replace mcps: %v", err)
	}

	mcps, err := s.ListMCPs()
	if err != nil {
		t.Fatalf("list mcps: %v", err)
	}
	if len(mcps) != 2 {
		t.Fatalf("mcps = %d, want 2", len(mcps))
	}

	for _, m := range mcps {
		if strings.TrimSpace(m.Name) == "" {
			t.Fatalf("mcp row missing name: %#v", m)
		}
		if strings.TrimSpace(m.Desc) == "" {
			t.Fatalf("mcp %q missing desc", m.Name)
		}
		if strings.TrimSpace(m.RegisteredAt) == "" {
			t.Fatalf("mcp %q missing registered_at", m.Name)
		}
	}

	if !mcps[0].IsDefault {
		t.Fatalf("filesystem is_default = false, want true")
	}
	if mcps[0].Path == nil || *mcps[0].Path != path {
		t.Fatalf("filesystem path = %v, want %q", mcps[0].Path, path)
	}
	if mcps[1].IsDefault {
		t.Fatalf("slack is_default = true, want false")
	}
	if mcps[1].Path != nil {
		t.Fatalf("slack path = %v, want nil", mcps[1].Path)
	}
}

func TestReplaceMCPsRejectsMissingRequiredFields(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		name string
		rows []MCPServerSnapshot
	}{
		{
			name: "missing name",
			rows: []MCPServerSnapshot{{Name: "   ", Desc: "Valid desc", RegisteredAt: testRegisteredAt()}},
		},
		{
			name: "missing desc",
			rows: []MCPServerSnapshot{{Name: "filesystem", Desc: "   ", RegisteredAt: testRegisteredAt()}},
		},
		{
			name: "missing registered_at",
			rows: []MCPServerSnapshot{{Name: "filesystem", Desc: "Valid desc", RegisteredAt: "   "}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.ReplaceMCPs(tc.rows); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestReplaceMCPsStoresKnownPathOnly(t *testing.T) {
	s := newTestStore(t)

	knownPath := "/tmp/prism/mcp"
	blankPath := "   "
	registeredAt := testRegisteredAt()
	if err := s.ReplaceMCPs([]MCPServerSnapshot{
		{Name: "known-server", Path: &knownPath, Desc: "Known server tools", IsDefault: false, RegisteredAt: registeredAt},
		{Name: "unknown-server", Path: &blankPath, Desc: "Unknown server tools", IsDefault: false, RegisteredAt: registeredAt},
		{Name: "nil-server", Desc: "Nil path tools", IsDefault: false, RegisteredAt: registeredAt},
	}); err != nil {
		t.Fatalf("ReplaceMCPs: %v", err)
	}

	type row struct {
		name string
		path sql.NullString
	}
	rows, err := s.db.Query(`SELECT name, path FROM mcp_server_snapshot ORDER BY name ASC`)
	if err != nil {
		t.Fatalf("query mcp paths: %v", err)
	}
	defer rows.Close()

	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.name, &r.path); err != nil {
			t.Fatalf("scan mcp path row: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate mcp path rows: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("row count = %d, want 3", len(got))
	}
	if got[0].name != "known-server" || !got[0].path.Valid || got[0].path.String != knownPath {
		t.Fatalf("known-server path = %#v, want valid %q", got[0].path, knownPath)
	}
	if got[1].name != "nil-server" || got[1].path.Valid {
		t.Fatalf("nil-server path = %#v, want NULL", got[1].path)
	}
	if got[2].name != "unknown-server" || got[2].path.Valid {
		t.Fatalf("unknown-server path = %#v, want NULL", got[2].path)
	}
}

func TestListMCPsReturnsNilPathWhenUnknown(t *testing.T) {
	s := newTestStore(t)

	if err := s.ReplaceMCPs([]MCPServerSnapshot{
		{Name: "unknown-server", Desc: "Unknown path", IsDefault: false, RegisteredAt: testRegisteredAt()},
	}); err != nil {
		t.Fatalf("ReplaceMCPs: %v", err)
	}

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

func TestMCPTableRejectsMissingRequiredValuesAtWriteTime(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.db.Exec(`
		INSERT INTO mcp_server_snapshot (name, path, desc, is_default)
		VALUES (?, NULL, ?, ?)
	`, "missing-registered-at", "desc", false); err == nil {
		t.Fatal("expected missing registered_at insert to fail")
	}

	if _, err := s.db.Exec(`
		INSERT INTO mcp_server_snapshot (name, path, desc, is_default, registered_at)
		VALUES (?, NULL, ?, ?, ?)
	`, "   ", "desc", false, testRegisteredAt()); err == nil {
		t.Fatal("expected blank name insert to fail")
	}

	if _, err := s.db.Exec(`
		INSERT INTO mcp_server_snapshot (name, path, desc, is_default, registered_at)
		VALUES (?, NULL, ?, ?, ?)
	`, "blank-desc", "   ", false, testRegisteredAt()); err == nil {
		t.Fatal("expected blank desc insert to fail")
	}

	if _, err := s.db.Exec(`
		INSERT INTO mcp_server_snapshot (name, path, desc, is_default, registered_at)
		VALUES (?, NULL, ?, ?, ?)
	`, "invalid-bool", "desc", 2, testRegisteredAt()); err == nil {
		t.Fatal("expected invalid is_default insert to fail")
	}
}

func TestVerifyMCPSnapshotTableSchemaRejectsLegacyTableEvenWhenTableQueriesWork(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
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

	if _, err := s.CountMCPs(); err == nil {
		t.Fatal("expected CountMCPs() to fail when the canonical runtime snapshot table is absent")
	}

	check, err := s.RuntimeSQLiteMCPSnapshotCheck()
	if err != nil {
		t.Fatalf("RuntimeSQLiteMCPSnapshotCheck(): %v", err)
	}
	if check.SchemaOK {
		t.Fatal("expected RuntimeSQLiteMCPSnapshotCheck().SchemaOK to reject legacy table schema")
	}
	if err := s.VerifyMCPSnapshotTableSchema(); err == nil {
		t.Fatal("expected VerifyMCPSnapshotTableSchema() to reject legacy table schema")
	}
}
