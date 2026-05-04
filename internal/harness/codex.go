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

func (codexHarness) SupportsScope(scope config.Scope) bool {
	return scope == config.ScopeGlobal
}

func (codexHarness) TargetDir(scope config.Scope, projectPath string) string {
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
	// Codex MCP support is not managed by cvm yet; config.toml stays user-owned
	// and outside ManagedDirItems until cvm has explicit TOML merge semantics.
	return false
}

func (codexHarness) IsMCPPath(profilePath string) bool {
	// Revisit this if Codex gains a managed MCP asset so additive merge rules apply.
	return false
}
