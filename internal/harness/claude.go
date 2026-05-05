package harness

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/settings"
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

func (claudeHarness) TargetDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
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

func (claudeHarness) ExternalManagedPath() (ManagedPath, bool) {
	home, _ := os.UserHomeDir()
	return ManagedPath{
		ProfilePath: ".claude.json",
		LivePath:    filepath.Join(home, ".claude.json"),
	}, true
}

func (h claudeHarness) ProfileDiscoveryItems() []string {
	items := append([]string{}, h.ManagedDirItems()...)
	items = append(items, ".claude.json")
	return items
}

func (claudeHarness) MarkdownInstructionsFile() string {
	return "CLAUDE.md"
}

func (claudeHarness) SupportsPortableSkills() bool {
	return true
}

func (claudeHarness) SupportsPortableAgents() bool {
	return true
}

func (claudeHarness) IsUserMCPPath(profilePath string) bool {
	return profilePath == ".claude.json"
}

func (claudeHarness) IsMCPPath(profilePath string) bool {
	return profilePath == ".claude.json"
}

// EnableBypass writes Claude's bypass permissions block to the profile's
// override settings.json. Reapply (called by the bypass command after) will
// then merge it into the live ~/.claude/settings.json.
func (claudeHarness) EnableBypass(profileName string) error {
	overrideDir := config.OverrideDir(profileName)
	overrideSettings := filepath.Join(overrideDir, "settings.json")

	cfg, err := settings.Read(overrideSettings)
	if err != nil {
		return err
	}
	for k, v := range settings.BypassConfig() {
		cfg[k] = v
	}
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		return err
	}
	return settings.Write(overrideSettings, cfg)
}

// DisableBypass strips the bypass permissions block from the profile's
// override settings.json. The override file is removed entirely if no other
// keys remain.
func (claudeHarness) DisableBypass(profileName string) error {
	overrideSettings := filepath.Join(config.OverrideDir(profileName), "settings.json")
	cfg, err := settings.Read(overrideSettings)
	if err != nil {
		return err
	}
	if settings.RemovePermissions(cfg) {
		if err := os.Remove(overrideSettings); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return settings.Write(overrideSettings, cfg)
}

// BypassStatus reads the override settings.json and returns the current
// permissions.defaultMode value. Empty string means "not bypassed".
func (claudeHarness) BypassStatus(profileName string) (string, error) {
	overrideSettings := filepath.Join(config.OverrideDir(profileName), "settings.json")
	mode, err := settings.GetPermissionsMode(overrideSettings)
	if err != nil {
		return "", err
	}
	return mode, nil
}
