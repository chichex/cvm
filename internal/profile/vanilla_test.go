package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/harness"
)

type testHarness struct {
	name      string
	targetDir string
}

func (h testHarness) Name() string {
	return h.name
}

func (h testHarness) TargetDir(scope config.Scope, projectPath string) string {
	if scope == config.ScopeLocal {
		return filepath.Join(projectPath, "."+h.name)
	}
	return h.targetDir
}

func (h testHarness) DefaultAssetDir(profileDir string) string {
	return h.name
}

func (h testHarness) ScaffoldAsset(kind, name string) (harness.ScaffoldAsset, error) {
	return harness.ScaffoldAsset{ProfilePath: "CONFIG.md", Content: "", Mode: 0644}, nil
}

func (h testHarness) ManagedDirItems() []string {
	return []string{"CONFIG.md"}
}

func (h testHarness) ExternalManagedPath(scope config.Scope, projectPath string) (harness.ManagedPath, bool) {
	return harness.ManagedPath{}, false
}

func (h testHarness) ProfileDiscoveryItems() []string {
	return h.ManagedDirItems()
}

func (h testHarness) MarkdownInstructionsFile() string {
	return "CONFIG.md"
}

func (h testHarness) IsUserMCPPath(profilePath string) bool {
	return false
}

func (h testHarness) IsMCPPath(profilePath string) bool {
	return false
}

func TestVanillaBackupIsScopedByHarness(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir claude dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("claude vanilla"), 0644); err != nil {
		t.Fatalf("write claude vanilla: %v", err)
	}
	if err := EnsureVanillaWithHarness(config.ScopeGlobal, "", harness.Claude()); err != nil {
		t.Fatalf("ensure claude vanilla: %v", err)
	}

	other := testHarness{
		name:      "opencode",
		targetDir: filepath.Join(home, ".opencode"),
	}
	if err := os.MkdirAll(other.targetDir, 0755); err != nil {
		t.Fatalf("mkdir other harness dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other.targetDir, "CONFIG.md"), []byte("opencode vanilla"), 0644); err != nil {
		t.Fatalf("write other harness vanilla: %v", err)
	}
	if err := EnsureVanillaWithHarness(config.ScopeGlobal, "", other); err != nil {
		t.Fatalf("ensure other harness vanilla: %v", err)
	}

	if _, err := os.Stat(filepath.Join(config.GlobalVanillaDir(), "CLAUDE.md")); err != nil {
		t.Fatalf("expected legacy Claude vanilla backup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(config.GlobalVanillaDir(), other.Name(), "CONFIG.md")); err != nil {
		t.Fatalf("expected harness-scoped vanilla backup: %v", err)
	}
}
