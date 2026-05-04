package harness

import (
	"path/filepath"
	"testing"

	"github.com/chichex/cvm/internal/config"
)

func TestCodexTargetDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")

	h := Codex()
	if got, want := h.TargetDir(config.ScopeGlobal, ""), filepath.Join(home, ".codex"); got != want {
		t.Fatalf("global target = %q, want %q", got, want)
	}

	project := filepath.Join(home, "project")
	if got, want := h.TargetDir(config.ScopeLocal, project), filepath.Join(project, ".codex"); got != want {
		t.Fatalf("local target = %q, want %q", got, want)
	}
}

func TestCodexTargetDirUsesCodexHome(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(home, "custom-codex")
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", custom)

	if got := Codex().TargetDir(config.ScopeGlobal, ""); got != custom {
		t.Fatalf("global target with CODEX_HOME = %q, want %q", got, custom)
	}
}

func TestCodexScaffoldAssetSupportsInstructionsOnly(t *testing.T) {
	asset, err := Codex().ScaffoldAsset("instructions", "")
	if err != nil {
		t.Fatalf("scaffold instructions: %v", err)
	}
	if got, want := asset.ProfilePath, "AGENTS.md"; got != want {
		t.Fatalf("profile path = %q, want %q", got, want)
	}

	if _, err := Codex().ScaffoldAsset("skill", "deploy"); err == nil {
		t.Fatal("codex skill scaffolding should fail")
	}
}
