package brownfield

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
