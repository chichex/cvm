package remote

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
)

// normalizeRepo ensures the repo URL is a valid git URL.
func normalizeRepo(repo string) string {
	if strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "git@") || strings.HasPrefix(repo, "ssh://") {
		return repo
	}
	// Assume github shorthand: owner/repo or github.com/owner/repo
	repo = strings.TrimPrefix(repo, "github.com/")
	return "https://github.com/" + repo
}

// looksLikeProfile checks if a directory contains a cvm profile layout.
func looksLikeProfile(dir string) bool {
	return profile.LooksLikeProfileDir(dir)
}

// discoverProfilePath tries to find a profile inside a cloned repo.
// Returns the path relative to the repo root, or error with suggestions.
func discoverProfilePath(cloneDir, profileName string) (string, error) {
	// 1. profiles/<name>/
	candidate := filepath.Join(cloneDir, "profiles", profileName)
	if looksLikeProfile(candidate) {
		fmt.Printf("  found profile at profiles/%s/\n", profileName)
		return "profiles/" + profileName, nil
	}

	// 2. <name>/ at root
	candidate = filepath.Join(cloneDir, profileName)
	if looksLikeProfile(candidate) {
		fmt.Printf("  found profile at %s/\n", profileName)
		return profileName, nil
	}

	// 3. Repo root itself is a profile
	if looksLikeProfile(cloneDir) {
		fmt.Println("  repo root is a profile")
		return "", nil
	}

	// 4. Scan for any directories that look like profiles
	var found []string

	// Check profiles/*/
	profilesDir := filepath.Join(cloneDir, "profiles")
	if entries, err := os.ReadDir(profilesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && looksLikeProfile(filepath.Join(profilesDir, e.Name())) {
				found = append(found, "profiles/"+e.Name())
			}
		}
	}

	// Check root-level dirs
	if entries, err := os.ReadDir(cloneDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "profiles" {
				if looksLikeProfile(filepath.Join(cloneDir, e.Name())) {
					found = append(found, e.Name())
				}
			}
		}
	}

	if len(found) == 1 {
		fmt.Printf("  found profile at %s/\n", found[0])
		return found[0], nil
	}

	if len(found) > 1 {
		msg := "multiple profiles found in repo, specify which one:\n"
		for _, p := range found {
			msg += fmt.Sprintf("  cvm add %s <url>/%s\n", profileName, p)
		}
		return "", errors.New(msg)
	}

	return "", fmt.Errorf("no profile found in repo. Make sure it contains cvm profile assets or a cvm.profile.toml manifest")
}

// Add clones a git repo (keeping its .git) into ~/.cvm/profiles/<name> and
// registers the resulting working tree as the profile's source. The clone is a
// real git repo on disk: editing a symlinked managed item writes straight into
// it, and `cvm pull`/`cvm push` operate on it directly.
func Add(profileName, repo, path, branch string) error {
	repoURL := normalizeRepo(repo)
	cloneDir := profile.ProfileDir(profileName)

	// Refuse to clobber an existing clone so we never destroy local edits.
	if _, err := os.Stat(filepath.Join(cloneDir, ".git")); err == nil {
		return fmt.Errorf("profile %q already exists at %s; remove it first with: cvm rm %s", profileName, cloneDir, profileName)
	}
	if entries, err := os.ReadDir(cloneDir); err == nil && len(entries) > 0 {
		return fmt.Errorf("profile %q already exists at %s; remove it first with: cvm rm %s", profileName, cloneDir, profileName)
	}

	if err := os.MkdirAll(filepath.Dir(cloneDir), 0755); err != nil {
		return err
	}

	fmt.Printf("Cloning %s...\n", repoURL)
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, cloneDir)
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Leave nothing half-cloned behind.
		os.RemoveAll(cloneDir)
		return fmt.Errorf("cloning repo: %w", err)
	}

	// Auto-discover the profile subpath if not provided.
	if path == "" {
		discovered, err := discoverProfilePath(cloneDir, profileName)
		if err != nil {
			os.RemoveAll(cloneDir)
			return err
		}
		path = discovered
	}

	srcDir := filepath.Join(cloneDir, path)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		os.RemoveAll(cloneDir)
		return fmt.Errorf("path %q not found in repo", path)
	}
	if !looksLikeProfile(srcDir) {
		os.RemoveAll(cloneDir)
		return fmt.Errorf("path %q exists but doesn't look like a cvm profile", path)
	}

	st, err := state.Load()
	if err != nil {
		return err
	}
	// Record the asset source dir (clone root or a subpath inside it) so the
	// symlink layer resolves <source>/<harnessSubdir>/<item>.
	st.SetSource(profileName, srcDir)
	st.PutRemote(state.Remote{
		Repo:    repo,
		Path:    path,
		Branch:  branch,
		Profile: profileName,
	})
	return st.Save()
}

// AddPath registers an existing local git repo as a profile source without
// cloning. The directory stays where the user keeps it (e.g. their own repo);
// cvm only records it in state and symlinks managed items out of it.
func AddPath(profileName, dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path %q does not exist", dir)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", dir)
	}
	if !looksLikeProfile(abs) {
		return fmt.Errorf("path %q doesn't look like a cvm profile (no profile assets or cvm.profile.toml manifest)", dir)
	}

	st, err := state.Load()
	if err != nil {
		return err
	}
	st.SetSource(profileName, abs)
	return st.Save()
}

// SourceRepoDir returns the git working tree that backs a profile: the registered
// source dir if present, else the default cloned location. The returned path is
// where `git` runs for pull/push; it may sit above the asset source dir when the
// profile lives in a subdirectory of a cloned repo.
func SourceRepoDir(profileName string) (string, error) {
	st, err := state.Load()
	if err != nil {
		return "", err
	}
	return gitRepoRoot(sourceForProfile(st, profileName)), nil
}

func sourceForProfile(st *state.State, profileName string) string {
	if dir, ok := st.GetSource(profileName); ok {
		return dir
	}
	return profile.ProfileDir(profileName)
}

// gitRepoRoot walks up from dir until it finds a directory containing .git,
// stopping at the filesystem root. Falls back to dir if none is found.
func gitRepoRoot(dir string) string {
	cur := dir
	for {
		if info, err := os.Stat(filepath.Join(cur, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return dir
		}
		cur = parent
	}
}

// Pull fast-forwards one or all git-backed profiles. With the symlink model the
// source repo IS live, so a successful pull is instantly reflected — no re-copy,
// no re-apply. cvm is a transparent pass-through: it refuses to touch a dirty
// tree and never auto-merges.
func Pull(profileName string) ([]string, error) {
	st, err := state.Load()
	if err != nil {
		return nil, err
	}

	var names []string
	if profileName != "" {
		names = []string{profileName}
	} else {
		seen := map[string]bool{}
		for name := range st.Sources {
			if !seen[name] {
				names = append(names, name)
				seen[name] = true
			}
		}
		for name := range st.Remotes {
			if !seen[name] {
				names = append(names, name)
				seen[name] = true
			}
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("no git-backed profiles found")
	}

	var updated []string
	for _, name := range names {
		repoDir := gitRepoRoot(sourceForProfile(st, name))
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
			if profileName != "" {
				return nil, fmt.Errorf("profile %q is not a git repo (%s)", name, repoDir)
			}
			fmt.Printf("  skipping %s: not a git repo (%s)\n", name, repoDir)
			continue
		}

		if dirty, err := worktreeDirty(repoDir); err != nil {
			if profileName != "" {
				return nil, err
			}
			fmt.Printf("  warning: %s: %v\n", name, err)
			continue
		} else if dirty {
			msg := fmt.Sprintf("profile %q has uncommitted changes in %s; commit or stash them before pulling", name, repoDir)
			if profileName != "" {
				return nil, errors.New(msg)
			}
			fmt.Printf("  skipping %s: %s\n", name, msg)
			continue
		}

		fmt.Printf("Pulling %s (%s)...\n", name, repoDir)
		cmd := exec.Command("git", "-C", repoDir, "pull", "--ff-only")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if profileName != "" {
				return nil, fmt.Errorf("pulling %s: %w", name, err)
			}
			fmt.Printf("  warning: pull failed for %s: %v\n", name, err)
			continue
		}
		updated = append(updated, name)
	}

	return updated, nil
}

// Push pushes the active/given profile's git repo, surfacing git's own output
// (including non-fast-forward rejections) verbatim. No merge smarts.
func Push(profileName string) error {
	repoDir, err := SourceRepoDir(profileName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		return fmt.Errorf("profile %q is not a git repo (%s)", profileName, repoDir)
	}
	fmt.Printf("Pushing %s (%s)...\n", profileName, repoDir)
	cmd := exec.Command("git", "-C", repoDir, "push")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// worktreeDirty reports whether the git working tree at dir has uncommitted
// changes (tracked or untracked).
func worktreeDirty(dir string) (bool, error) {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("checking git status in %s: %w", dir, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// List returns all remote-linked profiles.
func List() (map[string]state.Remote, error) {
	st, err := state.Load()
	if err != nil {
		return nil, err
	}
	return st.Remotes, nil
}

// Remove unlinks a profile from its remote and forgets its registered source.
// It never deletes the on-disk repo (Remove in the profile package handles
// deleting a cloned profile; a --path-registered repo is the user's own).
func Remove(profileName string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	st.RemoveRemotesByProfile(profileName)
	st.RemoveSource(profileName)
	return st.Save()
}
