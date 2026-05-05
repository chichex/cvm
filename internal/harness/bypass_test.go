package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chichex/cvm/internal/config"
)

// withTempHome points HOME (and harness-specific config-dir env vars) at a
// temp dir so each subtest has its own clean ~/.cvm and ~/.codex/~/.config.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	return home
}

// ---------------------------------------------------------------------------
// Claude
// ---------------------------------------------------------------------------

func TestClaudeBypassEnableWritesOverrideSettings(t *testing.T) {
	withTempHome(t)
	const profileName = "work"

	if err := Claude().EnableBypass(profileName); err != nil {
		t.Fatalf("enable bypass: %v", err)
	}

	overridePath := filepath.Join(config.OverrideDir(profileName), "settings.json")
	data, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read override settings: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal override settings: %v", err)
	}
	perms, ok := cfg["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("expected permissions map, got %T", cfg["permissions"])
	}
	if got, want := perms["defaultMode"], "bypassPermissions"; got != want {
		t.Fatalf("defaultMode = %v, want %v", got, want)
	}
	if _, ok := perms["allow"].([]any); !ok {
		t.Fatalf("expected allow list, got %T", perms["allow"])
	}

	mode, err := Claude().BypassStatus(profileName)
	if err != nil {
		t.Fatalf("bypass status: %v", err)
	}
	if mode != "bypassPermissions" {
		t.Fatalf("status = %q, want bypassPermissions", mode)
	}
}

func TestClaudeBypassDisablePreservesUnrelatedKeys(t *testing.T) {
	withTempHome(t)
	const profileName = "work"

	overridePath := filepath.Join(config.OverrideDir(profileName), "settings.json")
	if err := os.MkdirAll(filepath.Dir(overridePath), 0755); err != nil {
		t.Fatal(err)
	}
	preexisting := map[string]any{
		"theme": "dark",
		"permissions": map[string]any{
			"defaultMode": "ask",
		},
	}
	data, _ := json.MarshalIndent(preexisting, "", "  ")
	if err := os.WriteFile(overridePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := Claude().EnableBypass(profileName); err != nil {
		t.Fatalf("enable bypass: %v", err)
	}
	if err := Claude().DisableBypass(profileName); err != nil {
		t.Fatalf("disable bypass: %v", err)
	}

	// theme=dark should survive; permissions key should be gone — file may
	// remain because non-permissions keys are preserved.
	got, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read after disable: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(got, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg["theme"] != "dark" {
		t.Fatalf("expected theme to be preserved, got %v", cfg["theme"])
	}
	if _, ok := cfg["permissions"]; ok {
		t.Fatalf("permissions should have been removed: %v", cfg)
	}
}

func TestClaudeBypassDisableEmptyOverrideRemovesFile(t *testing.T) {
	withTempHome(t)
	const profileName = "work"

	if err := Claude().EnableBypass(profileName); err != nil {
		t.Fatal(err)
	}
	if err := Claude().DisableBypass(profileName); err != nil {
		t.Fatal(err)
	}

	overridePath := filepath.Join(config.OverrideDir(profileName), "settings.json")
	if _, err := os.Stat(overridePath); !os.IsNotExist(err) {
		t.Fatalf("expected override settings.json to be removed, stat err=%v", err)
	}
}

// ---------------------------------------------------------------------------
// OpenCode
// ---------------------------------------------------------------------------

func TestOpenCodeBypassEnableShorthand(t *testing.T) {
	withTempHome(t)
	const profileName = "work"

	if err := OpenCode().EnableBypass(profileName); err != nil {
		t.Fatalf("enable bypass: %v", err)
	}

	overridePath := filepath.Join(config.OverrideDir(profileName), "opencode.json")
	data, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read override opencode.json: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, want := cfg["permission"], "allow"; got != want {
		t.Fatalf("permission = %v, want %v", got, want)
	}

	status, err := OpenCode().BypassStatus(profileName)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "allow" {
		t.Fatalf("status = %q, want allow", status)
	}
}

func TestOpenCodeBypassPreservesOtherKeys(t *testing.T) {
	withTempHome(t)
	const profileName = "work"

	overridePath := filepath.Join(config.OverrideDir(profileName), "opencode.json")
	if err := os.MkdirAll(filepath.Dir(overridePath), 0755); err != nil {
		t.Fatal(err)
	}
	preexisting := map[string]any{
		"mcpServers": map[string]any{
			"foo": map[string]any{"command": "bar"},
		},
		"theme": "tokyo-night",
	}
	data, _ := json.MarshalIndent(preexisting, "", "  ")
	if err := os.WriteFile(overridePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := OpenCode().EnableBypass(profileName); err != nil {
		t.Fatalf("enable bypass: %v", err)
	}

	got, _ := os.ReadFile(overridePath)
	var cfg map[string]any
	if err := json.Unmarshal(got, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg["theme"] != "tokyo-night" {
		t.Fatalf("theme dropped: %v", cfg)
	}
	if _, ok := cfg["mcpServers"].(map[string]any); !ok {
		t.Fatalf("mcpServers dropped: %v", cfg)
	}
	if cfg["permission"] != "allow" {
		t.Fatalf("permission not set: %v", cfg)
	}

	if err := OpenCode().DisableBypass(profileName); err != nil {
		t.Fatalf("disable bypass: %v", err)
	}

	got2, _ := os.ReadFile(overridePath)
	var cfg2 map[string]any
	if err := json.Unmarshal(got2, &cfg2); err != nil {
		t.Fatalf("unmarshal after disable: %v", err)
	}
	if _, ok := cfg2["permission"]; ok {
		t.Fatalf("permission should be removed: %v", cfg2)
	}
	if cfg2["theme"] != "tokyo-night" {
		t.Fatalf("theme dropped after disable: %v", cfg2)
	}
}

func TestOpenCodeBypassDisableNonexistentIsNoOp(t *testing.T) {
	withTempHome(t)
	if err := OpenCode().DisableBypass("ghost"); err != nil {
		t.Fatalf("disable on missing file should be no-op, got %v", err)
	}
	mode, err := OpenCode().BypassStatus("ghost")
	if err != nil {
		t.Fatalf("status on missing file should be no-op, got %v", err)
	}
	if mode != "" {
		t.Fatalf("status = %q, want empty", mode)
	}
}

// ---------------------------------------------------------------------------
// Codex
// ---------------------------------------------------------------------------

func TestCodexBypassEnableWritesConfigToml(t *testing.T) {
	home := withTempHome(t)
	if err := Codex().EnableBypass("any"); err != nil {
		t.Fatalf("enable bypass: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, `approval_policy = "never"`) {
		t.Fatalf("missing approval_policy line:\n%s", out)
	}
	if !strings.Contains(out, `sandbox_mode = "danger-full-access"`) {
		t.Fatalf("missing sandbox_mode line:\n%s", out)
	}

	mode, err := Codex().BypassStatus("any")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if mode != "danger-full-access/never" {
		t.Fatalf("status = %q, want danger-full-access/never", mode)
	}
}

func TestCodexBypassPreservesExistingTomlContent(t *testing.T) {
	home := withTempHome(t)
	cfgDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := `# user comment at top
model = "o4-mini"
approval_policy = "on-request"

[mcp_servers.foo]
command = "bar"
`
	cfgPath := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Codex().EnableBypass("any"); err != nil {
		t.Fatalf("enable bypass: %v", err)
	}

	got, _ := os.ReadFile(cfgPath)
	out := string(got)

	// Existing top-level approval_policy should be replaced in place.
	if strings.Count(out, "approval_policy =") != 1 {
		t.Fatalf("expected exactly one approval_policy line:\n%s", out)
	}
	if !strings.Contains(out, `approval_policy = "never"`) {
		t.Fatalf("approval_policy not flipped:\n%s", out)
	}
	// sandbox_mode is new; must be appended before the [mcp_servers.foo] section.
	if !strings.Contains(out, `sandbox_mode = "danger-full-access"`) {
		t.Fatalf("sandbox_mode missing:\n%s", out)
	}
	idxSandbox := strings.Index(out, "sandbox_mode")
	idxSection := strings.Index(out, "[mcp_servers.foo]")
	if idxSandbox < 0 || idxSection < 0 || idxSandbox > idxSection {
		t.Fatalf("sandbox_mode should be in top-level region (before section):\n%s", out)
	}
	// Pre-existing comment, model line, and section body must survive.
	if !strings.Contains(out, "# user comment at top") {
		t.Fatalf("dropped top-of-file comment:\n%s", out)
	}
	if !strings.Contains(out, `model = "o4-mini"`) {
		t.Fatalf("dropped existing model line:\n%s", out)
	}
	if !strings.Contains(out, "[mcp_servers.foo]") || !strings.Contains(out, `command = "bar"`) {
		t.Fatalf("dropped mcp_servers section:\n%s", out)
	}
}

func TestCodexBypassDisableStripsKeys(t *testing.T) {
	home := withTempHome(t)
	if err := Codex().EnableBypass("any"); err != nil {
		t.Fatal(err)
	}
	if err := Codex().DisableBypass("any"); err != nil {
		t.Fatalf("disable: %v", err)
	}

	cfgPath := filepath.Join(home, ".codex", "config.toml")
	// File should be removed when only-bypass content is stripped.
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("expected config.toml removed when only bypass keys were present, stat=%v", err)
	}

	mode, err := Codex().BypassStatus("any")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if mode != "" {
		t.Fatalf("status = %q, want empty", mode)
	}
}

func TestCodexBypassDisableKeepsFileWithOtherContent(t *testing.T) {
	home := withTempHome(t)
	cfgDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("model = \"o4-mini\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Codex().EnableBypass("any"); err != nil {
		t.Fatal(err)
	}
	if err := Codex().DisableBypass("any"); err != nil {
		t.Fatalf("disable: %v", err)
	}

	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config.toml should still exist with non-bypass keys: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, `model = "o4-mini"`) {
		t.Fatalf("dropped model key:\n%s", out)
	}
	if strings.Contains(out, "approval_policy") || strings.Contains(out, "sandbox_mode") {
		t.Fatalf("bypass keys should be stripped:\n%s", out)
	}
}

func TestCodexBypassStatusMissingFileEmpty(t *testing.T) {
	withTempHome(t)
	mode, err := Codex().BypassStatus("any")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if mode != "" {
		t.Fatalf("status = %q, want empty", mode)
	}
}

func TestCodexBypassStatusOnlyOneKeyIsNotBypassed(t *testing.T) {
	home := withTempHome(t)
	cfgDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	// approval_policy=never alone (no sandbox_mode) must not count as bypass.
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"),
		[]byte(`approval_policy = "never"`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mode, err := Codex().BypassStatus("any")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if mode != "" {
		t.Fatalf("status = %q, want empty (only one key)", mode)
	}
}
