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
	"github.com/chichex/cvm/internal/state"
)

type ProfileInfo struct {
	Name   string
	Active bool
	Items  int
}

type Inventory struct {
	Name   string
	Scope  config.Scope
	Path   string
	Exists bool
	Files  []string
	Dirs   map[string][]string
}

func profilesDir(scope config.Scope) string {
	if scope == config.ScopeGlobal {
		return config.GlobalProfilesDir()
	}
	return config.LocalProfilesDir()
}

func targetDir(scope config.Scope, projectPath string) string {
	if scope == config.ScopeGlobal {
		return config.ClaudeHome()
	}
	return config.ProjectClaudeDir(projectPath)
}

func ProfileDir(scope config.Scope, name string) string {
	return filepath.Join(profilesDir(scope), name)
}

func Init(scope config.Scope, name string, from string, projectPath string) error {
	dir := ProfileDir(scope, name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if from != "" {
		srcDir := ProfileDir(scope, from)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			return fmt.Errorf("source profile %q not found", from)
		}
		if err := CopyDir(srcDir, dir); err != nil {
			return fmt.Errorf("copying from %q: %w", from, err)
		}
	} else {
		tgt := targetDir(scope, projectPath)
		if err := captureManagedItems(scope, tgt, dir, projectPath); err != nil {
			return fmt.Errorf("copying managed items: %w", err)
		}
	}
	return nil
}

func Use(scope config.Scope, name string, projectPath string) error {
	dir := ProfileDir(scope, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}

	st, err := state.Load()
	if err != nil {
		return err
	}

	// Ensure vanilla backup exists
	if err := EnsureVanilla(scope, projectPath); err != nil {
		return fmt.Errorf("ensuring vanilla backup: %w", err)
	}

	// Save current state to active profile
	var currentActive string
	if scope == config.ScopeGlobal {
		currentActive = st.Global.Active
	} else {
		currentActive = st.GetLocal(projectPath)
	}
	if currentActive != "" {
		if err := Save(scope, currentActive, projectPath); err != nil {
			return fmt.Errorf("saving current active profile %q: %w", currentActive, err)
		}
	}

	// Clean and apply
	tgt := targetDir(scope, projectPath)
	if err := CleanManagedItems(scope, tgt, projectPath); err != nil {
		return fmt.Errorf("cleaning target: %w", err)
	}
	if err := os.MkdirAll(tgt, 0755); err != nil {
		return err
	}
	if err := CopyManagedItems(scope, dir, tgt, projectPath); err != nil {
		return fmt.Errorf("applying profile: %w", err)
	}

	// Update state
	if scope == config.ScopeGlobal {
		st.SetGlobal(name)
	} else {
		st.SetLocal(projectPath, name)
	}
	return st.Save()
}

func UseNone(scope config.Scope, projectPath string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}

	var currentActive string
	if scope == config.ScopeGlobal {
		currentActive = st.Global.Active
	} else {
		currentActive = st.GetLocal(projectPath)
	}
	if currentActive != "" {
		if err := Save(scope, currentActive, projectPath); err != nil {
			return fmt.Errorf("saving current active profile %q: %w", currentActive, err)
		}
	}

	tgt := targetDir(scope, projectPath)
	if err := CleanManagedItems(scope, tgt, projectPath); err != nil {
		return fmt.Errorf("cleaning target: %w", err)
	}
	if err := RestoreVanilla(scope, projectPath); err != nil {
		return fmt.Errorf("restoring vanilla backup: %w", err)
	}

	if scope == config.ScopeGlobal {
		st.SetGlobal("")
	} else {
		st.ClearLocal(projectPath)
	}
	return st.Save()
}

func List(scope config.Scope, projectPath string) ([]ProfileInfo, error) {
	dir := profilesDir(scope)
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

	var active string
	if scope == config.ScopeGlobal {
		active = st.Global.Active
	} else {
		active = st.GetLocal(projectPath)
	}

	var profiles []ProfileInfo
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		items := countItems(scope, filepath.Join(dir, e.Name()))
		profiles = append(profiles, ProfileInfo{
			Name:   e.Name(),
			Active: e.Name() == active,
			Items:  items,
		})
	}
	return profiles, nil
}

func Current(scope config.Scope, projectPath string) (string, error) {
	st, err := state.Load()
	if err != nil {
		return "", err
	}
	if scope == config.ScopeGlobal {
		return st.Global.Active, nil
	}
	return st.GetLocal(projectPath), nil
}

func Inspect(scope config.Scope, name, projectPath string) (*Inventory, error) {
	dir := ProfileDir(scope, name)
	info := &Inventory{
		Name:  name,
		Scope: scope,
		Path:  dir,
		Dirs:  make(map[string][]string),
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
	return info, nil
}

func Save(scope config.Scope, name string, projectPath string) error {
	dir := ProfileDir(scope, name)
	tgt := targetDir(scope, projectPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	if _, err := os.Stat(tgt); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return captureManagedItems(scope, tgt, dir, projectPath)
}

func Remove(scope config.Scope, name string, projectPath string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	var active string
	if scope == config.ScopeGlobal {
		active = st.Global.Active
	} else {
		active = st.GetLocal(projectPath)
	}
	if active == name {
		return fmt.Errorf("cannot remove active profile %q, switch first with 'cvm %s use --none'", name, scope)
	}
	dir := ProfileDir(scope, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}
	return os.RemoveAll(dir)
}

// --- Backup/Vanilla operations (inlined to avoid import cycle) ---

func vanillaDir(scope config.Scope, projectPath string) string {
	if scope == config.ScopeGlobal {
		return config.GlobalVanillaDir()
	}
	return config.LocalVanillaDir(projectPath)
}

func EnsureVanilla(scope config.Scope, projectPath string) error {
	vdir := vanillaDir(scope, projectPath)
	if _, err := os.Stat(vdir); err == nil {
		return nil
	}
	if err := os.MkdirAll(vdir, 0755); err != nil {
		return err
	}
	src := targetDir(scope, projectPath)
	return captureManagedItems(scope, src, vdir, projectPath)
}

func RestoreVanilla(scope config.Scope, projectPath string) error {
	vdir := vanillaDir(scope, projectPath)
	if _, err := os.Stat(vdir); os.IsNotExist(err) {
		return nil
	}
	dst := targetDir(scope, projectPath)
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	return CopyManagedItems(scope, vdir, dst, projectPath)
}

func HasVanilla(scope config.Scope, projectPath string) bool {
	_, err := os.Stat(vanillaDir(scope, projectPath))
	return err == nil
}

func Nuke(scope config.Scope, projectPath string) error {
	dst := targetDir(scope, projectPath)
	return CleanManagedItems(scope, dst, projectPath)
}

// --- File operations ---

type managedPath struct {
	ProfilePath string
	LivePath    string
}

func CleanManagedItems(scope config.Scope, liveDir, projectPath string) error {
	for _, item := range managedPaths(scope, liveDir, projectPath) {
		if item.ProfilePath == ".claude.json" {
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

func CopyManagedItems(scope config.Scope, srcProfileDir, dstLiveDir, projectPath string) error {
	for _, item := range managedPaths(scope, dstLiveDir, projectPath) {
		srcPath := filepath.Join(srcProfileDir, item.ProfilePath)
		if item.ProfilePath == ".claude.json" {
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

func captureManagedItems(scope config.Scope, srcLiveDir, dstProfileDir, projectPath string) error {
	for _, item := range managedPaths(scope, srcLiveDir, projectPath) {
		srcPath := item.LivePath
		if item.ProfilePath == ".claude.json" {
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

func countItems(scope config.Scope, dir string) int {
	count := 0
	for _, item := range config.ManagedProfileItems(scope) {
		if _, err := os.Stat(filepath.Join(dir, item)); err == nil {
			count++
		}
	}
	return count
}

func managedPaths(scope config.Scope, liveDir, projectPath string) []managedPath {
	paths := make([]managedPath, 0, len(config.ManagedProfileItems(scope)))
	for _, item := range config.ManagedClaudeDirItems {
		paths = append(paths, managedPath{
			ProfilePath: item,
			LivePath:    filepath.Join(liveDir, item),
		})
	}

	switch scope {
	case config.ScopeGlobal:
		paths = append(paths, managedPath{
			ProfilePath: ".claude.json",
			LivePath:    config.ClaudeUserConfigPath(),
		})
	case config.ScopeLocal:
		paths = append(paths, managedPath{
			ProfilePath: ".mcp.json",
			LivePath:    config.ProjectMCPConfigPath(projectPath),
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
