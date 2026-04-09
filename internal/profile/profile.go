package profile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/state"
)

type ProfileInfo struct {
	Name   string
	Active bool
	Items  int
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
		if _, err := os.Stat(tgt); err == nil {
			CopyManagedItems(tgt, dir)
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
	EnsureVanilla(scope, projectPath)

	// Save current state to active profile
	var currentActive string
	if scope == config.ScopeGlobal {
		currentActive = st.Global.Active
	} else {
		currentActive = st.GetLocal(projectPath)
	}
	if currentActive != "" {
		Save(scope, currentActive, projectPath)
	}

	// Clean and apply
	tgt := targetDir(scope, projectPath)
	CleanManagedItems(tgt)
	os.MkdirAll(tgt, 0755)
	if err := CopyDir(dir, tgt); err != nil {
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
		Save(scope, currentActive, projectPath)
	}

	tgt := targetDir(scope, projectPath)
	CleanManagedItems(tgt)
	RestoreVanilla(scope, projectPath)

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
		items := countItems(filepath.Join(dir, e.Name()))
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

func Save(scope config.Scope, name string, projectPath string) error {
	dir := ProfileDir(scope, name)
	tgt := targetDir(scope, projectPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		os.RemoveAll(filepath.Join(dir, e.Name()))
	}
	CopyManagedItems(tgt, dir)
	return nil
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
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	CopyManagedItems(src, vdir)
	return nil
}

func RestoreVanilla(scope config.Scope, projectPath string) error {
	vdir := vanillaDir(scope, projectPath)
	if _, err := os.Stat(vdir); os.IsNotExist(err) {
		return nil
	}
	dst := targetDir(scope, projectPath)
	os.MkdirAll(dst, 0755)
	return CopyDir(vdir, dst)
}

func HasVanilla(scope config.Scope, projectPath string) bool {
	_, err := os.Stat(vanillaDir(scope, projectPath))
	return err == nil
}

func Nuke(scope config.Scope, projectPath string) error {
	dst := targetDir(scope, projectPath)
	for _, item := range config.ManagedItems {
		path := filepath.Join(dst, item)
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", item, err)
		}
	}
	return nil
}

// --- File operations ---

func CleanManagedItems(dir string) {
	for _, item := range config.ManagedItems {
		os.RemoveAll(filepath.Join(dir, item))
	}
}

func CopyManagedItems(src, dst string) {
	for _, item := range config.ManagedItems {
		srcPath := filepath.Join(src, item)
		if info, err := os.Stat(srcPath); err == nil {
			dstPath := filepath.Join(dst, item)
			if info.IsDir() {
				CopyDir(srcPath, dstPath)
			} else {
				CopyFile(srcPath, dstPath)
			}
		}
	}
}

func countItems(dir string) int {
	count := 0
	for _, item := range config.ManagedItems {
		if _, err := os.Stat(filepath.Join(dir, item)); err == nil {
			count++
		}
	}
	return count
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
	os.MkdirAll(filepath.Dir(dst), 0755)
	info, err := os.Stat(src)
	if err != nil {
		return os.WriteFile(dst, data, 0644)
	}
	return os.WriteFile(dst, data, info.Mode())
}
