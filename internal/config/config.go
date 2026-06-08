package config

import (
	"os"
	"path/filepath"
)

const (
	CvmDirName    = ".cvm"
	ClaudeDirName = ".claude"
)

// CvmHome returns ~/.cvm
func CvmHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, CvmDirName)
}

// GlobalProfilesDir returns ~/.cvm/profiles, where cloned profiles live as
// real git working repos.
func GlobalProfilesDir() string {
	return filepath.Join(CvmHome(), "profiles")
}

// VanillaDir returns ~/.cvm/vanilla, where pre-cvm (real, non-symlink) files
// are stashed once before a profile symlink takes their place.
func VanillaDir() string {
	return filepath.Join(CvmHome(), "vanilla")
}

// VanillaDirForHarness returns ~/.cvm/vanilla/<harness>.
func VanillaDirForHarness(harnessName string) string {
	return filepath.Join(VanillaDir(), harnessName)
}

// StatePath returns ~/.cvm/state.json
func StatePath() string {
	return filepath.Join(CvmHome(), "state.json")
}
