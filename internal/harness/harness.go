package harness

import (
	"os"
)

type ManagedPath struct {
	ProfilePath string
	LivePath    string
}

type ScaffoldAsset struct {
	ProfilePath string
	Content     string
	Mode        os.FileMode
}

type Harness interface {
	Name() string
	TargetDir() string
	DefaultAssetDir(profileDir string) string
	ScaffoldAsset(kind, name string) (ScaffoldAsset, error)
	ManagedDirItems() []string
	ExternalManagedPath() (ManagedPath, bool)
	ProfileDiscoveryItems() []string
	MarkdownInstructionsFile() string
	SupportsPortableSkills() bool
	SupportsPortableAgents() bool
	IsUserMCPPath(profilePath string) bool
	IsMCPPath(profilePath string) bool
}

func ManagedProfileItems(h Harness) []string {
	items := append([]string{}, h.ManagedDirItems()...)
	if extra, ok := h.ExternalManagedPath(); ok {
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
