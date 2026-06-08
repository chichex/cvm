package harness

type Harness interface {
	Name() string
	TargetDir() string
	ManagedDirItems() []string
	ProfileDiscoveryItems() []string
}

// ManagedProfileItems returns every item a profile manages for the harness.
// With the pure-symlink model these are exactly the managed dir items — there
// is no longer any externally-located (e.g. ~/.claude.json) managed path.
func ManagedProfileItems(h Harness) []string {
	return append([]string{}, h.ManagedDirItems()...)
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
