package brownfield

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/heechul/prism-mcp/internal/engine"
)

// readmeFiles is the priority-ordered list of README candidates.
var readmeFiles = []string{
	"CLAUDE.md",
	"README.md",
	"README.rst",
	"README.txt",
	"README",
}

const (
	maxReadmeChars = 3000
	maxDescLen     = 120

	descSystemPrompt = `You are a concise technical writer.
Given the content of a project's README or CLAUDE.md,
produce exactly ONE short sentence (max 15 words) describing the project.
Reply with only that sentence — no quotes, no bullet points.`
)

// GenerateDesc reads a README from the repo and generates a one-line description via LLM.
func GenerateDesc(ctx context.Context, repoPath string) (string, error) {
	content := readReadme(repoPath)
	if content == "" {
		return "", nil
	}

	desc, err := engine.QueryLLMWithSystemPrompt(ctx, descSystemPrompt, content)
	if err != nil {
		return "", err
	}

	desc = strings.TrimSpace(desc)
	// Strip surrounding quotes if present
	if len(desc) >= 2 && desc[0] == '"' && desc[len(desc)-1] == '"' {
		desc = desc[1 : len(desc)-1]
	}
	if runes := []rune(desc); len(runes) > maxDescLen {
		desc = string(runes[:maxDescLen])
	}
	return desc, nil
}

// readReadme finds and reads the first available README file.
func readReadme(repoPath string) string {
	for _, name := range readmeFiles {
		p := filepath.Join(repoPath, name)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := string(data)
		if runes := []rune(content); len(runes) > maxReadmeChars {
			content = string(runes[:maxReadmeChars])
		}
		return content
	}
	return ""
}
