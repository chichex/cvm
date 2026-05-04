package harness

import "github.com/chichex/cvm/internal/config"

type ManagedPath struct {
	ProfilePath string
	LivePath    string
}

type Harness interface {
	Name() string
	SupportsScope(scope config.Scope) bool
	TargetDir(scope config.Scope, projectPath string) string
	ManagedDirItems() []string
	ExternalManagedPath(scope config.Scope, projectPath string) (ManagedPath, bool)
	ProfileDiscoveryItems() []string
	MarkdownInstructionsFile() string
	IsUserMCPPath(profilePath string) bool
	IsMCPPath(profilePath string) bool
}

func ManagedProfileItems(h Harness, scope config.Scope, projectPath string) []string {
	items := append([]string{}, h.ManagedDirItems()...)
	if extra, ok := h.ExternalManagedPath(scope, projectPath); ok {
		items = append(items, extra.ProfilePath)
	}
	return items
}

func ByName(name string) (Harness, bool) {
	switch name {
	case "claude":
		return Claude(), true
	case "opencode":
		return OpenCode(), true
	case "codex":
		return Codex(), true
	default:
		return nil, false
	}
}

func All() []Harness {
	return []Harness{Claude(), OpenCode(), Codex()}
}
