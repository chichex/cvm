package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
// KB workflow (global)
// ---------------------------------------------------------------------------

func TestKBGlobalWorkflow(t *testing.T) {
	e := newTestEnv(t)

	// put
	out := e.mustRun("kb", "put", "go-patterns", "--body", "Use table-driven tests", "--tag", "go,testing")
	assertContains(t, out, "Saved KB entry")

	// ls
	out = e.mustRun("kb", "ls")
	assertContains(t, out, "go-patterns")

	// ls with tag filter
	out = e.mustRun("kb", "ls", "--tag", "go")
	assertContains(t, out, "go-patterns")

	out = e.mustRun("kb", "ls", "--tag", "nonexistent")
	assertContains(t, out, "No KB entries")

	// show
	out = e.mustRun("kb", "show", "go-patterns")
	assertContains(t, out, "table-driven tests")

	// search
	out = e.mustRun("kb", "search", "table-driven")
	assertContains(t, out, "go-patterns")

	// disable
	out = e.mustRun("kb", "disable", "go-patterns")
	assertContains(t, out, "Disabled KB entry")

	// enable
	out = e.mustRun("kb", "enable", "go-patterns")
	assertContains(t, out, "Enabled KB entry")

	// rm
	out = e.mustRun("kb", "rm", "go-patterns")
	assertContains(t, out, "Removed KB entry")

	out = e.mustRun("kb", "ls")
	assertContains(t, out, "No KB entries")
}

// ---------------------------------------------------------------------------
// KB workflow (local)
// ---------------------------------------------------------------------------

func TestKBLocalWorkflow(t *testing.T) {
	e := newTestEnv(t)

	// put --local
	out := e.mustRun("kb", "put", "proj-notes", "--body", "Project-specific note", "--tag", "local", "--local")
	assertContains(t, out, "Saved KB entry")
	assertContains(t, out, "local")

	// ls --local
	out = e.mustRun("kb", "ls", "--local")
	assertContains(t, out, "proj-notes")

	// show --local
	out = e.mustRun("kb", "show", "proj-notes", "--local")
	assertContains(t, out, "Project-specific note")

	// search --local
	out = e.mustRun("kb", "search", "Project-specific", "--local")
	assertContains(t, out, "proj-notes")

	// disable --local
	out = e.mustRun("kb", "disable", "proj-notes", "--local")
	assertContains(t, out, "Disabled KB entry")

	// enable --local
	out = e.mustRun("kb", "enable", "proj-notes", "--local")
	assertContains(t, out, "Enabled KB entry")

	// rm --local
	out = e.mustRun("kb", "rm", "proj-notes", "--local")
	assertContains(t, out, "Removed KB entry")

	out = e.mustRun("kb", "ls", "--local")
	assertContains(t, out, "No KB entries")
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

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func TestHealth(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	out := e.mustRun("health")
	assertContains(t, out, "cvm health")
	assertContains(t, out, "global profile:")
	assertContains(t, out, "profiles:")
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

func TestEdgeKBRmNonexistent(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("kb", "rm", "nope")
	assertContains(t, out, "not found")
}

func TestEdgeKBShowNonexistent(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("kb", "show", "nope")
	assertContains(t, out, "not found")
}

func TestEdgeKBDisableNonexistent(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustFail("kb", "disable", "nope")
	assertContains(t, out, "not found")
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func TestLifecycle(t *testing.T) {
	e := newTestEnv(t)

	// status before any session
	out := e.mustRun("lifecycle", "status")
	assertContains(t, out, "No active cvm session")

	// start
	out = e.mustRun("lifecycle", "start")
	assertContains(t, out, "cvm session started")

	// status after start
	out = e.mustRun("lifecycle", "status")
	assertContains(t, out, "Session active since")

	// end
	out = e.mustRun("lifecycle", "end")
	assertContains(t, out, "session ended")

	// status after end
	out = e.mustRun("lifecycle", "status")
	assertContains(t, out, "No active cvm session")
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

func TestLifecycleEndSavesAddedMCPServersToActiveProfile(t *testing.T) {
	e := newTestEnv(t)
	e.seedGlobalClaude("# vanilla")

	e.mustRun("global", "init", "chiche")
	e.mustRun("use", "chiche")

	activeSettings := filepath.Join(e.home, ".claude.json")
	data, err := os.ReadFile(activeSettings)
	if os.IsNotExist(err) {
		data = []byte("{}")
	} else if err != nil {
		t.Fatalf("read active settings: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal active settings: %v", err)
	}

	mcpServers, _ := cfg["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = map[string]any{}
	}
	mcpServers["sequential-thinking"] = map[string]any{
		"command": "npx",
		"args":    []any{"-y", "@modelcontextprotocol/server-sequential-thinking"},
	}
	cfg["mcpServers"] = mcpServers

	updated, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal updated settings: %v", err)
	}
	updated = append(updated, '\n')
	if err := os.WriteFile(activeSettings, updated, 0644); err != nil {
		t.Fatalf("write active settings: %v", err)
	}

	e.mustRun("lifecycle", "end")

	profileSettings := filepath.Join(e.home, ".cvm", "global", "profiles", "chiche", ".claude.json")
	assertMCPServerExists(t, activeSettings, "sequential-thinking")
	assertMCPServerExists(t, profileSettings, "sequential-thinking")
}

// ---------------------------------------------------------------------------
// KB update existing entry
// ---------------------------------------------------------------------------

func TestKBUpdateEntry(t *testing.T) {
	e := newTestEnv(t)

	e.mustRun("kb", "put", "mykey", "--body", "original body", "--tag", "v1")
	out := e.mustRun("kb", "show", "mykey")
	assertContains(t, out, "original body")

	// Update the same key
	e.mustRun("kb", "put", "mykey", "--body", "updated body", "--tag", "v2")
	out = e.mustRun("kb", "show", "mykey")
	assertContains(t, out, "updated body")
}

// ---------------------------------------------------------------------------
// KB search no results
// ---------------------------------------------------------------------------

func TestKBSearchNoResults(t *testing.T) {
	e := newTestEnv(t)

	out := e.mustRun("kb", "search", "nonexistent")
	assertContains(t, out, "No matches")
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
