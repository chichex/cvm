package harness

import (
	"path/filepath"
	"testing"

	"github.com/chichex/cvm/internal/config"
)

func TestOpenCodeTargetDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	h := OpenCode()
	if got, want := h.TargetDir(config.ScopeGlobal, ""), filepath.Join(home, ".config", "opencode"); got != want {
		t.Fatalf("global target = %q, want %q", got, want)
	}

	project := filepath.Join(home, "project")
	if got, want := h.TargetDir(config.ScopeLocal, project), filepath.Join(project, ".opencode"); got != want {
		t.Fatalf("local target = %q, want %q", got, want)
	}
}

func TestOpenCodeTargetDirUsesConfigEnv(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(home, "custom-opencode")
	t.Setenv("HOME", home)
	t.Setenv("OPENCODE_CONFIG_DIR", custom)

	if got := OpenCode().TargetDir(config.ScopeGlobal, ""); got != custom {
		t.Fatalf("global target with OPENCODE_CONFIG_DIR = %q, want %q", got, custom)
	}
}
