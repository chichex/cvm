package harness

import (
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
)

type codexHarness struct{}

var managedCodexDirItems = []string{
	"AGENTS.md",
}

func Codex() Harness {
	return codexHarness{}
}

func (codexHarness) Name() string {
	return "codex"
}

func (codexHarness) TargetDir(scope config.Scope, projectPath string) string {
	if scope == config.ScopeLocal {
		return filepath.Join(projectPath, ".codex")
	}
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

func (codexHarness) ManagedDirItems() []string {
	return append([]string{}, managedCodexDirItems...)
}

func (codexHarness) ExternalManagedPath(scope config.Scope, projectPath string) (ManagedPath, bool) {
	return ManagedPath{}, false
}

func (h codexHarness) ProfileDiscoveryItems() []string {
	return h.ManagedDirItems()
}

func (codexHarness) MarkdownInstructionsFile() string {
	return "AGENTS.md"
}

func (codexHarness) IsUserMCPPath(profilePath string) bool {
	return false
}

func (codexHarness) IsMCPPath(profilePath string) bool {
	return false
}
