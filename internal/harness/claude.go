package harness

import (
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
)

type claudeHarness struct{}

var managedClaudeDirItems = []string{
	"CLAUDE.md",
	"settings.json",
	"settings.local.json",
	"keybindings.json",
	"statusline-command.sh",
	"commands",
	"skills",
	"agents",
	"hooks",
	"rules",
	"output-styles",
	"teams",
}

func Claude() Harness {
	return claudeHarness{}
}

func (claudeHarness) Name() string {
	return "claude"
}

func (claudeHarness) SupportsScope(scope config.Scope) bool {
	return scope == config.ScopeGlobal || scope == config.ScopeLocal
}

func (claudeHarness) TargetDir(scope config.Scope, projectPath string) string {
	if scope == config.ScopeGlobal {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".claude")
	}
	return filepath.Join(projectPath, ".claude")
}

func (claudeHarness) ManagedDirItems() []string {
	return append([]string{}, managedClaudeDirItems...)
}

func (claudeHarness) ExternalManagedPath(scope config.Scope, projectPath string) (ManagedPath, bool) {
	home, _ := os.UserHomeDir()
	switch scope {
	case config.ScopeGlobal:
		return ManagedPath{
			ProfilePath: ".claude.json",
			LivePath:    filepath.Join(home, ".claude.json"),
		}, true
	case config.ScopeLocal:
		return ManagedPath{
			ProfilePath: ".mcp.json",
			LivePath:    filepath.Join(projectPath, ".mcp.json"),
		}, true
	default:
		return ManagedPath{}, false
	}
}

func (h claudeHarness) ProfileDiscoveryItems() []string {
	items := append([]string{}, h.ManagedDirItems()...)
	items = append(items, ".claude.json", ".mcp.json")
	return items
}

func (claudeHarness) MarkdownInstructionsFile() string {
	return "CLAUDE.md"
}

func (claudeHarness) IsUserMCPPath(profilePath string) bool {
	return profilePath == ".claude.json"
}

func (claudeHarness) IsMCPPath(profilePath string) bool {
	return profilePath == ".claude.json" || profilePath == ".mcp.json"
}
