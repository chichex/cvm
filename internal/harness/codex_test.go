package harness

import (
	"path/filepath"
	"testing"
)

func TestCodexTargetDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")

	if got, want := Codex().TargetDir(), filepath.Join(home, ".codex"); got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}
}

func TestCodexTargetDirUsesCodexHome(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(home, "custom-codex")
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", custom)

	if got := Codex().TargetDir(); got != custom {
		t.Fatalf("target with CODEX_HOME = %q, want %q", got, custom)
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
