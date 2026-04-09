package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const releasesAPI = "https://api.github.com/repos/chichex/cvm/releases/latest"

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade cvm to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		current := Version

		// Fetch latest version
		fmt.Println("Checking for updates...")
		latest, downloadURL, err := getLatestRelease()
		if err != nil {
			return fmt.Errorf("checking latest version: %w", err)
		}

		if current == latest {
			fmt.Printf("Already on latest version (%s)\n", current)
			return nil
		}

		fmt.Printf("Upgrading: %s → %s\n", current, latest)

		// Check if installed via brew
		if isBrewInstall() {
			fmt.Println("Detected brew install, upgrading...")
			// Uninstall first so we can re-clone the tap
			run("brew", "uninstall", "cvm")
			run("brew", "untap", "chichex/tap")
			if err := run("brew", "install", "chichex/tap/cvm"); err != nil {
				return fmt.Errorf("brew install failed: %w", err)
			}
			return nil
		}

		// Direct binary upgrade
		if downloadURL == "" {
			return fmt.Errorf("could not find download URL for %s/%s", runtime.GOOS, runtime.GOARCH)
		}

		fmt.Printf("Downloading %s...\n", downloadURL)
		binPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("finding current binary: %w", err)
		}

		tmpDir, err := os.MkdirTemp("", "cvm-upgrade-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		// Download and extract
		tarPath := tmpDir + "/cvm.tar.gz"
		dlCmd := exec.Command("curl", "-sL", downloadURL, "-o", tarPath)
		if err := dlCmd.Run(); err != nil {
			return fmt.Errorf("downloading: %w", err)
		}

		exCmd := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir)
		if err := exCmd.Run(); err != nil {
			return fmt.Errorf("extracting: %w", err)
		}

		newBin := tmpDir + "/cvm"
		if _, err := os.Stat(newBin); os.IsNotExist(err) {
			return fmt.Errorf("binary not found in archive")
		}

		// Replace binary
		cpCmd := exec.Command("cp", newBin, binPath)
		if err := cpCmd.Run(); err != nil {
			// Try with sudo
			fmt.Println("Need sudo to replace binary...")
			cpCmd = exec.Command("sudo", "cp", newBin, binPath)
			cpCmd.Stdin = os.Stdin
			cpCmd.Stdout = os.Stdout
			cpCmd.Stderr = os.Stderr
			if err := cpCmd.Run(); err != nil {
				return fmt.Errorf("replacing binary: %w", err)
			}
		}

		fmt.Printf("Upgraded to %s\n", latest)
		return nil
	},
}

func getLatestRelease() (version, downloadURL string, err error) {
	resp, err := http.Get(releasesAPI)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &release); err != nil {
		return "", "", err
	}

	version = strings.TrimPrefix(release.TagName, "v")

	// Find matching asset
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	want := fmt.Sprintf("cvm_%s_%s_%s.tar.gz", version, goos, goarch)

	for _, a := range release.Assets {
		if a.Name == want {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}

	return version, downloadURL, nil
}

func run(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func isBrewInstall() bool {
	binPath, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(binPath, "homebrew") || strings.Contains(binPath, "Cellar")
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}
