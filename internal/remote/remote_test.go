package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
)

func TestPullReappliesActiveLocalProfile(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "project")

	t.Setenv("HOME", home)
	t.Setenv("PATH", withFakeGit(t))

	writeFile(t, filepath.Join(project, ".claude", "CLAUDE.md"), "old local active")
	writeFile(t, filepath.Join(profile.ProfileDir(config.ScopeLocal, "work"), "CLAUDE.md"), "old saved profile")
	writeFile(t, filepath.Join(CacheDirFor("example/local"), "profiles", "work", "CLAUDE.md"), "new remote local")

	st := &state.State{
		Local:   make(map[string]state.LocalState),
		Remotes: make(map[string]state.Remote),
	}
	st.SetLocal(project, "work")
	st.PutRemote(state.Remote{
		Repo:        "example/local",
		Path:        filepath.Join("profiles", "work"),
		Branch:      "main",
		Scope:       string(config.ScopeLocal),
		Profile:     "work",
		ProjectPath: project,
	})
	if err := st.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	updated, err := Pull("work")
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(updated) != 1 || updated[0] != "work (local)" {
		t.Fatalf("unexpected updated profiles: %v", updated)
	}

	assertFileContent(t, filepath.Join(profile.ProfileDir(config.ScopeLocal, "work"), "CLAUDE.md"), "new remote local")
	assertFileContent(t, filepath.Join(project, ".claude", "CLAUDE.md"), "new remote local")
}

func TestPullByProfileUpdatesGlobalAndLocalMatches(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "project")

	t.Setenv("HOME", home)
	t.Setenv("PATH", withFakeGit(t))

	writeFile(t, filepath.Join(config.ClaudeHome(), "CLAUDE.md"), "old global active")
	writeFile(t, filepath.Join(project, ".claude", "CLAUDE.md"), "old local active")
	writeFile(t, filepath.Join(profile.ProfileDir(config.ScopeGlobal, "work"), "CLAUDE.md"), "old global profile")
	writeFile(t, filepath.Join(profile.ProfileDir(config.ScopeLocal, "work"), "CLAUDE.md"), "old local profile")
	writeFile(t, filepath.Join(CacheDirFor("example/global"), "profiles", "work", "CLAUDE.md"), "new remote global")
	writeFile(t, filepath.Join(CacheDirFor("example/local"), "profiles", "work", "CLAUDE.md"), "new remote local")

	st := &state.State{
		Local:   make(map[string]state.LocalState),
		Remotes: make(map[string]state.Remote),
	}
	st.SetGlobal("work")
	st.SetLocal(project, "work")
	st.PutRemote(state.Remote{
		Repo:    "example/global",
		Path:    filepath.Join("profiles", "work"),
		Branch:  "main",
		Scope:   string(config.ScopeGlobal),
		Profile: "work",
	})
	st.PutRemote(state.Remote{
		Repo:        "example/local",
		Path:        filepath.Join("profiles", "work"),
		Branch:      "main",
		Scope:       string(config.ScopeLocal),
		Profile:     "work",
		ProjectPath: project,
	})
	if err := st.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	updated, err := Pull("work")
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("expected both remotes to update, got %v", updated)
	}

	assertFileContent(t, filepath.Join(config.ClaudeHome(), "CLAUDE.md"), "new remote global")
	assertFileContent(t, filepath.Join(project, ".claude", "CLAUDE.md"), "new remote local")
}

func withFakeGit(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	script := filepath.Join(dir, "git")
	content := "#!/bin/sh\nif [ \"$1\" = \"-C\" ] && [ \"$3\" = \"pull\" ] && [ \"$4\" = \"--ff-only\" ]; then\n  exit 0\nfi\necho \"unexpected git command: $@\" >&2\nexit 1\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}

	return dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.TrimSpace(string(data)) != want {
		t.Fatalf("unexpected content in %s: got %q want %q", path, strings.TrimSpace(string(data)), want)
	}
}
