package profile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

func profilesDir() string {
	return config.GlobalProfilesDir()
}

func targetDirForHarness(h harness.Harness) string {
	return h.TargetDir()
}

// ProfileDir returns the default cloned location for a profile: ~/.cvm/profiles/<name>.
func ProfileDir(name string) string {
	return filepath.Join(profilesDir(), name)
}

// sourceDir resolves the on-disk source repo dir backing a profile. It prefers
// a registered source (cloned path or `cvm add --path <dir>`) recorded in state
// and falls back to the default cloned location.
func sourceDir(name string) (string, error) {
	st, err := state.Load()
	if err != nil {
		return "", err
	}
	return sourceDirFromState(st, name), nil
}

func sourceDirFromState(st *state.State, name string) string {
	if dir, ok := st.GetSource(name); ok {
		return dir
	}
	return ProfileDir(name)
}

// assetDir resolves the harness-specific subdirectory inside the profile's
// source repo (e.g. <source>/claude or <source>/. per the manifest/discovery).
func assetDir(srcDir string, h harness.Harness) (string, error) {
	manifest, err := LoadManifest(srcDir)
	if err != nil {
		return "", fmt.Errorf("loading manifest for profile %q: %w", filepath.Base(srcDir), err)
	}
	if !manifest.SupportsHarness(h.Name()) {
		return "", fmt.Errorf("profile %q does not support harness %q", filepath.Base(srcDir), h.Name())
	}
	return manifest.AssetDir(srcDir, h)
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
	}
	return nil
}

func Use(name string) error {
	return UseWithHarness(name, defaultHarness())
}

// UseWithHarness activates a profile for a harness by symlinking each managed
// item from the profile's source repo into the harness target dir.
func UseWithHarness(name string, h harness.Harness) error {
	st, err := state.Load()
	if err != nil {
		return err
	}

	srcDir := sourceDirFromState(st, name)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}

	src, err := assetDir(srcDir, h)
	if err != nil {
		return err
	}

	tgt := targetDirForHarness(h)
	if err := os.MkdirAll(tgt, 0755); err != nil {
		return err
	}

	// Remove any symlinks the previously-active profile left behind so we don't
	// leave dangling links for items the new profile doesn't own.
	if err := unlinkManagedItems(h, tgt); err != nil {
		return fmt.Errorf("clearing previous links: %w", err)
	}

	for _, item := range h.ManagedDirItems() {
		srcPath := filepath.Join(src, item)
		if _, err := os.Stat(srcPath); err != nil {
			if os.IsNotExist(err) {
				continue // profile doesn't provide this item
			}
			return err
		}
		tgtPath := filepath.Join(tgt, item)
		if err := stashVanilla(h.Name(), item, tgtPath); err != nil {
			return fmt.Errorf("stashing vanilla %s: %w", item, err)
		}
		if err := linkItem(srcPath, tgtPath); err != nil {
			return fmt.Errorf("linking %s: %w", item, err)
		}
	}

	st.SetGlobalHarness(h.Name(), name)
	return st.Save()
}

func UseNone() error {
	return UseNoneWithHarness(defaultHarness())
}

// UseNoneWithHarness deactivates the harness: it removes the profile symlinks
// and restores any pre-cvm vanilla files that were stashed aside.
func UseNoneWithHarness(h harness.Harness) error {
	st, err := state.Load()
	if err != nil {
		return err
	}

	tgt := targetDirForHarness(h)
	if err := unlinkManagedItems(h, tgt); err != nil {
		return fmt.Errorf("removing links: %w", err)
	}
	if err := restoreVanilla(h); err != nil {
		return fmt.Errorf("restoring vanilla: %w", err)
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
		profiles = append(profiles, ProfileInfo{
			Name:   e.Name(),
			Active: e.Name() == active,
			Items:  profileItemCount(st, e.Name(), h),
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

func ManifestForProfile(name string) (*Manifest, error) {
	srcDir, err := sourceDir(name)
	if err != nil {
		return nil, err
	}
	return LoadManifest(srcDir)
}

func Remove(name string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	for _, h := range harness.All() {
		if st.GetGlobalHarness(h.Name()) == name {
			return fmt.Errorf("cannot remove active profile %q, switch first with 'cvm off'", name)
		}
	}

	_, registered := st.GetSource(name)
	dir := ProfileDir(name)
	_, statErr := os.Stat(dir)
	if os.IsNotExist(statErr) && !registered {
		return fmt.Errorf("profile %q not found", name)
	}

	// Only delete the cloned copy under ~/.cvm/profiles. A path-registered
	// source belongs to the user and must never be removed.
	if statErr == nil {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}

	st.RemoveSource(name)
	return st.Save()
}

// --- Vanilla stash (lightweight move-aside) ---

// stashVanilla moves a pre-existing REAL (non-symlink) file/dir at tgtPath aside
// to ~/.cvm/vanilla/<harness>/<item> exactly once, before a profile symlink
// takes its place. A symlink at tgtPath (cvm-managed) is simply removed.
func stashVanilla(harnessName, item, tgtPath string) error {
	info, err := os.Lstat(tgtPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Existing cvm symlink: just remove it so we can relink.
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(tgtPath)
	}

	stashPath := filepath.Join(config.VanillaDirForHarness(harnessName), item)
	if _, err := os.Lstat(stashPath); err == nil {
		// A vanilla copy is already stashed; the live one is cvm leftovers.
		return os.RemoveAll(tgtPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(stashPath), 0755); err != nil {
		return err
	}
	return os.Rename(tgtPath, stashPath)
}

// restoreVanilla moves every stashed vanilla item for a harness back into the
// target dir, replacing the (already-removed) profile symlinks.
func restoreVanilla(h harness.Harness) error {
	stashDir := config.VanillaDirForHarness(h.Name())
	tgt := targetDirForHarness(h)
	for _, item := range h.ManagedDirItems() {
		stashPath := filepath.Join(stashDir, item)
		if _, err := os.Lstat(stashPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		tgtPath := filepath.Join(tgt, item)
		if err := os.RemoveAll(tgtPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(tgtPath), 0755); err != nil {
			return err
		}
		if err := os.Rename(stashPath, tgtPath); err != nil {
			return err
		}
	}
	return nil
}

// --- Symlink helpers ---

// linkItem creates tgtPath as a symlink to srcPath, replacing whatever is there.
func linkItem(srcPath, tgtPath string) error {
	if err := os.MkdirAll(filepath.Dir(tgtPath), 0755); err != nil {
		return err
	}
	if _, err := os.Lstat(tgtPath); err == nil {
		if err := os.RemoveAll(tgtPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(srcPath, tgtPath)
}

// unlinkManagedItems removes symlinks for managed items in the target dir.
// Real (non-symlink) files are left untouched — only cvm's links are removed.
func unlinkManagedItems(h harness.Harness, tgt string) error {
	for _, item := range h.ManagedDirItems() {
		tgtPath := filepath.Join(tgt, item)
		info, err := os.Lstat(tgtPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue // real file the user owns; not ours to remove
		}
		if err := os.Remove(tgtPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing link %s: %w", tgtPath, err)
		}
	}
	return nil
}

// profileItemCount counts how many managed items the profile actually provides
// for the harness (used for display).
func profileItemCount(st *state.State, name string, h harness.Harness) int {
	src, err := assetDir(sourceDirFromState(st, name), h)
	if err != nil {
		return 0
	}
	count := 0
	for _, item := range h.ManagedDirItems() {
		if _, err := os.Stat(filepath.Join(src, item)); err == nil {
			count++
		}
	}
	return count
}

func defaultHarness() harness.Harness {
	return harness.Claude()
}

// --- File copy helpers (still used for clone/scaffold flows) ---

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
