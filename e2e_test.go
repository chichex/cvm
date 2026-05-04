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

// runWithEnv executes cvm with extra environment variables merged into the environment.
func (e *testEnv) runWithEnv(extra map[string]string, args ...string) string {
	e.t.Helper()
	cmd := exec.Command(cvmBin, args...)
	cmd.Dir = e.projectDir
	env := append(os.Environ(), "HOME="+e.home, "CODEX_HOME=", "OPENCODE_CONFIG_DIR=")
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("cvm %s failed: %v\noutput: %s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

// seedClaudeDir creates a minimal ~/.claude/ with a CLAUDE.md so profiles
// have something to snapshot.
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

// seedLocalClaude creates a .claude/ inside the project dir.
func (e *testEnv) seedLocalClaude(content string) {
	e.t.Helper()
	dir := filepath.Join(e.projectDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.t.Fatalf("creating local .claude dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0644); err != nil {
		e.t.Fatalf("writing local CLAUDE.md: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Global profile workflow
// ---------------------------------------------------------------------------

func TestGlobalWorkflow(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# global vanilla")

	// init
	out := e.mustRun("global", "init", "work")
	assertContains(t, out, "Created global profile")

	// ls
	out = e.mustRun("global", "ls")
	assertContains(t, out, "work")

	// current before use → vanilla
	out = e.mustRun("global", "current")
	assertContains(t, out, "(vanilla)")

	// use
	out = e.mustRun("global", "use", "work")
	assertContains(t, out, "Switched to global profile")

	// current after use
	out = e.mustRun("global", "current")
	assertContains(t, out, "work")

	// ls shows active marker
	out = e.mustRun("global", "ls")
	assertContains(t, out, "* ")

	// save
	out = e.mustRun("global", "save")
	assertContains(t, out, "Saved current state")

	// use --none
	out = e.mustRun("global", "use", "--none")
	assertContains(t, out, "vanilla")

	out = e.mustRun("global", "current")
	assertContains(t, out, "(vanilla)")

	// rm (now that it's not active)
	out = e.mustRun("global", "rm", "work")
	assertContains(t, out, "Removed global profile")

	// ls should be empty
	out = e.mustRun("global", "ls")
	assertContains(t, out, "No global profiles")
}

// ---------------------------------------------------------------------------
// Local profile workflow
// ---------------------------------------------------------------------------

func TestLocalWorkflow(t *testing.T) {
	e := newTestEnv(t)
	e.seedLocalClaude("# local vanilla")

	// init with explicit name
	out := e.mustRun("local", "init", "dev")
	assertContains(t, out, "Created local profile")

	// ls
	out = e.mustRun("local", "ls")
	assertContains(t, out, "dev")

	// current before use → vanilla
	out = e.mustRun("local", "current")
	assertContains(t, out, "(vanilla)")

	// use
	out = e.mustRun("local", "use", "dev")
	assertContains(t, out, "Switched to local profile")

	// current after use
	out = e.mustRun("local", "current")
	assertContains(t, out, "dev")

	// save
	out = e.mustRun("local", "save")
	assertContains(t, out, "Saved current state")

	// use --none
	out = e.mustRun("local", "use", "--none")
	assertContains(t, out, "vanilla")

	// rm
	out = e.mustRun("local", "rm", "dev")
	assertContains(t, out, "Removed local profile")

	out = e.mustRun("local", "ls")
	assertContains(t, out, "No local profiles")
}

// ---------------------------------------------------------------------------
// Status
// ---------------------------------------------------------------------------

func TestStatus(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	// Status with no profiles set
	out := e.mustRun("status")
	assertContains(t, out, "Global:")
	assertContains(t, out, "(vanilla)")
	assertContains(t, out, "Local:")

	// Set a global profile and check again
	e.mustRun("global", "init", "statustest")
	e.mustRun("global", "use", "statustest")

	out = e.mustRun("status")
	assertContains(t, out, "statustest")
}

func TestUseHarnessPersistsActiveByHarness(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.mustRun("global", "init", "work")
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

	out = e.mustRun("status", "--harness", "claude")
	assertContains(t, out, "claude harness:")
	assertContains(t, out, "work")

	e.mustRun("nuke", "--global", "--harness", "claude", "--force")
	data, err = os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read nuked state: %v", err)
	}
	raw = struct {
		Global struct {
			Active    string            `json:"active"`
			Harnesses map[string]string `json:"harnesses"`
		} `json:"global"`
	}{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse nuked state: %v", err)
	}
	if raw.Global.Harnesses != nil && raw.Global.Harnesses["claude"] != "" {
		t.Fatalf("expected claude harness to be cleared after nuke, got state: %s", string(data))
	}
}

func TestUseSupportsManifestBackedClaudeProfile(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "manifested")
	if err := os.MkdirAll(filepath.Join(profileRoot, "claude"), 0755); err != nil {
		t.Fatalf("mkdir manifest profile: %v", err)
	}
	manifest := "name = \"manifested\"\nharnesses = [\"claude\"]\n\n[assets]\nclaude = \"claude\"\n"
	if err := os.WriteFile(filepath.Join(profileRoot, "cvm.profile.toml"), []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileRoot, "claude", "CLAUDE.md"), []byte("# manifest profile"), 0644); err != nil {
		t.Fatalf("write manifest CLAUDE.md: %v", err)
	}

	e.mustRun("use", "manifested")

	liveClaude := filepath.Join(e.home, ".claude", "CLAUDE.md")
	data, err := os.ReadFile(liveClaude)
	if err != nil {
		t.Fatalf("read live CLAUDE.md: %v", err)
	}
	if strings.TrimSpace(string(data)) != "# manifest profile" {
		t.Fatalf("unexpected live CLAUDE.md: %q", strings.TrimSpace(string(data)))
	}

	if err := os.WriteFile(liveClaude, []byte("# saved update"), 0644); err != nil {
		t.Fatalf("overwrite live CLAUDE.md: %v", err)
	}
	e.mustRun("global", "save")

	if _, err := os.Stat(filepath.Join(profileRoot, "cvm.profile.toml")); err != nil {
		t.Fatalf("manifest should be preserved after save: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(profileRoot, "claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read saved profile CLAUDE.md: %v", err)
	}
	if strings.TrimSpace(string(data)) != "# saved update" {
		t.Fatalf("unexpected saved CLAUDE.md: %q", strings.TrimSpace(string(data)))
	}
}

func TestOpenCodeHarnessGlobalWorkflow(t *testing.T) {
	e := newTestEnv(t)
	opencodeDir := filepath.Join(e.home, ".config", "opencode")
	writeTestFile(t, filepath.Join(opencodeDir, "opencode.json"), `{"theme":"system","mcpServers":{"user-server":{"type":"local"}}}`)

	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "open")
	writeTestFile(t, filepath.Join(profileRoot, "cvm.profile.toml"), "name = \"open\"\nharnesses = [\"opencode\"]\n\n[assets]\nopencode = \"opencode\"\n")
	writeTestFile(t, filepath.Join(profileRoot, "opencode", "AGENTS.md"), "# opencode profile")
	writeTestFile(t, filepath.Join(profileRoot, "opencode", "skills", "deploy", "SKILL.md"), "---\nname: deploy\ndescription: Deploy app\n---\n")
	writeTestFile(t, filepath.Join(profileRoot, "opencode", "opencode.json"), `{"mcpServers":{"context7":{"type":"local"}}}`)

	out := e.mustRun("use", "open", "--harness", "opencode")
	assertContains(t, out, "Switched opencode harness")

	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# opencode profile")
	if _, err := os.Stat(filepath.Join(opencodeDir, "skills", "deploy", "SKILL.md")); err != nil {
		t.Fatalf("expected opencode skill to be installed: %v", err)
	}
	assertJSONKeyExists(t, filepath.Join(opencodeDir, "opencode.json"), "theme")
	assertMCPServerExists(t, filepath.Join(opencodeDir, "opencode.json"), "context7")
	if _, err := os.Stat(filepath.Join(e.home, ".claude", "AGENTS.md")); err == nil {
		t.Fatal("opencode use should not install into Claude paths")
	}

	out = e.mustRun("status", "--harness", "opencode")
	assertContains(t, out, "opencode harness:")
	assertContains(t, out, "open")
	assertContains(t, out, filepath.Join(e.home, ".config", "opencode"))

	e.mustRun("nuke", "--global", "--harness", "opencode", "--force")
	if _, err := os.Stat(filepath.Join(opencodeDir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected opencode AGENTS.md to be nuked, got err %v", err)
	}
	assertJSONKeyExists(t, filepath.Join(opencodeDir, "opencode.json"), "theme")
	assertMCPServerNotExists(t, filepath.Join(opencodeDir, "opencode.json"), "context7")
}

func TestPortableAssetsRenderForOpenCode(t *testing.T) {
	e := newTestEnv(t)
	opencodeDir := filepath.Join(e.home, ".config", "opencode")
	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "portable-open")
	writeTestFile(t, filepath.Join(profileRoot, "cvm.profile.toml"), "name = \"portable-open\"\nharnesses = [\"opencode\"]\n\n[assets]\nportable = \"portable\"\n")
	writeTestFile(t, filepath.Join(profileRoot, "portable", "instructions.md"), "# portable instructions")
	writeTestFile(t, filepath.Join(profileRoot, "portable", "skills", "deploy.md"), "---\ndescription: Deploy app\n---\n")
	writeTestFile(t, filepath.Join(profileRoot, "portable", "agents", "reviewer.md"), "# reviewer\n")

	out := e.mustRun("use", "portable-open", "--harness", "opencode")
	assertContains(t, out, "Switched opencode harness")
	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# portable instructions")
	assertFileContent(t, filepath.Join(opencodeDir, "skills", "deploy", "SKILL.md"), "---\ndescription: Deploy app\n---")
	assertFileContent(t, filepath.Join(opencodeDir, "agents", "reviewer.md"), "# reviewer")

	out = e.mustFail("use", "--none", "--harness", "opencode")
	assertContains(t, out, "live changes cannot be saved safely")
	assertFileContent(t, filepath.Join(profileRoot, "portable", "instructions.md"), "# portable instructions")
	assertFileContent(t, filepath.Join(profileRoot, "portable", "skills", "deploy.md"), "---\ndescription: Deploy app\n---")
	assertFileContent(t, filepath.Join(profileRoot, "portable", "agents", "reviewer.md"), "# reviewer")
	if _, err := os.Stat(filepath.Join(profileRoot, "portable", "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("portable dir should not capture native AGENTS.md, got err %v", err)
	}
	if _, err := os.Stat(filepath.Join(profileRoot, "portable", "skills", "deploy", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("portable dir should not capture native skill layout, got err %v", err)
	}
}

func TestHarnessAssetsOverrideRenderedPortableAssets(t *testing.T) {
	e := newTestEnv(t)
	opencodeDir := filepath.Join(e.home, ".config", "opencode")
	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "portable-open")
	writeTestFile(t, filepath.Join(profileRoot, "cvm.profile.toml"), "name = \"portable-open\"\nharnesses = [\"opencode\"]\n\n[assets]\nportable = \"portable\"\nopencode = \"opencode\"\n")
	writeTestFile(t, filepath.Join(profileRoot, "portable", "instructions.md"), "# portable instructions")
	writeTestFile(t, filepath.Join(profileRoot, "portable", "skills", "deploy.md"), "portable skill")
	writeTestFile(t, filepath.Join(profileRoot, "opencode", "AGENTS.md"), "# opencode override")
	writeTestFile(t, filepath.Join(profileRoot, "opencode", "skills", "deploy", "SKILL.md"), "opencode skill")

	e.mustRun("use", "portable-open", "--harness", "opencode")
	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# opencode override")
	assertFileContent(t, filepath.Join(opencodeDir, "skills", "deploy", "SKILL.md"), "opencode skill")

	otherRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "other-open")
	writeTestFile(t, filepath.Join(otherRoot, "cvm.profile.toml"), "name = \"other-open\"\nharnesses = [\"opencode\"]\n\n[assets]\nopencode = \"opencode\"\n")
	writeTestFile(t, filepath.Join(otherRoot, "opencode", "AGENTS.md"), "# other")
	writeTestFile(t, filepath.Join(opencodeDir, "AGENTS.md"), "# live edit")

	e.mustRun("use", "other-open", "--harness", "opencode")
	assertFileContent(t, filepath.Join(profileRoot, "opencode", "AGENTS.md"), "# live edit")
	assertFileContent(t, filepath.Join(profileRoot, "portable", "instructions.md"), "# portable instructions")
}

func TestPortableInstructionsRenderForCodex(t *testing.T) {
	e := newTestEnv(t)
	codexDir := filepath.Join(e.home, ".codex")
	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "portable-codex")
	writeTestFile(t, filepath.Join(profileRoot, "cvm.profile.toml"), "name = \"portable-codex\"\nharnesses = [\"codex\"]\n\n[assets]\nportable = \"portable\"\n")
	writeTestFile(t, filepath.Join(profileRoot, "portable", "instructions.md"), "# codex instructions")
	writeTestFile(t, filepath.Join(profileRoot, "portable", "skills", "deploy.md"), "portable skill")

	out := e.mustRun("use", "portable-codex", "--harness", "codex")
	assertContains(t, out, "Switched codex harness")
	assertFileContent(t, filepath.Join(codexDir, "AGENTS.md"), "# codex instructions")
	if _, err := os.Stat(filepath.Join(codexDir, "skills")); !os.IsNotExist(err) {
		t.Fatalf("codex should not install portable skills without native support, got err %v", err)
	}

	out = e.mustFail("use", "--none", "--harness", "codex")
	assertContains(t, out, "live changes cannot be saved safely")
	assertFileContent(t, filepath.Join(profileRoot, "portable", "instructions.md"), "# codex instructions")
	assertFileContent(t, filepath.Join(profileRoot, "portable", "skills", "deploy.md"), "portable skill")
	if _, err := os.Stat(filepath.Join(profileRoot, "portable", "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("portable dir should not capture codex AGENTS.md, got err %v", err)
	}
}

func TestLiteProfileActivatesForAllHarnesses(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")
	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "lite")
	if err := cvmprofile.CopyDir(filepath.Join("profiles", "lite"), profileRoot); err != nil {
		t.Fatalf("copy lite profile: %v", err)
	}

	out := e.mustRun("use", "lite", "--harness", "claude")
	assertContains(t, out, "Switched claude harness")
	assertFileContains(t, filepath.Join(e.home, ".claude", "CLAUDE.md"), "# Lite Profile")
	writeTestFile(t, filepath.Join(e.home, ".claude", "CLAUDE.md"), "# live lite edit")
	out = e.mustFail("global", "save")
	assertContains(t, out, "live changes cannot be saved safely")
	out = e.mustFail("use", "--none", "--harness", "claude")
	assertContains(t, out, "live changes cannot be saved safely")
	assertFileContains(t, filepath.Join(profileRoot, "cvm.profile.toml"), "harnesses = [\"claude\", \"opencode\", \"codex\"]")
	assertFileContains(t, filepath.Join(profileRoot, "portable", "instructions.md"), "# Lite Profile")

	out = e.mustRun("use", "lite", "--harness", "opencode")
	assertContains(t, out, "Switched opencode harness")
	assertFileContains(t, filepath.Join(e.home, ".config", "opencode", "AGENTS.md"), "# Lite Profile")

	out = e.mustRun("use", "lite", "--harness", "codex")
	assertContains(t, out, "Switched codex harness")
	assertFileContains(t, filepath.Join(e.home, ".codex", "AGENTS.md"), "# Lite Profile")
}

func TestOpenCodeHarnessRestoreGlobalVanilla(t *testing.T) {
	e := newTestEnv(t)
	opencodeDir := filepath.Join(e.home, ".config", "opencode")
	writeTestFile(t, filepath.Join(opencodeDir, "AGENTS.md"), "# vanilla opencode")

	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "open")
	writeTestFile(t, filepath.Join(profileRoot, "cvm.profile.toml"), "name = \"open\"\nharnesses = [\"opencode\"]\n\n[assets]\nopencode = \"opencode\"\n")
	writeTestFile(t, filepath.Join(profileRoot, "opencode", "AGENTS.md"), "# profile opencode")

	e.mustRun("use", "open", "--harness", "opencode")
	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# profile opencode")

	out := e.mustRun("restore", "--global", "--harness", "opencode")
	assertContains(t, out, "Restored global config to vanilla (opencode harness)")
	assertFileContent(t, filepath.Join(opencodeDir, "AGENTS.md"), "# vanilla opencode")
}

func TestOpenCodeHarnessLocalWorkflow(t *testing.T) {
	e := newTestEnv(t)

	profileRoot := filepath.Join(e.home, ".cvm", "local", "profiles", "open-local")
	writeTestFile(t, filepath.Join(profileRoot, "cvm.profile.toml"), "name = \"open-local\"\nharnesses = [\"opencode\"]\n\n[assets]\nopencode = \"opencode\"\n")
	writeTestFile(t, filepath.Join(profileRoot, "opencode", "AGENTS.md"), "# local opencode profile")

	out := e.mustRun("use", "open-local", "--local", "--harness", "opencode")
	assertContains(t, out, "Switched opencode harness")
	assertFileContent(t, filepath.Join(e.projectDir, ".opencode", "AGENTS.md"), "# local opencode profile")
	if _, err := os.Stat(filepath.Join(e.home, ".config", "opencode", "AGENTS.md")); err == nil {
		t.Fatal("local opencode use should not install into global OpenCode paths")
	}

	e.mustRun("nuke", "--local", "--harness", "opencode", "--force")
	if _, err := os.Stat(filepath.Join(e.projectDir, ".opencode", "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected local opencode AGENTS.md to be nuked, got err %v", err)
	}
}

func TestManifestBackedProfileOverridesRestoreFromAssetDir(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "manifested")
	writeTestFile(t, filepath.Join(profileRoot, "cvm.profile.toml"), "name = \"manifested\"\nharnesses = [\"claude\"]\n\n[assets]\nclaude = \"claude\"\n")
	writeTestFile(t, filepath.Join(profileRoot, "claude", "skills", "deploy", "SKILL.md"), "base skill")

	e.mustRun("use", "manifested")

	liveSkill := filepath.Join(e.home, ".claude", "skills", "deploy", "SKILL.md")
	overrideSkill := filepath.Join(e.home, ".cvm", "global", "overrides", "manifested", "skills", "deploy", "SKILL.md")
	writeTestFile(t, overrideSkill, "override skill")
	e.mustRun("override", "apply")

	data, err := os.ReadFile(liveSkill)
	if err != nil {
		t.Fatalf("read live skill after apply: %v", err)
	}
	if strings.TrimSpace(string(data)) != "override skill" {
		t.Fatalf("unexpected live override content: %q", strings.TrimSpace(string(data)))
	}

	e.mustRun("global", "save")

	data, err = os.ReadFile(filepath.Join(profileRoot, "claude", "skills", "deploy", "SKILL.md"))
	if err != nil {
		t.Fatalf("read base skill after save: %v", err)
	}
	if strings.TrimSpace(string(data)) != "base skill" {
		t.Fatalf("base skill should have been restored from manifest asset dir before save, got %q", strings.TrimSpace(string(data)))
	}
}

func TestProfileInspect(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")
	e.seedLocalClaude("# local vanilla")

	e.mustRun("global", "init", "inspect-global")
	e.mustRun("local", "init", "inspect-local")

	globalSkillDir := filepath.Join(e.home, ".cvm", "global", "profiles", "inspect-global", "skills")
	globalAgentDir := filepath.Join(e.home, ".cvm", "global", "profiles", "inspect-global", "agents")
	globalHookDir := filepath.Join(e.home, ".cvm", "global", "profiles", "inspect-global", "hooks")
	if err := os.MkdirAll(globalSkillDir, 0755); err != nil {
		t.Fatalf("mkdir global skill dir: %v", err)
	}
	if err := os.MkdirAll(globalAgentDir, 0755); err != nil {
		t.Fatalf("mkdir global agent dir: %v", err)
	}
	if err := os.MkdirAll(globalHookDir, 0755); err != nil {
		t.Fatalf("mkdir global hook dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalSkillDir, "deploy.md"), []byte(""), 0644); err != nil {
		t.Fatalf("write global skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalAgentDir, "reviewer.md"), []byte(""), 0644); err != nil {
		t.Fatalf("write global agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalHookDir, "post.sh"), []byte(""), 0644); err != nil {
		t.Fatalf("write global hook: %v", err)
	}

	localRuleDir := filepath.Join(e.home, ".cvm", "local", "profiles", "inspect-local", "rules")
	if err := os.MkdirAll(localRuleDir, 0755); err != nil {
		t.Fatalf("mkdir local rule dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localRuleDir, "scope.md"), []byte(""), 0644); err != nil {
		t.Fatalf("write local rule: %v", err)
	}

	e.mustRun("global", "use", "inspect-global")
	e.mustRun("local", "use", "inspect-local")

	out := e.mustRun("profile")
	assertContains(t, out, "Global profile: inspect-global")
	assertContains(t, out, "Skills (1): deploy.md")
	assertContains(t, out, "Agents (1): reviewer.md")
	assertContains(t, out, "Hooks (1): post.sh")
	assertContains(t, out, "Local profile: inspect-local")
	assertContains(t, out, "Rules (1): scope.md")

	out = e.mustRun("profile", "show", "inspect-global")
	assertContains(t, out, "Global profile: inspect-global")
	assertContains(t, out, "Skills (1): deploy.md")

	out = e.mustRun("profile", "show", "inspect-local", "--local")
	assertContains(t, out, "Local profile: inspect-local")
	assertContains(t, out, "Rules (1): scope.md")
}

func TestProfileAddScaffoldsPortableAssets(t *testing.T) {
	e := newTestEnv(t)

	e.mustRun("add", "portable")

	out := e.mustRun("profile", "add", "skill", "deploy", "--profile", "portable")
	assertContains(t, out, "Created portable skill")
	assertContains(t, out, "Created manifest:")
	assertContains(t, out, "portable assets are authored now")
	out = e.mustRun("profile", "add", "agent", "reviewer", "--profile", "portable")
	assertContains(t, out, "Created portable agent")
	out = e.mustRun("profile", "add", "instructions", "--profile", "portable")
	assertContains(t, out, "Created portable instructions")
	out = e.mustRun("profile", "add", "instructions", "--profile", "portable", "--harness", "opencode")
	assertContains(t, out, "Created opencode instructions")

	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "portable")
	assertFileContains(t, filepath.Join(profileRoot, "portable", "skills", "deploy.md"), "description:")
	assertFileContains(t, filepath.Join(profileRoot, "portable", "agents", "reviewer.md"), "# reviewer")
	assertFileContains(t, filepath.Join(profileRoot, "portable", "instructions.md"), "# Profile Instructions")
	assertFileContains(t, filepath.Join(profileRoot, "opencode", "AGENTS.md"), "# Profile Instructions")
	assertFileContains(t, filepath.Join(profileRoot, "cvm.profile.toml"), "portable = \"portable\"")
	assertFileContains(t, filepath.Join(profileRoot, "cvm.profile.toml"), "opencode = \"opencode\"")
}

func TestProfileAddScaffoldsHarnessSpecificHookFromFile(t *testing.T) {
	e := newTestEnv(t)

	e.mustRun("add", "hooks")

	out := e.mustFail("profile", "add", "hook", "post", "--profile", "hooks")
	assertContains(t, out, "--harness")

	source := filepath.Join(e.projectDir, "post.sh")
	writeTestFile(t, source, "#!/bin/sh\nexit 0\n")
	out = e.mustRun("profile", "add", "hook", "post", "--profile", "hooks", "--harness", "claude", "--from-file", source)
	assertContains(t, out, "Created claude hook")

	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "hooks")
	assertFileContent(t, filepath.Join(profileRoot, "claude", "hooks", "post.sh"), "#!/bin/sh\nexit 0")
	assertFileContains(t, filepath.Join(profileRoot, "cvm.profile.toml"), "claude = \"claude\"")
}

func TestProfileAddRejectsInvalidHarnessAndSource(t *testing.T) {
	e := newTestEnv(t)

	e.mustRun("add", "invalid")

	out := e.mustFail("profile", "add", "skill", "deploy", "--profile", "invalid", "--harness", "missing")
	assertContains(t, out, "unknown harness")

	sourceDir := filepath.Join(e.projectDir, "source-dir")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	out = e.mustFail("profile", "add", "skill", "deploy", "--profile", "invalid", "--from-file", sourceDir)
	assertContains(t, out, "is a directory")

	out = e.mustFail("profile", "add", "hook", "post", "--profile", "invalid", "--harness", "opencode")
	assertContains(t, out, "opencode does not support hook scaffolding")
}

func TestProfileAddDoesNotOverwriteExistingAsset(t *testing.T) {
	e := newTestEnv(t)

	e.mustRun("add", "existing")
	source := filepath.Join(e.projectDir, "deploy.md")
	writeTestFile(t, source, "first")
	e.mustRun("profile", "add", "skill", "deploy", "--profile", "existing", "--from-file", source)

	writeTestFile(t, source, "second")
	out := e.mustRun("profile", "add", "skill", "deploy", "--profile", "existing", "--from-file", source)
	assertContains(t, out, "Profile asset already exists")

	assertFileContent(t, filepath.Join(e.home, ".cvm", "global", "profiles", "existing", "portable", "skills", "deploy.md"), "first")
}

func TestProfileAddDefaultsToActiveProfile(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.mustRun("add", "active")
	e.mustRun("use", "active")

	out := e.mustRun("profile", "add", "instructions")
	assertContains(t, out, "Created portable instructions")

	profileRoot := filepath.Join(e.home, ".cvm", "global", "profiles", "active")
	assertFileContains(t, filepath.Join(profileRoot, "portable", "instructions.md"), "# Profile Instructions")
	assertFileContains(t, filepath.Join(profileRoot, "cvm.profile.toml"), "claude = \".\"")
}

func TestProfileAddHelpExplainsAuthoringLayers(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustRun("profile", "add", "--help")
	assertContains(t, out, "Portable assets")
	assertContains(t, out, "Hooks are always harness-specific")
	assertContains(t, out, "Harness rendering")
}

func TestLsShowsInUseProfiles(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")
	e.seedLocalClaude("# local vanilla")

	e.mustRun("global", "init", "work")
	e.mustRun("local", "init", "dev")
	e.mustRun("global", "use", "work")
	e.mustRun("local", "use", "dev")

	out := e.mustRun("ls")
	assertContains(t, out, "work")
	assertContains(t, out, "dev")
	assertContains(t, out, "IN USE")
}

func TestBypassCommand(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.mustRun("global", "init", "bypass-global")
	e.mustRun("global", "use", "bypass-global")

	out := e.mustRun("bypass", "status")
	assertContains(t, out, "global profile \"bypass-global\"")

	out = e.mustRun("bypass", "on")
	assertContains(t, out, "bypassPermissions")

	overrideSettings := filepath.Join(e.home, ".cvm", "global", "overrides", "bypass-global", "settings.json")
	activeSettings := filepath.Join(e.home, ".claude", "settings.json")
	assertSettingsMode(t, overrideSettings, "bypassPermissions")
	assertSettingsMode(t, activeSettings, "bypassPermissions")

	out = e.mustRun("bypass", "off")
	assertContains(t, out, "default")
	if _, err := os.Stat(overrideSettings); !os.IsNotExist(err) {
		t.Fatalf("expected override settings.json to be removed after bypass off")
	}
	// Active settings.json should not have bypassPermissions (may not exist if base profile had none)
	if _, err := os.Stat(activeSettings); err == nil {
		assertSettingsNotMode(t, activeSettings, "bypassPermissions")
	}
}

// ---------------------------------------------------------------------------
// Nuke --force (global)
// ---------------------------------------------------------------------------

func TestNukeGlobal(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# will be nuked")

	// Verify the file exists
	claudeMD := filepath.Join(e.home, ".claude", "CLAUDE.md")
	if _, err := os.Stat(claudeMD); err != nil {
		t.Fatal("seed CLAUDE.md should exist before nuke")
	}

	out := e.mustRun("nuke", "--global", "--force")
	assertContains(t, out, "Nuked global config")

	// CLAUDE.md should be gone
	if _, err := os.Stat(claudeMD); err == nil {
		t.Fatal("CLAUDE.md should have been removed by nuke")
	}
}

// ---------------------------------------------------------------------------
// Nuke --force (local)
// ---------------------------------------------------------------------------

func TestNukeLocal(t *testing.T) {
	e := newTestEnv(t)
	e.seedLocalClaude("# will be nuked locally")

	localClaudeMD := filepath.Join(e.projectDir, ".claude", "CLAUDE.md")
	if _, err := os.Stat(localClaudeMD); err != nil {
		t.Fatal("local CLAUDE.md should exist before nuke")
	}

	out := e.mustRun("nuke", "--local", "--force")
	assertContains(t, out, "Nuked local config")

	if _, err := os.Stat(localClaudeMD); err == nil {
		t.Fatal("local CLAUDE.md should have been removed by nuke")
	}
}

// ---------------------------------------------------------------------------
// Restore
// ---------------------------------------------------------------------------

func TestRestore(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# original vanilla content")

	// Create and use a profile to trigger vanilla backup
	e.mustRun("global", "init", "temp")
	e.mustRun("global", "use", "temp")

	// Write something different to ~/.claude/CLAUDE.md
	claudeMD := filepath.Join(e.home, ".claude", "CLAUDE.md")
	os.WriteFile(claudeMD, []byte("# modified by profile"), 0644)

	// Nuke global
	e.mustRun("nuke", "--global", "--force")

	// Restore
	out := e.mustRun("restore", "--global")
	assertContains(t, out, "Restored global config to vanilla")

	// Verify the original content is back
	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("CLAUDE.md should exist after restore: %v", err)
	}
	if !strings.Contains(string(data), "original vanilla content") {
		t.Fatalf("expected vanilla content, got: %s", string(data))
	}
}

func TestRestoreWithHarness(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# original vanilla content")

	e.mustRun("global", "init", "temp")
	e.mustRun("use", "temp", "--harness", "claude")

	claudeMD := filepath.Join(e.home, ".claude", "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte("# modified by profile"), 0644); err != nil {
		t.Fatalf("modify CLAUDE.md: %v", err)
	}

	e.mustRun("nuke", "--global", "--harness", "claude", "--force")
	out := e.mustRun("restore", "--global", "--harness", "claude")
	assertContains(t, out, "Restored global config to vanilla")

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("CLAUDE.md should exist after harness restore: %v", err)
	}
	if !strings.Contains(string(data), "original vanilla content") {
		t.Fatalf("expected vanilla content, got: %s", string(data))
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestEdgeDuplicateInit(t *testing.T) {
	e := newTestEnv(t)

	e.mustRun("global", "init", "dup")
	out := e.mustFail("global", "init", "dup")
	assertContains(t, out, "already exists")
}

func TestEdgeRmActiveProfile(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.mustRun("global", "init", "active")
	e.mustRun("global", "use", "active")

	out := e.mustFail("global", "rm", "active")
	assertContains(t, out, "cannot remove active profile")
}

func TestEdgeUseNonexistent(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("global", "use", "ghost")
	assertContains(t, out, "not found")
}

func TestEdgeFromFlag(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# base content")

	e.mustRun("global", "init", "base")
	e.mustRun("global", "init", "derived", "--from", "base")

	// derived should exist and be listable
	out := e.mustRun("global", "ls")
	assertContains(t, out, "base")
	assertContains(t, out, "derived")
}

func TestEdgeFromNonexistent(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("global", "init", "bad", "--from", "nope")
	assertContains(t, out, "not found")
}

func TestEdgeSaveWithNoActiveProfile(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("global", "save")
	assertContains(t, out, "no active global profile")
}

func TestEdgeLocalSaveNoActive(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("local", "save")
	assertContains(t, out, "no active local profile")
}

func TestEdgeUseNoArgs(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("global", "use")
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
// Multiple profiles coexist
// ---------------------------------------------------------------------------

func TestMultipleProfilesCoexist(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.mustRun("global", "init", "alpha")
	e.mustRun("global", "init", "beta")
	e.mustRun("global", "init", "gamma")

	out := e.mustRun("global", "ls")
	assertContains(t, out, "alpha")
	assertContains(t, out, "beta")
	assertContains(t, out, "gamma")

	// Switch between them
	e.mustRun("global", "use", "alpha")
	out = e.mustRun("global", "current")
	assertContains(t, out, "alpha")

	e.mustRun("global", "use", "beta")
	out = e.mustRun("global", "current")
	assertContains(t, out, "beta")

	// Remove non-active profile
	e.mustRun("global", "rm", "gamma")
	out = e.mustRun("global", "ls")
	assertNotContains(t, out, "gamma")
	assertContains(t, out, "alpha")
	assertContains(t, out, "beta")
}

// ---------------------------------------------------------------------------
// Restore with no vanilla backup
// ---------------------------------------------------------------------------

func TestRestoreNoVanilla(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustRun("restore", "--global")
	assertContains(t, out, "No vanilla backup found")
}

// ---------------------------------------------------------------------------
// Profile content isolation: switching profiles applies correct content
// ---------------------------------------------------------------------------

func TestProfileContentIsolation(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla state")

	e.mustRun("global", "init", "p1")
	e.mustRun("global", "use", "p1")

	// Write unique content while p1 is active
	claudeMD := filepath.Join(e.home, ".claude", "CLAUDE.md")
	os.WriteFile(claudeMD, []byte("# p1 content"), 0644)
	e.mustRun("global", "save")

	// Create p2 from scratch and switch
	e.mustRun("global", "init", "p2")
	e.mustRun("global", "use", "p2")

	// Write different content for p2
	os.WriteFile(claudeMD, []byte("# p2 content"), 0644)
	e.mustRun("global", "save")

	// Switch back to p1 and verify its content
	e.mustRun("global", "use", "p1")
	data, _ := os.ReadFile(claudeMD)
	if !strings.Contains(string(data), "p1 content") {
		t.Fatalf("expected p1 content after switching back, got: %s", string(data))
	}

	// Switch to p2 and verify its content
	e.mustRun("global", "use", "p2")
	data, _ = os.ReadFile(claudeMD)
	if !strings.Contains(string(data), "p2 content") {
		t.Fatalf("expected p2 content after switching, got: %s", string(data))
	}
}

func TestUseAppliesChicheMCPServersToClaudeUserConfig(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")
	if err := os.WriteFile(filepath.Join(e.home, ".claude.json"), []byte(`{
  "theme": "dark",
  "oauthAccount": {
    "emailAddress": "user@example.com"
  }
}
`), 0644); err != nil {
		t.Fatalf("write live user config: %v", err)
	}

	e.mustRun("global", "init", "chiche")

	profileUserConfig := filepath.Join(e.home, ".cvm", "global", "profiles", "chiche", ".claude.json")
	if err := os.WriteFile(profileUserConfig, []byte(`{
  "mcpServers": {
    "playwright": {
      "command": "npx",
      "args": ["-y", "@playwright/mcp@latest"]
    },
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp@latest"]
    }
  }
}
`), 0644); err != nil {
		t.Fatalf("write profile user config: %v", err)
	}

	e.mustRun("use", "chiche")

	claudeUserConfig := filepath.Join(e.home, ".claude.json")
	assertMCPServerExists(t, claudeUserConfig, "playwright")
	assertMCPServerExists(t, claudeUserConfig, "context7")
	assertJSONKeyExists(t, claudeUserConfig, "oauthAccount")
}

// ---------------------------------------------------------------------------
// Nuke both scopes
// ---------------------------------------------------------------------------

func TestNukeBothScopes(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# global content")
	e.seedLocalClaude("# local content")

	out := e.mustRun("nuke", "--force")
	assertContains(t, out, "Nuked global config")
	assertContains(t, out, "Nuked local config")

	globalMD := filepath.Join(e.home, ".claude", "CLAUDE.md")
	localMD := filepath.Join(e.projectDir, ".claude", "CLAUDE.md")

	if _, err := os.Stat(globalMD); err == nil {
		t.Fatal("global CLAUDE.md should be removed after nuke")
	}
	if _, err := os.Stat(localMD); err == nil {
		t.Fatal("local CLAUDE.md should be removed after nuke")
	}
}

// ---------------------------------------------------------------------------
// Local restore
// ---------------------------------------------------------------------------

func TestLocalRestore(t *testing.T) {
	e := newTestEnv(t)
	e.seedLocalClaude("# original local vanilla")

	// Create and use a local profile to trigger vanilla backup
	e.mustRun("local", "init", "localtemp")
	e.mustRun("local", "use", "localtemp")

	// Nuke local
	e.mustRun("nuke", "--local", "--force")

	// Restore local
	out := e.mustRun("restore", "--local")
	assertContains(t, out, "Restored local config to vanilla")

	localMD := filepath.Join(e.projectDir, ".claude", "CLAUDE.md")
	data, err := os.ReadFile(localMD)
	if err != nil {
		t.Fatalf("local CLAUDE.md should exist after restore: %v", err)
	}
	if !strings.Contains(string(data), "original local vanilla") {
		t.Fatalf("expected vanilla content, got: %s", string(data))
	}
}

// ---------------------------------------------------------------------------
// Default profile name for local init (project dir name)
// ---------------------------------------------------------------------------

func TestLocalInitDefaultName(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustRun("local", "init")
	// Default name is the project dir basename = "myproject"
	assertContains(t, out, "Created local profile")
	assertContains(t, out, "myproject")
}

// ---------------------------------------------------------------------------
// Global init default name
// ---------------------------------------------------------------------------

func TestGlobalInitDefaultName(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustRun("global", "init")
	assertContains(t, out, "Created global profile")
	assertContains(t, out, "default")
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

func assertSettingsMode(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings %s: %v", path, err)
	}

	var cfg struct {
		Permissions struct {
			DefaultMode string `json:"defaultMode"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal settings %s: %v", path, err)
	}
	if cfg.Permissions.DefaultMode != want {
		t.Fatalf("settings %s defaultMode = %q, want %q", path, cfg.Permissions.DefaultMode, want)
	}
}

func assertSettingsNotMode(t *testing.T, path, notWant string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings %s: %v", path, err)
	}

	var cfg struct {
		Permissions struct {
			DefaultMode string `json:"defaultMode"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal settings %s: %v", path, err)
	}
	if cfg.Permissions.DefaultMode == notWant {
		t.Fatalf("settings %s defaultMode = %q, expected it NOT to be %q", path, cfg.Permissions.DefaultMode, notWant)
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

func assertMCPServerNotExists(t *testing.T, path, notWant string) {
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
	if _, ok := cfg.MCPServers[notWant]; ok {
		t.Fatalf("settings %s should not contain mcp server %q", path, notWant)
	}
}

func assertJSONKeyExists(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json %s: %v", path, err)
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal json %s: %v", path, err)
	}
	if _, ok := cfg[want]; !ok {
		t.Fatalf("json %s missing key %q", path, want)
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
