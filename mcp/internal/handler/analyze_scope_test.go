// NOTE: Tests in this file mutate package globals (PrismBaseDir, TaskStore)
// and MUST NOT use t.Parallel().
package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heechul/prism-mcp/internal/analysisstore"
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
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)")
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

func cancelTaskIfPresent(taskID string) {
	if TaskStore == nil {
		return
	}
	task := TaskStore.Get(taskID)
	if task == nil || task.Cancel == nil {
		return
	}
	task.Cancel()
	time.Sleep(10 * time.Millisecond)
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
	defer cancelTaskIfPresent(snap.ID)

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
	defer cancelTaskIfPresent(snap.ID)

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
	scope, err := pipeline.BuildOntologyScopeFromPaths(paths)
	if err != nil {
		t.Fatalf("BuildOntologyScopeFromPaths: %v", err)
	}

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

func TestHandleAnalyze_DefaultModelUsesRuntimeDefaultAlias(t *testing.T) {
	dir := setupTestBrownfieldDB(t, []brownfield.Repo{
		{Path: t.TempDir(), Name: "repo"},
	}, []string{})

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	explicitScope := `{"sources":[{"id":1,"type":"doc","path":"` + t.TempDir() + `","domain":"explicit","summary":"Explicit scope","status":"available","access":{"tools":["Read"],"instructions":"Use Read"}}],"totals":{"doc":1}}`

	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "test default model alias",
		"ontology_scope": explicitScope,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].(mcp.TextContent).Text)
	}

	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &snap); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	defer cancelTaskIfPresent(snap.ID)

	stateDir := filepath.Join(dir, "state", snap.ID)
	configData, err := os.ReadFile(filepath.Join(stateDir, "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(configData, &cfg); err != nil {
		t.Fatalf("parse config.json: %v", err)
	}

	if got, _ := cfg["model"].(string); got != "default" {
		t.Fatalf("config model = %q, want %q", got, "default")
	}
}

func TestHandleAnalyze_ExplicitAdaptorPersistsToConfig(t *testing.T) {
	dir := setupTestBrownfieldDB(t, []brownfield.Repo{
		{Path: t.TempDir(), Name: "repo"},
	}, []string{})

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	explicitScope := `{"sources":[{"id":1,"type":"doc","path":"` + t.TempDir() + `","domain":"explicit","summary":"Explicit scope","status":"available","access":{"tools":["Read"],"instructions":"Use Read"}}],"totals":{"doc":1}}`

	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "test explicit adaptor",
		"ontology_scope": explicitScope,
		"adaptor":        "codex",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].(mcp.TextContent).Text)
	}

	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &snap); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	defer cancelTaskIfPresent(snap.ID)

	cfg, err := pipeline.ReadAnalysisConfig(filepath.Join(dir, "state", snap.ID))
	if err != nil {
		t.Fatalf("ReadAnalysisConfig() error = %v", err)
	}
	if cfg.Adaptor != "codex" {
		t.Fatalf("config adaptor = %q, want %q", cfg.Adaptor, "codex")
	}
}

func TestResolveRequestedAdaptorFallbackOrder(t *testing.T) {
	t.Setenv("PRISM_AGENT_RUNTIME", "")
	t.Setenv("PRISM_LLM_BACKEND", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())

	if got := resolveRequestedAdaptor("codex"); got != "codex" {
		t.Fatalf("resolveRequestedAdaptor(explicit codex) = %q, want codex", got)
	}

	t.Setenv("PRISM_AGENT_RUNTIME", "codex")
	if got := resolveRequestedAdaptor(""); got != "codex" {
		t.Fatalf("resolveRequestedAdaptor(empty) with runtime codex = %q, want codex", got)
	}

	t.Setenv("PRISM_AGENT_RUNTIME", "")
	t.Setenv("PRISM_LLM_BACKEND", "")
	if got := resolveRequestedAdaptor(""); got != "claude" {
		t.Fatalf("resolveRequestedAdaptor(empty) final fallback = %q, want claude", got)
	}
}

func TestHandleAnalyzeInvalidOntologyScopeCleansUpArtifacts(t *testing.T) {
	repoPath := t.TempDir()
	dir := setupTestBrownfieldDB(t, []brownfield.Repo{
		{Path: repoPath, Name: "repo"},
	}, nil)

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	sessionID := "cleanup-invalid-ontology"
	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "cleanup invalid ontology",
		"session_id":     sessionID,
		"ontology_scope": `{"sources":[`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected invalid ontology scope to fail")
	}

	taskID := "analyze-" + sessionID
	if TaskStore.Get(taskID) != nil {
		t.Fatalf("expected task %s to be removed from memory", taskID)
	}

	stateDir := filepath.Join(dir, "state", taskID)
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("expected state dir to be removed, stat err=%v", err)
	}

	reportDir := filepath.Join(dir, "reports", taskID)
	if _, err := os.Stat(reportDir); !os.IsNotExist(err) {
		t.Fatalf("expected report dir to be removed, stat err=%v", err)
	}

	if _, ok, err := analysisstore.LoadAnalysisConfig(dir, taskID); err != nil {
		t.Fatalf("load analysis config: %v", err)
	} else if ok {
		t.Fatalf("expected sqlite row for %s to be removed", taskID)
	}
}

func TestHandleAnalyzeInvalidOntologyScopePreservesExistingDeterministicRun(t *testing.T) {
	repoPath := t.TempDir()
	dir := setupTestBrownfieldDB(t, []brownfield.Repo{
		{Path: repoPath, Name: "repo"},
	}, nil)

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	taskID := "analyze-existing-session"
	stateDir := filepath.Join(dir, "state", taskID)
	reportDir := filepath.Join(dir, "reports", taskID)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatalf("mkdir report dir: %v", err)
	}
	configContent := []byte(`{"task_id":"` + taskID + `","adaptor":"codex"}`)
	if err := os.WriteFile(filepath.Join(stateDir, "config.json"), configContent, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	scopeContent := []byte(`{"sources":[{"path":"/old"}]}`)
	if err := os.WriteFile(filepath.Join(stateDir, "ontology-scope.json"), scopeContent, 0o644); err != nil {
		t.Fatalf("write ontology: %v", err)
	}

	if err := analysisstore.SaveAnalysisConfig(dir, analysisstore.AnalysisConfigRecord{
		TaskID:    taskID,
		Topic:     "old topic",
		Model:     "default",
		Adaptor:   "codex",
		ContextID: taskID,
		StateDir:  stateDir,
		ReportDir: reportDir,
	}); err != nil {
		t.Fatalf("persist config: %v", err)
	}
	oldTask := taskpkg.NewAnalysisTask(taskID, "default", stateDir, reportDir, "existing-session")
	oldTask.SetReportPath(filepath.Join(reportDir, "report.md"))
	if err := analysisstore.SaveTaskSnapshot(dir, oldTask.Snapshot(), 4); err != nil {
		t.Fatalf("persist snapshot: %v", err)
	}

	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "bad rerun",
		"session_id":     "existing-session",
		"ontology_scope": `{"sources":[`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected invalid ontology scope to fail")
	}

	if got, err := os.ReadFile(filepath.Join(stateDir, "config.json")); err != nil {
		t.Fatalf("read config after failure: %v", err)
	} else if string(got) != string(configContent) {
		t.Fatalf("expected existing config to be preserved, got %q", string(got))
	}
	if got, err := os.ReadFile(filepath.Join(stateDir, "ontology-scope.json")); err != nil {
		t.Fatalf("read ontology after failure: %v", err)
	} else if string(got) != string(scopeContent) {
		t.Fatalf("expected existing ontology to be preserved, got %q", string(got))
	}

	snapshot, pollCount, ok, err := analysisstore.LoadTaskSnapshot(dir, taskID)
	if err != nil {
		t.Fatalf("load persisted snapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected persisted snapshot to remain")
	}
	if snapshot.Status != taskpkg.TaskStatusCompleted {
		t.Fatalf("expected completed snapshot to remain, got %s", snapshot.Status)
	}
	if pollCount != 4 {
		t.Fatalf("expected poll count 4 to remain, got %d", pollCount)
	}
}

func TestHandleAnalyzeFailsFastOnExistingBackupReadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "prism.db"), []byte("not-a-sqlite-db"), 0o644); err != nil {
		t.Fatalf("write corrupt prism db: %v", err)
	}

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	sessionID := "backup-read-error"
	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "backup read error",
		"session_id":     sessionID,
		"ontology_scope": `{"sources":[],"totals":{"doc":0}}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected corrupt sqlite backup read to fail")
	}

	errText := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(errText, "failed to load existing analysis backup") {
		t.Fatalf("expected backup read failure, got %q", errText)
	}

	taskID := "analyze-" + sessionID
	if TaskStore.Get(taskID) != nil {
		t.Fatalf("expected task %s to be removed from memory", taskID)
	}
	if _, err := os.Stat(filepath.Join(dir, "state", taskID)); !os.IsNotExist(err) {
		t.Fatalf("expected state dir to remain absent, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "reports", taskID)); !os.IsNotExist(err) {
		t.Fatalf("expected report dir to remain absent, stat err=%v", err)
	}
}

func TestHandleAnalyzeRejectsDeterministicRerunWhileTaskIsRunning(t *testing.T) {
	dir := t.TempDir()

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	task := TaskStore.Create("", "default", "", "", "same-session")
	task.SetStatus(taskpkg.TaskStatusRunning)
	task.StartStage(taskpkg.StageScope, "already running")

	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "rerun while running",
		"session_id":     "same-session",
		"ontology_scope": `{"sources":[],"totals":{"doc":0}}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected deterministic rerun to fail while previous task is running")
	}

	errText := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(errText, "already running") {
		t.Fatalf("expected running task rejection, got %q", errText)
	}
	if TaskStore.Get(task.ID) == nil {
		t.Fatalf("expected existing running task %s to remain registered", task.ID)
	}
}

func TestHandleAnalyzeRejectsDeterministicRerunUntilTerminalTaskFullyExits(t *testing.T) {
	dir := t.TempDir()

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	task := TaskStore.Create("", "default", "", "", "same-session-terminal")
	task.SetError("already failed")

	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "rerun while cleanup pending",
		"session_id":     "same-session-terminal",
		"ontology_scope": `{"sources":[],"totals":{"doc":0}}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected deterministic rerun to fail until previous terminal task exits")
	}
	errText := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(errText, "has not fully exited yet") {
		t.Fatalf("expected cleanup-pending rejection, got %q", errText)
	}

	task.CloseDone()
	result, err = HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "rerun after cleanup",
		"session_id":     "same-session-terminal",
		"ontology_scope": `{"sources":[],"totals":{"doc":0}}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error after close: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected rerun after done close to succeed, got %s", result.Content[0].(mcp.TextContent).Text)
	}
	if rerunTask := TaskStore.Get("analyze-same-session-terminal"); rerunTask != nil {
		if rerunTask.Cancel != nil {
			rerunTask.Cancel()
		}
		rerunTask.CloseDone()
		TaskStore.Remove(rerunTask.ID)
	}
}

func TestHandleAnalyzeRejectsDeterministicRerunWhilePersistedTaskIsRunning(t *testing.T) {
	dir := t.TempDir()

	origBase := PrismBaseDir
	PrismBaseDir = dir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()

	taskID := "analyze-persisted-running"
	if err := analysisstore.SaveAnalysisConfig(dir, analysisstore.AnalysisConfigRecord{
		TaskID:        taskID,
		Topic:         "existing persisted run",
		Model:         "default",
		Adaptor:       "claude",
		ContextID:     taskID,
		StateDir:      filepath.Join(dir, "state", taskID),
		ReportDir:     filepath.Join(dir, "reports", taskID),
		OntologyScope: `{"sources":[],"totals":{"doc":0}}`,
	}); err != nil {
		t.Fatalf("SaveAnalysisConfig() error = %v", err)
	}

	runningTask := taskpkg.NewAnalysisTask(taskID, "default", filepath.Join(dir, "state", taskID), filepath.Join(dir, "reports", taskID), "persisted-running")
	runningTask.Status = taskpkg.TaskStatusRunning
	runningTask.ContextID = taskID
	runningTask.StartStage(taskpkg.StageScope, "persisted running row")
	snapshot, pollCount := runningTask.SnapshotWithPollCount()
	if err := analysisstore.SaveTaskSnapshot(dir, snapshot, pollCount); err != nil {
		t.Fatalf("SaveTaskSnapshot() error = %v", err)
	}

	result, err := HandleAnalyze(context.Background(), makeAnalyzeRequest(map[string]interface{}{
		"topic":          "rerun while persisted run is running",
		"session_id":     "persisted-running",
		"ontology_scope": `{"sources":[],"totals":{"doc":0}}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected deterministic rerun to fail while persisted task is still running")
	}

	errText := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(errText, "persisted state") {
		t.Fatalf("expected persisted-state rejection, got %q", errText)
	}
}
