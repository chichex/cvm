package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chichex/cvm/internal/harness"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
)

func TestPullByProfileUpdatesAndReappliesActiveProfile(t *testing.T) {
	home := t.TempDir()

	t.Setenv("HOME", home)
	t.Setenv("PATH", withFakeGit(t))

	writeFile(t, filepath.Join(harness.Claude().TargetDir(), "CLAUDE.md"), "old active")
	writeFile(t, filepath.Join(profile.ProfileDir("work"), "CLAUDE.md"), "old profile")
	writeFile(t, filepath.Join(CacheDirFor("example/global"), "profiles", "work", "CLAUDE.md"), "new remote")

	st := &state.State{
		Remotes: make(map[string]state.Remote),
	}
	st.SetGlobal("work")
	st.PutRemote(state.Remote{
		Repo:    "example/global",
		Path:    filepath.Join("profiles", "work"),
		Branch:  "main",
		Profile: "work",
	})
	if err := st.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	updated, err := Pull("work")
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(updated) != 1 || updated[0] != "work" {
		t.Fatalf("unexpected updated profiles: %v", updated)
	}

	assertFileContent(t, filepath.Join(harness.Claude().TargetDir(), "CLAUDE.md"), "new remote")
}

func TestPullByProfileUpdatesAndReappliesActiveOpenCodeProfile(t *testing.T) {
	home := t.TempDir()

	t.Setenv("HOME", home)
	t.Setenv("PATH", withFakeGit(t))

	manifest := "name = \"open\"\nharnesses = [\"opencode\"]\n\n[assets]\nopencode = \"opencode\"\n"
	writeFile(t, filepath.Join(profile.ProfileDir("open"), "cvm.profile.toml"), manifest)
	writeFile(t, filepath.Join(profile.ProfileDir("open"), "opencode", "AGENTS.md"), "old profile")
	writeFile(t, filepath.Join(CacheDirFor("example/global"), "profiles", "open", "cvm.profile.toml"), manifest)
	writeFile(t, filepath.Join(CacheDirFor("example/global"), "profiles", "open", "opencode", "AGENTS.md"), "new remote")
	writeFile(t, filepath.Join(CacheDirFor("example/global"), "profiles", "open", "opencode", "skills", "portable-plan", "SKILL.md"), "plan skill")

	st := &state.State{
		Remotes: make(map[string]state.Remote),
	}
	st.SetGlobalHarness("opencode", "open")
	st.PutRemote(state.Remote{
		Repo:    "example/global",
		Path:    filepath.Join("profiles", "open"),
		Branch:  "main",
		Profile: "open",
	})
	if err := st.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	updated, err := Pull("open")
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(updated) != 1 || updated[0] != "open" {
		t.Fatalf("unexpected updated profiles: %v", updated)
	}

	assertFileContent(t, filepath.Join(harness.OpenCode().TargetDir(), "AGENTS.md"), "new remote")
	assertFileContent(t, filepath.Join(harness.OpenCode().TargetDir(), "skills", "portable-plan", "SKILL.md"), "plan skill")
}

func TestLooksLikeProfileWithManifestBackedClaudeAssets(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "cvm.profile.toml"), "name = \"work\"\nharnesses = [\"claude\"]\n\n[assets]\nclaude = \"claude\"\n")
	writeFile(t, filepath.Join(root, "claude", "CLAUDE.md"), "hello")

	if !looksLikeProfile(root) {
		t.Fatal("expected manifest-backed profile layout to be detected")
	}
}

func TestLooksLikeProfileIgnoresProjectMCPOnlyClaudeProfile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{"mcpServers":{"local":{}}}`)

	if looksLikeProfile(root) {
		t.Fatal("expected .mcp.json-only profile to be ignored")
	}
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
