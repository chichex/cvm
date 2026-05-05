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

	// EnableBypass puts the harness into "bypass permissions" mode for the
	// given active profile. Implementations decide whether the change is
	// persisted as a profile override (so it survives `cvm pull`) or written
	// directly to the live config dir.
	EnableBypass(profileName string) error
	// DisableBypass restores the harness to its default permissions mode.
	DisableBypass(profileName string) error
	// BypassStatus returns a short human-readable status of the current
	// bypass state ("" or "(default)" means "not bypassed").
	BypassStatus(profileName string) (string, error)
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
