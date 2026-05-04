package state

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/chichex/cvm/internal/config"
)

func TestRemoteIndexingByScopeAndProject(t *testing.T) {
	st := &State{
		Remotes: make(map[string]Remote),
	}

	st.PutRemote(Remote{
		Repo:    "global/repo",
		Profile: "work",
	})
	st.PutRemote(Remote{
		Repo:    "new/repo",
		Profile: "work",
	})

	remote, ok := st.FindRemote("work")
	if !ok || remote.Repo != "new/repo" {
		t.Fatalf("expected latest remote, got %+v (ok=%v)", remote, ok)
	}

	if matches := st.FindRemotesByProfile("work"); len(matches) != 1 {
		t.Fatalf("expected 1 remote for profile name, got %d", len(matches))
	}
}

func TestLoadMigratesLegacyRemoteKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	raw := map[string]any{
		"global": map[string]any{"active": ""},
		"local":  map[string]any{},
		"remotes": map[string]any{
			"work": map[string]any{
				"repo":    "legacy/repo",
				"path":    "profiles/work",
				"branch":  "main",
				"profile": "work",
			},
		},
	}

	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}
	if err := os.MkdirAll(config.CvmHome(), 0755); err != nil {
		t.Fatalf("mkdir cvm home: %v", err)
	}
	if err := os.WriteFile(config.StatePath(), data, 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	st, err := Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	remote, ok := st.FindRemote("work")
	if !ok {
		t.Fatal("expected migrated remote to be queryable")
	}
	if remote.Repo != "legacy/repo" {
		t.Fatalf("unexpected migrated repo: %s", remote.Repo)
	}
}

func TestLoadInterpretsLegacyActiveAsClaudeHarness(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	raw := map[string]any{
		"global": map[string]any{"active": "work"},
		"local": map[string]any{
			"/tmp/project": map[string]any{"active": "dev"},
		},
	}

	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}
	if err := os.MkdirAll(config.CvmHome(), 0755); err != nil {
		t.Fatalf("mkdir cvm home: %v", err)
	}
	if err := os.WriteFile(config.StatePath(), data, 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	st, err := Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if got := st.GetGlobalHarness("claude"); got != "work" {
		t.Fatalf("expected legacy global active to map to claude, got %q", got)
	}
	if got := st.GetGlobalHarness("opencode"); got != "" {
		t.Fatalf("expected unrelated harness to stay vanilla, got %q", got)
	}
}

func TestHarnessStateDoesNotOverwriteOtherHarnesses(t *testing.T) {
	st := &State{
		Remotes: make(map[string]Remote),
	}

	st.SetGlobalHarness("claude", "work")
	st.SetGlobalHarness("opencode", "open")
	st.ClearGlobalHarness("claude")

	if got := st.GetGlobalHarness("claude"); got != "" {
		t.Fatalf("expected claude to be vanilla, got %q", got)
	}
	if got := st.GetGlobalHarness("opencode"); got != "open" {
		t.Fatalf("expected opencode to remain active, got %q", got)
	}

}
