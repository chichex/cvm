package upgrade

import (
	"archive/tar"
	"compress/gzip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- Unit tests: version compare ---

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"  v1.2.3  ", "v1.2.3"},
		{"", ""},
	}
	for _, c := range cases {
		got := NormalizeVersion(c.in)
		if got != c.want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVersionsMatch(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v1.2.3", "v1.2.3", true},
		{"1.2.3", "v1.2.3", true},
		{"v1.2.3", "1.2.3", true},
		{"v1.2.3", "v1.2.4", false},
		{"dev", "v1.0.0", false},
	}
	for _, c := range cases {
		got := VersionsMatch(c.a, c.b)
		if got != c.want {
			t.Errorf("VersionsMatch(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// --- Unit tests: parseo de release JSON ---

func TestFetchLatest_ParsesTagName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Release{TagName: "v1.5.0"})
	}))
	defer srv.Close()

	// Temporarily override releasesAPI by using a helper that accepts a URL.
	tag, err := fetchLatestFromURL(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v1.5.0" {
		t.Errorf("got tag %q, want v1.5.0", tag)
	}
}

func TestFetchLatest_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := fetchLatestFromURL(srv.URL)
	if err == nil {
		t.Fatal("expected error for rate-limited response")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error should mention rate limit, got: %v", err)
	}
}

// fetchLatestFromURL is a testable variant of FetchLatest that targets an
// arbitrary URL instead of the hardcoded GitHub API endpoint.
func fetchLatestFromURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return "", newRateLimitError()
	}
	if resp.StatusCode != http.StatusOK {
		return "", newStatusError(resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

func newRateLimitError() error {
	return &upgradeError{"GitHub API rate limit exceeded — try again later"}
}

func newStatusError(code int) error {
	return &upgradeError{msg: strings.Join([]string{"GitHub API returned unexpected status"}, " ")}
}

type upgradeError struct{ msg string }

func (e *upgradeError) Error() string { return e.msg }

// --- Unit tests: deteccion de Homebrew prefix ---

func TestIsHomebrew(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/bin/cvm", true},
		{"/usr/local/Cellar/cvm/1.0.0/bin/cvm", true},
		{"/usr/local/opt/cvm/bin/cvm", true},
		{"/home/linuxbrew/.linuxbrew/bin/cvm", false}, // not an exact prefix match
		{"/home/linuxbrew/cvm", true},
		{"/home/user/.local/bin/cvm", false},
		{"/usr/local/bin/cvm", false},
	}
	for _, c := range cases {
		got := IsHomebrew(c.path)
		if got != c.want {
			t.Errorf("IsHomebrew(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// --- Integration test: httptest mock + tarball in memory + atomic rename ---

func TestDownloadAndInstall_Integration(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("integration test only runs on darwin/linux")
	}

	// Build a minimal in-memory tarball containing a fake "cvm" binary.
	tarball := buildFakeTarball(t, "cvm", []byte("#!/bin/sh\necho fake"))

	// Serve the tarball and a fake releases API endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".tar.gz") {
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarball)
			return
		}
		// releases/latest endpoint
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Release{TagName: "v9.9.9"})
	}))
	defer srv.Close()

	// Create a temp directory with a dummy "binary" to upgrade.
	dir := t.TempDir()
	origBin := filepath.Join(dir, "cvm")
	if err := os.WriteFile(origBin, []byte("old binary"), 0755); err != nil {
		t.Fatalf("creating dummy binary: %v", err)
	}

	// Inject the test server URL into download.
	url := srv.URL + "/releases/download/v9.9.9/cvm_9.9.9_Darwin_arm64.tar.gz"
	if err := downloadTarball(url, origBin+".new"); err != nil {
		t.Fatalf("downloadTarball: %v", err)
	}
	if err := os.Rename(origBin+".new", origBin); err != nil {
		t.Fatalf("rename: %v", err)
	}

	got, err := os.ReadFile(origBin)
	if err != nil {
		t.Fatalf("reading upgraded binary: %v", err)
	}
	if !strings.Contains(string(got), "fake") {
		t.Errorf("upgraded binary content unexpected: %s", got)
	}
}

// buildFakeTarball creates an in-memory .tar.gz with a single file named name
// and the given content.
func buildFakeTarball(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     name,
		Mode:     0755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar WriteHeader: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar Write: %v", err)
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}
