package profile

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/harness"
	"github.com/chichex/cvm/internal/state"
)

type ProfileInfo struct {
	Name   string
	Active bool
	Items  int
}

type Inventory struct {
	Name       string
	Path       string
	Exists     bool
	Files      []string
	Dirs       map[string][]string
	MCPServers []string
}

func profilesDir() string {
	return config.GlobalProfilesDir()
}

func targetDir() string {
	return defaultHarness().TargetDir()
}

func targetDirForHarness(h harness.Harness) string {
	return h.TargetDir()
}

func ProfileDir(name string) string {
	return filepath.Join(profilesDir(), name)
}

func Init(name string, from string) error {
	dir := ProfileDir(name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if from != "" {
		srcDir := ProfileDir(from)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			return fmt.Errorf("source profile %q not found", from)
		}
		if err := CopyDir(srcDir, dir); err != nil {
			return fmt.Errorf("copying from %q: %w", from, err)
		}
	} else {
		tgt := targetDir()
		if err := captureManagedItems(defaultHarness(), tgt, dir); err != nil {
			return fmt.Errorf("copying managed items: %w", err)
		}
	}
	return nil
}

func Use(name string) error {
	return UseWithHarness(name, defaultHarness())
}

func UseWithHarness(name string, h harness.Harness) error {
	profileDir := ProfileDir(name)
	dir, cleanup, err := profileActivationDir(profileDir, h)
	if err != nil {
		return err
	}
	defer cleanup()
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}

	st, err := state.Load()
	if err != nil {
		return err
	}

	// Ensure vanilla backup exists
	if err := EnsureVanillaWithHarness(h); err != nil {
		return fmt.Errorf("ensuring vanilla backup: %w", err)
	}

	currentActive := st.GetGlobalHarness(h.Name())
	if currentActive != "" {
		if err := SaveWithHarness(currentActive, h); err != nil {
			return fmt.Errorf("saving current active profile %q: %w", currentActive, err)
		}
	}

	// Clean and apply
	tgt := targetDirForHarness(h)
	if err := CleanManagedItems(h, tgt); err != nil {
		return fmt.Errorf("cleaning target: %w", err)
	}
	if err := os.MkdirAll(tgt, 0755); err != nil {
		return err
	}
	if err := CopyManagedItems(h, dir, tgt); err != nil {
		return fmt.Errorf("applying profile: %w", err)
	}
	if err := ApplyOverrides(h, name, tgt); err != nil {
		return fmt.Errorf("applying overrides: %w", err)
	}

	// Update state
	st.SetGlobalHarness(h.Name(), name)
	return st.Save()
}

func UseNone() error {
	return UseNoneWithHarness(defaultHarness())
}

func UseNoneWithHarness(h harness.Harness) error {
	st, err := state.Load()
	if err != nil {
		return err
	}

	currentActive := st.GetGlobalHarness(h.Name())
	if currentActive != "" {
		if err := SaveWithHarness(currentActive, h); err != nil {
			return fmt.Errorf("saving current active profile %q: %w", currentActive, err)
		}
	}

	tgt := targetDirForHarness(h)
	if err := CleanManagedItems(h, tgt); err != nil {
		return fmt.Errorf("cleaning target: %w", err)
	}
	if err := RestoreVanillaWithHarness(h); err != nil {
		return fmt.Errorf("restoring vanilla backup: %w", err)
	}

	st.ClearGlobalHarness(h.Name())
	return st.Save()
}

func List() ([]ProfileInfo, error) {
	return ListWithHarness(defaultHarness())
}

func ListWithHarness(h harness.Harness) ([]ProfileInfo, error) {
	dir := profilesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	st, err := state.Load()
	if err != nil {
		return nil, err
	}

	active := st.GetGlobalHarness(h.Name())

	var profiles []ProfileInfo
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		assetDir, cleanup, err := profileActivationDir(filepath.Join(dir, e.Name()), h)
		if err != nil {
			return nil, err
		}
		items := countItems(h, assetDir)
		cleanup()
		profiles = append(profiles, ProfileInfo{
			Name:   e.Name(),
			Active: e.Name() == active,
			Items:  items,
		})
	}
	return profiles, nil
}

func Current() (string, error) {
	return CurrentWithHarness(defaultHarness())
}

func CurrentWithHarness(h harness.Harness) (string, error) {
	st, err := state.Load()
	if err != nil {
		return "", err
	}
	return st.GetGlobalHarness(h.Name()), nil
}

func Inspect(name string) (*Inventory, error) {
	h := defaultHarness()
	profileDir := ProfileDir(name)
	dir, err := profileAssetDir(profileDir, h)
	if err != nil {
		return nil, err
	}
	info := &Inventory{
		Name: name,
		Path: dir,
		Dirs: make(map[string][]string),
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return info, nil
		}
		return nil, err
	}

	info.Exists = true
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			children, err := os.ReadDir(filepath.Join(dir, name))
			if err != nil {
				return nil, err
			}
			for _, child := range children {
				info.Dirs[name] = append(info.Dirs[name], child.Name())
			}
			sort.Strings(info.Dirs[name])
			continue
		}
		info.Files = append(info.Files, name)
	}

	sort.Strings(info.Files)

	// Extract MCP server names from the config file
	if extra, ok := h.ExternalManagedPath(); ok {
		mcpPath := filepath.Join(dir, extra.ProfilePath)
		if cfg, err := readJSONFile(mcpPath); err == nil {
			if servers, ok := cfg["mcpServers"].(map[string]any); ok {
				for name := range servers {
					info.MCPServers = append(info.MCPServers, name)
				}
				sort.Strings(info.MCPServers)
			}
		}
	}

	return info, nil
}

func Save(name string) error {
	return SaveWithHarness(name, defaultHarness())
}

func SaveWithHarness(name string, h harness.Harness) error {
	profileDir := ProfileDir(name)
	dir, ok, err := profileCaptureDir(profileDir, h)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("profile %q uses rendered portable assets for harness %q; live changes cannot be saved safely", name, h.Name())
	}
	tgt := targetDirForHarness(h)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}
	// Strip overrides from live dir before capturing to prevent
	// baking them into the base profile
	if err := StripOverrides(h, name, tgt); err != nil {
		return fmt.Errorf("stripping overrides before save: %w", err)
	}
	// Always re-apply overrides to live dir, even if capture fails
	defer ApplyOverrides(h, name, tgt) //nolint:errcheck

	if err := clearDirContents(dir); err != nil {
		return err
	}
	if _, err := os.Stat(tgt); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return captureManagedItems(h, tgt, dir)
}

func Remove(name string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	active := st.GetGlobalHarness(defaultHarness().Name())
	if active == name {
		return fmt.Errorf("cannot remove active profile %q, switch first with 'cvm use --none'", name)
	}
	dir := ProfileDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}
	return os.RemoveAll(dir)
}

// --- Backup/Vanilla operations (inlined to avoid import cycle) ---

func vanillaDir() string {
	return vanillaDirForHarness(defaultHarness())
}

func vanillaDirForHarness(h harness.Harness) string {
	baseDir := config.GlobalVanillaDir()
	if h.Name() == defaultHarness().Name() {
		return baseDir
	}
	return filepath.Join(baseDir, h.Name())
}

func EnsureVanilla() error {
	return EnsureVanillaWithHarness(defaultHarness())
}

func EnsureVanillaWithHarness(h harness.Harness) error {
	vdir := vanillaDirForHarness(h)
	if _, err := os.Stat(vdir); err == nil {
		return nil
	}
	if err := os.MkdirAll(vdir, 0755); err != nil {
		return err
	}
	src := targetDirForHarness(h)
	return captureManagedItems(h, src, vdir)
}

func RestoreVanilla() error {
	return RestoreVanillaWithHarness(defaultHarness())
}

func RestoreVanillaWithHarness(h harness.Harness) error {
	vdir := vanillaDirForHarness(h)
	if _, err := os.Stat(vdir); os.IsNotExist(err) {
		return nil
	}
	dst := targetDirForHarness(h)
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	return CopyManagedItems(h, vdir, dst)
}

func HasVanilla() bool {
	return HasVanillaWithHarness(defaultHarness())
}

func HasVanillaWithHarness(h harness.Harness) bool {
	_, err := os.Stat(vanillaDirForHarness(h))
	return err == nil
}

func Nuke() error {
	return NukeWithHarness(defaultHarness())
}

func NukeWithHarness(h harness.Harness) error {
	dst := targetDirForHarness(h)
	return CleanManagedItems(h, dst)
}

// --- File operations ---

type managedPath struct {
	ProfilePath string
	LivePath    string
}

func CleanManagedItems(h harness.Harness, liveDir string) error {
	for _, item := range managedPaths(h, liveDir) {
		if h.IsUserMCPPath(item.ProfilePath) {
			if err := removeUserMCPServers(item.LivePath); err != nil {
				return err
			}
			continue
		}
		if err := os.RemoveAll(item.LivePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", item.LivePath, err)
		}
	}
	return nil
}

func CopyManagedItems(h harness.Harness, srcProfileDir, dstLiveDir string) error {
	for _, item := range managedPaths(h, dstLiveDir) {
		srcPath := filepath.Join(srcProfileDir, item.ProfilePath)
		if h.IsUserMCPPath(item.ProfilePath) {
			if err := applyUserMCPServers(srcPath, item.LivePath); err != nil {
				return err
			}
			continue
		}
		info, err := os.Stat(srcPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		dstPath := item.LivePath
		if info.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func captureManagedItems(h harness.Harness, srcLiveDir, dstProfileDir string) error {
	for _, item := range managedPaths(h, srcLiveDir) {
		srcPath := item.LivePath
		if h.IsUserMCPPath(item.ProfilePath) {
			if err := captureUserMCPServers(srcPath, filepath.Join(dstProfileDir, item.ProfilePath)); err != nil {
				return err
			}
			continue
		}
		info, err := os.Stat(srcPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		dstPath := filepath.Join(dstProfileDir, item.ProfilePath)
		if info.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func countItems(h harness.Harness, dir string) int {
	count := 0
	for _, item := range harness.ManagedProfileItems(h) {
		if _, err := os.Stat(filepath.Join(dir, item)); err == nil {
			count++
		}
	}
	return count
}

func managedPaths(h harness.Harness, liveDir string) []managedPath {
	paths := make([]managedPath, 0, len(harness.ManagedProfileItems(h)))
	for _, item := range h.ManagedDirItems() {
		paths = append(paths, managedPath{
			ProfilePath: item,
			LivePath:    filepath.Join(liveDir, item),
		})
	}

	if extra, ok := h.ExternalManagedPath(); ok {
		paths = append(paths, managedPath{
			ProfilePath: extra.ProfilePath,
			LivePath:    extra.LivePath,
		})
	}

	return paths
}

func applyUserMCPServers(srcProfilePath, dstLivePath string) error {
	cfg, err := readJSONFile(srcProfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	mcpServers, ok := cfg["mcpServers"]
	if !ok {
		return nil
	}

	live, err := readJSONFile(dstLivePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if live == nil {
		live = map[string]any{}
	}
	live["mcpServers"] = mcpServers

	return writeJSONFile(dstLivePath, live)
}

func captureUserMCPServers(srcLivePath, dstProfilePath string) error {
	live, err := readJSONFile(srcLivePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	mcpServers, ok := live["mcpServers"]
	if !ok {
		return nil
	}

	return writeJSONFile(dstProfilePath, map[string]any{
		"mcpServers": mcpServers,
	})
}

func removeUserMCPServers(path string) error {
	cfg, err := readJSONFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if _, ok := cfg["mcpServers"]; !ok {
		return nil
	}
	delete(cfg, "mcpServers")

	if len(cfg) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	return writeJSONFile(path, cfg)
}

func readJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := map[string]any{}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func writeJSONFile(path string, cfg map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return CopyFile(path, target)
	})
}

func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return os.WriteFile(dst, data, 0644)
	}
	return os.WriteFile(dst, data, info.Mode())
}

// OverrideDir returns the override directory for the given profile name.
func OverrideDir(name string) string {
	return config.OverrideDir(name)
}

// ApplyOverrides merges the user's override layer on top of the already-applied
// profile in the live directory. This is called after CopyManagedItems.
func ApplyOverrides(h harness.Harness, name string, liveDir string) error {
	overDir := OverrideDir(name)
	if _, err := os.Stat(overDir); os.IsNotExist(err) {
		return nil // no overrides — nothing to do
	}

	for _, item := range managedPaths(h, liveDir) {
		overSrc := filepath.Join(overDir, item.ProfilePath)
		if _, err := os.Stat(overSrc); os.IsNotExist(err) {
			continue
		}

		switch {
		// Directories: union merge (override files added or replace base by name)
		case isDirectory(overSrc):
			if err := mergeDirectories(overSrc, item.LivePath); err != nil {
				return fmt.Errorf("merging override dir %s: %w", item.ProfilePath, err)
			}

		// Markdown instructions files are additive so user guidance layers on top.
		case item.ProfilePath == h.MarkdownInstructionsFile():
			if err := appendFile(overSrc, item.LivePath); err != nil {
				return fmt.Errorf("appending override %s: %w", h.MarkdownInstructionsFile(), err)
			}

		// MCP config: sub-key merge for mcpServers (additive), top-level for others
		case h.IsMCPPath(item.ProfilePath):
			override, err := readJSONFile(overSrc)
			if err != nil {
				return fmt.Errorf("reading override %s: %w", item.ProfilePath, err)
			}
			live, err := readJSONFile(item.LivePath)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("reading live %s: %w", item.ProfilePath, err)
			}
			if live == nil {
				live = map[string]any{}
			}
			// Merge mcpServers at sub-key level (additive)
			if overServers, ok := override["mcpServers"].(map[string]any); ok {
				liveServers, _ := live["mcpServers"].(map[string]any)
				if liveServers == nil {
					liveServers = map[string]any{}
				}
				for sname, v := range overServers {
					liveServers[sname] = v
				}
				live["mcpServers"] = liveServers
			}
			// Merge other top-level keys normally
			for k, v := range override {
				if k == "mcpServers" {
					continue
				}
				live[k] = v
			}
			if err := writeJSONFile(item.LivePath, live); err != nil {
				return fmt.Errorf("writing merged %s: %w", item.ProfilePath, err)
			}

		// JSON files: top-level merge (override keys win)
		case isJSONFile(item.ProfilePath):
			if err := mergeJSONFiles(overSrc, item.LivePath); err != nil {
				return fmt.Errorf("merging override %s: %w", item.ProfilePath, err)
			}

		// Everything else (statusline-command.sh, etc.): override replaces
		default:
			if err := CopyFile(overSrc, item.LivePath); err != nil {
				return fmt.Errorf("copying override %s: %w", item.ProfilePath, err)
			}
		}
	}
	return nil
}

// mergeDirectories copies all files from overrideDir into targetDir,
// overwriting existing files by name (union merge).
func mergeDirectories(overrideDir, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	return filepath.WalkDir(overrideDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(overrideDir, path)
		target := filepath.Join(targetDir, rel)

		// Handle type conflicts: remove existing target if it's a different type
		if info, statErr := os.Lstat(target); statErr == nil {
			if d.IsDir() != info.IsDir() {
				if err := os.RemoveAll(target); err != nil {
					return err
				}
			}
		}

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return CopyFile(path, target)
	})
}

// mergeJSONFiles reads both files as JSON objects, merges top-level keys
// (override wins on conflict), and writes the result to targetPath.
func mergeJSONFiles(overridePath, targetPath string) error {
	base, err := readJSONFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if base == nil {
		base = map[string]any{}
	}

	override, err := readJSONFile(overridePath)
	if err != nil {
		return err
	}

	for k, v := range override {
		base[k] = v
	}

	return writeJSONFile(targetPath, base)
}

// appendFile appends the content of overridePath to targetPath with a separator.
func appendFile(overridePath, targetPath string) error {
	overrideData, err := os.ReadFile(overridePath)
	if err != nil {
		return err
	}
	if len(overrideData) == 0 {
		return nil
	}

	existing, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if len(existing) == 0 {
		return os.WriteFile(targetPath, overrideData, 0644)
	}

	separator := []byte("\n\n# --- User Overrides ---\n\n")
	combined := append(existing, separator...)
	combined = append(combined, overrideData...)
	return os.WriteFile(targetPath, combined, 0644)
}

// isDirectory checks if the given path is a directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// isJSONFile checks if the filename has a .json extension.
func isJSONFile(name string) bool {
	return strings.HasSuffix(name, ".json")
}

// StripOverrides removes override contributions from the live directory so that
// Save() captures only the base profile state. This prevents overrides from being
// permanently baked into the base profile on profile switch.
func StripOverrides(h harness.Harness, name string, liveDir string) error {
	overDir := OverrideDir(name)
	if _, err := os.Stat(overDir); os.IsNotExist(err) {
		return nil
	}

	profileDir := ProfileDir(name)
	baseProfileDir, err := profileAssetDir(profileDir, h)
	if err != nil {
		return err
	}

	for _, item := range managedPaths(h, liveDir) {
		overSrc := filepath.Join(overDir, item.ProfilePath)
		if _, err := os.Stat(overSrc); os.IsNotExist(err) {
			continue
		}

		switch {
		// Directories: remove override files, restore base versions if they existed
		case isDirectory(overSrc):
			if err := filepath.WalkDir(overSrc, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil || d.IsDir() {
					return walkErr
				}
				rel, _ := filepath.Rel(overSrc, path)
				target := filepath.Join(item.LivePath, rel)
				if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
					return err
				}
				// Restore from base profile if this file existed there too
				basePath := filepath.Join(baseProfileDir, item.ProfilePath, rel)
				if _, err := os.Stat(basePath); err == nil {
					if err := CopyFile(basePath, target); err != nil {
						return err
					}
				} else {
					// Clean up empty ancestor directories up to the item root
					for parent := filepath.Dir(target); parent != item.LivePath; parent = filepath.Dir(parent) {
						entries, err := os.ReadDir(parent)
						if err != nil || len(entries) > 0 {
							break
						}
						os.Remove(parent)
					}
				}
				return nil
			}); err != nil {
				return fmt.Errorf("stripping override dir %s: %w", item.ProfilePath, err)
			}

		// Markdown instructions files are additive; strip back to the base content.
		case item.ProfilePath == h.MarkdownInstructionsFile():
			data, err := os.ReadFile(item.LivePath)
			if err != nil {
				continue
			}
			separator := "\n\n# --- User Overrides ---\n\n"
			if idx := strings.Index(string(data), separator); idx >= 0 {
				if err := os.WriteFile(item.LivePath, data[:idx], 0644); err != nil {
					return fmt.Errorf("stripping override from %s: %w", h.MarkdownInstructionsFile(), err)
				}
			}

		// MCP config: strip at sub-key level for mcpServers, top-level for others
		case h.IsMCPPath(item.ProfilePath):
			override, err := readJSONFile(overSrc)
			if err != nil {
				continue
			}
			live, err := readJSONFile(item.LivePath)
			if err != nil {
				continue
			}
			baseSrc := filepath.Join(baseProfileDir, item.ProfilePath)
			base, _ := readJSONFile(baseSrc)
			if base == nil {
				base = map[string]any{}
			}

			// Handle mcpServers at sub-key level to preserve user-added servers
			if overServers, ok := override["mcpServers"].(map[string]any); ok {
				if liveServers, ok := live["mcpServers"].(map[string]any); ok {
					baseServers, _ := base["mcpServers"].(map[string]any)
					for name := range overServers {
						delete(liveServers, name)
						// Restore base version of this server if it existed
						if baseServers != nil {
							if v, ok := baseServers[name]; ok {
								liveServers[name] = v
							}
						}
					}
					if len(liveServers) == 0 {
						delete(live, "mcpServers")
					}
				}
			}

			// Handle other top-level keys normally
			for k := range override {
				if k == "mcpServers" {
					continue
				}
				delete(live, k)
				if v, ok := base[k]; ok {
					live[k] = v
				}
			}

			if len(live) == 0 {
				if err := os.Remove(item.LivePath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("removing empty %s: %w", item.ProfilePath, err)
				}
			} else {
				if err := writeJSONFile(item.LivePath, live); err != nil {
					return fmt.Errorf("stripping override from %s: %w", item.ProfilePath, err)
				}
			}

		// Other files (JSON, scripts, etc.): restore from base profile
		default:
			baseSrc := filepath.Join(baseProfileDir, item.ProfilePath)
			if baseInfo, err := os.Stat(baseSrc); err == nil {
				if baseInfo.IsDir() {
					if err := os.RemoveAll(item.LivePath); err != nil {
						return fmt.Errorf("removing overridden %s: %w", item.ProfilePath, err)
					}
					if err := CopyDir(baseSrc, item.LivePath); err != nil {
						return fmt.Errorf("restoring base %s: %w", item.ProfilePath, err)
					}
				} else {
					if err := CopyFile(baseSrc, item.LivePath); err != nil {
						return fmt.Errorf("restoring base %s: %w", item.ProfilePath, err)
					}
				}
			} else {
				if err := os.RemoveAll(item.LivePath); err != nil {
					return fmt.Errorf("removing override-only %s: %w", item.ProfilePath, err)
				}
			}
		}
	}
	return nil
}

// Reapply re-applies the active profile and its overrides to the live directory
// without saving the current state first. Used by "cvm override apply".
func Reapply(name string) error {
	h := defaultHarness()
	profileDir := ProfileDir(name)
	dir, cleanup, err := profileActivationDir(profileDir, h)
	if err != nil {
		return err
	}
	defer cleanup()
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}
	tgt := targetDir()
	// Strip current overrides to clean stale keys before fresh re-apply
	if err := StripOverrides(h, name, tgt); err != nil {
		return fmt.Errorf("stripping overrides: %w", err)
	}
	if err := CleanManagedItems(h, tgt); err != nil {
		return fmt.Errorf("cleaning target: %w", err)
	}
	if err := os.MkdirAll(tgt, 0755); err != nil {
		return err
	}
	if err := CopyManagedItems(h, dir, tgt); err != nil {
		return fmt.Errorf("applying profile: %w", err)
	}
	if err := ApplyOverrides(h, name, tgt); err != nil {
		return fmt.Errorf("applying overrides: %w", err)
	}
	return nil
}

func defaultHarness() harness.Harness {
	return harness.Claude()
}

func profileAssetDir(profileDir string, h harness.Harness) (string, error) {
	manifest, err := LoadManifest(profileDir)
	if err != nil {
		return "", fmt.Errorf("loading manifest for profile %q: %w", filepath.Base(profileDir), err)
	}
	if !manifest.SupportsHarness(h.Name()) {
		return "", fmt.Errorf("profile %q does not support harness %q", filepath.Base(profileDir), h.Name())
	}
	return manifest.AssetDir(profileDir, h)
}

func profileActivationDir(profileDir string, h harness.Harness) (string, func(), error) {
	manifest, err := LoadManifest(profileDir)
	if err != nil {
		return "", func() {}, fmt.Errorf("loading manifest for profile %q: %w", filepath.Base(profileDir), err)
	}
	if !manifest.SupportsHarness(h.Name()) {
		return "", func() {}, fmt.Errorf("profile %q does not support harness %q", filepath.Base(profileDir), h.Name())
	}

	portableRaw, hasPortable := manifest.Assets["portable"]
	if !hasPortable {
		dir, err := manifest.AssetDir(profileDir, h)
		return dir, func() {}, err
	}

	portableDir, err := assetDirFromRaw(profileDir, portableRaw)
	if err != nil {
		return "", func() {}, err
	}
	renderedDir, err := os.MkdirTemp("", "cvm-render-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(renderedDir) }

	if err := renderPortableAssets(portableDir, renderedDir, h); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if harnessRaw, ok := manifest.Assets[h.Name()]; ok {
		harnessDir, err := assetDirFromRaw(profileDir, harnessRaw)
		if err != nil {
			cleanup()
			return "", func() {}, err
		}
		if err := CopyDir(harnessDir, renderedDir); err != nil {
			cleanup()
			return "", func() {}, err
		}
	}

	return renderedDir, cleanup, nil
}

func renderPortableAssets(portableDir, renderedDir string, h harness.Harness) error {
	if _, err := os.Stat(portableDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := renderPortableFile(filepath.Join(portableDir, "instructions.md"), filepath.Join(renderedDir, h.MarkdownInstructionsFile())); err != nil {
		return err
	}
	if h.SupportsPortableSkills() {
		if err := renderPortableCollection(filepath.Join(portableDir, "skills"), renderedDir, filepath.Join("skills", "%s", "SKILL.md")); err != nil {
			return err
		}
	}
	if h.SupportsPortableAgents() {
		if err := renderPortableCollection(filepath.Join(portableDir, "agents"), renderedDir, filepath.Join("agents", "%s.md")); err != nil {
			return err
		}
	}
	return nil
}

func renderPortableFile(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return CopyFile(src, dst)
}

func renderPortableCollection(srcDir, renderedDir, targetPattern string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		targetRel := fmt.Sprintf(targetPattern, name)
		if err := CopyFile(filepath.Join(srcDir, entry.Name()), filepath.Join(renderedDir, targetRel)); err != nil {
			return err
		}
	}
	return nil
}

func profileCaptureDir(profileDir string, h harness.Harness) (string, bool, error) {
	manifest, err := LoadManifest(profileDir)
	if err != nil {
		return "", false, fmt.Errorf("loading manifest for profile %q: %w", filepath.Base(profileDir), err)
	}
	if !manifest.SupportsHarness(h.Name()) {
		return "", false, fmt.Errorf("profile %q does not support harness %q", filepath.Base(profileDir), h.Name())
	}
	if raw, ok := manifest.Assets[h.Name()]; ok {
		_, hasPortable := manifest.Assets["portable"]
		if (strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == ".") && hasPortable {
			return "", false, nil
		}
		dir, err := assetDirFromRaw(profileDir, raw)
		return dir, true, err
	}
	if _, ok := manifest.Assets["portable"]; ok {
		return "", false, nil
	}
	dir, err := manifest.AssetDir(profileDir, h)
	return dir, true, err
}

func clearDirContents(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
