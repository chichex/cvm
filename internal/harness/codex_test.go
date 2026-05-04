package harness

import (
	"path/filepath"
	"testing"

	"github.com/chichex/cvm/internal/config"
)

func TestCodexTargetDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	h := Codex()
	if !h.SupportsScope(config.ScopeGlobal) {
		t.Fatal("codex should support global scope")
	}
	if h.SupportsScope(config.ScopeLocal) {
		t.Fatal("codex should not support local scope")
	}
	if got, want := h.TargetDir(config.ScopeGlobal, ""), filepath.Join(home, ".codex"); got != want {
		t.Fatalf("global target = %q, want %q", got, want)
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
