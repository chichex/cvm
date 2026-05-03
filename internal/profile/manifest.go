package profile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chichex/cvm/internal/harness"
)

const manifestFileName = "cvm.profile.toml"

// LoadManifest intentionally supports only the small manifest subset cvm owns
// today. Replace this with a real TOML parser before expanding the schema.
type Manifest struct {
	Name      string
	Harnesses []string
	Assets    map[string]string
}

func LoadManifest(profileDir string) (*Manifest, error) {
	manifest := &Manifest{
		Name:      filepath.Base(profileDir),
		Harnesses: []string{"claude"},
		Assets:    map[string]string{},
	}

	data, err := os.ReadFile(filepath.Join(profileDir, manifestFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return manifest, nil
		}
		return nil, err
	}

	section := ""
	parsedHarnesses := false
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid manifest line %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch section {
		case "":
			switch key {
			case "name":
				parsed, err := parseStringValue(value)
				if err != nil {
					return nil, fmt.Errorf("parsing name: %w", err)
				}
				manifest.Name = parsed
			case "harnesses":
				parsed, err := parseStringListValue(value)
				if err != nil {
					return nil, fmt.Errorf("parsing harnesses: %w", err)
				}
				parsedHarnesses = true
				if len(parsed) == 0 {
					return nil, fmt.Errorf("harnesses must not be empty")
				}
				manifest.Harnesses = parsed
			}
		case "assets":
			parsed, err := parseStringValue(value)
			if err != nil {
				return nil, fmt.Errorf("parsing asset %q: %w", key, err)
			}
			manifest.Assets[key] = parsed
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if !parsedHarnesses && len(manifest.Harnesses) == 0 {
		manifest.Harnesses = []string{"claude"}
	}
	return manifest, nil
}

func (m *Manifest) SupportsHarness(name string) bool {
	for _, harnessName := range m.Harnesses {
		if harnessName == name {
			return true
		}
	}
	return false
}

func (m *Manifest) AssetDir(profileDir string, h harness.Harness) (string, error) {
	raw := strings.TrimSpace(m.Assets[h.Name()])
	if raw == "" || raw == "." {
		return profileDir, nil
	}

	clean := filepath.Clean(raw)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("asset dir %q must be relative", raw)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("asset dir %q escapes the profile root", raw)
	}
	return filepath.Join(profileDir, clean), nil
}

func HasManifest(profileDir string) bool {
	_, err := os.Stat(filepath.Join(profileDir, manifestFileName))
	return err == nil
}

func LooksLikeProfileDir(profileDir string) bool {
	if HasManifest(profileDir) {
		manifest, err := LoadManifest(profileDir)
		if err != nil {
			return false
		}
		for _, harnessName := range manifest.Harnesses {
			h, ok := harness.ByName(harnessName)
			if !ok {
				continue
			}
			assetDir, err := manifest.AssetDir(profileDir, h)
			if err != nil {
				return false
			}
			for _, item := range h.ProfileDiscoveryItems() {
				if _, err := os.Stat(filepath.Join(assetDir, item)); err == nil {
					return true
				}
			}
		}
		return false
	}

	claude := harness.Claude()
	for _, item := range claude.ProfileDiscoveryItems() {
		if _, err := os.Stat(filepath.Join(profileDir, item)); err == nil {
			return true
		}
	}
	return false
}

func stripComment(line string) string {
	inString := false
	for i, r := range line {
		switch r {
		case '"':
			inString = !inString
		case '#':
			if !inString {
				return line[:i]
			}
		}
	}
	return line
}

func parseStringValue(value string) (string, error) {
	parsed, err := strconv.Unquote(value)
	if err != nil {
		return "", fmt.Errorf("expected quoted string, got %q", value)
	}
	return parsed, nil
}

func parseStringListValue(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("expected array, got %q", value)
	}
	body := strings.TrimSpace(value[1 : len(value)-1])
	if body == "" {
		return nil, nil
	}

	parts := strings.Split(body, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		parsed, err := parseStringValue(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		items = append(items, parsed)
	}
	return items, nil
}
