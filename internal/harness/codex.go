package harness

import (
	"os"
	"path/filepath"
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

func (codexHarness) TargetDir() string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

func (codexHarness) ManagedDirItems() []string {
	return append([]string{}, managedCodexDirItems...)
}

func (h codexHarness) ProfileDiscoveryItems() []string {
	return h.ManagedDirItems()
}
