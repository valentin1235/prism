package brownfield

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
	if !strings.Contains(text, "set_defaults") {
		t.Errorf("expected 'set_defaults' in result, got: %s", text)
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
	if !strings.Contains(text, "All defaults cleared") {
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

// --- helpers ---

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
