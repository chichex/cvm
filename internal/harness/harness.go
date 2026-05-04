package harness

import (
	"os"

	"github.com/chichex/cvm/internal/config"
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
	TargetDir(scope config.Scope, projectPath string) string
	DefaultAssetDir(profileDir string) string
	ScaffoldAsset(kind, name string) (ScaffoldAsset, error)
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
	default:
		return nil, false
	}
}

func All() []Harness {
	return []Harness{Claude(), OpenCode()}
}
