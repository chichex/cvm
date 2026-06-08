package remote

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
)

func TestAddPathRegistersExistingRepoAsSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// A user's own repo dir, anywhere on disk (not under ~/.cvm/profiles).
	repo := filepath.Join(t.TempDir(), "myprofile")
	writeFile(t, filepath.Join(repo, "cvm.profile.toml"), claudeManifest)
	writeFile(t, filepath.Join(repo, "claude", "CLAUDE.md"), "hello")

	if err := AddPath("mine", repo); err != nil {
		t.Fatalf("AddPath: %v", err)
	}

	st, err := state.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	got, ok := st.GetSource("mine")
	if !ok {
		t.Fatal("expected source to be registered")
	}
	if got != repo {
		t.Fatalf("source dir = %q, want %q", got, repo)
	}
	// No clone happened: the default cloned location must stay empty.
	if _, err := os.Stat(profile.ProfileDir("mine")); !os.IsNotExist(err) {
		t.Fatalf("expected no clone at %s", profile.ProfileDir("mine"))
	}
}

func TestAddPathRejectsNonProfileDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir() // empty, not a profile
	if err := AddPath("nope", dir); err == nil {
		t.Fatal("expected AddPath to reject a non-profile dir")
	}
}

func TestSourceRepoDirResolvesRegisteredSourceToGitRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := filepath.Join(t.TempDir(), "repo")
	// Profile lives in a subdir of the git repo.
	sub := filepath.Join(root, "profiles", "work")
	writeFile(t, filepath.Join(sub, "cvm.profile.toml"), claudeManifest)
	writeFile(t, filepath.Join(sub, "claude", "CLAUDE.md"), "hi")
	gitInit(t, root)

	st, _ := state.Load()
	st.SetSource("work", sub)
	if err := st.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := SourceRepoDir("work")
	if err != nil {
		t.Fatalf("SourceRepoDir: %v", err)
	}
	if got != root {
		t.Fatalf("repo root = %q, want %q", got, root)
	}
}

func TestPullRefusesDirtyWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := filepath.Join(t.TempDir(), "dirty")
	writeFile(t, filepath.Join(repo, "cvm.profile.toml"), claudeManifest)
	writeFile(t, filepath.Join(repo, "claude", "CLAUDE.md"), "v1")
	gitInit(t, repo)
	// Introduce an uncommitted change.
	writeFile(t, filepath.Join(repo, "claude", "CLAUDE.md"), "v2 uncommitted")

	st, _ := state.Load()
	st.SetSource("work", repo)
	if err := st.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err := Pull("work")
	if err == nil {
		t.Fatal("expected Pull to refuse a dirty worktree")
	}
	if !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPullFastForwardsCleanRepo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// upstream <- clone. Pull --ff-only should advance the clone.
	upstream := filepath.Join(t.TempDir(), "upstream")
	writeFile(t, filepath.Join(upstream, "cvm.profile.toml"), claudeManifest)
	writeFile(t, filepath.Join(upstream, "claude", "CLAUDE.md"), "v1")
	gitInit(t, upstream)

	clone := profile.ProfileDir("work")
	if err := os.MkdirAll(filepath.Dir(clone), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGit(t, "", "clone", upstream, clone)

	// Advance upstream by one commit.
	writeFile(t, filepath.Join(upstream, "claude", "CLAUDE.md"), "v2")
	runGit(t, upstream, "add", "-A")
	runGit(t, upstream, "commit", "-m", "v2")

	st, _ := state.Load()
	st.SetSource("work", clone)
	if err := st.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	updated, err := Pull("work")
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if len(updated) != 1 || updated[0] != "work" {
		t.Fatalf("unexpected updated: %v", updated)
	}
	assertFileContent(t, filepath.Join(clone, "claude", "CLAUDE.md"), "v2")
}

func TestLooksLikeProfileWithManifestBackedClaudeAssets(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "cvm.profile.toml"), claudeManifest)
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

const claudeManifest = "name = \"work\"\nharnesses = [\"claude\"]\n\n[assets]\nclaude = \"claude\"\n"

func gitInit(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
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
