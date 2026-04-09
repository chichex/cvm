package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
)

// Remote tracks a profile's git source.
type Remote struct {
	Repo    string `json:"repo"`    // e.g. "github.com/chichex/cvm"
	Path    string `json:"path"`    // subdirectory in repo, e.g. "profiles/chiche"
	Branch  string `json:"branch"`  // e.g. "main"
	Scope   string `json:"scope"`   // "global" or "local"
	Profile string `json:"profile"` // profile name
}

// State tracks which profiles are active globally and per-project.
type State struct {
	Global  GlobalState            `json:"global"`
	Local   map[string]LocalState  `json:"local"`
	Remotes map[string]Remote      `json:"remotes,omitempty"` // key = profile name
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
		Local: make(map[string]LocalState),
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
	if s.Remotes == nil {
		s.Remotes = make(map[string]Remote)
	}

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
