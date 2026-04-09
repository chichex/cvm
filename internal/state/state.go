package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ayrtonmarini/cvm/internal/config"
)

// State tracks which profiles are active globally and per-project.
type State struct {
	Global GlobalState            `json:"global"`
	Local  map[string]LocalState  `json:"local"`
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
