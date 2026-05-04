package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
)

// Remote tracks a profile's git source.
type Remote struct {
	Repo        string `json:"repo"`                   // e.g. "github.com/chichex/cvm"
	Path        string `json:"path"`                   // subdirectory in repo, e.g. "profiles/chiche"
	Branch      string `json:"branch"`                 // e.g. "main"
	Profile     string `json:"profile"`                // profile name
	ProjectPath string `json:"project_path,omitempty"` // legacy, ignored
}

// State tracks which profiles are active.
type State struct {
	Global  GlobalState       `json:"global"`
	Local   map[string]any    `json:"local,omitempty"`   // legacy, ignored
	Remotes map[string]Remote `json:"remotes,omitempty"` // key = profile name
}

type GlobalState struct {
	Active    string            `json:"active,omitempty"`    // legacy Claude active profile; empty = vanilla
	Harnesses map[string]string `json:"harnesses,omitempty"` // use State setters to keep Active mirror in sync
}

// Load reads state from disk. Returns empty state if file doesn't exist.
func Load() (*State, error) {
	s := &State{
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

	s.Global.normalize()
	s.Remotes = normalizeRemotes(s.Remotes)
	s.Local = nil

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

// SetGlobal sets the active profile.
func (s *State) SetGlobal(name string) {
	s.SetGlobalHarness("claude", name)
}

// SetGlobalHarness sets the active profile for a harness.
func (s *State) SetGlobalHarness(harnessName, name string) {
	s.Global.setHarness(harnessName, name)
}

// GetGlobalHarness returns the active profile for a harness.
func (s *State) GetGlobalHarness(harnessName string) string {
	return s.Global.getHarness(harnessName)
}

// ClearGlobalHarness clears the active profile for a harness.
func (s *State) ClearGlobalHarness(harnessName string) {
	s.SetGlobalHarness(harnessName, "")
}

// PutRemote stores a remote by profile name.
func (s *State) PutRemote(r Remote) {
	if s.Remotes == nil {
		s.Remotes = make(map[string]Remote)
	}
	if r.Profile == "" {
		return
	}
	r.ProjectPath = ""
	s.Remotes[r.Profile] = r
}

// FindRemote returns the remote for a profile.
func (s *State) FindRemote(profile string) (Remote, bool) {
	if s.Remotes == nil {
		return Remote{}, false
	}
	r, ok := s.Remotes[profile]
	return r, ok
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
		r.ProjectPath = ""
		normalized[r.Profile] = r
	}
	return normalized
}

func (gs *GlobalState) normalize() {
	gs.Harnesses = normalizeHarnesses(gs.Active, gs.Harnesses)
	gs.Active = gs.Harnesses["claude"]
}

func (gs *GlobalState) getHarness(harnessName string) string {
	if gs.Harnesses != nil {
		return gs.Harnesses[harnessName]
	}
	if harnessName == "claude" {
		return gs.Active
	}
	return ""
}

func (gs *GlobalState) setHarness(harnessName, name string) {
	gs.Harnesses = setHarness(gs.Active, gs.Harnesses, harnessName, name)
	gs.Active = gs.Harnesses["claude"]
}

func normalizeHarnesses(legacyActive string, harnesses map[string]string) map[string]string {
	normalized := make(map[string]string)
	for harnessName, active := range harnesses {
		if active != "" {
			normalized[harnessName] = active
		}
	}
	if _, ok := normalized["claude"]; !ok && legacyActive != "" {
		normalized["claude"] = legacyActive
	}
	return normalized
}

func setHarness(legacyActive string, harnesses map[string]string, harnessName, name string) map[string]string {
	normalized := normalizeHarnesses(legacyActive, harnesses)
	if name == "" {
		delete(normalized, harnessName)
		return normalized
	}
	normalized[harnessName] = name
	return normalized
}
