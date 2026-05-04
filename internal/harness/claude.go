package harness

import (
	"fmt"
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

func (claudeHarness) TargetDir(scope config.Scope, projectPath string) string {
	if scope == config.ScopeGlobal {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".claude")
	}
	return filepath.Join(projectPath, ".claude")
}

func (h claudeHarness) DefaultAssetDir(profileDir string) string {
	for _, item := range h.ProfileDiscoveryItems() {
		if _, err := os.Stat(filepath.Join(profileDir, item)); err == nil {
			return "."
		}
	}
	return h.Name()
}

func (h claudeHarness) ScaffoldAsset(kind, name string) (ScaffoldAsset, error) {
	switch kind {
	case "instructions":
		return ScaffoldAsset{ProfilePath: h.MarkdownInstructionsFile(), Content: "# Profile Instructions\n\n", Mode: 0644}, nil
	case "skill":
		return ScaffoldAsset{ProfilePath: filepath.Join("skills", name, "SKILL.md"), Content: "---\ndescription: TODO\n---\n\n", Mode: 0644}, nil
	case "agent":
		return ScaffoldAsset{ProfilePath: filepath.Join("agents", name+".md"), Content: "# " + name + "\n\n", Mode: 0644}, nil
	case "hook":
		return ScaffoldAsset{ProfilePath: filepath.Join("hooks", name+".sh"), Content: "#!/usr/bin/env bash\nset -euo pipefail\n\n", Mode: 0755}, nil
	default:
		return ScaffoldAsset{}, fmt.Errorf("claude does not support %s scaffolding", kind)
	}
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
