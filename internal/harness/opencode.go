package harness

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
)

type opencodeHarness struct{}

var managedOpenCodeDirItems = []string{
	"AGENTS.md",
	// Only mcpServers inside opencode.json are managed; other user config is preserved.
	"opencode.json",
	"skills",
	"agents",
	"commands",
}

func OpenCode() Harness {
	return opencodeHarness{}
}

func (opencodeHarness) Name() string {
	return "opencode"
}

func (opencodeHarness) TargetDir(scope config.Scope, projectPath string) string {
	if scope == config.ScopeLocal {
		// OPENCODE_CONFIG_DIR only redirects the global config; project config remains local.
		return filepath.Join(projectPath, ".opencode")
	}
	if dir := os.Getenv("OPENCODE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode")
}

func (h opencodeHarness) DefaultAssetDir(profileDir string) string {
	return h.Name()
}

func (h opencodeHarness) ScaffoldAsset(kind, name string) (ScaffoldAsset, error) {
	switch kind {
	case "instructions":
		return ScaffoldAsset{ProfilePath: h.MarkdownInstructionsFile(), Content: "# Profile Instructions\n\n", Mode: 0644}, nil
	case "skill":
		return ScaffoldAsset{ProfilePath: filepath.Join("skills", name, "SKILL.md"), Content: "---\ndescription: TODO\n---\n\n", Mode: 0644}, nil
	case "agent":
		return ScaffoldAsset{ProfilePath: filepath.Join("agents", name+".md"), Content: "# " + name + "\n\n", Mode: 0644}, nil
	default:
		return ScaffoldAsset{}, fmt.Errorf("opencode does not support %s scaffolding", kind)
	}
}

func (opencodeHarness) ManagedDirItems() []string {
	return append([]string{}, managedOpenCodeDirItems...)
}

func (opencodeHarness) ExternalManagedPath(scope config.Scope, projectPath string) (ManagedPath, bool) {
	// OpenCode keeps config inside TargetDir, unlike Claude's external MCP files.
	return ManagedPath{}, false
}

func (h opencodeHarness) ProfileDiscoveryItems() []string {
	return h.ManagedDirItems()
}

func (opencodeHarness) MarkdownInstructionsFile() string {
	return "AGENTS.md"
}

func (opencodeHarness) IsUserMCPPath(profilePath string) bool {
	return profilePath == "opencode.json"
}

func (opencodeHarness) IsMCPPath(profilePath string) bool {
	return profilePath == "opencode.json"
}
