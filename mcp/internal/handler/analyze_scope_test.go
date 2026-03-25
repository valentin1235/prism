package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heechul/prism-mcp/internal/brownfield"
	"github.com/heechul/prism-mcp/internal/pipeline"
	taskpkg "github.com/heechul/prism-mcp/internal/task"
	"github.com/mark3labs/mcp-go/mcp"
	_ "modernc.org/sqlite"
)

// setupTestBrownfieldDB creates a temporary brownfield store with given repos
// and returns the temp dir (which acts as PrismBaseDir).
func setupTestBrownfieldDB(t *testing.T, repos []brownfield.Repo, defaultPaths []string) string {
	t.Helper()
	dir := t.TempDir()

	dbPath := filepath.Join(dir, "prism.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brownfield_repos (
			path TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			desc TEXT,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS ix_brownfield_repos_is_default ON brownfield_repos (is_default);
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	for _, r := range repos {
		_, err := db.Exec("INSERT INTO brownfield_repos (path, name, desc) VALUES (?, ?, ?)",
			r.Path, r.Name, r.Desc)
		if err != nil {
			t.Fatalf("insert repo: %v", err)
		}
	}

	for _, p := range defaultPaths {
		_, err := db.Exec("UPDATE brownfield_repos SET is_default = 1 WHERE path = ?", p)
		if err != nil {
			t.Fatalf("set default: %v", err)
		}
	}

	return dir
}

func makeAnalyzeRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Arguments: args,
		},
	}
}

// TestMultipleDefaultReposMerge verifies that multiple is_default=1 repos
// are merged into a single ontology scope with all paths as sources.
func TestMultipleDefaultReposMerge(t *testing.T) {
	repos := []brownfield.Repo{
		{Path: "/tmp/repo-alpha", Name: "alpha"},
		{Path: "/tmp/repo-beta", Name: "beta"},
		{Path: "/tmp/repo-gamma", Name: "gamma"},
	}
	defaultPaths := []string{"/tmp/repo-alpha", "/tmp/repo-gamma"}

	dir := setupTestBrownfieldDB(t, repos, defaultPaths)

	// Override PrismBaseDir to point to temp dir
	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	// Initialize TaskStore
	TaskStore = taskpkg.NewTaskStore()

	// Call HandleAnalyze without ontology_scope — should auto-resolve from brownfield
	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic": "test multi-default merge",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		errText := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("unexpected error result: %s", errText)
	}

	// Parse response to get task_id
	text := result.Content[0].(mcp.TextContent).Text
	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	// Read the ontology-scope.json written by the handler
	stateDir := filepath.Join(dir, "state", snap.ID)
	scopeData, err := os.ReadFile(filepath.Join(stateDir, "ontology-scope.json"))
	if err != nil {
		t.Fatalf("read ontology-scope.json: %v", err)
	}

	// Parse and verify the scope contains both default repos
	var scope struct {
		Sources []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"sources"`
		Totals struct {
			Doc int `json:"doc"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(scopeData, &scope); err != nil {
		t.Fatalf("parse scope: %v", err)
	}

	if len(scope.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(scope.Sources))
	}
	if scope.Totals.Doc != 2 {
		t.Errorf("expected totals.doc=2, got %d", scope.Totals.Doc)
	}

	// Verify both default paths are present
	paths := map[string]bool{}
	for _, s := range scope.Sources {
		paths[s.Path] = true
		if s.Type != "doc" {
			t.Errorf("expected type 'doc', got %q", s.Type)
		}
	}
	if !paths["/tmp/repo-alpha"] {
		t.Error("missing /tmp/repo-alpha in scope sources")
	}
	if !paths["/tmp/repo-gamma"] {
		t.Error("missing /tmp/repo-gamma in scope sources")
	}
	// repo-beta should NOT be in scope (not default)
	if paths["/tmp/repo-beta"] {
		t.Error("/tmp/repo-beta should not be in scope (not default)")
	}
}

// TestExplicitOntologyScopeTakesPriority verifies that an explicit ontology_scope
// parameter takes priority over brownfield defaults.
func TestExplicitOntologyScopeTakesPriority(t *testing.T) {
	// Set up brownfield with defaults
	repos := []brownfield.Repo{
		{Path: "/tmp/repo-bf", Name: "bf-repo"},
	}
	dir := setupTestBrownfieldDB(t, repos, []string{"/tmp/repo-bf"})

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	// Provide explicit ontology_scope
	explicitScope := `{"sources":[{"id":1,"type":"doc","path":"/explicit/path","domain":"explicit","summary":"Explicit scope","status":"available","access":{"tools":["Read"],"instructions":"Use Read"}}],"totals":{"doc":1}}`

	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "test explicit priority",
		"ontology_scope": explicitScope,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		errText := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("unexpected error result: %s", errText)
	}

	// Parse response
	text := result.Content[0].(mcp.TextContent).Text
	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	// Read the ontology-scope.json
	stateDir := filepath.Join(dir, "state", snap.ID)
	scopeData, err := os.ReadFile(filepath.Join(stateDir, "ontology-scope.json"))
	if err != nil {
		t.Fatalf("read ontology-scope.json: %v", err)
	}

	// Verify it's the explicit scope, not the brownfield scope
	var scope struct {
		Sources []struct {
			Path string `json:"path"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(scopeData, &scope); err != nil {
		t.Fatalf("parse scope: %v", err)
	}

	if len(scope.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(scope.Sources))
	}
	if scope.Sources[0].Path != "/explicit/path" {
		t.Errorf("expected explicit path, got %q", scope.Sources[0].Path)
	}
}

// TestNoScopeAndNoDefaultsReturnsError verifies that when both ontology_scope
// and brownfield defaults are absent, an appropriate error is returned.
func TestNoScopeAndNoDefaultsReturnsError(t *testing.T) {
	// Set up brownfield with repos but NO defaults
	repos := []brownfield.Repo{
		{Path: "/tmp/repo-no-default", Name: "no-default"},
	}
	dir := setupTestBrownfieldDB(t, repos, nil)

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic": "test no scope no defaults",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when no scope and no defaults")
	}

	errText := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(errText, "brownfield") {
		t.Errorf("error should mention brownfield, got %q", errText)
	}
}

// TestNoBrownfieldDBReturnsError verifies that when the brownfield DB doesn't exist
// and no ontology_scope is provided, an appropriate error is returned.
func TestNoBrownfieldDBReturnsError(t *testing.T) {
	// Point to a temp dir with no prism.db
	dir := t.TempDir()

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic": "test no db",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when no brownfield DB")
	}

	errText := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(errText, "brownfield store를 먼저 설정해주세요") {
		t.Errorf("error should contain setup instruction, got %q", errText)
	}
}

// TestBuildOntologyScopeFromPaths verifies the scope JSON construction.
func TestBuildOntologyScopeFromPaths(t *testing.T) {
	paths := []string{"/repo/a", "/repo/b", "/repo/c"}
	scope := pipeline.BuildOntologyScopeFromPaths(paths)

	var parsed struct {
		Sources []struct {
			ID     int    `json:"id"`
			Type   string `json:"type"`
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"sources"`
		Totals struct {
			Doc int `json:"doc"`
		} `json:"totals"`
	}
	if err := json.Unmarshal([]byte(scope), &parsed); err != nil {
		t.Fatalf("parse scope: %v", err)
	}

	if len(parsed.Sources) != 3 {
		t.Errorf("expected 3 sources, got %d", len(parsed.Sources))
	}
	if parsed.Totals.Doc != 3 {
		t.Errorf("expected totals.doc=3, got %d", parsed.Totals.Doc)
	}

	for i, s := range parsed.Sources {
		if s.ID != i+1 {
			t.Errorf("source %d: expected id %d, got %d", i, i+1, s.ID)
		}
		if s.Type != "doc" {
			t.Errorf("source %d: expected type 'doc', got %q", i, s.Type)
		}
		if s.Path != paths[i] {
			t.Errorf("source %d: expected path %q, got %q", i, paths[i], s.Path)
		}
		if s.Status != "available" {
			t.Errorf("source %d: expected status 'available', got %q", i, s.Status)
		}
	}
}
