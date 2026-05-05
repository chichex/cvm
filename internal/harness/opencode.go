package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
)

// opencodeBypassValue is the global "allow" shorthand documented at
// https://opencode.ai/docs/permissions/. Setting permission to the literal
// string "allow" disables every approval prompt.
const opencodeBypassValue = "allow"

type opencodeHarness struct{}

var managedOpenCodeDirItems = []string{
	"AGENTS.md",
	// Only mcpServers inside opencode.json are managed; other user config is preserved.
	"opencode.json",
	"skills",
	"agents",
	"commands",
}

func OpenCode() Harness {
	return opencodeHarness{}
}

func (opencodeHarness) Name() string {
	return "opencode"
}

func (opencodeHarness) TargetDir() string {
	if dir := os.Getenv("OPENCODE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode")
}

func (h opencodeHarness) DefaultAssetDir(profileDir string) string {
	return h.Name()
}

func (h opencodeHarness) ScaffoldAsset(kind, name string) (ScaffoldAsset, error) {
	switch kind {
	case "instructions":
		return ScaffoldAsset{ProfilePath: h.MarkdownInstructionsFile(), Content: "# Profile Instructions\n\n", Mode: 0644}, nil
	case "skill":
		return ScaffoldAsset{ProfilePath: filepath.Join("skills", name, "SKILL.md"), Content: "---\ndescription: TODO\n---\n\n", Mode: 0644}, nil
	case "agent":
		return ScaffoldAsset{ProfilePath: filepath.Join("agents", name+".md"), Content: "# " + name + "\n\n", Mode: 0644}, nil
	default:
		return ScaffoldAsset{}, fmt.Errorf("opencode does not support %s scaffolding", kind)
	}
}

func (opencodeHarness) ManagedDirItems() []string {
	return append([]string{}, managedOpenCodeDirItems...)
}

func (opencodeHarness) ExternalManagedPath() (ManagedPath, bool) {
	// OpenCode keeps config inside TargetDir, unlike Claude's external MCP files.
	return ManagedPath{}, false
}

func (h opencodeHarness) ProfileDiscoveryItems() []string {
	return h.ManagedDirItems()
}

func (opencodeHarness) MarkdownInstructionsFile() string {
	return "AGENTS.md"
}

func (opencodeHarness) SupportsPortableSkills() bool {
	return true
}

func (opencodeHarness) SupportsPortableAgents() bool {
	return true
}

func (opencodeHarness) IsUserMCPPath(profilePath string) bool {
	return profilePath == "opencode.json"
}

func (opencodeHarness) IsMCPPath(profilePath string) bool {
	return profilePath == "opencode.json"
}

// EnableBypass writes the OpenCode bypass permission block to the profile's
// override opencode.json. opencode.json is a managed item, so reapply merges
// the override into ~/.config/opencode/opencode.json.
func (opencodeHarness) EnableBypass(profileName string) error {
	overrideDir := config.OverrideDir(profileName)
	overridePath := filepath.Join(overrideDir, "opencode.json")
	cfg, err := readOpenCodeJSON(overridePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfg["permission"] = opencodeBypassValue
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		return err
	}
	return writeOpenCodeJSON(overridePath, cfg)
}

// DisableBypass removes the bypass permission key from the profile's override
// opencode.json (and removes the file entirely if it becomes empty).
func (opencodeHarness) DisableBypass(profileName string) error {
	overridePath := filepath.Join(config.OverrideDir(profileName), "opencode.json")
	cfg, err := readOpenCodeJSON(overridePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if _, ok := cfg["permission"]; !ok {
		return nil
	}
	delete(cfg, "permission")
	if len(cfg) == 0 {
		if err := os.Remove(overridePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeOpenCodeJSON(overridePath, cfg)
}

// BypassStatus reports the current permission value persisted in the
// profile's override opencode.json.
func (opencodeHarness) BypassStatus(profileName string) (string, error) {
	overridePath := filepath.Join(config.OverrideDir(profileName), "opencode.json")
	cfg, err := readOpenCodeJSON(overridePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	switch v := cfg["permission"].(type) {
	case string:
		return v, nil
	case map[string]any:
		// Pretty-print object form as e.g. "{bash: allow, edit: ask}".
		// Status only needs to convey "non-default", not full content.
		if len(v) == 0 {
			return "", nil
		}
		return "custom", nil
	default:
		return "", nil
	}
}

func readOpenCodeJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := map[string]any{}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func writeOpenCodeJSON(path string, cfg map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
