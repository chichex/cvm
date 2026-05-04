package harness

import (
	"path/filepath"
	"testing"
)

func TestOpenCodeTargetDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	h := OpenCode()
	if got, want := h.TargetDir(), filepath.Join(home, ".config", "opencode"); got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}
}

func TestOpenCodeTargetDirUsesConfigEnv(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(home, "custom-opencode")
	t.Setenv("HOME", home)
	t.Setenv("OPENCODE_CONFIG_DIR", custom)

	if got := OpenCode().TargetDir(); got != custom {
		t.Fatalf("target with OPENCODE_CONFIG_DIR = %q, want %q", got, custom)
	}
}
