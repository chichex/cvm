package harness

import (
	"os"
	"path/filepath"
)

type opencodeHarness struct{}

var managedOpenCodeDirItems = []string{
	"AGENTS.md",
	// The whole opencode.json is owned by the profile and symlinked in.
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

func (opencodeHarness) TargetDir() string {
	if dir := os.Getenv("OPENCODE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode")
}

func (opencodeHarness) ManagedDirItems() []string {
	return append([]string{}, managedOpenCodeDirItems...)
}

func (h opencodeHarness) ProfileDiscoveryItems() []string {
	return h.ManagedDirItems()
}
