package upgrade

import (
	"fmt"
	"os"
	"os/exec"
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

// RunHomebrewUpgrade delegates the upgrade to Homebrew by invoking
// `brew upgrade chichex/tap/cvm`, streaming output to the current process so
// the user sees brew's progress and errors directly.
func RunHomebrewUpgrade() error {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return fmt.Errorf("cvm was installed via Homebrew but `brew` is not in PATH — install Homebrew or run manually: brew upgrade chichex/tap/cvm")
	}
	cmd := exec.Command(brew, "upgrade", "chichex/tap/cvm")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew upgrade chichex/tap/cvm failed: %w", err)
	}
	return nil
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
