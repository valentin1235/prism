package brownfield

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// skipDirs mirrors the ouroboros brownfield scan skip list.
var skipDirs = map[string]bool{
	"node_modules":  true,
	".venv":         true,
	"__pycache__":   true,
	".cache":        true,
	"Library":       true,
	".Trash":        true,
	"vendor":        true,
	".gradle":       true,
	"build":         true,
	"dist":          true,
	"target":        true,
	".tox":          true,
	".mypy_cache":   true,
	".pytest_cache": true,
	".cargo":        true,
	"Pods":          true,
	".npm":          true,
	".nvm":          true,
	".local":        true,
	".docker":       true,
	".rustup":       true,
	"go":            true,
}

// ScanHomeForRepos walks the directory tree from root and returns repos with GitHub origins.
func ScanHomeForRepos(root string) ([]Repo, error) {
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		root = home
	}

	var repos []Repo

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()

		// Check if this directory contains .git
		gitDir := filepath.Join(path, ".git")
		if info, statErr := os.Stat(gitDir); statErr == nil && info.IsDir() {
			if hasGitHubOrigin(path) {
				resolved, err := filepath.Abs(path)
				if err == nil {
					if evalPath, err := filepath.EvalSymlinks(resolved); err == nil {
						resolved = evalPath
					}
					repos = append(repos, Repo{
						Path: resolved,
						Name: filepath.Base(resolved),
					})
				}
			}
			return filepath.SkipDir // Don't descend into repos
		}

		// Prune skip directories and hidden directories
		if skipDirs[name] || (strings.HasPrefix(name, ".") && name != ".") {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Path < repos[j].Path
	})
	return repos, nil
}

// hasGitHubOrigin checks if the repo at dirPath has a GitHub remote origin.
func hasGitHubOrigin(dirPath string) bool {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dirPath
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "github.com")
}
