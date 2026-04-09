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

	// Verify the path exists in the repo
	srcDir := filepath.Join(cacheDir, path)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("path %q not found in repo", path)
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
