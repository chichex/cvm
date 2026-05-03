package state

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/chichex/cvm/internal/config"
)

func TestRemoteIndexingByScopeAndProject(t *testing.T) {
	projectA := t.TempDir()
	projectB := t.TempDir()

	st := &State{
		Local:   make(map[string]LocalState),
		Remotes: make(map[string]Remote),
	}

	st.PutRemote(Remote{
		Repo:    "global/repo",
		Scope:   string(config.ScopeGlobal),
		Profile: "work",
	})
	st.PutRemote(Remote{
		Repo:        "local/repo-a",
		Scope:       string(config.ScopeLocal),
		Profile:     "work",
		ProjectPath: projectA,
	})
	st.PutRemote(Remote{
		Repo:        "local/repo-b",
		Scope:       string(config.ScopeLocal),
		Profile:     "work",
		ProjectPath: projectB,
	})

	globalRemote, ok := st.FindRemote(config.ScopeGlobal, "work", "")
	if !ok || globalRemote.Repo != "global/repo" {
		t.Fatalf("expected global remote, got %+v (ok=%v)", globalRemote, ok)
	}

	localRemoteA, ok := st.FindRemote(config.ScopeLocal, "work", projectA)
	if !ok || localRemoteA.Repo != "local/repo-a" {
		t.Fatalf("expected local remote A, got %+v (ok=%v)", localRemoteA, ok)
	}

	localRemoteB, ok := st.FindRemote(config.ScopeLocal, "work", projectB)
	if !ok || localRemoteB.Repo != "local/repo-b" {
		t.Fatalf("expected local remote B, got %+v (ok=%v)", localRemoteB, ok)
	}

	if matches := st.FindRemotesByProfile("work"); len(matches) != 3 {
		t.Fatalf("expected 3 remotes for profile name, got %d", len(matches))
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
				"scope":   "global",
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

	remote, ok := st.FindRemote(config.ScopeGlobal, "work", "")
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
	if got := st.GetLocalHarness("/tmp/project", "claude"); got != "dev" {
		t.Fatalf("expected legacy local active to map to claude, got %q", got)
	}
	if got := st.GetGlobalHarness("opencode"); got != "" {
		t.Fatalf("expected unrelated harness to stay vanilla, got %q", got)
	}
}

func TestHarnessStateDoesNotOverwriteOtherHarnesses(t *testing.T) {
	st := &State{
		Local:   make(map[string]LocalState),
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

	st.SetLocalHarness("/tmp/project", "claude", "dev")
	st.SetLocalHarness("/tmp/project", "opencode", "open-dev")
	st.ClearLocalHarness("/tmp/project", "claude")

	if got := st.GetLocalHarness("/tmp/project", "claude"); got != "" {
		t.Fatalf("expected local claude to be vanilla, got %q", got)
	}
	if got := st.GetLocalHarness("/tmp/project", "opencode"); got != "open-dev" {
		t.Fatalf("expected local opencode to remain active, got %q", got)
	}
}
