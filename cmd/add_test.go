package cmd

import "testing"

func TestParseURL(t *testing.T) {
	tests := []struct {
		input    string
		wantRepo string
		wantPath string
	}{
		// Shorthand
		{"chichex/cvm/profiles/chiche", "chichex/cvm", "profiles/chiche"},
		// With github.com
		{"github.com/chichex/cvm/profiles/chiche", "chichex/cvm", "profiles/chiche"},
		// HTTPS
		{"https://github.com/chichex/cvm/profiles/chiche", "chichex/cvm", "profiles/chiche"},
		// SSH
		{"git@github.com:chichex/cvm/profiles/chiche", "chichex/cvm", "profiles/chiche"},
		// SSH with .git
		{"git@github.com:chichex/cvm.git/profiles/chiche", "chichex/cvm", "profiles/chiche"},
		// HTTPS with .git
		{"https://github.com/chichex/cvm.git/profiles/chiche", "chichex/cvm", "profiles/chiche"},
		// Repo only (no path)
		{"chichex/cvm", "chichex/cvm", ""},
		// Repo with .git suffix only
		{"github.com/chichex/cvm.git", "chichex/cvm", ""},
		// Trailing slash
		{"github.com/chichex/cvm/profiles/chiche/", "chichex/cvm", "profiles/chiche"},
		// Deep path
		{"chichex/configs/profiles/work/v2", "chichex/configs", "profiles/work/v2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			repo, path := parseURL(tt.input)
			if repo != tt.wantRepo {
				t.Errorf("parseURL(%q) repo = %q, want %q", tt.input, repo, tt.wantRepo)
			}
			if path != tt.wantPath {
				t.Errorf("parseURL(%q) path = %q, want %q", tt.input, path, tt.wantPath)
			}
		})
	}
}
