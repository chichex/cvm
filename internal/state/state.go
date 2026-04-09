package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
)

// Remote tracks a profile's git source.
type Remote struct {
	Repo        string `json:"repo"`                   // e.g. "github.com/chichex/cvm"
	Path        string `json:"path"`                   // subdirectory in repo, e.g. "profiles/chiche"
	Branch      string `json:"branch"`                 // e.g. "main"
	Scope       string `json:"scope"`                  // "global" or "local"
	Profile     string `json:"profile"`                // profile name
	ProjectPath string `json:"project_path,omitempty"` // absolute project path for local remotes
}

// State tracks which profiles are active globally and per-project.
type State struct {
	Global  GlobalState           `json:"global"`
	Local   map[string]LocalState `json:"local"`
	Remotes map[string]Remote     `json:"remotes,omitempty"` // key = profile name
}

type GlobalState struct {
	Active string `json:"active"` // empty = vanilla
}

type LocalState struct {
	Active string `json:"active"` // empty = vanilla
}

// Load reads state from disk. Returns empty state if file doesn't exist.
func Load() (*State, error) {
	s := &State{
		Local:   make(map[string]LocalState),
		Remotes: make(map[string]Remote),
	}

	data, err := os.ReadFile(config.StatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}

	if s.Local == nil {
		s.Local = make(map[string]LocalState)
	}
	s.Remotes = normalizeRemotes(s.Remotes)

	return s, nil
}

// Save writes state to disk.
func (s *State) Save() error {
	dir := filepath.Dir(config.StatePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(config.StatePath(), data, 0644)
}

// SetGlobal sets the active global profile.
func (s *State) SetGlobal(name string) {
	s.Global.Active = name
}

// SetLocal sets the active local profile for a project path.
func (s *State) SetLocal(projectPath, name string) {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		abs = projectPath
	}
	s.Local[abs] = LocalState{Active: name}
}

// GetLocal returns the active local profile for a project path.
func (s *State) GetLocal(projectPath string) string {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		abs = projectPath
	}
	if ls, ok := s.Local[abs]; ok {
		return ls.Active
	}
	return ""
}

// ClearLocal removes local state for a project path.
func (s *State) ClearLocal(projectPath string) {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		abs = projectPath
	}
	delete(s.Local, abs)
}

// PutRemote stores a remote using a composite key so global and local remotes
// can safely share the same profile name.
func (s *State) PutRemote(r Remote) {
	if s.Remotes == nil {
		s.Remotes = make(map[string]Remote)
	}
	if r.Profile == "" {
		return
	}
	r.ProjectPath = normalizeProjectPathForScope(r.Scope, r.ProjectPath)
	s.Remotes[remoteKey(r.Scope, r.Profile, r.ProjectPath)] = r
}

// FindRemote returns the remote for an exact scope/profile/project match.
func (s *State) FindRemote(scope config.Scope, profile, projectPath string) (Remote, bool) {
	if s.Remotes == nil {
		return Remote{}, false
	}

	key := remoteKey(string(scope), profile, projectPath)
	if r, ok := s.Remotes[key]; ok {
		return r, true
	}

	// Legacy local remotes were stored without project paths. If there is a
	// single unscoped local match, use it as a best-effort migration path.
	if scope == config.ScopeLocal {
		var match Remote
		matches := 0
		for _, r := range s.Remotes {
			if r.Scope == string(scope) && r.Profile == profile && r.ProjectPath == "" {
				match = r
				matches++
			}
		}
		if matches == 1 {
			return match, true
		}
	}

	return Remote{}, false
}

// FindRemotesByProfile returns every remote linked to a given profile name.
func (s *State) FindRemotesByProfile(profile string) map[string]Remote {
	matches := make(map[string]Remote)
	for key, r := range s.Remotes {
		if r.Profile == profile {
			matches[key] = r
		}
	}
	return matches
}

// RemoveRemotesByProfile removes every remote linked to a given profile name.
func (s *State) RemoveRemotesByProfile(profile string) int {
	removed := 0
	for key, r := range s.Remotes {
		if r.Profile == profile {
			delete(s.Remotes, key)
			removed++
		}
	}
	return removed
}

func normalizeRemotes(remotes map[string]Remote) map[string]Remote {
	normalized := make(map[string]Remote)
	for key, r := range remotes {
		profile := r.Profile
		if profile == "" {
			profile = key
		}
		r.Profile = profile
		r.ProjectPath = normalizeProjectPathForScope(r.Scope, r.ProjectPath)
		normalized[remoteKey(r.Scope, r.Profile, r.ProjectPath)] = r
	}
	return normalized
}

func remoteKey(scope, profile, projectPath string) string {
	return fmt.Sprintf("%s|%s|%s", scope, normalizeProjectPathForScope(scope, projectPath), profile)
}

func normalizeProjectPathForScope(scope, projectPath string) string {
	if scope != string(config.ScopeLocal) {
		return ""
	}
	return normalizeProjectPath(projectPath)
}

func normalizeProjectPath(projectPath string) string {
	if projectPath == "" {
		return ""
	}

	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return projectPath
	}
	return abs
}
