// Package upgrade implements the cvm self-upgrade logic.
package upgrade

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const releasesAPI = "https://api.github.com/repos/chichex/cvm/releases/latest"

// Release holds the fields we care about from the GitHub Releases API.
type Release struct {
	TagName string `json:"tag_name"`
}

// FetchLatest queries the GitHub Releases API and returns the latest release tag.
// Returns an error if the request fails or if rate-limited.
func FetchLatest() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", releasesAPI, nil)
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cvm-upgrade")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("GitHub API rate limit exceeded — try again later")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned unexpected status %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", fmt.Errorf("parsing release JSON: %w", err)
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("release has no tag_name")
	}
	return rel.TagName, nil
}

// NormalizeVersion ensures a version string is prefixed with "v".
// "1.2.3" → "v1.2.3", "v1.2.3" → "v1.2.3".
func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// VersionsMatch reports whether current and latest represent the same version.
// Both inputs are normalized before comparison.
func VersionsMatch(current, latest string) bool {
	return NormalizeVersion(current) == NormalizeVersion(latest)
}
