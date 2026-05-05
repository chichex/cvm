package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// homebrewPrefixes are paths managed by Homebrew on macOS and Linux.
var homebrewPrefixes = []string{
	"/opt/homebrew/",
	"/usr/local/Cellar/",
	"/usr/local/opt/",
	"/home/linuxbrew/",
}

// ResolveExecutable returns the absolute, symlink-resolved path of the running binary.
func ResolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable path: %w", err)
	}
	// Follow symlinks so we operate on the real file, not a wrapper.
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("evaluating symlinks for %q: %w", exe, err)
	}
	return real, nil
}

// IsHomebrew reports whether the given binary path is managed by Homebrew.
func IsHomebrew(binPath string) bool {
	for _, prefix := range homebrewPrefixes {
		if strings.HasPrefix(binPath, prefix) {
			return true
		}
	}
	return false
}

// CheckWritable verifies that the running binary's directory is writable
// (i.e., we can atomically rename a file there).
func CheckWritable(binPath string) error {
	dir := filepath.Dir(binPath)
	// Try creating a temp file in the same directory.
	tmp, err := os.CreateTemp(dir, ".cvm-upgrade-check-*")
	if err != nil {
		return fmt.Errorf("binary directory %q is not writable — reinstall via install.sh to a user-writable path (e.g. ~/.local/bin): %w", dir, err)
	}
	tmp.Close()
	os.Remove(tmp.Name())
	return nil
}
