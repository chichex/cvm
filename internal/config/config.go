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

// GlobalProfilesDir returns ~/.cvm/global/profiles
func GlobalProfilesDir() string {
	return filepath.Join(CvmHome(), "global", "profiles")
}

// GlobalVanillaDir returns ~/.cvm/global/vanilla
func GlobalVanillaDir() string {
	return filepath.Join(CvmHome(), "global", "vanilla")
}

// GlobalOverridesDir returns ~/.cvm/global/overrides
func GlobalOverridesDir() string {
	return filepath.Join(CvmHome(), "global", "overrides")
}

// OverrideDir returns the override directory for a profile name.
func OverrideDir(name string) string {
	return filepath.Join(GlobalOverridesDir(), name)
}

// StatePath returns ~/.cvm/state.json
func StatePath() string {
	return filepath.Join(CvmHome(), "state.json")
}
