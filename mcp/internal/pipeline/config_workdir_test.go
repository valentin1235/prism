package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAnalysisWorkDir_PrefersCommonOntologyRoot(t *testing.T) {
	root := t.TempDir()
	repoA := filepath.Join(root, "repo-a")
	repoB := filepath.Join(root, "repo-b")
	for _, dir := range []string{repoA, repoB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	cfg := AnalysisConfig{
		StateDir: filepath.Join(root, ".prism", "state", "analyze-test"),
		OntologyScope: `{"sources":[
			{"id":1,"type":"doc","path":"` + repoA + `","status":"available"},
			{"id":2,"type":"doc","path":"` + repoB + `","status":"available"}
		]}`,
	}

	want := normalizeExistingDir(root)
	if got := ResolveAnalysisWorkDir(cfg); got != want {
		t.Fatalf("ResolveAnalysisWorkDir() = %q, want %q", got, want)
	}
}

func TestResolveAnalysisWorkDir_UsesInputContextWhenOntologyMissing(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "docs", "brief.md")
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(inputPath, []byte("brief"), 0o644); err != nil {
		t.Fatalf("write input context: %v", err)
	}

	cfg := AnalysisConfig{
		StateDir:      filepath.Join(root, ".prism", "state", "analyze-test"),
		InputContext:  inputPath,
		OntologyScope: "",
	}

	want := normalizeExistingDir(filepath.Dir(inputPath))
	if got := ResolveAnalysisWorkDir(cfg); got != want {
		t.Fatalf("ResolveAnalysisWorkDir() = %q, want %q", got, want)
	}
}
