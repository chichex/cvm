package config

import (
	"os"
	"path/filepath"
)

type Scope string

const (
	ScopeGlobal Scope = "global"
	ScopeLocal  Scope = "local"
)

const (
	CvmDirName    = ".cvm"
	ClaudeDirName = ".claude"
)

// ManagedClaudeDirItems are the config files/dirs that cvm manages inside
// ~/.claude/ or .claude/. Additional MCP config files live outside those
// directories and are handled per-scope.
// Everything else (sessions/, cache/, history.jsonl, etc.) is runtime and never touched.
var ManagedClaudeDirItems = []string{
	"CLAUDE.md",
	"settings.json",
	"settings.local.json",
	"keybindings.json",
	"statusline-command.sh",
	"commands",
	"skills",
	"agents",
	"hooks",
	"rules",
	"output-styles",
	"teams",
}

func ManagedProfileItems(scope Scope) []string {
	items := append([]string{}, ManagedClaudeDirItems...)
	if scope == ScopeGlobal {
		items = append(items, ".claude.json")
	} else {
		items = append(items, ".mcp.json")
	}
	return items
}

func ProfileDiscoveryItems() []string {
	items := append([]string{}, ManagedClaudeDirItems...)
	items = append(items, ".claude.json", ".mcp.json")
	return items
}

// CvmHome returns ~/.cvm
func CvmHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, CvmDirName)
}

// ClaudeHome returns ~/.claude
func ClaudeHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ClaudeDirName)
}

// ClaudeUserConfigPath returns ~/.claude.json
func ClaudeUserConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude.json")
}

// ProjectMCPConfigPath returns <projectPath>/.mcp.json
func ProjectMCPConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".mcp.json")
}

// GlobalProfilesDir returns ~/.cvm/global/profiles
func GlobalProfilesDir() string {
	return filepath.Join(CvmHome(), "global", "profiles")
}

// LocalProfilesDir returns ~/.cvm/local/profiles
func LocalProfilesDir() string {
	return filepath.Join(CvmHome(), "local", "profiles")
}

// GlobalVanillaDir returns ~/.cvm/global/vanilla
func GlobalVanillaDir() string {
	return filepath.Join(CvmHome(), "global", "vanilla")
}

// LocalVanillaDir returns ~/.cvm/local/vanilla/<project-hash>
func LocalVanillaDir(projectPath string) string {
	return filepath.Join(CvmHome(), "local", "vanilla", hashPath(projectPath))
}

// GlobalKBDir returns ~/.cvm/global/kb
func GlobalKBDir() string {
	return filepath.Join(CvmHome(), "global", "kb")
}

// LocalKBDir returns ~/.cvm/local/kb/<project-hash>
func LocalKBDir(projectPath string) string {
	return filepath.Join(CvmHome(), "local", "kb", hashPath(projectPath))
}

// StatePath returns ~/.cvm/state.json
func StatePath() string {
	return filepath.Join(CvmHome(), "state.json")
}

// ProjectClaudeDir returns <projectPath>/.claude
func ProjectClaudeDir(projectPath string) string {
	return filepath.Join(projectPath, ClaudeDirName)
}

// hashPath creates a filesystem-safe hash of a path for per-project storage
func hashPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	// Simple hash: replace / with - and trim leading -
	safe := ""
	for _, c := range abs {
		if c == '/' || c == '\\' {
			safe += "-"
		} else {
			safe += string(c)
		}
	}
	if len(safe) > 0 && safe[0] == '-' {
		safe = safe[1:]
	}
	// Truncate to avoid overly long paths
	if len(safe) > 100 {
		safe = safe[len(safe)-100:]
	}
	return safe
}
