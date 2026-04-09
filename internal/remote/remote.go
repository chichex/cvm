package remote

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
)

// CacheDirFor returns ~/.cvm/remotes/<safe-repo-name>
func CacheDirFor(repo string) string {
	safe := strings.NewReplacer("/", "-", ":", "-", ".", "-").Replace(repo)
	return filepath.Join(config.CvmHome(), "remotes", safe)
}

// normalizeRepo ensures the repo URL is a valid git URL.
func normalizeRepo(repo string) string {
	if strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "git@") || strings.HasPrefix(repo, "ssh://") {
		return repo
	}
	// Assume github shorthand: owner/repo or github.com/owner/repo
	repo = strings.TrimPrefix(repo, "github.com/")
	return "https://github.com/" + repo
}

// looksLikeProfile checks if a directory contains Claude Code config files.
func looksLikeProfile(dir string) bool {
	for _, item := range config.ManagedItems {
		if _, err := os.Stat(filepath.Join(dir, item)); err == nil {
			return true
		}
	}
	return false
}

// discoverProfilePath tries to find a profile inside a cloned repo.
// Returns the path relative to the repo root, or error with suggestions.
func discoverProfilePath(cacheDir, profileName string) (string, error) {
	// 1. profiles/<name>/
	candidate := filepath.Join(cacheDir, "profiles", profileName)
	if looksLikeProfile(candidate) {
		fmt.Printf("  found profile at profiles/%s/\n", profileName)
		return "profiles/" + profileName, nil
	}

	// 2. <name>/ at root
	candidate = filepath.Join(cacheDir, profileName)
	if looksLikeProfile(candidate) {
		fmt.Printf("  found profile at %s/\n", profileName)
		return profileName, nil
	}

	// 3. Repo root itself is a profile
	if looksLikeProfile(cacheDir) {
		fmt.Println("  repo root is a profile")
		return "", nil
	}

	// 4. Scan for any directories that look like profiles
	var found []string

	// Check profiles/*/
	profilesDir := filepath.Join(cacheDir, "profiles")
	if entries, err := os.ReadDir(profilesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && looksLikeProfile(filepath.Join(profilesDir, e.Name())) {
				found = append(found, "profiles/"+e.Name())
			}
		}
	}

	// Check root-level dirs
	if entries, err := os.ReadDir(cacheDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "profiles" {
				if looksLikeProfile(filepath.Join(cacheDir, e.Name())) {
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
		msg := fmt.Sprintf("multiple profiles found in repo, specify which one:\n")
		for _, p := range found {
			msg += fmt.Sprintf("  cvm add %s <url>/%s\n", profileName, p)
		}
		return "", fmt.Errorf(msg)
	}

	return "", fmt.Errorf("no profile found in repo. Make sure it contains CLAUDE.md, skills/, or other Claude Code config")
}

// Add registers a remote profile source and clones it.
func Add(profileName, repo, path, branch string, scope config.Scope) error {
	if branch == "" {
		branch = "main"
	}

	repoURL := normalizeRepo(repo)
	cacheDir := CacheDirFor(repo)

	// Clone or update the cache
	if _, err := os.Stat(filepath.Join(cacheDir, ".git")); os.IsNotExist(err) {
		fmt.Printf("Cloning %s...\n", repoURL)
		cmd := exec.Command("git", "clone", "--depth=1", "--branch", branch, repoURL, cacheDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cloning repo: %w", err)
		}
	}

	// Auto-discover profile path if not provided
	if path == "" {
		discovered, err := discoverProfilePath(cacheDir, profileName)
		if err != nil {
			return err
		}
		path = discovered
	}

	// Verify the path exists in the repo
	srcDir := filepath.Join(cacheDir, path)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("path %q not found in repo", path)
	}

	if !looksLikeProfile(srcDir) {
		return fmt.Errorf("path %q exists but doesn't look like a Claude Code profile (no CLAUDE.md, skills/, etc.)", path)
	}

	// Copy to profile
	profileDir := profile.ProfileDir(scope, profileName)
	if _, err := os.Stat(profileDir); err == nil {
		// Profile exists, overwrite with remote contents
		entries, _ := os.ReadDir(profileDir)
		for _, e := range entries {
			os.RemoveAll(filepath.Join(profileDir, e.Name()))
		}
	} else {
		os.MkdirAll(profileDir, 0755)
	}

	if err := profile.CopyDir(srcDir, profileDir); err != nil {
		return fmt.Errorf("copying profile: %w", err)
	}

	// Save remote info in state
	st, err := state.Load()
	if err != nil {
		return err
	}
	if st.Remotes == nil {
		st.Remotes = make(map[string]state.Remote)
	}
	st.Remotes[profileName] = state.Remote{
		Repo:    repo,
		Path:    path,
		Branch:  branch,
		Scope:   string(scope),
		Profile: profileName,
	}
	return st.Save()
}

// Pull updates one or all remote-linked profiles.
func Pull(profileName string) ([]string, error) {
	st, err := state.Load()
	if err != nil {
		return nil, err
	}

	var updated []string
	var targets map[string]state.Remote

	if profileName != "" {
		r, ok := st.Remotes[profileName]
		if !ok {
			return nil, fmt.Errorf("profile %q has no remote", profileName)
		}
		targets = map[string]state.Remote{profileName: r}
	} else {
		targets = st.Remotes
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no remote-linked profiles found")
	}

	for name, r := range targets {
		cacheDir := CacheDirFor(r.Repo)

		// Pull latest
		fmt.Printf("Pulling %s from %s...\n", name, r.Repo)
		cmd := exec.Command("git", "-C", cacheDir, "pull", "--ff-only")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("  warning: pull failed for %s: %v\n", name, err)
			continue
		}

		// Re-copy to profile
		srcDir := filepath.Join(cacheDir, r.Path)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			fmt.Printf("  warning: path %q no longer exists in repo\n", r.Path)
			continue
		}

		scope := config.Scope(r.Scope)
		profileDir := profile.ProfileDir(scope, name)

		// Clear and re-copy
		entries, _ := os.ReadDir(profileDir)
		for _, e := range entries {
			os.RemoveAll(filepath.Join(profileDir, e.Name()))
		}
		if err := profile.CopyDir(srcDir, profileDir); err != nil {
			fmt.Printf("  warning: copy failed for %s: %v\n", name, err)
			continue
		}

		updated = append(updated, name)

		// If this profile is currently active, re-apply it
		var active string
		if scope == config.ScopeGlobal {
			active = st.Global.Active
		} else {
			// For local, we'd need project path context
			active = ""
		}
		if active == name {
			fmt.Printf("  re-applying active profile %q...\n", name)
			profile.Use(scope, name, "")
		}
	}

	return updated, nil
}

// List returns all remote-linked profiles.
func List() (map[string]state.Remote, error) {
	st, err := state.Load()
	if err != nil {
		return nil, err
	}
	return st.Remotes, nil
}

// Remove unlinks a profile from its remote (keeps local copy).
func Remove(profileName string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	if _, ok := st.Remotes[profileName]; !ok {
		return fmt.Errorf("profile %q has no remote", profileName)
	}
	delete(st.Remotes, profileName)
	return st.Save()
}
