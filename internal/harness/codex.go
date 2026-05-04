package harness

import (
	"fmt"
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

func (h codexHarness) DefaultAssetDir(profileDir string) string {
	return h.Name()
}

func (h codexHarness) ScaffoldAsset(kind, name string) (ScaffoldAsset, error) {
	switch kind {
	case "instructions":
		return ScaffoldAsset{ProfilePath: h.MarkdownInstructionsFile(), Content: "# Profile Instructions\n\n", Mode: 0644}, nil
	default:
		return ScaffoldAsset{}, fmt.Errorf("codex does not support %s scaffolding", kind)
	}
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
