package upgrade

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const tarballURLTemplate = "https://github.com/chichex/cvm/releases/download/%s/cvm_%s_%s_%s.tar.gz"

// TarballURL builds the download URL for a given version, OS and architecture.
// version must already be normalized (with leading "v").
func TarballURL(version, goos, goarch string) (string, error) {
	// Map runtime values to goreleaser naming conventions.
	osName, archName, err := platformNames(goos, goarch)
	if err != nil {
		return "", err
	}
	// Strip leading "v" for the filename segment (goreleaser uses the bare version).
	bare := strings.TrimPrefix(version, "v")
	return fmt.Sprintf(tarballURLTemplate, version, bare, osName, archName), nil
}

func platformNames(goos, goarch string) (string, string, error) {
	var osName string
	switch goos {
	case "darwin":
		osName = "Darwin"
	case "linux":
		osName = "Linux"
	default:
		return "", "", fmt.Errorf("unsupported platform: %s/%s — cvm upgrade is only available on darwin and linux", goos, goarch)
	}

	var archName string
	switch goarch {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	default:
		return "", "", fmt.Errorf("unsupported architecture: %s — cvm upgrade supports amd64 and arm64", goarch)
	}

	return osName, archName, nil
}

// DownloadAndInstall downloads the tarball for the given version, extracts the
// "cvm" binary, optionally codesigns it on macOS, and atomically renames it
// over binPath.
//
// If any step before the rename fails, the .new file is removed and the
// original binary is left untouched.
func DownloadAndInstall(version, binPath string) error {
	url, err := TarballURL(version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	newPath := binPath + ".new"
	// Ensure cleanup on any failure before rename.
	success := false
	defer func() {
		if !success {
			os.Remove(newPath)
		}
	}()

	if err := downloadTarball(url, newPath); err != nil {
		return err
	}

	// On macOS, ad-hoc codesign the new binary so Gatekeeper accepts it.
	// Tolerate failure if codesign is not available (same policy as install.sh).
	if runtime.GOOS == "darwin" {
		codesignBinary(newPath)
	}

	// Atomic rename: on same filesystem this is instantaneous and safe even
	// while the current binary is running (inode swap — old inode kept alive
	// by the running process's open fd).
	if err := os.Rename(newPath, binPath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	success = true
	return nil
}

// downloadTarball fetches the tarball from url, extracts the "cvm" entry, and
// writes it as an executable to dest.
func downloadTarball(url, dest string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d for %s", resp.StatusCode, url)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("opening gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// We only want the top-level "cvm" binary.
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != "cvm" {
			continue
		}

		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("creating %s: %w", dest, err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return fmt.Errorf("writing binary: %w", err)
		}
		if err := out.Close(); err != nil {
			return fmt.Errorf("closing binary file: %w", err)
		}
		return nil
	}

	return fmt.Errorf("binary \"cvm\" not found inside tarball at %s", url)
}

// codesignBinary runs `codesign --sign - --force` on path.
// Errors are silently ignored (codesign may not be available in all envs).
func codesignBinary(path string) {
	cmd := exec.Command("codesign", "--sign", "-", "--force", path)
	_ = cmd.Run()
}
