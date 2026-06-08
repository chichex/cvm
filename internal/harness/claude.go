package harness

import (
	"os"
	"path/filepath"
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

func (claudeHarness) ManagedDirItems() []string {
	return append([]string{}, managedClaudeDirItems...)
}

func (h claudeHarness) ProfileDiscoveryItems() []string {
	return h.ManagedDirItems()
}
