package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/harness"
)

type ScaffoldAssetOptions struct {
	Scope       config.Scope
	ProfileName string
	ProjectPath string
	Kind        string
	Name        string
	HarnessName string
	FromFile    string
}

type ScaffoldedAsset struct {
	Kind            string
	Layer           string
	Path            string
	Created         bool
	ManifestCreated bool
	Portable        bool
}

func ScaffoldAsset(opts ScaffoldAssetOptions) (*ScaffoldedAsset, error) {
	kind := strings.ToLower(strings.TrimSpace(opts.Kind))
	if err := validateScaffoldRequest(kind, opts.Name); err != nil {
		return nil, err
	}
	if opts.ProfileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	profileDir := ProfileDir(opts.Scope, opts.ProfileName)
	info, err := os.Stat(profileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("profile %q not found", opts.ProfileName)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("profile %q is not a directory", opts.ProfileName)
	}

	harnessName := strings.TrimSpace(opts.HarnessName)
	if kind == "hook" && harnessName == "" {
		return nil, fmt.Errorf("hook assets are harness-specific; pass --harness <name>")
	}

	portable := harnessName == "" && kind != "hook"
	var selectedHarness harness.Harness
	if !portable {
		var ok bool
		selectedHarness, ok = harness.ByName(harnessName)
		if !ok {
			return nil, fmt.Errorf("unknown harness %q", harnessName)
		}
	}

	hadManifest := HasManifest(profileDir)
	manifest, err := LoadManifest(profileDir)
	if err != nil {
		return nil, err
	}
	if manifest.Assets == nil {
		manifest.Assets = map[string]string{}
	}
	if manifest.Name == "" {
		manifest.Name = opts.ProfileName
	}

	assetDir, layer, changed, err := scaffoldAssetDir(profileDir, manifest, hadManifest, portable, selectedHarness)
	if err != nil {
		return nil, err
	}

	relPath, defaultContent, defaultMode, err := scaffoldFile(kind, opts.Name, portable, selectedHarness)
	if err != nil {
		return nil, err
	}
	targetPath := filepath.Join(assetDir, relPath)
	result := &ScaffoldedAsset{Kind: kind, Layer: layer, Path: targetPath, Portable: portable}
	if _, err := os.Stat(targetPath); err == nil {
		if changed || !hadManifest {
			if err := SaveManifest(profileDir, manifest); err != nil {
				return nil, err
			}
			result.ManifestCreated = !hadManifest
		}
		return result, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	content := []byte(defaultContent)
	mode := defaultMode
	if opts.FromFile != "" {
		content, mode, err = readScaffoldSource(opts.FromFile, defaultMode, kind)
		if err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(targetPath, content, mode); err != nil {
		return nil, err
	}
	if changed || !hadManifest {
		if err := SaveManifest(profileDir, manifest); err != nil {
			return nil, err
		}
		result.ManifestCreated = !hadManifest
	}
	result.Created = true
	return result, nil
}

func validateScaffoldRequest(kind, name string) error {
	switch kind {
	case "instructions":
		if name != "" {
			return fmt.Errorf("instructions does not take a name")
		}
		return nil
	case "skill", "agent", "hook":
		return validateAssetName(name)
	default:
		return fmt.Errorf("unknown type %q: must be one of skill, hook, agent, instructions", kind)
	}
}

func validateAssetName(name string) error {
	if name == "" {
		return fmt.Errorf("asset name is required")
	}
	if name == "." || name == ".." || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid name %q: must not contain path separators or '..'", name)
	}
	return nil
}

func scaffoldAssetDir(profileDir string, manifest *Manifest, hadManifest bool, portable bool, h harness.Harness) (string, string, bool, error) {
	changed := false
	if portable {
		if _, ok := manifest.Assets["portable"]; !ok {
			manifest.Assets["portable"] = "portable"
			changed = true
		}
		if !hadManifest && manifest.SupportsHarness("claude") {
			if _, ok := manifest.Assets["claude"]; !ok {
				claude, _ := harness.ByName("claude")
				manifest.Assets["claude"] = claude.DefaultAssetDir(profileDir)
				changed = true
			}
		}
		assetDir, err := assetDirFromRaw(profileDir, manifest.Assets["portable"])
		return assetDir, "portable", changed, err
	}

	if !manifest.SupportsHarness(h.Name()) {
		manifest.Harnesses = append(manifest.Harnesses, h.Name())
		changed = true
	}
	if _, ok := manifest.Assets[h.Name()]; !ok {
		manifest.Assets[h.Name()] = h.DefaultAssetDir(profileDir)
		changed = true
	}
	assetDir, err := assetDirFromRaw(profileDir, manifest.Assets[h.Name()])
	return assetDir, h.Name(), changed, err
}

func assetDirFromRaw(profileDir, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
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

func scaffoldFile(kind, name string, portable bool, h harness.Harness) (string, string, os.FileMode, error) {
	if portable {
		switch kind {
		case "instructions":
			return "instructions.md", "# Profile Instructions\n\n", 0644, nil
		case "skill":
			return filepath.Join("skills", name+".md"), "---\ndescription: TODO\n---\n\n", 0644, nil
		case "agent":
			return filepath.Join("agents", name+".md"), "# " + name + "\n\n", 0644, nil
		}
	}

	asset, err := h.ScaffoldAsset(kind, name)
	if err != nil {
		return "", "", 0, err
	}
	return asset.ProfilePath, asset.Content, asset.Mode, nil
}

func readScaffoldSource(path string, defaultMode os.FileMode, kind string) ([]byte, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("reading --from-file %q: %w", path, err)
	}
	if info.IsDir() {
		return nil, 0, fmt.Errorf("--from-file %q is a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("reading --from-file %q: %w", path, err)
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = defaultMode
	}
	if kind == "hook" {
		mode = 0755
	}
	return data, mode, nil
}
