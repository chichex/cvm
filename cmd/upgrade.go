package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/upgrade"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade cvm to the latest stable release",
	Long: `Upgrade cvm to the latest stable release published on GitHub Releases.

Fetches the latest version tag, compares it to the running version, and if a
newer release exists downloads the appropriate tarball, extracts the binary, and
atomically replaces the current executable.

If cvm was installed via Homebrew, upgrade is delegated to Homebrew by
invoking: brew upgrade chichex/tap/cvm`,
	RunE: runUpgrade,
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	current := Version

	// Resolve binary path and perform preflight checks.
	binPath, err := upgrade.ResolveExecutable()
	if err != nil {
		return err
	}

	if upgrade.IsHomebrew(binPath) {
		fmt.Println("cvm was installed via Homebrew — delegating to brew...")
		return upgrade.RunHomebrewUpgrade()
	}

	if err := upgrade.CheckWritable(binPath); err != nil {
		return err
	}

	// Fetch latest release from GitHub.
	latest, err := upgrade.FetchLatest("")
	if err != nil {
		return err
	}

	currentNorm := upgrade.NormalizeVersion(current)
	latestNorm := upgrade.NormalizeVersion(latest)

	fmt.Printf("current: %s\n", currentNorm)
	fmt.Printf("latest:  %s\n", latestNorm)

	// "dev" build policy: allow upgrade but warn explicitly.
	if current == "dev" {
		fmt.Printf("warning: running dev build, upgrading to %s\n", latestNorm)
	} else if upgrade.VersionsMatch(current, latest) {
		fmt.Printf("Already on latest version (%s)\n", latestNorm)
		return nil
	}

	fmt.Printf("Downloading %s...\n", latestNorm)
	if err := upgrade.DownloadAndInstall(latestNorm, binPath); err != nil {
		return err
	}

	fmt.Printf("Upgraded to %s\n", latestNorm)
	return nil
}
