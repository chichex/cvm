package remote

import (
	"errors"
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

// looksLikeProfile checks if a directory contains a cvm profile layout.
func looksLikeProfile(dir string) bool {
	return profile.LooksLikeProfileDir(dir)
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
		return "", errors.New(msg)
	}

	return "", fmt.Errorf("no profile found in repo. Make sure it contains cvm profile assets or a cvm.profile.toml manifest")
}

// Add registers a remote profile source and clones it.
func Add(profileName, repo, path, branch string, scope config.Scope, projectPath string) error {
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
	} else {
		fmt.Printf("Updating cached repo...\n")
		cmd := exec.Command("git", "-C", cacheDir, "pull", "--ff-only")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("updating cached repo: %w", err)
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
		return fmt.Errorf("path %q exists but doesn't look like a cvm profile", path)
	}

	// Copy to profile
	profileDir := profile.ProfileDir(scope, profileName)
	if _, err := os.Stat(profileDir); err == nil {
		fmt.Printf("  profile %q already exists, updating...\n", profileName)
		entries, err := os.ReadDir(profileDir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := os.RemoveAll(filepath.Join(profileDir, e.Name())); err != nil {
				return err
			}
		}
	} else {
		if err := os.MkdirAll(profileDir, 0755); err != nil {
			return err
		}
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
	st.PutRemote(state.Remote{
		Repo:        repo,
		Path:        path,
		Branch:      branch,
		Scope:       string(scope),
		Profile:     profileName,
		ProjectPath: projectPath,
	})
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
		targets = st.FindRemotesByProfile(profileName)
		if len(targets) == 0 {
			return nil, fmt.Errorf("profile %q has no remote", profileName)
		}
	} else {
		targets = st.Remotes
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no remote-linked profiles found")
	}

	for _, r := range targets {
		name := r.Profile
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
			fmt.Printf("  ⚠ profile %q was removed from the remote repo (path %q no longer exists)\n", name, r.Path)
			fmt.Printf("    → the local copy is still installed but will not receive updates\n")
			fmt.Printf("    → to clean up: cvm remote rm %s && cvm rm %s\n", name, name)
			continue
		}
		if !looksLikeProfile(srcDir) {
			fmt.Printf("  ⚠ path %q exists in repo but no longer looks like a profile\n", r.Path)
			fmt.Printf("    → to clean up: cvm remote rm %s\n", name)
			continue
		}

		scope := config.Scope(r.Scope)
		profileDir := profile.ProfileDir(scope, name)

		// Clear and re-copy
		entries, err := os.ReadDir(profileDir)
		if err != nil && !os.IsNotExist(err) {
			fmt.Printf("  warning: could not inspect profile dir for %s: %v\n", name, err)
			continue
		}
		for _, e := range entries {
			if err := os.RemoveAll(filepath.Join(profileDir, e.Name())); err != nil {
				fmt.Printf("  warning: failed to clear %s before copy: %v\n", name, err)
				continue
			}
		}
		if err := profile.CopyDir(srcDir, profileDir); err != nil {
			fmt.Printf("  warning: copy failed for %s: %v\n", name, err)
			continue
		}

		updated = append(updated, updatedLabel(r))

		// If this profile is currently active, re-apply directly
		// (don't use profile.Use() because it saves ~/.claude/ back to
		// the profile dir first, which would overwrite what we just pulled)
		var active string
		if scope == config.ScopeGlobal {
			active = st.Global.Active
		} else {
			projectPath := resolveLocalProjectPath(st, r)
			if projectPath != "" {
				active = st.GetLocal(projectPath)
				r.ProjectPath = projectPath
			}
		}
		if active == name {
			fmt.Printf("  re-applying active profile %q...\n", name)
			if scope == config.ScopeLocal && r.ProjectPath == "" {
				fmt.Printf("  warning: cannot re-apply local profile %q without a project path\n", name)
				continue
			}
			if err := profile.Reapply(scope, name, r.ProjectPath); err != nil {
				fmt.Printf("  warning: re-apply failed for active profile %s: %v\n", name, err)
			}
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
	if removed := st.RemoveRemotesByProfile(profileName); removed == 0 {
		return fmt.Errorf("profile %q has no remote", profileName)
	}
	return st.Save()
}

func resolveLocalProjectPath(st *state.State, r state.Remote) string {
	if r.ProjectPath != "" {
		return r.ProjectPath
	}

	var match string
	for projectPath, local := range st.Local {
		if local.Active != r.Profile {
			continue
		}
		if match != "" {
			return ""
		}
		match = projectPath
	}
	return match
}

func updatedLabel(r state.Remote) string {
	if r.Scope == string(config.ScopeLocal) {
		return fmt.Sprintf("%s (local)", r.Profile)
	}
	return fmt.Sprintf("%s (global)", r.Profile)
}
