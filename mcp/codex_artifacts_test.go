package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

var codexArtifactsMu sync.Mutex

func TestCodexRuleRegistersInitialPSMCommands(t *testing.T) {
	t.Parallel()

	content := readRepoFile(t, "../codex/rules/prism.md")

	for _, needle := range []string{
		"`psm analyze`",
		"`prism-analyze`",
		"`psm brownfield`",
		"`prism-brownfield`",
		"`psm incident`",
		"`prism-incident`",
		"`psm prd /path/to/prd.md`",
		"`prism-prd`",
		"`psm setup`",
		"`prism-setup`",
		"This initial Codex registration only covers `psm analyze`, `psm brownfield`, `psm incident`, `psm prd`, and `psm setup`.",
		"`psm analyze-workspace`",
		"`psm test-analyze`",
		"`codex/lib/command-registry.tsv` as the executable closed set",
		"`codex/lib/command-ontology.tsv` as the broader command ontology",
		"`non-acceptance-bearing` and `unregistered`",
		"`PRISM_REPO_PATH` when it points to a Prism repo containing the required shared analyze assets.",
		"The installed `repo-root` pointer shipped with the shared `psm` integration layer.",
		"Do not resolve shared Prism analyze assets from the user's working directory.",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex rule", needle)
		}
	}
}

func TestCodexAnalyzeSkillDispatchesToSharedPrismWorkflow(t *testing.T) {
	t.Parallel()

	content := renderGeneratedPSMSkill(t, "analyze")

	for _, needle := range []string{
		"name: prism-analyze",
		"# psm analyze",
		"Glob(pattern=\"**/skills/analyze/SKILL.md\")",
		"Treat `PRISM_REPO_PATH/skills/analyze/SKILL.md` as the canonical shared skill path when it is available.",
		"If multiple matches exist, prefer the Prism-owned path under `PRISM_REPO_PATH` over matches from the user's target repository or working directory.",
		"Follow that shared skill exactly.",
		"/prism:analyze` -> `psm analyze`",
		"`Use \\`psm brownfield\\``",
		"PRISM_REPO_PATH/skills/analyze/SKILL.md",
		"`PRISM_REPO_PATH` when it points to a Prism repo containing the required shared analyze assets.",
		"The installed `repo-root` pointer shipped with the shared `psm` integration layer.",
		"A Prism repo root inferred relative to the shared `psm` library.",
		"Never resolve shared Prism analyze assets from the user's working directory.",
		"Pass shared analyze config paths such as `report_template` through unchanged",
		"Do not reimplement or paraphrase the Prism analyze workflow in this Codex wrapper.",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex analyze skill", needle)
		}
	}
}

func TestCodexAnalyzeSkillUsesSharedDispatchFramework(t *testing.T) {
	t.Parallel()

	content := renderGeneratedPSMSkill(t, "analyze")
	for _, needle := range []string{
		"## Shared Codex Dispatch",
		"## Codex Normalization Rules",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex analyze skill", needle)
		}
	}
	for _, forbidden := range []string{
		"### Step 2.1: Call prism_analyze",
		"`prism_task_status`",
		"`prism_analyze_result`",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("expected analyze wrapper to avoid embedded workflow detail %q", forbidden)
		}
	}
}

func TestInstalledCodexAnalyzeSkillIsPortableAcrossWorkingDirectories(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()

	cmd := exec.Command("bash", "-lc", "pwd")
	cmd.Dir = unrelatedDir
	cmd.Env = append(
		os.Environ(),
		"HOME="+install.homeDir,
		"CODEX_HOME="+install.codexHome,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("verify unrelated working directory: %v\n%s", err, output)
	}

	ruleContent := readFile(t, filepath.Join(install.codexHome, "rules", "prism.md"))
	for _, needle := range []string{
		"| `psm analyze` | `prism-analyze` |",
		"| `psm analyze <topic>` | `prism-analyze` |",
		"| `psm analyze --config /path/to/config.json` | `prism-analyze` |",
		"This initial Codex registration only covers `psm analyze`, `psm brownfield`, `psm incident`, `psm prd`, and `psm setup`.",
		"`PRISM_REPO_PATH` when it points to a Prism repo containing the required shared analyze assets.",
		"The installed `repo-root` pointer shipped with the shared `psm` integration layer.",
	} {
		if !strings.Contains(ruleContent, needle) {
			t.Fatalf("expected %q in installed Codex rule", needle)
		}
	}

	config := readFile(t, install.configPath)
	canonicalCodexHome := canonicalPath(t, install.codexHome)
	for _, needle := range []string{
		`command = "` + install.runScript + `"`,
		`PRISM_AGENT_RUNTIME = "codex"`,
		`PRISM_LLM_BACKEND = "codex"`,
		`PRISM_REPO_PATH = "` + install.repoRoot + `"`,
		`# PRISM_REPO_PATH is the source of truth for shared Prism skill, prompt, template, and MCP assets.`,
		`PRISM_SHARED_SKILLS_ROOT = "` + filepath.Join(install.repoRoot, "skills") + `"`,
		`PRISM_CODEX_SKILLS_ROOT = "` + filepath.Join(canonicalCodexHome, "skills") + `"`,
		`PRISM_CODEX_RULES_ROOT = "` + filepath.Join(canonicalCodexHome, "rules") + `"`,
	} {
		if !strings.Contains(config, needle) {
			t.Fatalf("expected %q in generated Codex config", needle)
		}
	}

	installedContent := readFile(t, filepath.Join(install.codexHome, "skills", "prism-analyze", "SKILL.md"))
	sharedContent := readRepoFile(t, "../skills/analyze/SKILL.md")
	if installedContent != sharedContent {
		t.Fatalf("expected installed Codex analyze skill to match the shared repo skill")
	}
	for _, needle := range []string{
		"# Multi-Perspective Analysis",
		"Calls `prism_analyze` to start the analysis pipeline",
		"Polls `prism_task_status` for progress updates",
		"Retrieves results via `prism_analyze_result` when complete",
	} {
		if !strings.Contains(installedContent, needle) {
			t.Fatalf("expected %q in installed Codex analyze skill", needle)
		}
	}

	for _, relativePath := range []string{
		"skills/analyze/SKILL.md",
		"skills/analyze/prompts/seed-analyst.md",
		"skills/analyze/prompts/perspective-generator.md",
		"skills/analyze/prompts/finding-protocol.md",
		"skills/analyze/prompts/verification-protocol.md",
		"skills/analyze/templates/report.md",
	} {
		assetPath := filepath.Join(install.repoRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(assetPath); err != nil {
			t.Fatalf("expected shared analyze asset %s: %v", assetPath, err)
		}
	}
}

func TestCodexDevModeRoutesPSMBrownfieldThroughCodexSkill(t *testing.T) {
	t.Parallel()

	content := readRepoFile(t, "../CLAUDE.md")

	for _, needle := range []string{
		"`psm brownfield`",
		"`skills/brownfield/SKILL.md`",
		"Treat `psm brownfield` as a command, not as natural language.",
		"Reuse Prism's bundled MCP tools and skill assets; do not reimplement the workflow ad hoc.",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex dev-mode command routing", needle)
		}
	}
}

func TestInstallCodexScriptUsesAbsolutePrismRepoPath(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	config := readFile(t, install.configPath)
	canonicalCodexHome := canonicalPath(t, install.codexHome)

	for _, needle := range []string{
		`[mcp_servers.prism]`,
		`command = "` + install.runScript + `"`,
		`PRISM_AGENT_RUNTIME = "codex"`,
		`PRISM_LLM_BACKEND = "codex"`,
		`PRISM_REPO_PATH = "` + install.repoRoot + `"`,
		`# PRISM_REPO_PATH is the source of truth for shared Prism skill, prompt, template, and MCP assets.`,
		`PRISM_SHARED_SKILLS_ROOT = "` + filepath.Join(install.repoRoot, "skills") + `"`,
		`PRISM_CODEX_SKILLS_ROOT = "` + filepath.Join(canonicalCodexHome, "skills") + `"`,
		`PRISM_CODEX_RULES_ROOT = "` + filepath.Join(canonicalCodexHome, "rules") + `"`,
	} {
		if !strings.Contains(config, needle) {
			t.Fatalf("expected %q in generated Codex config", needle)
		}
	}

	for _, installedPath := range []string{
		filepath.Join(install.codexHome, "bin", "psm"),
		filepath.Join(install.codexHome, "lib", "prism", "psm.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "framework.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "bridges", "analyze.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "bridges", "incident.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "bridges", "prd.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "commands", "analyze.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "commands", "brownfield.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "commands", "incident.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "commands", "prd.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "commands", "setup.sh"),
		filepath.Join(install.codexHome, "lib", "prism", "repo-root"),
		filepath.Join(install.codexHome, "rules", "prism.md"),
		filepath.Join(install.codexHome, "skills", "prism-analyze", "SKILL.md"),
		filepath.Join(install.codexHome, "skills", "prism-brownfield", "SKILL.md"),
		filepath.Join(install.codexHome, "skills", "prism-incident", "SKILL.md"),
		filepath.Join(install.codexHome, "skills", "prism-prd", "SKILL.md"),
		filepath.Join(install.codexHome, "skills", "prism-setup", "SKILL.md"),
	} {
		if _, err := os.Stat(installedPath); err != nil {
			t.Fatalf("expected installed artifact %s: %v", installedPath, err)
		}
	}
}

func TestInstalledPSMWrapperUsesSharedIntegrationLayer(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)

	wrapperContent := readFile(t, filepath.Join(install.codexHome, "bin", "psm"))
	if !strings.Contains(wrapperContent, `source "${LIB_DIR}/psm.sh"`) {
		t.Fatalf("expected installed psm wrapper to source the shared Codex integration layer")
	}

	sharedLibContent := readFile(t, filepath.Join(install.codexHome, "lib", "prism", "psm.sh"))
	for _, needle := range []string{
		`source "${PRISM_PSM_LIB_DIR}/framework.sh"`,
		"prism_psm_load_bridges",
		"prism_psm_load_command_configs",
	} {
		if !strings.Contains(sharedLibContent, needle) {
			t.Fatalf("expected %q in shared psm integration layer", needle)
		}
	}

	frameworkContent := readFile(t, filepath.Join(install.codexHome, "lib", "prism", "framework.sh"))
	for _, needle := range []string{
		"prism_psm_supported_commands",
		"repo-root",
		"prism_psm_define_command_config",
		"prism_psm_require_command_config",
		"prism_psm_repo_root_has_required_assets",
		"unable to locate the shared Prism asset root",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"PRISM_TARGET_CWD",
		"Codex execution failed",
	} {
		if !strings.Contains(frameworkContent, needle) {
			t.Fatalf("expected %q in shared psm framework", needle)
		}
	}

	analyzeCommandContent := readFile(t, filepath.Join(install.codexHome, "lib", "prism", "commands", "analyze.sh"))
	for _, needle := range []string{
		"prism_psm_analyze_asset_paths",
		`prism_psm_define_command_config "analyze" "shared_skill_relative_path" "skills/analyze/SKILL.md"`,
		`prism_psm_define_command_config "analyze" "prepare_function" "prism_psm_prepare_analyze_args"`,
	} {
		if !strings.Contains(analyzeCommandContent, needle) {
			t.Fatalf("expected %q in analyze command config", needle)
		}
	}

	repoRootPointer := strings.TrimSpace(readFile(t, filepath.Join(install.codexHome, "lib", "prism", "repo-root")))
	if repoRootPointer != install.repoRoot {
		t.Fatalf("repo-root pointer = %q, want %q", repoRootPointer, install.repoRoot)
	}

	registryContent := readFile(t, filepath.Join(install.codexHome, "lib", "prism", "command-registry.tsv"))
	for _, needle := range []string{
		"analyze\tanalyze\tprism-analyze",
		"brownfield\tbrownfield\tprism-brownfield",
		"incident\tincident\tprism-incident",
		"prd\tprd\tprism-prd",
		"setup\tsetup\tprism-setup",
	} {
		if !strings.Contains(registryContent, needle) {
			t.Fatalf("expected %q in installed psm command registry", needle)
		}
	}

	ontologyContent := readFile(t, filepath.Join(install.codexHome, "lib", "prism", "command-ontology.tsv"))
	for _, needle := range []string{
		"analyze\tprism-analyze\tacceptance-bearing\tregistered",
		"brownfield\tprism-brownfield\tacceptance-bearing\tregistered",
		"incident\tprism-incident\tacceptance-bearing\tregistered",
		"prd\tprism-prd\tacceptance-bearing\tregistered",
		"setup\tprism-setup\tacceptance-bearing\tregistered",
		"analyze-workspace\tprism-analyze-workspace\tnon-acceptance-bearing\tunregistered",
		"test-analyze\tprism-test-analyze\tnon-acceptance-bearing\tunregistered",
	} {
		if !strings.Contains(ontologyContent, needle) {
			t.Fatalf("expected %q in installed psm command ontology", needle)
		}
	}
}

func TestInstalledPSMWrapperDispatchesViaSharedCodexRunner(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	fakeCodexDir := t.TempDir()
	argsPath := filepath.Join(fakeCodexDir, "args.txt")
	stdinPath := filepath.Join(fakeCodexDir, "stdin.txt")

	fakeCodexPath := filepath.Join(fakeCodexDir, "codex")
	fakeCodex := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$@\" > \"" + argsPath + "\"\ncat > \"" + stdinPath + "\"\nprintf 'ok\\n'\n"
	if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	cmd := exec.Command(filepath.Join(install.codexHome, "bin", "psm"), "prd", "/tmp/spec doc.md")
	cmd.Dir = unrelatedDir
	cmd.Env = append(
		os.Environ(),
		"HOME="+install.homeDir,
		"CODEX_HOME="+install.codexHome,
		"PRISM_REPO_PATH="+install.repoRoot,
		"PSM_CODEX_CLI_PATH="+fakeCodexPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run psm wrapper: %v\n%s", err, output)
	}

	argsOutput := readFile(t, argsPath)
	for _, needle := range []string{
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-C",
		install.repoRoot,
		"--add-dir",
		unrelatedDir,
		"-",
	} {
		if !strings.Contains(argsOutput, needle) {
			t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
		}
	}

	stdinOutput := readFile(t, stdinPath)
	for _, needle := range []string{
		"psm prd /tmp/spec\\ doc.md",
		install.repoRoot,
		unrelatedDir,
		"Treat the following as an exact Prism command invocation",
		filepath.Join(install.repoRoot, "skills", "analyze", "templates", "report.md"),
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledPSMWrapperDispatchesClosedMilestoneCommandsViaSharedRunner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		expectedSkill string
		expectedText  []string
	}{
		{
			name:          "analyze",
			args:          []string{"analyze", "cache invalidation"},
			expectedSkill: "prism-analyze",
			expectedText: []string{
				"psm analyze cache\\ invalidation",
				"Registered Prism Codex skill:\nprism-analyze",
				"skills/analyze/prompts/seed-analyst.md",
				"The shared skill is the only workflow definition. Do not restate, paraphrase, or reorder its phases, exit gates, or MCP contract in the Codex layer.",
			},
		},
		{
			name:          "brownfield",
			args:          []string{"brownfield", "defaults"},
			expectedSkill: "prism-brownfield",
			expectedText: []string{
				"psm brownfield defaults",
				"Registered Prism Codex skill:\nprism-brownfield",
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer.",
				"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
			},
		},
		{
			name:          "incident",
			args:          []string{"incident", "checkout outage"},
			expectedSkill: "prism-incident",
			expectedText: []string{
				"psm incident checkout\\ outage",
				"Registered Prism Codex skill:\nprism-incident",
				"Prism Incident Compatibility Bridge",
				"dispatch to the shared Prism incident workflow entrypoint",
			},
		},
		{
			name:          "prd",
			args:          []string{"prd", "/tmp/spec doc.md"},
			expectedSkill: "prism-prd",
			expectedText: []string{
				"psm prd /tmp/spec\\ doc.md",
				"Registered Prism Codex skill:\nprism-prd",
				"Use the shared Prism PRD skill at `",
				"Resolve shared PRD prompts, templates, and analyze handoff assets from `",
				"Do not assume the command was launched from within `~/prism`",
			},
		},
		{
			name:          "setup",
			args:          []string{"setup"},
			expectedSkill: "prism-setup",
			expectedText: []string{
				"psm setup",
				"Registered Prism Codex skill:\nprism-setup",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			install := installCodexArtifacts(t)
			unrelatedDir := t.TempDir()
			fakeCodexDir := t.TempDir()
			argsPath := filepath.Join(fakeCodexDir, "args.txt")
			stdinPath := filepath.Join(fakeCodexDir, "stdin.txt")

			fakeCodexPath := filepath.Join(fakeCodexDir, "codex")
			fakeCodex := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$@\" > \"" + argsPath + "\"\ncat > \"" + stdinPath + "\"\nprintf 'ok\\n'\n"
			if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
				t.Fatalf("write fake codex: %v", err)
			}

			cmd := exec.Command(filepath.Join(install.codexHome, "bin", "psm"), tt.args...)
			cmd.Dir = unrelatedDir
			cmd.Env = append(
				os.Environ(),
				"HOME="+install.homeDir,
				"CODEX_HOME="+install.codexHome,
				"PRISM_REPO_PATH="+install.repoRoot,
				"PSM_CODEX_CLI_PATH="+fakeCodexPath,
			)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("run psm wrapper: %v\n%s", err, output)
			}

			argsOutput := readFile(t, argsPath)
			for _, needle := range []string{
				"exec",
				"--skip-git-repo-check",
				"--dangerously-bypass-approvals-and-sandbox",
				"-C",
				install.repoRoot,
				"--add-dir",
				unrelatedDir,
				"-",
			} {
				if !strings.Contains(argsOutput, needle) {
					t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
				}
			}

			stdinOutput := readFile(t, stdinPath)
			for _, needle := range append(tt.expectedText,
				install.repoRoot,
				unrelatedDir,
				"Treat the following as an exact Prism command invocation",
			) {
				if !strings.Contains(stdinOutput, needle) {
					t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
				}
			}
		})
	}
}

func TestInstalledPSMWrapperResolvesSharedLibraryViaSymlinkedGlobalCommand(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	fakeCodexDir := t.TempDir()
	argsPath := filepath.Join(fakeCodexDir, "args.txt")
	stdinPath := filepath.Join(fakeCodexDir, "stdin.txt")

	fakeCodexPath := filepath.Join(fakeCodexDir, "codex")
	fakeCodex := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$@\" > \"" + argsPath + "\"\ncat > \"" + stdinPath + "\"\nprintf 'ok\\n'\n"
	if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	symlinkPath := filepath.Join(t.TempDir(), "psm")
	if err := os.Symlink(filepath.Join(install.codexHome, "bin", "psm"), symlinkPath); err != nil {
		t.Fatalf("symlink psm wrapper: %v", err)
	}

	cmd := exec.Command(symlinkPath, "incident", "checkout outage")
	cmd.Dir = unrelatedDir
	cmd.Env = append(
		os.Environ(),
		"HOME="+install.homeDir,
		"CODEX_HOME="+install.codexHome,
		"PRISM_REPO_PATH="+install.repoRoot,
		"PSM_CODEX_CLI_PATH="+fakeCodexPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run symlinked psm wrapper: %v\n%s", err, output)
	}

	argsOutput := readFile(t, argsPath)
	for _, needle := range []string{
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-C",
		install.repoRoot,
		"--add-dir",
		unrelatedDir,
		"-",
	} {
		if !strings.Contains(argsOutput, needle) {
			t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
		}
	}

	stdinOutput := readFile(t, stdinPath)
	for _, needle := range []string{
		"psm incident checkout\\ outage",
		"Registered Prism Codex skill:\nprism-incident",
		install.repoRoot,
		unrelatedDir,
		"Treat the following as an exact Prism command invocation",
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledPSMBrownfieldResolvesRepoRootWithoutEnvOverride(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	argsOutput, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, "brownfield", "defaults")

	for _, needle := range []string{
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-C",
		install.repoRoot,
		"--add-dir",
		unrelatedDir,
		"-",
	} {
		if !strings.Contains(argsOutput, needle) {
			t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
		}
	}

	for _, needle := range []string{
		"psm brownfield defaults",
		"Registered Prism Codex skill:\nprism-brownfield",
		install.repoRoot,
		unrelatedDir,
		"Treat the following as an exact Prism command invocation",
		filepath.Join(install.repoRoot, "skills", "brownfield", "SKILL.md"),
		"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer.",
		"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledPSMBrownfieldPreservesSharedParityAcrossSubcommands(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()

	sharedContent := readRepoFile(t, "../skills/brownfield/SKILL.md")
	for _, needle := range append([]string{
		"/prism:brownfield                # Scan repos and set defaults",
		"/prism:brownfield scan           # Scan only (no default selection)",
		"/prism:brownfield defaults       # Show current defaults",
		"/prism:brownfield set 6,18,19   # Set defaults by repo numbers",
		"In Codex, this same shared workflow is invoked through `psm brownfield`.",
		"any installed `~/.codex/skills/prism-brownfield` copy is just a managed mirror refreshed by setup.",
	}, sharedBrownfieldUserFacingBehaviorNeedles()...) {
		if !strings.Contains(sharedContent, needle) {
			t.Fatalf("expected %q in shared Prism brownfield skill", needle)
		}
	}

	tests := []struct {
		name         string
		args         []string
		promptLine   string
		promptChecks []string
	}{
		{
			name:       "default_flow",
			args:       []string{"brownfield"},
			promptLine: "psm brownfield",
			promptChecks: []string{
				filepath.Join(install.repoRoot, "skills", "brownfield", "SKILL.md"),
				"Treat the invocation as one of these exact shared-skill forms: `psm brownfield`, `psm brownfield scan`, `psm brownfield defaults`, or `psm brownfield set <indices>`.",
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer.",
			},
		},
		{
			name:       "scan_subcommand",
			args:       []string{"brownfield", "scan"},
			promptLine: "psm brownfield scan",
			promptChecks: []string{
				filepath.Join(install.repoRoot, "skills", "brownfield", "SKILL.md"),
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer.",
			},
		},
		{
			name:       "defaults_subcommand",
			args:       []string{"brownfield", "defaults"},
			promptLine: "psm brownfield defaults",
			promptChecks: []string{
				filepath.Join(install.repoRoot, "skills", "brownfield", "SKILL.md"),
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer.",
				"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
			},
		},
		{
			name:       "set_subcommand",
			args:       []string{"brownfield", "set", "6,18,19"},
			promptLine: "psm brownfield set 6\\,18\\,19",
			promptChecks: []string{
				filepath.Join(install.repoRoot, "skills", "brownfield", "SKILL.md"),
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer.",
				"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			argsOutput, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, tt.args...)

			for _, needle := range []string{
				"exec",
				"--skip-git-repo-check",
				"--dangerously-bypass-approvals-and-sandbox",
				"-C",
				install.repoRoot,
				"--add-dir",
				unrelatedDir,
				"-",
			} {
				if !strings.Contains(argsOutput, needle) {
					t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
				}
			}

			for _, needle := range append([]string{
				tt.promptLine,
				"Registered Prism Codex skill:\nprism-brownfield",
				install.repoRoot,
				unrelatedDir,
				"Treat the following as an exact Prism command invocation",
				"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
			}, tt.promptChecks...) {
				if !strings.Contains(stdinOutput, needle) {
					t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
				}
			}
		})
	}
}

func TestInstalledPSMBrownfieldDefaultsCommandUsesRepoSkillAsCanonicalSource(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	_, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, "brownfield", "defaults")

	for _, needle := range []string{
		"psm brownfield defaults",
		"Registered Prism Codex skill:\nprism-brownfield",
		"The canonical shared Prism skill for this command is:\n" + filepath.Join(install.repoRoot, "skills", "brownfield", "SKILL.md"),
		"Read and follow that shared Prism skill from the resolved Prism asset root. Treat any installed ~/.codex skill copy as a managed mirror, not as the authored source.",
		"Treat installed `~/.codex/skills/prism-brownfield` entries as setup-refreshed mirrors of the shared repo skill, not as the authored workflow source.",
		"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer.",
		"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in brownfield defaults prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledPSMSetupPreservesSharedParityAcrossSubcommands(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()

	sharedContent := readRepoFile(t, "../skills/setup/SKILL.md")
	for _, needle := range []string{
		"/prism:setup",
		"/prism:setup scan",
		"/prism:setup defaults",
		"/prism:setup set 6,18,19",
		"No GitHub repositories found in your home directory.",
		"No default repos set. Run '/prism:setup' to configure.",
		"prism is bundled as a built-in MCP server",
		"prism is bundled as a built-in MCP server — no restart needed",
		"After displaying the defaults or the empty-defaults message, confirm that prism is bundled as a built-in MCP server and no restart is needed.",
	} {
		if !strings.Contains(sharedContent, needle) {
			t.Fatalf("expected %q in shared Prism setup skill", needle)
		}
	}

	tests := []struct {
		name         string
		args         []string
		promptLine   string
		promptChecks []string
	}{
		{
			name:       "default_flow",
			args:       []string{"setup"},
			promptLine: "psm setup",
			promptChecks: []string{
				filepath.Join(install.repoRoot, "skills", "setup", "SKILL.md"),
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, brownfield flow, status text, or stop conditions in the Codex command layer.",
			},
		},
		{
			name:       "scan_subcommand",
			args:       []string{"setup", "scan"},
			promptLine: "psm setup scan",
			promptChecks: []string{
				filepath.Join(install.repoRoot, "skills", "setup", "SKILL.md"),
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, brownfield flow, status text, or stop conditions in the Codex command layer.",
			},
		},
		{
			name:       "defaults_subcommand",
			args:       []string{"setup", "defaults"},
			promptLine: "psm setup defaults",
			promptChecks: []string{
				filepath.Join(install.repoRoot, "skills", "setup", "SKILL.md"),
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, brownfield flow, status text, or stop conditions in the Codex command layer.",
				"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
			},
		},
		{
			name:       "set_subcommand",
			args:       []string{"setup", "set", "6,18,19"},
			promptLine: "psm setup set 6\\,18\\,19",
			promptChecks: []string{
				filepath.Join(install.repoRoot, "skills", "setup", "SKILL.md"),
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, brownfield flow, status text, or stop conditions in the Codex command layer.",
				"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, tt.args...)
			if !strings.Contains(stdinOutput, tt.promptLine) {
				t.Fatalf("expected %q in codex prompt\n%s", tt.promptLine, stdinOutput)
			}
			for _, needle := range tt.promptChecks {
				if !strings.Contains(stdinOutput, needle) {
					t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
				}
			}
		})
	}
}

func TestInstalledPSMAnalyzeResolvesSharedAssetRootWithoutEnvOverride(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	argsOutput, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, "analyze", "cache invalidation")

	for _, needle := range []string{
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-C",
		install.repoRoot,
		"--add-dir",
		unrelatedDir,
		"-",
	} {
		if !strings.Contains(argsOutput, needle) {
			t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
		}
	}

	for _, needle := range []string{
		"psm analyze cache\\ invalidation",
		"Registered Prism Codex skill:\nprism-analyze",
		"Deterministic Prism asset locator for the shared Prism command workflow:",
		"1. Use PRISM_REPO_PATH when it points to a Prism repo containing skills/analyze/SKILL.md.",
		"2. Otherwise use the installed repo-root pointer shipped with the shared psm integration layer.",
		"3. Otherwise infer the Prism repo root relative to the shared psm library.",
		"The resolved asset root for this invocation is:",
		"Do not resolve Prism assets from the user's working directory.",
		"The shared skill is the only workflow definition. Do not restate, paraphrase, or reorder its phases, exit gates, or MCP contract in the Codex layer.",
		"Preserve the shared analyze config schema and MCP payload contract exactly.",
		"When the shared skill asks `SELECT who you are: codex | claude`, choose `codex`, store it as `{ADAPTOR}`, and pass `adaptor: \"{ADAPTOR}\"` to `prism_analyze`.",
		filepath.Join(install.repoRoot, "skills", "analyze", "SKILL.md"),
		filepath.Join(install.repoRoot, "skills", "analyze", "templates", "report.md"),
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledPSMAnalyzeFallsBackFromInvalidEnvRepoRoot(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	fakeCodexDir := t.TempDir()
	argsPath := filepath.Join(fakeCodexDir, "args.txt")
	stdinPath := filepath.Join(fakeCodexDir, "stdin.txt")
	fakeCodexPath := filepath.Join(fakeCodexDir, "codex")

	fakeCodex := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$@\" > \"" + argsPath + "\"\ncat > \"" + stdinPath + "\"\nprintf 'ok\\n'\n"
	if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	cmd := exec.Command(filepath.Join(install.codexHome, "bin", "psm"), "analyze", "cache invalidation")
	cmd.Dir = unrelatedDir
	cmd.Env = append(
		os.Environ(),
		"HOME="+install.homeDir,
		"CODEX_HOME="+install.codexHome,
		"PRISM_REPO_PATH="+filepath.Join(unrelatedDir, "not-a-prism-repo"),
		"PSM_CODEX_CLI_PATH="+fakeCodexPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run psm wrapper with invalid PRISM_REPO_PATH: %v\n%s", err, output)
	}

	argsOutput := readFile(t, argsPath)
	if !strings.Contains(argsOutput, install.repoRoot) {
		t.Fatalf("expected wrapper to fall back to installed repo-root pointer\n%s", argsOutput)
	}

	stdinOutput := readFile(t, stdinPath)
	for _, needle := range []string{
		"The resolved asset root for this invocation is:",
		install.repoRoot,
		filepath.Join(install.repoRoot, "skills", "analyze", "SKILL.md"),
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledPSMAnalyzePrefersExplicitEnvRepoRootOverInstalledPointer(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	overrideRoot := t.TempDir()

	for _, relativePath := range []string{
		"agents/devils-advocate.md",
		"skills/analyze/SKILL.md",
		"skills/analyze/prompts/seed-analyst.md",
		"skills/analyze/prompts/perspective-generator.md",
		"skills/analyze/prompts/finding-protocol.md",
		"skills/analyze/prompts/verification-protocol.md",
		"skills/analyze/templates/report.md",
	} {
		targetPath := filepath.Join(overrideRoot, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatalf("mkdir override asset dir: %v", err)
		}
		if err := os.WriteFile(targetPath, []byte("override asset\n"), 0o644); err != nil {
			t.Fatalf("write override asset %s: %v", targetPath, err)
		}
	}

	argsOutput, stdinOutput := runInstalledPSMWithFakeCodexOptions(t, install, unrelatedDir, psmInvocationOptions{
		extraEnv: []string{
			"PRISM_REPO_PATH=" + overrideRoot,
		},
		args: []string{"analyze", "cache invalidation"},
	})

	for _, needle := range []string{
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-C",
		overrideRoot,
		"--add-dir",
		unrelatedDir,
		"-",
	} {
		if !strings.Contains(argsOutput, needle) {
			t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
		}
	}

	for _, needle := range []string{
		"The resolved asset root for this invocation is:",
		overrideRoot,
		filepath.Join(overrideRoot, "skills", "analyze", "SKILL.md"),
		filepath.Join(overrideRoot, "skills", "analyze", "templates", "report.md"),
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}

	if strings.Contains(stdinOutput, filepath.Join(install.repoRoot, "skills", "analyze", "SKILL.md")) {
		t.Fatalf("expected explicit PRISM_REPO_PATH override to win over installed repo-root pointer\n%s", stdinOutput)
	}
}

func TestInstalledPSMIsGloballyExecutableViaPATHFromUnrelatedDirectory(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()

	argsOutput, stdinOutput := runInstalledPSMWithFakeCodexOptions(t, install, unrelatedDir, psmInvocationOptions{
		invokeViaPath: true,
		extraEnv: []string{
			"PATH=" + filepath.Join(install.codexHome, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
		},
		args: []string{"analyze", "cache invalidation"},
	})

	for _, needle := range []string{
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-C",
		install.repoRoot,
		"--add-dir",
		unrelatedDir,
		"-",
	} {
		if !strings.Contains(argsOutput, needle) {
			t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
		}
	}

	for _, needle := range []string{
		"psm analyze cache\\ invalidation",
		"Registered Prism Codex skill:\nprism-analyze",
		install.repoRoot,
		unrelatedDir,
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledPSMAnalyzePromptPreservesSharedSkillDecisionFlow(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	_, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, "analyze", "cache invalidation")

	for _, needle := range []string{
		"The shared skill is the only workflow definition. Do not restate, paraphrase, or reorder its phases, exit gates, or MCP contract in the Codex layer.",
		"When the shared skill asks `SELECT who you are: codex | claude`, choose `codex`, store it as `{ADAPTOR}`, and pass `adaptor: \"{ADAPTOR}\"` to `prism_analyze`.",
		"Preserve the shared analyze config schema and MCP payload contract exactly. Do not rename, drop, or reinterpret fields such as `topic`, `input_context`, `report_template`, `seed_hints`, `session_id`, `model`, `ontology_scope`, or `perspective_injection`.",
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledPSMAnalyzeConfigAdapterPreservesSharedArtifactContract(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	invokeDir := t.TempDir()
	configDir := filepath.Join(invokeDir, "configs")
	inputDir := filepath.Join(invokeDir, "inputs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatalf("mkdir input dir: %v", err)
	}

	inputContextPath := filepath.Join(inputDir, "request.md")
	if err := os.WriteFile(inputContextPath, []byte("analyze this\n"), 0o644); err != nil {
		t.Fatalf("write input context: %v", err)
	}

	reportTemplatePath := filepath.Join(install.repoRoot, "skills", "analyze", "templates", "report.md")
	perspectiveInjectionPath := filepath.Join("skills", "analyze", "prompts", "finding-protocol.md")
	configPath := filepath.Join(configDir, "analyze.json")
	originalConfig := map[string]string{
		"topic":                 "Adapter contract coverage",
		"input_context":         filepath.Join("inputs", "request.md"),
		"report_template":       reportTemplatePath,
		"seed_hints":            "Focus on unchanged artifact contract",
		"session_id":            "session-123",
		"model":                 "claude-sonnet-4-6",
		"ontology_scope":        "repo:payments",
		"perspective_injection": perspectiveInjectionPath,
	}
	configBytes, err := json.MarshalIndent(originalConfig, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configBytes = append(configBytes, '\n')
	if err := os.WriteFile(configPath, configBytes, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	fakeCodexDir := t.TempDir()
	stdinPath := filepath.Join(fakeCodexDir, "stdin.txt")
	normalizedConfigCopy := filepath.Join(fakeCodexDir, "normalized-config.json")
	fakeCodexPath := filepath.Join(fakeCodexDir, "codex")
	fakeCodex := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
cat > %q
config_path="$(python3 - %q <<'PY'
from __future__ import annotations

import re
import sys

text = open(sys.argv[1], "r", encoding="utf-8").read()
match = re.search(r"^psm analyze --config (\S+)", text, re.MULTILINE)
if not match:
    raise SystemExit("missing normalized analyze config path in prompt")
print(match.group(1))
PY
)"
cp "${config_path}" %q
printf 'ok\n'
`, stdinPath, stdinPath, normalizedConfigCopy)
	if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	cmd := exec.Command(filepath.Join(install.codexHome, "bin", "psm"), "analyze", "--config", filepath.Join("configs", "analyze.json"))
	cmd.Dir = invokeDir
	cmd.Env = append(
		os.Environ(),
		"HOME="+install.homeDir,
		"CODEX_HOME="+install.codexHome,
		"PSM_CODEX_CLI_PATH="+fakeCodexPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run psm analyze --config: %v\n%s", err, output)
	}

	stdinOutput := readFile(t, stdinPath)
	for _, needle := range []string{
		"psm analyze --config ",
		"Prism Analyze Compatibility Bridge",
		"Shared analyze assets that remain available to Codex-side wrappers and adapters when this command delegates into analyze:",
		filepath.Join(install.repoRoot, "skills", "analyze", "SKILL.md"),
		filepath.Join(install.repoRoot, "skills", "analyze", "prompts", "seed-analyst.md"),
		filepath.Join(install.repoRoot, "skills", "analyze", "prompts", "verification-protocol.md"),
		reportTemplatePath,
		"Preserve the shared analyze config schema and MCP payload contract exactly.",
		"Path-valued analyze config fields have already been normalized for Codex execution context. Pass them through unchanged once read.",
		"The shared skill is the only workflow definition. Do not restate, paraphrase, or reorder its phases, exit gates, or MCP contract in the Codex layer.",
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}

	var gotOriginal map[string]string
	if err := json.Unmarshal([]byte(readFile(t, configPath)), &gotOriginal); err != nil {
		t.Fatalf("unmarshal original config: %v", err)
	}
	if gotOriginal["input_context"] != filepath.Join("inputs", "request.md") {
		t.Fatalf("original config input_context = %q, want relative path preserved", gotOriginal["input_context"])
	}
	if gotOriginal["perspective_injection"] != perspectiveInjectionPath {
		t.Fatalf("original config perspective_injection = %q, want %q", gotOriginal["perspective_injection"], perspectiveInjectionPath)
	}

	var normalized map[string]string
	if err := json.Unmarshal([]byte(readFile(t, normalizedConfigCopy)), &normalized); err != nil {
		t.Fatalf("unmarshal normalized config copy: %v", err)
	}

	wantNormalized := map[string]string{
		"topic":                 originalConfig["topic"],
		"input_context":         canonicalPath(t, filepath.Join(invokeDir, "inputs", "request.md")),
		"report_template":       reportTemplatePath,
		"seed_hints":            originalConfig["seed_hints"],
		"session_id":            originalConfig["session_id"],
		"model":                 originalConfig["model"],
		"ontology_scope":        originalConfig["ontology_scope"],
		"perspective_injection": filepath.Join(install.repoRoot, "skills", "analyze", "prompts", "finding-protocol.md"),
	}
	if len(normalized) != len(wantNormalized) {
		t.Fatalf("normalized config field count = %d, want %d: %#v", len(normalized), len(wantNormalized), normalized)
	}
	for key, want := range wantNormalized {
		if normalized[key] != want {
			t.Fatalf("normalized config %s = %q, want %q", key, normalized[key], want)
		}
	}
}

func TestPSMRegistryEntriesUseSharedDispatchContract(t *testing.T) {
	t.Parallel()

	registry := readPSMRegistry(t, "../codex/lib/command-registry.tsv")
	repCases := representativePSMInvocations()

	if len(registry) != len(repCases) {
		t.Fatalf("registry has %d commands but representative invocation coverage has %d", len(registry), len(repCases))
	}

	for _, entry := range registry {
		entry := entry
		t.Run(entry.Command, func(t *testing.T) {
			t.Parallel()

			content := renderGeneratedPSMSkill(t, entry.Command)

			for _, needle := range []string{
				fmt.Sprintf("name: %s", entry.SkillID),
				fmt.Sprintf("# psm %s", entry.Command),
				fmt.Sprintf("Treat `psm %s` as a command, not as natural language.", entry.Command),
				fmt.Sprintf("Glob(pattern=\"**/skills/%s/SKILL.md\")", entry.SkillDir),
				"## Shared Codex Dispatch",
				"## Codex Normalization Rules",
				"Follow that shared skill exactly.",
			} {
				if !strings.Contains(content, needle) {
					t.Fatalf("expected %q in rendered skill for %s", needle, entry.Command)
				}
			}
			if !strings.Contains(content, "Read the first match.") &&
				!strings.Contains(content, "Read the resolved shared Prism") {
				t.Fatalf("expected shared skill read instruction in rendered skill for %s", entry.Command)
			}

			if _, ok := repCases[entry.Command]; !ok {
				t.Fatalf("missing representative invocation for registry command %q", entry.Command)
			}
		})
	}
}

func TestInstalledPSMWrapperDispatchesRepresentativeRegistryCommands(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	registry := readPSMRegistry(t, "../codex/lib/command-registry.tsv")
	repCases := representativePSMInvocations()

	if len(registry) != len(repCases) {
		t.Fatalf("registry has %d commands but representative invocation coverage has %d", len(registry), len(repCases))
	}

	for _, entry := range registry {
		entry := entry
		tc, ok := repCases[entry.Command]
		if !ok {
			t.Fatalf("missing representative invocation for registry command %q", entry.Command)
		}

		t.Run(entry.Command, func(t *testing.T) {
			t.Parallel()

			argsOutput, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, tc.args...)

			for _, needle := range []string{
				"exec",
				"--skip-git-repo-check",
				"--dangerously-bypass-approvals-and-sandbox",
				"-C",
				install.repoRoot,
				"--add-dir",
				unrelatedDir,
				"-",
			} {
				if !strings.Contains(argsOutput, needle) {
					t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
				}
			}

			for _, needle := range append([]string{
				tc.promptCommand,
				"Registered Prism Codex skill:\n" + entry.SkillID,
				install.repoRoot,
				unrelatedDir,
				"Treat the following as an exact Prism command invocation",
			}, expandPromptNeedles(tc.promptNeedles, install.repoRoot)...) {
				if !strings.Contains(stdinOutput, needle) {
					t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
				}
			}
		})
	}
}

func TestInstalledPSMWrapperReusesSharedRunnerWithConfigScopedCommandDifferences(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	registry := readPSMRegistry(t, "../codex/lib/command-registry.tsv")
	repCases := representativePSMInvocations()

	type promptCapture struct {
		command    string
		skillID    string
		argsOutput string
		stdin      string
	}

	var captures []promptCapture
	for _, entry := range registry {
		tc, ok := repCases[entry.Command]
		if !ok {
			t.Fatalf("missing representative invocation for registry command %q", entry.Command)
		}

		argsOutput, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, tc.args...)
		captures = append(captures, promptCapture{
			command:    entry.Command,
			skillID:    entry.SkillID,
			argsOutput: argsOutput,
			stdin:      stdinOutput,
		})
	}

	baselineArgs := captures[0].argsOutput
	for _, capture := range captures[1:] {
		if capture.argsOutput != baselineArgs {
			t.Fatalf(
				"expected %s to reuse the same shared Codex runner args as %s\nwant:\n%s\ngot:\n%s",
				capture.command,
				captures[0].command,
				baselineArgs,
				capture.argsOutput,
			)
		}
	}

	for _, capture := range captures {
		sharedSkillPath := filepath.Join(install.repoRoot, "skills", capture.command, "SKILL.md")
		for _, needle := range []string{
			"Treat the following as an exact Prism command invocation and execute it via the installed Prism Codex skills and MCP server:",
			"Deterministic Prism asset locator for the shared Prism command workflow:",
			"The resolved asset root for this invocation is:",
			install.repoRoot,
			"The user's original working directory is:",
			unrelatedDir,
			"Use the original working directory as the project context for repo analysis and file operations. Use the Prism repository only for shared skill, prompt, template, and MCP assets.",
			"Shared analyze assets that remain available to Codex-side wrappers and adapters when this command delegates into analyze:",
			"Registered Prism Codex skill:\n" + capture.skillID,
			"The canonical shared Prism skill for this command is:\n" + sharedSkillPath,
		} {
			if !strings.Contains(capture.stdin, needle) {
				t.Fatalf("expected %q in shared framework prompt for %s\n%s", needle, capture.command, capture.stdin)
			}
		}
	}
}

func TestInstalledPSMWrapperRejectsUnsupportedCommands(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)

	cmd := exec.Command(filepath.Join(install.codexHome, "bin", "psm"), "analyze-workspace")
	cmd.Env = append(
		os.Environ(),
		"HOME="+install.homeDir,
		"CODEX_HOME="+install.codexHome,
		"PRISM_REPO_PATH="+install.repoRoot,
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected unsupported psm command to fail")
	}

	text := string(output)
	for _, needle := range []string{
		"unsupported command 'analyze-workspace'",
		"Supported commands:",
		"psm analyze",
		"psm brownfield",
		"psm incident",
		"psm prd",
		"psm setup",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected %q in unsupported-command output\n%s", needle, text)
		}
	}
}

func TestInstallCodexScriptOnlyInstallsInitialClosedSkillSet(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	entries, err := os.ReadDir(filepath.Join(install.codexHome, "skills"))
	if err != nil {
		t.Fatalf("read installed skill directory: %v", err)
	}

	var got []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		got = append(got, entry.Name())
	}

	want := []string{
		"prism-analyze",
		"prism-brownfield",
		"prism-incident",
		"prism-prd",
		"prism-setup",
	}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected installed Codex skills %v, got %v", want, got)
	}

	for _, unsupported := range []string{
		filepath.Join(install.codexHome, "skills", "prism-analyze-workspace"),
		filepath.Join(install.codexHome, "skills", "prism-test-analyze"),
	} {
		if _, err := os.Stat(unsupported); !os.IsNotExist(err) {
			t.Fatalf("expected unsupported Codex skill to be absent: %s", unsupported)
		}
	}
}

func TestPSMCommandOntologyKeepsOutOfScopeCommandsNonAcceptanceBearing(t *testing.T) {
	t.Parallel()

	ontology := readPSMCommandOntology(t, "../codex/lib/command-ontology.tsv")
	registry := readPSMRegistry(t, "../codex/lib/command-registry.tsv")

	registered := make(map[string]struct{}, len(registry))
	for _, entry := range registry {
		registered[entry.Command] = struct{}{}
	}

	for _, entry := range ontology {
		_, isRegistered := registered[entry.Command]

		switch entry.MilestoneStatus {
		case "acceptance-bearing":
			if entry.RegistrationStatus != "registered" {
				t.Fatalf("acceptance-bearing command %q must be marked registered", entry.Command)
			}
			if !isRegistered {
				t.Fatalf("acceptance-bearing command %q must remain in the executable closed set", entry.Command)
			}
		case "non-acceptance-bearing":
			if entry.RegistrationStatus != "unregistered" {
				t.Fatalf("non-acceptance-bearing command %q must be marked unregistered", entry.Command)
			}
			if isRegistered {
				t.Fatalf("non-acceptance-bearing command %q must not expand the executable closed set", entry.Command)
			}
		default:
			t.Fatalf("unknown milestone status %q for command %q", entry.MilestoneStatus, entry.Command)
		}
	}
}

func TestInstalledCodexBrownfieldSkillIsPortableAcrossWorkingDirectories(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()

	cmd := exec.Command("bash", "-lc", "pwd")
	cmd.Dir = unrelatedDir
	cmd.Env = append(
		os.Environ(),
		"HOME="+install.homeDir,
		"CODEX_HOME="+install.codexHome,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("verify unrelated working directory: %v\n%s", err, output)
	}

	ruleContent := readFile(t, filepath.Join(install.codexHome, "rules", "prism.md"))
	for _, needle := range []string{
		"| `psm brownfield` | `prism-brownfield` |",
		"| `psm brownfield scan` | `prism-brownfield` |",
		"| `psm brownfield defaults` | `prism-brownfield` |",
		"| `psm brownfield set 6,18,19` | `prism-brownfield` |",
		"| `psm setup` | `prism-setup` |",
	} {
		if !strings.Contains(ruleContent, needle) {
			t.Fatalf("expected %q in installed Codex rule", needle)
		}
	}

	config := readFile(t, install.configPath)
	for _, needle := range []string{
		`command = "` + install.runScript + `"`,
		`PRISM_AGENT_RUNTIME = "codex"`,
		`PRISM_LLM_BACKEND = "codex"`,
		`PRISM_REPO_PATH = "` + install.repoRoot + `"`,
	} {
		if !strings.Contains(config, needle) {
			t.Fatalf("expected %q in generated Codex config", needle)
		}
	}
}

func TestCodexBrownfieldSkillUsesSharedPrismMCPTool(t *testing.T) {
	t.Parallel()

	content := renderGeneratedPSMSkill(t, "brownfield")

	for _, needle := range []string{
		"name: prism-brownfield",
		"# psm brownfield",
		"Glob(pattern=\"**/skills/brownfield/SKILL.md\")",
		"Follow that shared skill exactly.",
		"psm brownfield set 6,18,19",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex skill", needle)
		}
	}
}

func TestCodexBrownfieldSkillUsesSharedDispatchFramework(t *testing.T) {
	t.Parallel()

	content := renderGeneratedPSMSkill(t, "brownfield")
	for _, needle := range []string{
		"## Shared Codex Dispatch",
		"## Codex Normalization Rules",
		"/prism:brownfield` -> `psm brownfield`",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex brownfield skill", needle)
		}
	}
	for _, forbidden := range []string{
		"ToolSearch query: \"+prism brownfield\"",
		"Tool: prism_brownfield",
		"Display only the repos marked with `*` (defaults).",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("expected brownfield wrapper to avoid embedded workflow detail %q", forbidden)
		}
	}
}

func TestInstalledCodexBrownfieldSkillMatchesRepresentativeClaudeScenarios(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	installedContent := readFile(t, filepath.Join(install.codexHome, "skills", "prism-brownfield", "SKILL.md"))
	sharedContent := readRepoFile(t, "../skills/brownfield/SKILL.md")
	if installedContent != sharedContent {
		t.Fatalf("expected installed Codex brownfield skill to match the shared repo skill")
	}

	for _, needle := range []string{
		"# /prism:brownfield",
		"/prism:brownfield scan",
		"/prism:brownfield defaults",
		"/prism:brownfield set 6,18,19",
		"Tool: prism_brownfield",
		"AskUserQuestion",
	} {
		if !strings.Contains(installedContent, needle) {
			t.Fatalf("expected %q in installed Codex brownfield skill", needle)
		}
	}
}

func TestCodexIncidentSkillDispatchesToSharedPrismWorkflow(t *testing.T) {
	t.Parallel()

	content := renderGeneratedPSMSkill(t, "incident")

	for _, needle := range []string{
		"name: prism-incident",
		"# psm incident",
		"Glob(pattern=\"**/skills/incident/SKILL.md\")",
		"`PRISM_REPO_PATH/skills/incident/SKILL.md` as the canonical shared skill path when it is available.",
		"Follow that shared skill exactly.",
		"/prism:incident` -> `psm incident`",
		"any Codex-side analyze invocation must use `psm analyze`",
		"`PRISM_REPO_PATH` when it points to a Prism repo containing `skills/incident/SKILL.md`.",
		"Do not assume the command was launched from within `~/prism` or from the user's current working directory.",
		"Treat the shared incident skill as the only workflow definition.",
		"Reuse Prism's bundled MCP tools, prompts, perspectives, and templates from the shared skill directory.",
		"Do not reimplement or paraphrase the Prism incident workflow in this Codex wrapper.",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex incident skill", needle)
		}
	}
}

func TestCodexDevModeRoutesPSMIncidentThroughCodexSkill(t *testing.T) {
	t.Parallel()

	content := readRepoFile(t, "../CLAUDE.md")

	for _, needle := range []string{
		"`psm incident`",
		"`skills/incident/SKILL.md`",
		"Treat `psm incident` as a command, not as natural language.",
		"Reuse Prism's bundled MCP tools and skill assets; do not reimplement the workflow ad hoc.",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex dev-mode incident routing", needle)
		}
	}
}

func TestInstalledPSMWrapperPreservesIncidentDescriptionAsSingleArgument(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()

	_, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, "incident", "checkout", "outage")

	for _, needle := range []string{
		"psm incident checkout\\ outage",
		"Prism Incident Compatibility Bridge",
		"passing through the shared incident report template and UX perspective injection assets unchanged.",
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledCodexIncidentSkillMatchesRepresentativeClaudeScenario(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	installedContent := readFile(t, filepath.Join(install.codexHome, "skills", "prism-incident", "SKILL.md"))
	sharedContent := readRepoFile(t, "../skills/incident/SKILL.md")

	for _, needle := range []string{
		"Incident RCA Analysis (Thin Wrapper for prism_analyze)",
		"AskUserQuestion",
		"`~/.prism/state/analyze-{short-id}/`",
		"perspective_injection",
		"`prism_analyze_result` when complete",
	} {
		if !strings.Contains(sharedContent, needle) {
			t.Fatalf("expected %q in shared Prism incident skill", needle)
		}
	}

	for _, needle := range []string{
		"# Incident RCA Analysis (Thin Wrapper for prism_analyze)",
		"`~/.prism/state/analyze-{short-id}/`",
		"`prism_analyze_result` when complete",
		"templates/report.md",
		"perspectives/ux-impact.json",
	} {
		if !strings.Contains(installedContent, needle) {
			t.Fatalf("expected %q in installed Codex incident skill", needle)
		}
	}
	if installedContent != sharedContent {
		t.Fatalf("expected installed Codex incident skill to match the shared repo skill")
	}
}

func TestCodexPRDSkillDispatchesToSharedPrismWorkflow(t *testing.T) {
	t.Parallel()

	content := renderGeneratedPSMSkill(t, "prd")

	for _, needle := range []string{
		"name: prism-prd",
		"# psm prd",
		"`PRISM_REPO_PATH` when it points to a Prism repo containing `skills/prd/SKILL.md`; otherwise fall back to `Glob(pattern=\"**/skills/prd/SKILL.md\")`.",
		"Follow that shared skill exactly.",
		"/prism:prd` -> `psm prd`",
		"`Skill(skill=\"prism:analyze\", args=\"...\")` -> `psm analyze ...`",
		"Resolve the PRD input path from the user's launch directory, not from the shared Prism repo, and fail if the referenced file does not exist.",
		"Treat the shared PRD skill as the only workflow definition.",
		"When the shared PRD skill resolves files relative to its own `SKILL.md`, bind `SKILL_DIR` to `PRISM_REPO_PATH/skills/prd`.",
		"Do not assume the command was launched from within `~/prism` or from the user's current working directory.",
		"Do not reimplement or paraphrase the Prism PRD workflow in this Codex wrapper.",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex PRD skill", needle)
		}
	}
}

func TestCodexDevModeRoutesPSMPRDThroughCodexSkill(t *testing.T) {
	t.Parallel()

	content := readRepoFile(t, "../CLAUDE.md")

	for _, needle := range []string{
		"`psm prd`",
		"`skills/prd/SKILL.md`",
		"Treat `psm prd` as a command, not as natural language.",
		"Reuse Prism's bundled MCP tools and skill assets; do not reimplement the workflow ad hoc.",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex dev-mode PRD routing", needle)
		}
	}
}

func TestInstalledCodexPRDSkillIsPortableAcrossWorkingDirectories(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()

	cmd := exec.Command("bash", "-lc", "pwd")
	cmd.Dir = unrelatedDir
	cmd.Env = append(
		os.Environ(),
		"HOME="+install.homeDir,
		"CODEX_HOME="+install.codexHome,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("verify unrelated working directory: %v\n%s", err, output)
	}

	ruleContent := readFile(t, filepath.Join(install.codexHome, "rules", "prism.md"))
	for _, needle := range []string{
		"| `psm prd /path/to/prd.md` | `prism-prd` |",
		"This initial Codex registration only covers `psm analyze`, `psm brownfield`, `psm incident`, `psm prd`, and `psm setup`.",
	} {
		if !strings.Contains(ruleContent, needle) {
			t.Fatalf("expected %q in installed Codex rule", needle)
		}
	}

	config := readFile(t, install.configPath)
	for _, needle := range []string{
		`command = "` + install.runScript + `"`,
		`PRISM_AGENT_RUNTIME = "codex"`,
		`PRISM_LLM_BACKEND = "codex"`,
		`PRISM_REPO_PATH = "` + install.repoRoot + `"`,
	} {
		if !strings.Contains(config, needle) {
			t.Fatalf("expected %q in generated Codex config", needle)
		}
	}

	installedContent := readFile(t, filepath.Join(install.codexHome, "skills", "prism-prd", "SKILL.md"))
	sharedContent := readRepoFile(t, "../skills/prd/SKILL.md")
	if installedContent != sharedContent {
		t.Fatalf("expected installed Codex PRD skill to match the shared repo skill")
	}
	for _, needle := range []string{
		"# PRD Policy Analysis (Wrapper for analyze)",
		"Skill(skill=\"prism:analyze\", args=\"--config ~/.prism/state/prd-{short-id}/analyze-config.json\")",
		"`~/.prism/state/prd-{short-id}/analyze-config.json`",
		"~/.prism/state/prd-{short-id}/prd-policy-review-report.md",
		"{PRD_DIR}/prd-policy-review-report.md",
	} {
		if !strings.Contains(installedContent, needle) {
			t.Fatalf("expected %q in installed Codex PRD skill", needle)
		}
	}
}

func TestInstalledCodexPRDSkillMatchesRepresentativeClaudeScenario(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	installedContent := readFile(t, filepath.Join(install.codexHome, "skills", "prism-prd", "SKILL.md"))
	sharedContent := readRepoFile(t, "../skills/prd/SKILL.md")

	for _, needle := range []string{
		"`~/.prism/state/prd-{short-id}/analyze-config.json`",
		"`~/.prism/state/analyze-{short-id}`",
		"`prd-policy-review-report.md` exists",
		"Return the report file path: `{PRD_STATE_DIR}/prd-policy-review-report.md`",
		"PM Decision Checklist",
		"PRD policy analysis complete.",
	} {
		if !strings.Contains(sharedContent, needle) {
			t.Fatalf("expected %q in shared Prism PRD skill", needle)
		}
	}

	for _, needle := range []string{
		"Skill(skill=\"prism:analyze\", args=\"--config ~/.prism/state/prd-{short-id}/analyze-config.json\")",
		"`~/.prism/state/prd-{short-id}/analyze-config.json`",
		"~/.prism/state/prd-{short-id}/prd-policy-review-report.md",
		"{PRD_DIR}/prd-policy-review-report.md",
		"PM Decision Checklist",
	} {
		if !strings.Contains(installedContent, needle) {
			t.Fatalf("expected %q in installed Codex PRD skill", needle)
		}
	}
	if installedContent != sharedContent {
		t.Fatalf("expected installed Codex PRD skill to match the shared repo skill")
	}
}

func TestInstalledPSMWrapperDispatchesPRDFromOutsideRepoRootWithSharedArtifactContract(t *testing.T) {
	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()

	argsOutput, stdinOutput := runInstalledPSMWithFakeCodex(t, install, unrelatedDir, "prd", "docs/spec doc.md")

	for _, needle := range []string{
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-C",
		install.repoRoot,
		"--add-dir",
		unrelatedDir,
		"-",
	} {
		if !strings.Contains(argsOutput, needle) {
			t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
		}
	}

	for _, needle := range []string{
		"psm prd docs/spec\\ doc.md",
		install.repoRoot,
		unrelatedDir,
		"Treat the following as an exact Prism command invocation",
		"Treat `psm prd ...` as the exact Codex equivalent of Claude Code `/prism:prd ...`.",
		"Preserve the PRD input path semantics from the shared skill: resolve user-provided PRD paths from `PRISM_TARGET_CWD`, not from the shared Prism repo.",
		"Preserve the generated analyze config and report artifact paths exactly as written by the shared skill.",
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestInstalledPSMSetupIsGloballyExecutableViaPATHFromUnrelatedDirectory(t *testing.T) {
	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()
	pathBinDir := t.TempDir()
	if err := os.Symlink(filepath.Join(install.codexHome, "bin", "psm"), filepath.Join(pathBinDir, "psm")); err != nil {
		t.Fatalf("symlink psm into PATH dir: %v", err)
	}

	argsOutput, stdinOutput := runInstalledPSMWithFakeCodexOptions(t, install, unrelatedDir, psmInvocationOptions{
		invokeViaPath: true,
		extraEnv: []string{
			"PATH=" + pathBinDir + string(os.PathListSeparator) + "/usr/bin:/bin:/usr/sbin:/sbin",
		},
		args: []string{"setup", "defaults"},
	})

	for _, needle := range []string{
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-C",
		install.repoRoot,
		"--add-dir",
		unrelatedDir,
		"-",
	} {
		if !strings.Contains(argsOutput, needle) {
			t.Fatalf("expected %q in codex args\n%s", needle, argsOutput)
		}
	}

	for _, needle := range []string{
		"psm setup defaults",
		"Registered Prism Codex skill:\nprism-setup",
		install.repoRoot,
		unrelatedDir,
		filepath.Join(install.repoRoot, "skills", "setup", "SKILL.md"),
		"Resolve the shared Prism setup skill deterministically from `",
		"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, brownfield flow, status text, or stop conditions in the Codex command layer.",
		"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
	} {
		if !strings.Contains(stdinOutput, needle) {
			t.Fatalf("expected %q in codex prompt\n%s", needle, stdinOutput)
		}
	}
}

func TestCodexSetupSkillDispatchesToSharedPrismWorkflow(t *testing.T) {
	t.Parallel()

	content := renderGeneratedPSMSkill(t, "setup")

	for _, needle := range []string{
		"name: prism-setup",
		"# psm setup",
		"psm setup scan",
		"psm setup defaults",
		"psm setup set 6,18,19",
		"Glob(pattern=\"**/skills/setup/SKILL.md\")",
		"`PRISM_REPO_PATH` when it points to a Prism repo containing `skills/setup/SKILL.md`; otherwise fall back to `Glob(pattern=\"**/skills/setup/SKILL.md\")`.",
		"Follow that shared skill exactly.",
		"/prism:setup` -> `psm setup`",
		"/prism:brownfield` -> `psm brownfield`",
		"Do not reimplement or paraphrase the Prism setup workflow in this Codex wrapper.",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex setup skill", needle)
		}
	}
}

func TestCodexSetupSkillUsesSharedDispatchFramework(t *testing.T) {
	t.Parallel()

	content := renderGeneratedPSMSkill(t, "setup")
	for _, needle := range []string{
		"## Shared Codex Dispatch",
		"## Codex Normalization Rules",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex setup skill", needle)
		}
	}
	for _, forbidden := range []string{
		"ToolSearch query: \"+prism brownfield\"",
		"Tool: prism_brownfield",
		"Default repo selection — IMMEDIATELY after showing the list:",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("expected setup wrapper to avoid embedded workflow detail %q", forbidden)
		}
	}
}

func TestInstalledCodexSetupSkillIsPortableAcrossWorkingDirectories(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	unrelatedDir := t.TempDir()

	cmd := exec.Command("bash", "-lc", "pwd")
	cmd.Dir = unrelatedDir
	cmd.Env = append(
		os.Environ(),
		"HOME="+install.homeDir,
		"CODEX_HOME="+install.codexHome,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("verify unrelated working directory: %v\n%s", err, output)
	}

	ruleContent := readFile(t, filepath.Join(install.codexHome, "rules", "prism.md"))
	for _, needle := range []string{
		"| `psm setup` | `prism-setup` |",
		"This initial Codex registration only covers `psm analyze`, `psm brownfield`, `psm incident`, `psm prd`, and `psm setup`.",
	} {
		if !strings.Contains(ruleContent, needle) {
			t.Fatalf("expected %q in installed Codex rule", needle)
		}
	}

	config := readFile(t, install.configPath)
	for _, needle := range []string{
		`command = "` + install.runScript + `"`,
		`PRISM_REPO_PATH = "` + install.repoRoot + `"`,
	} {
		if !strings.Contains(config, needle) {
			t.Fatalf("expected %q in generated Codex config", needle)
		}
	}

	installedContent := readFile(t, filepath.Join(install.codexHome, "skills", "prism-setup", "SKILL.md"))
	sharedContent := readRepoFile(t, "../skills/setup/SKILL.md")
	if installedContent != sharedContent {
		t.Fatalf("expected installed Codex setup skill to match the shared repo skill")
	}
	for _, needle := range []string{
		"# Prism Setup",
		"/prism:setup scan",
		"/prism:setup defaults",
		"/prism:setup set 6,18,19",
		"`bash ${PRISM_REPO_PATH}/scripts/setup.sh --runtime codex`",
		"`~/.prism/config.yaml`",
	} {
		if !strings.Contains(installedContent, needle) {
			t.Fatalf("expected %q in installed Codex setup skill", needle)
		}
	}
}

func TestCodexDevModeRoutesPSMSetupThroughCodexSkill(t *testing.T) {
	t.Parallel()

	content := readRepoFile(t, "../CLAUDE.md")

	for _, needle := range []string{
		"`psm setup`",
		"`skills/setup/SKILL.md`",
		"Treat `psm setup` as a command, not as natural language.",
		"Reuse Prism's bundled MCP tools and skill assets; do not reimplement the workflow ad hoc.",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in Codex dev-mode setup routing", needle)
		}
	}
}

func TestClaudeDevModeSlashCommandsResolveToRepoSkills(t *testing.T) {
	t.Parallel()

	claudeContent := readRepoFile(t, "../CLAUDE.md")
	registry := readPSMRegistry(t, "../codex/lib/command-registry.tsv")

	for _, entry := range registry {
		entry := entry
		t.Run(entry.Command, func(t *testing.T) {
			t.Parallel()

			slashCommand := fmt.Sprintf("`/prism:%s ...`", entry.Command)
			sharedSkillPath := fmt.Sprintf("`skills/%s/SKILL.md`", entry.SkillDir)
			if !strings.Contains(claudeContent, slashCommand) {
				t.Fatalf("expected %q in CLAUDE.md slash-command routing", slashCommand)
			}
			if !strings.Contains(claudeContent, sharedSkillPath) {
				t.Fatalf("expected %q in CLAUDE.md slash-command routing", sharedSkillPath)
			}

			commandContent := readRepoFile(t, filepath.Join("..", "commands", entry.Command+".md"))
			if !strings.Contains(commandContent, fmt.Sprintf("skills/%s/SKILL.md", entry.SkillDir)) {
				t.Fatalf("expected commands/%s.md to resolve the shared repo skill", entry.Command)
			}
		})
	}
}

func TestClaudeSetupDoesNotInstallDuplicateSlashCommandArtifacts(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	repoRoot := filepath.Clean(filepath.Join(mustGetwd(t), ".."))
	setupScript := filepath.Join(repoRoot, "scripts", "setup.sh")

	cmd := exec.Command("bash", setupScript, "--runtime", "claude")
	cmd.Dir = t.TempDir()
	cmd.Env = append(
		os.Environ(),
		"HOME="+homeDir,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup.sh --runtime claude failed: %v\n%s", err, output)
	}

	outputText := string(output)
	for _, needle := range []string{
		"Prism runtime configured for Claude Code.",
		"Claude uses the checked-in Prism commands/ and skills/ directories directly.",
		"No duplicate Claude slash-command artifacts were installed or synced.",
	} {
		if !strings.Contains(outputText, needle) {
			t.Fatalf("expected %q in Claude setup output\n%s", needle, outputText)
		}
	}

	configPath := filepath.Join(homeDir, ".prism", "config.yaml")
	configContent := readFile(t, configPath)
	for _, needle := range []string{
		"backend: claude",
		"llm:",
	} {
		if !strings.Contains(configContent, needle) {
			t.Fatalf("expected %q in Claude runtime config\n%s", needle, configContent)
		}
	}

	for _, forbiddenPath := range []string{
		filepath.Join(homeDir, ".prism", "commands"),
		filepath.Join(homeDir, ".prism", "skills"),
		filepath.Join(homeDir, ".claude", "commands"),
		filepath.Join(homeDir, ".claude", "skills"),
		filepath.Join(homeDir, ".codex", "skills", "prism-setup"),
	} {
		if _, err := os.Stat(forbiddenPath); !os.IsNotExist(err) {
			t.Fatalf("expected no generated duplicate Claude command artifact at %s", forbiddenPath)
		}
	}
}

func TestPRDCommandDispatchesToSharedSkill(t *testing.T) {
	t.Parallel()

	content := readRepoFile(t, "../commands/prd.md")

	for _, needle := range []string{
		`description: "Run Prism PRD policy analysis"`,
		"`${CLAUDE_PLUGIN_ROOT}/skills/prd/SKILL.md`",
		"follow its instructions exactly",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in PRD command wiring", needle)
		}
	}
}

func TestSetupCommandDispatchesToSharedSkill(t *testing.T) {
	t.Parallel()

	content := readRepoFile(t, "../commands/setup.md")

	for _, needle := range []string{
		`description: "Run Prism setup workflow"`,
		"`${CLAUDE_PLUGIN_ROOT}/skills/setup/SKILL.md`",
		"follow its instructions exactly",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in setup command wiring", needle)
		}
	}
}

func TestBrownfieldCommandDispatchesToSharedSkill(t *testing.T) {
	t.Parallel()

	content := readRepoFile(t, "../commands/brownfield.md")

	for _, needle := range []string{
		`description: "Run Prism brownfield repository setup"`,
		"`${CLAUDE_PLUGIN_ROOT}/skills/brownfield/SKILL.md`",
		"follow its instructions exactly",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in brownfield command wiring", needle)
		}
	}
}

func TestPSMEntrypointsAreRenderedFromSharedFrameworkConfigs(t *testing.T) {
	t.Parallel()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, ".."))
	psmSource := filepath.Join(repoRoot, "codex", "lib", "psm.sh")
	commands := []string{"analyze", "brownfield", "incident", "prd", "setup"}

	for _, commandName := range commands {
		commandName := commandName
		t.Run(commandName, func(t *testing.T) {
			t.Parallel()

			renderedCommand := renderPSMEntrypoint(t, repoRoot, psmSource, "prism_psm_render_command_markdown", commandName)
			actualCommand := readRepoFile(t, filepath.Join("..", "commands", commandName+".md"))
			if actualCommand != renderedCommand {
				t.Fatalf("command entrypoint for %s drifted from shared framework renderer", commandName)
			}
		})
	}
}

func readRepoFile(t *testing.T, relativePath string) string {
	t.Helper()

	path := filepath.Join(mustGetwd(t), relativePath)
	return readFile(t, path)
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func renderPSMEntrypoint(t *testing.T, repoRoot, psmSource, renderFunc, commandName string) string {
	t.Helper()

	codexArtifactsMu.Lock()
	defer codexArtifactsMu.Unlock()

	cmd := exec.Command(
		"bash",
		"-lc",
		fmt.Sprintf("source %q && %s %q", psmSource, renderFunc, commandName),
	)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("render %s for %s: %v\n%s", renderFunc, commandName, err, output)
	}
	return string(output)
}

func renderGeneratedPSMSkill(t *testing.T, commandName string) string {
	t.Helper()

	repoRoot := filepath.Clean(filepath.Join(mustGetwd(t), ".."))
	psmSource := filepath.Join(repoRoot, "codex", "lib", "psm.sh")
	return renderPSMEntrypoint(t, repoRoot, psmSource, "prism_psm_render_codex_skill", commandName)
}

func mustGetwd(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}
	return filepath.Dir(file)
}

type codexInstallPaths struct {
	homeDir    string
	codexHome  string
	configPath string
	repoRoot   string
	runScript  string
}

func installCodexArtifacts(t *testing.T) codexInstallPaths {
	t.Helper()

	codexArtifactsMu.Lock()
	defer codexArtifactsMu.Unlock()

	homeDir, err := os.MkdirTemp("/tmp", "prism-codex-install-*")
	if err != nil {
		t.Fatalf("mktemp for codex install home: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(homeDir)
	})
	codexHome := filepath.Join(homeDir, ".codex")
	repoRoot := filepath.Clean(filepath.Join(mustGetwd(t), ".."))
	installScript := filepath.Join(repoRoot, "scripts", "install-codex.sh")

	cmd := exec.Command("bash", installScript)
	cmd.Dir = t.TempDir()
	cmd.Env = buildTestEnv(
		os.Environ(),
		"HOME="+homeDir,
		"CODEX_HOME="+codexHome,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install-codex.sh failed: %v\n%s", err, output)
	}

	return codexInstallPaths{
		homeDir:    homeDir,
		codexHome:  codexHome,
		configPath: filepath.Join(codexHome, "config.toml"),
		repoRoot:   repoRoot,
		runScript:  filepath.Join(repoRoot, "mcp", "run.sh"),
	}
}

type psmRegistryEntry struct {
	Command  string
	SkillDir string
	SkillID  string
}

type psmCommandOntologyEntry struct {
	Command            string
	SkillID            string
	MilestoneStatus    string
	RegistrationStatus string
}

type psmRepresentativeInvocation struct {
	args          []string
	promptCommand string
	promptNeedles []string
}

func readPSMRegistry(t *testing.T, relativePath string) []psmRegistryEntry {
	t.Helper()

	content := readRepoFile(t, relativePath)
	var entries []psmRegistryEntry
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("invalid registry row %q in %s", rawLine, relativePath)
		}

		entries = append(entries, psmRegistryEntry{
			Command:  fields[0],
			SkillDir: fields[1],
			SkillID:  fields[2],
		})
	}

	return entries
}

func readPSMCommandOntology(t *testing.T, relativePath string) []psmCommandOntologyEntry {
	t.Helper()

	content := readRepoFile(t, relativePath)
	var entries []psmCommandOntologyEntry
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("invalid ontology row %q in %s", rawLine, relativePath)
		}

		entries = append(entries, psmCommandOntologyEntry{
			Command:            fields[0],
			SkillID:            fields[1],
			MilestoneStatus:    fields[2],
			RegistrationStatus: fields[3],
		})
	}

	return entries
}

func representativePSMInvocations() map[string]psmRepresentativeInvocation {
	return map[string]psmRepresentativeInvocation{
		"analyze": {
			args:          []string{"analyze", "cache invalidation"},
			promptCommand: "psm analyze cache\\ invalidation",
			promptNeedles: []string{
				"{REPO_ROOT}/skills/analyze/templates/report.md",
				"The shared skill is the only workflow definition. Do not restate, paraphrase, or reorder its phases, exit gates, or MCP contract in the Codex layer.",
				"When the shared skill asks `SELECT who you are: codex | claude`, choose `codex`, store it as `{ADAPTOR}`, and pass `adaptor: \"{ADAPTOR}\"` to `prism_analyze`.",
			},
		},
		"brownfield": {
			args:          []string{"brownfield", "defaults"},
			promptCommand: "psm brownfield defaults",
			promptNeedles: []string{
				"{REPO_ROOT}/skills/brownfield/SKILL.md",
				"Treat installed `~/.codex/skills/prism-brownfield` entries as setup-refreshed mirrors of the shared repo skill, not as the authored workflow source.",
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer.",
				"Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary.",
			},
		},
		"incident": {
			args:          []string{"incident", "checkout outage"},
			promptCommand: "psm incident checkout\\ outage",
			promptNeedles: []string{
				"{REPO_ROOT}/skills/incident/SKILL.md",
				"Prism Incident Compatibility Bridge",
				"by passing through the shared incident report template and UX perspective injection assets unchanged.",
			},
		},
		"prd": {
			args:          []string{"prd", "/tmp/spec doc.md"},
			promptCommand: "psm prd /tmp/spec\\ doc.md",
		},
		"setup": {
			args:          []string{"setup", "defaults"},
			promptCommand: "psm setup defaults",
			promptNeedles: []string{
				"{REPO_ROOT}/skills/setup/SKILL.md",
				"Resolve the shared Prism setup skill deterministically from `",
				"Treat that shared skill as the only workflow definition; do not duplicate its phase logic, brownfield flow, status text, or stop conditions in the Codex command layer.",
			},
		},
	}
}

func TestGeneratedCodexSkillVersionMatchesSharedSkillFrontmatter(t *testing.T) {
	t.Parallel()

	install := installCodexArtifacts(t)
	tests := []struct {
		command string
		version string
	}{
		{command: "analyze", version: "7.2.0"},
		{command: "brownfield", version: "2.0.0"},
		{command: "incident", version: "2.1.0"},
		{command: "prd", version: "1.0.0"},
		{command: "setup", version: "2.0.0"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.command, func(t *testing.T) {
			content := readFile(t, filepath.Join(install.codexHome, "skills", "prism-"+tt.command, "SKILL.md"))
			needle := "version: " + tt.version
			if !strings.Contains(content, needle) {
				t.Fatalf("expected %q in generated Codex skill for %s", needle, tt.command)
			}
		})
	}
}

func sharedBrownfieldUserFacingBehaviorNeedles() []string {
	return []string{
		"No GitHub repositories found in your home directory.",
		"No default repos set. Run '/prism:brownfield' to configure.",
		"No default repos set. Interviews will run in greenfield mode.",
		"Brownfield defaults updated!",
	}
}

func expandPromptNeedles(needles []string, repoRoot string) []string {
	expanded := make([]string, 0, len(needles))
	for _, needle := range needles {
		expanded = append(expanded, strings.ReplaceAll(needle, "{REPO_ROOT}", repoRoot))
	}
	return expanded
}

func shellEscapeArg(arg string) string {
	return "'" + strings.ReplaceAll(arg, "'", `'"'"'`) + "'"
}

func runInstalledPSMWithFakeCodex(t *testing.T, install codexInstallPaths, workingDir string, args ...string) (string, string) {
	t.Helper()

	return runInstalledPSMWithFakeCodexOptions(t, install, workingDir, psmInvocationOptions{
		args: args,
	})
}

type psmInvocationOptions struct {
	args          []string
	extraEnv      []string
	invokeViaPath bool
}

func runInstalledPSMWithFakeCodexOptions(t *testing.T, install codexInstallPaths, workingDir string, opts psmInvocationOptions) (string, string) {
	t.Helper()

	fakeCodexDir := t.TempDir()
	argsPath := filepath.Join(fakeCodexDir, "args.txt")
	stdinPath := filepath.Join(fakeCodexDir, "stdin.txt")
	fakeCodexPath := filepath.Join(fakeCodexDir, "codex")

	fakeCodex := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$@\" > \"" + argsPath + "\"\ncat > \"" + stdinPath + "\"\nprintf 'ok\\n'\n"
	if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	commandPath := filepath.Join(install.codexHome, "bin", "psm")
	commandArgs := opts.args
	if opts.invokeViaPath {
		commandPath = "bash"
		escapedArgs := make([]string, 0, len(commandArgs)+1)
		escapedArgs = append(escapedArgs, "psm")
		for _, arg := range commandArgs {
			escapedArgs = append(escapedArgs, shellEscapeArg(arg))
		}
		commandArgs = []string{"-c", strings.Join(escapedArgs, " ")}
	}

	cmd := exec.Command(commandPath, commandArgs...)
	cmd.Dir = workingDir
	cmd.Env = buildTestEnv(
		os.Environ(),
		append([]string{
			"HOME=" + install.homeDir,
			"CODEX_HOME=" + install.codexHome,
			"PSM_CODEX_CLI_PATH=" + fakeCodexPath,
		}, opts.extraEnv...)...,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run psm wrapper: %v\n%s", err, output)
	}

	return readFile(t, argsPath), readFile(t, stdinPath)
}

func buildTestEnv(base []string, overrides ...string) []string {
	envMap := make(map[string]string, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))

	record := func(entry string) {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			return
		}
		if _, exists := envMap[key]; !exists {
			order = append(order, key)
		}
		envMap[key] = value
	}

	for _, entry := range base {
		record(entry)
	}
	for _, entry := range overrides {
		record(entry)
	}

	sort.Strings(order)
	result := make([]string, 0, len(order))
	for _, key := range order {
		result = append(result, key+"="+envMap[key])
	}
	return result
}
