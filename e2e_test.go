package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cvmprofile "github.com/chichex/cvm/internal/profile"
)

var cvmBin string

func TestMain(m *testing.M) {
	// Build the cvm binary into a temp location
	tmp, err := os.MkdirTemp("", "cvm-build-*")
	if err != nil {
		panic("cannot create temp dir for build: " + err.Error())
	}
	cvmBin = filepath.Join(tmp, "cvm")

	cmd := exec.Command("go", "build", "-o", cvmBin, ".")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmp)
		panic("failed to build cvm: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// testEnv creates an isolated HOME with a fake project dir and returns
// a helper to run cvm commands in that environment.
type testEnv struct {
	t          *testing.T
	home       string
	projectDir string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	home := t.TempDir() // cleaned up automatically by testing
	projectDir := filepath.Join(home, "myproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("creating project dir: %v", err)
	}
	return &testEnv{t: t, home: home, projectDir: projectDir}
}

// run executes cvm with the given args. The working directory is set to
// the fake project dir so that local commands pick it up.
func (e *testEnv) run(args ...string) (string, error) {
	e.t.Helper()
	cmd := exec.Command(cvmBin, args...)
	cmd.Dir = e.projectDir
	cmd.Env = append(os.Environ(),
		"HOME="+e.home,
		"CODEX_HOME=",
		"OPENCODE_CONFIG_DIR=",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// mustRun is like run but fails the test on error.
func (e *testEnv) mustRun(args ...string) string {
	e.t.Helper()
	out, err := e.run(args...)
	if err != nil {
		e.t.Fatalf("cvm %s failed: %v\noutput: %s", strings.Join(args, " "), err, out)
	}
	return out
}

// mustFail is like run but fails the test if the command succeeds.
func (e *testEnv) mustFail(args ...string) string {
	e.t.Helper()
	out, err := e.run(args...)
	if err == nil {
		e.t.Fatalf("expected cvm %s to fail, but it succeeded\noutput: %s", strings.Join(args, " "), out)
	}
	return out
}

// profilesDir returns ~/.cvm/profiles for this env.
func (e *testEnv) profilesDir() string {
	return filepath.Join(e.home, ".cvm", "profiles")
}

// seedGlobalClaude creates a minimal ~/.claude/ with a CLAUDE.md so the
// vanilla-stash machinery has a real file to move aside.
func (e *testEnv) seedGlobalClaude(content string) {
	e.t.Helper()
	dir := filepath.Join(e.home, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.t.Fatalf("creating .claude dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0644); err != nil {
		e.t.Fatalf("writing CLAUDE.md: %v", err)
	}
}

// writeProfile lays out a profile source repo under ~/.cvm/profiles/<name>
// with a manifest and the given asset files (paths relative to the profile root).
func (e *testEnv) writeProfile(name, manifest string, assets map[string]string) string {
	e.t.Helper()
	root := filepath.Join(e.profilesDir(), name)
	writeTestFile(e.t, filepath.Join(root, "cvm.profile.toml"), manifest)
	for rel, body := range assets {
		writeTestFile(e.t, filepath.Join(root, rel), body)
	}
	return root
}

// ---------------------------------------------------------------------------
// Global symlink workflow
// ---------------------------------------------------------------------------

func TestGlobalWorkflow(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# global vanilla")

	// add
	out := e.mustRun("add", "work")
	assertContains(t, out, "Created profile")

	// ls
	out = e.mustRun("ls")
	assertContains(t, out, "work")

	// give the profile something to symlink, then use
	writeTestFile(t, filepath.Join(e.profilesDir(), "work", "CLAUDE.md"), "# work profile")
	out = e.mustRun("use", "work")
	assertContains(t, out, "Switched claude harness")

	// ls shows active marker
	out = e.mustRun("ls")
	assertContains(t, out, "IN USE")

	// off
	out = e.mustRun("off")
	assertContains(t, out, "vanilla")

	// rm (now that it's not active)
	out = e.mustRun("rm", "work")
	assertContains(t, out, "Removed profile")

	// ls should be empty
	out = e.mustRun("ls")
	assertContains(t, out, "No profiles")
}

func TestUseHarnessPersistsActiveByHarness(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.mustRun("add", "work")
	writeTestFile(t, filepath.Join(e.profilesDir(), "work", "CLAUDE.md"), "# work")
	out := e.mustRun("use", "work", "--harness", "claude")
	assertContains(t, out, "Switched claude harness")

	statePath := filepath.Join(e.home, ".cvm", "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	var raw struct {
		Global struct {
			Active    string            `json:"active"`
			Harnesses map[string]string `json:"harnesses"`
		} `json:"global"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if raw.Global.Harnesses["claude"] != "work" {
		t.Fatalf("expected claude harness active profile to be work, got state: %s", string(data))
	}
	if raw.Global.Active != "work" {
		t.Fatalf("expected legacy active mirror to be work, got state: %s", string(data))
	}
}

// ---------------------------------------------------------------------------
// Symlink-apply: use symlinks the profile's managed items into the target dir
// ---------------------------------------------------------------------------

func TestUseSymlinksManifestBackedClaudeProfile(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	root := e.writeProfile("manifested",
		"name = \"manifested\"\nharnesses = [\"claude\"]\n\n[assets]\nclaude = \"claude\"\n",
		map[string]string{"claude/CLAUDE.md": "# manifest profile"})

	e.mustRun("use", "manifested")

	liveClaude := filepath.Join(e.home, ".claude", "CLAUDE.md")
	// The live path must be a symlink pointing into the profile source repo.
	info, err := os.Lstat(liveClaude)
	if err != nil {
		t.Fatalf("lstat live CLAUDE.md: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink", liveClaude)
	}
	resolved, err := filepath.EvalSymlinks(liveClaude)
	if err != nil {
		t.Fatalf("eval symlink: %v", err)
	}
	wantSrc, _ := filepath.EvalSymlinks(filepath.Join(root, "claude", "CLAUDE.md"))
	if resolved != wantSrc {
		t.Fatalf("symlink resolves to %q, want %q", resolved, wantSrc)
	}
	assertFileContent(t, liveClaude, "# manifest profile")

	// Editing through the symlink writes straight into the source repo.
	if err := os.WriteFile(liveClaude, []byte("# edited live"), 0644); err != nil {
		t.Fatalf("write through symlink: %v", err)
	}
	assertFileContent(t, filepath.Join(root, "claude", "CLAUDE.md"), "# edited live")
}

func TestUseAppliesAllManifestHarnessesByDefault(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# claude vanilla")
	opencodeDir := filepath.Join(e.home, ".config", "opencode")
	writeTestFile(t, filepath.Join(opencodeDir, "AGENTS.md"), "# opencode vanilla")

	e.writeProfile("portable",
		"name = \"portable\"\nharnesses = [\"claude\", \"opencode\"]\n\n[assets]\nclaude = \"claude\"\nopencode = \"opencode\"\n",
		map[string]string{
			"claude/CLAUDE.md":                          "# claude portable",
			"opencode/AGENTS.md":                        "# opencode portable",
			"opencode/skills/portable-spec/SKILL.md":    "---\ndescription: portable spec\n---\n",
		})

	out := e.mustRun("use", "portable")
	assertContains(t, out, "Switched claude harness")
	assertContains(t, out, "Switched opencode harness")
	assertFileContent(t, filepath.Join(e.home, ".claude", "CLAUDE.md"), "# claude portable")
	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# opencode portable")
	assertFileContent(t, filepath.Join(opencodeDir, "skills", "portable-spec", "SKILL.md"), "---\ndescription: portable spec\n---")

	statePath := filepath.Join(e.home, ".cvm", "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var raw struct {
		Global struct {
			Harnesses map[string]string `json:"harnesses"`
		} `json:"global"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if raw.Global.Harnesses["claude"] != "portable" || raw.Global.Harnesses["opencode"] != "portable" {
		t.Fatalf("expected both harnesses active, got state: %s", string(data))
	}
}

func TestUseHarnessFlagLimitsManifestProfileToOneHarness(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# claude vanilla")
	opencodeDir := filepath.Join(e.home, ".config", "opencode")
	writeTestFile(t, filepath.Join(opencodeDir, "AGENTS.md"), "# opencode vanilla")

	e.writeProfile("portable",
		"name = \"portable\"\nharnesses = [\"claude\", \"opencode\"]\n\n[assets]\nclaude = \"claude\"\nopencode = \"opencode\"\n",
		map[string]string{
			"claude/CLAUDE.md":   "# claude portable",
			"opencode/AGENTS.md": "# opencode portable",
		})

	out := e.mustRun("use", "portable", "--harness", "opencode")
	assertContains(t, out, "Switched opencode harness")
	assertNotContains(t, out, "Switched claude harness")
	// Claude was never touched: its real vanilla file is still in place.
	assertFileContent(t, filepath.Join(e.home, ".claude", "CLAUDE.md"), "# claude vanilla")
	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# opencode portable")
}

func TestOpenCodeHarnessGlobalWorkflow(t *testing.T) {
	e := newTestEnv(t)
	opencodeDir := filepath.Join(e.home, ".config", "opencode")
	writeTestFile(t, filepath.Join(opencodeDir, "AGENTS.md"), "# vanilla opencode")

	e.writeProfile("open",
		"name = \"open\"\nharnesses = [\"opencode\"]\n\n[assets]\nopencode = \"opencode\"\n",
		map[string]string{
			"opencode/AGENTS.md":              "# opencode profile",
			"opencode/skills/deploy/SKILL.md": "---\nname: deploy\ndescription: Deploy app\n---\n",
			"opencode/opencode.json":          `{"mcpServers":{"context7":{"type":"local"}}}`,
		})

	out := e.mustRun("use", "open", "--harness", "opencode")
	assertContains(t, out, "Switched opencode harness")

	// The profile fully owns these items — they are symlinked in verbatim.
	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# opencode profile")
	if _, err := os.Stat(filepath.Join(opencodeDir, "skills", "deploy", "SKILL.md")); err != nil {
		t.Fatalf("expected opencode skill to be installed: %v", err)
	}
	// opencode.json is owned by the profile, not merged: it contains exactly the
	// profile's content (no user-side keys are blended in).
	assertMCPServerExists(t, filepath.Join(opencodeDir, "opencode.json"), "context7")

	// opencode use must not touch Claude paths.
	if _, err := os.Stat(filepath.Join(e.home, ".claude", "AGENTS.md")); err == nil {
		t.Fatal("opencode use should not install into Claude paths")
	}
}

func TestUseCodexProfile(t *testing.T) {
	e := newTestEnv(t)
	codexDir := filepath.Join(e.home, ".codex")

	e.writeProfile("codexer",
		"name = \"codexer\"\nharnesses = [\"codex\"]\n\n[assets]\ncodex = \"codex\"\n",
		map[string]string{"codex/AGENTS.md": "# codex instructions"})

	out := e.mustRun("use", "codexer", "--harness", "codex")
	assertContains(t, out, "Switched codex harness")
	assertFileContent(t, filepath.Join(codexDir, "AGENTS.md"), "# codex instructions")
}

func TestLiteProfileActivatesForClaude(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")
	profileRoot := filepath.Join(e.profilesDir(), "lite")
	if err := cvmprofile.CopyDir(filepath.Join("profiles", "lite"), profileRoot); err != nil {
		t.Fatalf("copy lite profile: %v", err)
	}

	out := e.mustRun("use", "lite", "--harness", "claude")
	assertContains(t, out, "Switched claude harness")
	assertFileContains(t, filepath.Join(e.home, ".claude", "CLAUDE.md"), "# Lite Profile")
	assertFileContains(t, filepath.Join(profileRoot, "cvm.profile.toml"), "harnesses = [\"claude\"]")
}

// ---------------------------------------------------------------------------
// Vanilla stash round-trip via the CLI: off restores the pre-cvm real file
// ---------------------------------------------------------------------------

func TestOffRestoresVanillaStash(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# original vanilla content")

	e.writeProfile("temp",
		"name = \"temp\"\nharnesses = [\"claude\"]\n\n[assets]\nclaude = \".\"\n",
		map[string]string{"CLAUDE.md": "# profile content"})

	e.mustRun("use", "temp")
	// Live file is now a symlink serving profile content.
	assertFileContent(t, filepath.Join(e.home, ".claude", "CLAUDE.md"), "# profile content")

	out := e.mustRun("off")
	assertContains(t, out, "vanilla")

	claudeMD := filepath.Join(e.home, ".claude", "CLAUDE.md")
	info, err := os.Lstat(claudeMD)
	if err != nil {
		t.Fatalf("CLAUDE.md should exist after off: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected restored real file, got symlink")
	}
	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("read restored CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(data), "original vanilla content") {
		t.Fatalf("expected vanilla content, got: %s", string(data))
	}
}

func TestOpenCodeHarnessRestoreGlobalVanilla(t *testing.T) {
	e := newTestEnv(t)
	opencodeDir := filepath.Join(e.home, ".config", "opencode")
	writeTestFile(t, filepath.Join(opencodeDir, "AGENTS.md"), "# vanilla opencode")

	e.writeProfile("open",
		"name = \"open\"\nharnesses = [\"opencode\"]\n\n[assets]\nopencode = \"opencode\"\n",
		map[string]string{"opencode/AGENTS.md": "# profile opencode"})

	e.mustRun("use", "open", "--harness", "opencode")
	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# profile opencode")

	out := e.mustRun("off", "--harness", "opencode")
	assertContains(t, out, "Switched opencode harness to vanilla")
	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# vanilla opencode")
}

// ---------------------------------------------------------------------------
// Profile switch: switching profiles re-points the symlinks
// ---------------------------------------------------------------------------

func TestProfileSwitchRepointsSymlinks(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.writeProfile("alpha",
		"name = \"alpha\"\nharnesses = [\"claude\"]\n\n[assets]\nclaude = \".\"\n",
		map[string]string{"CLAUDE.md": "# alpha"})
	e.writeProfile("beta",
		"name = \"beta\"\nharnesses = [\"claude\"]\n\n[assets]\nclaude = \".\"\n",
		map[string]string{"CLAUDE.md": "# beta"})

	e.mustRun("use", "alpha")
	assertFileContent(t, filepath.Join(e.home, ".claude", "CLAUDE.md"), "# alpha")

	e.mustRun("use", "beta")
	assertFileContent(t, filepath.Join(e.home, ".claude", "CLAUDE.md"), "# beta")

	// Switching back to vanilla restores the originally-stashed real file.
	e.mustRun("off")
	assertFileContent(t, filepath.Join(e.home, ".claude", "CLAUDE.md"), "# vanilla")
}

func TestLsShowsInUseProfiles(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.mustRun("add", "work")
	writeTestFile(t, filepath.Join(e.profilesDir(), "work", "CLAUDE.md"), "# work")
	e.mustRun("use", "work")

	out := e.mustRun("ls")
	assertContains(t, out, "work")
	assertContains(t, out, "IN USE")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestEdgeDuplicateAdd(t *testing.T) {
	e := newTestEnv(t)

	e.mustRun("add", "dup")
	out := e.mustFail("add", "dup")
	assertContains(t, out, "already exists")
}

func TestEdgeRmActiveProfile(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.mustRun("add", "active")
	writeTestFile(t, filepath.Join(e.profilesDir(), "active", "CLAUDE.md"), "# active")
	e.mustRun("use", "active")

	out := e.mustFail("rm", "active")
	assertContains(t, out, "cannot remove active profile")
}

func TestEdgeUseNonexistent(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("use", "ghost")
	assertContains(t, out, "not found")
}

func TestEdgeFromFlag(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# base content")

	e.mustRun("add", "base")
	e.mustRun("add", "derived", "--from", "base")

	// derived should exist and be listable
	out := e.mustRun("ls")
	assertContains(t, out, "base")
	assertContains(t, out, "derived")
}

func TestEdgeFromNonexistent(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("add", "bad", "--from", "nope")
	assertContains(t, out, "not found")
}

func TestEdgeUseNoArgs(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("use")
	assertContains(t, out, "provide a profile name")
}

// ---------------------------------------------------------------------------
// Version
// ---------------------------------------------------------------------------

func TestVersion(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustRun("--version")
	assertContains(t, out, "cvm version")
}

// ---------------------------------------------------------------------------
// Core CLI surface: help output matches the post-refactor commands
// ---------------------------------------------------------------------------

func TestCoreCommandSurface(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustRun("--help")

	// Core commands must be present.
	for _, want := range []string{"add", "use", "off", "ls", "rm", "pull", "push"} {
		assertContains(t, out, want)
	}

	// Removed commands must not appear as top-level commands. Use Cobra's
	// two-space-prefixed row format ("  <cmd> ") to avoid false positives
	// against description text.
	for _, notWant := range []string{
		"  current ", "  save ", "  status ", "  nuke ", "  remote ",
		"  completion ", "  override ", "  edit ", "  profile ",
		"  restore ", "  bypass ", "  local ",
	} {
		assertNotContains(t, out, notWant)
	}
}

// ---------------------------------------------------------------------------
// Multiple profiles coexist
// ---------------------------------------------------------------------------

func TestMultipleProfilesCoexist(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	for _, name := range []string{"alpha", "beta", "gamma"} {
		e.mustRun("add", name)
		writeTestFile(t, filepath.Join(e.profilesDir(), name, "CLAUDE.md"), "# "+name)
	}

	out := e.mustRun("ls")
	assertContains(t, out, "alpha")
	assertContains(t, out, "beta")
	assertContains(t, out, "gamma")

	// Switch between them
	e.mustRun("use", "alpha")
	out = e.mustRun("ls")
	assertContains(t, out, "alpha")

	e.mustRun("use", "beta")
	out = e.mustRun("ls")
	assertContains(t, out, "beta")

	// Remove non-active profile
	e.mustRun("rm", "gamma")
	out = e.mustRun("ls")
	assertNotContains(t, out, "gamma")
	assertContains(t, out, "alpha")
	assertContains(t, out, "beta")
}

// ---------------------------------------------------------------------------
// off with no active profile is a clean no-op
// ---------------------------------------------------------------------------

func TestOffNoActiveProfile(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustRun("off")
	assertContains(t, out, "Already vanilla")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output NOT to contain %q, got:\n%s", needle, haystack)
	}
}

func assertMCPServerExists(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings %s: %v", path, err)
	}

	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal settings %s: %v", path, err)
	}
	if _, ok := cfg.MCPServers[want]; !ok {
		t.Fatalf("settings %s missing mcp server %q", path, want)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.TrimSpace(string(data)) != want {
		t.Fatalf("%s content = %q, want %q", path, strings.TrimSpace(string(data)), want)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s content should contain %q, got %q", path, want, string(data))
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
