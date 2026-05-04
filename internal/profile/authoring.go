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
	Kind    string
	Layer   string
	Path    string
	Created bool
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
	if !portable {
		h, ok := harness.ByName(harnessName)
		if !ok {
			return nil, fmt.Errorf("unknown harness %q", harnessName)
		}
		if err := validateHarnessScope(h, opts.Scope); err != nil {
			return nil, err
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

	assetDir, layer, changed, err := scaffoldAssetDir(profileDir, manifest, hadManifest, portable, harnessName)
	if err != nil {
		return nil, err
	}

	relPath, defaultContent, defaultMode := scaffoldFile(kind, opts.Name, harnessName, portable)
	targetPath := filepath.Join(assetDir, relPath)
	result := &ScaffoldedAsset{Kind: kind, Layer: layer, Path: targetPath}
	if _, err := os.Stat(targetPath); err == nil {
		if changed || !hadManifest {
			if err := SaveManifest(profileDir, manifest); err != nil {
				return nil, err
			}
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

func scaffoldAssetDir(profileDir string, manifest *Manifest, hadManifest bool, portable bool, harnessName string) (string, string, bool, error) {
	changed := false
	if portable {
		if _, ok := manifest.Assets["portable"]; !ok {
			manifest.Assets["portable"] = "portable"
			changed = true
		}
		if !hadManifest && manifest.SupportsHarness("claude") {
			if _, ok := manifest.Assets["claude"]; !ok {
				manifest.Assets["claude"] = defaultHarnessAssetDir(profileDir, "claude")
				changed = true
			}
		}
		assetDir, err := assetDirFromRaw(profileDir, manifest.Assets["portable"])
		return assetDir, "portable", changed, err
	}

	if !manifest.SupportsHarness(harnessName) {
		manifest.Harnesses = append(manifest.Harnesses, harnessName)
		changed = true
	}
	if _, ok := manifest.Assets[harnessName]; !ok {
		manifest.Assets[harnessName] = defaultHarnessAssetDir(profileDir, harnessName)
		changed = true
	}
	assetDir, err := assetDirFromRaw(profileDir, manifest.Assets[harnessName])
	return assetDir, harnessName, changed, err
}

func defaultHarnessAssetDir(profileDir string, harnessName string) string {
	if harnessName == "claude" && legacyClaudeRootHasAssets(profileDir) {
		return "."
	}
	return harnessName
}

func legacyClaudeRootHasAssets(profileDir string) bool {
	for _, item := range harness.Claude().ProfileDiscoveryItems() {
		if _, err := os.Stat(filepath.Join(profileDir, item)); err == nil {
			return true
		}
	}
	return false
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

func scaffoldFile(kind, name, harnessName string, portable bool) (string, string, os.FileMode) {
	if portable {
		switch kind {
		case "instructions":
			return "instructions.md", "# Profile Instructions\n\n", 0644
		case "skill":
			return filepath.Join("skills", name+".md"), "---\ndescription: TODO\n---\n\n", 0644
		case "agent":
			return filepath.Join("agents", name+".md"), "# " + name + "\n\n", 0644
		}
	}

	switch kind {
	case "instructions":
		if h, ok := harness.ByName(harnessName); ok {
			return h.MarkdownInstructionsFile(), "# Profile Instructions\n\n", 0644
		}
	case "skill":
		return filepath.Join("skills", name, "SKILL.md"), "---\ndescription: TODO\n---\n\n", 0644
	case "agent":
		return filepath.Join("agents", name+".md"), "# " + name + "\n\n", 0644
	case "hook":
		return filepath.Join("hooks", name+".sh"), "#!/usr/bin/env bash\nset -euo pipefail\n\n", 0755
	}
	return name, "", 0644
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
