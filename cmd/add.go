package cmd

import (
	"fmt"
	"path"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/remote"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <name> [url]",
	Short: "Add a profile (empty or from a GitHub repo)",
	Long: `Add a new profile. With just a name, creates an empty profile.
With a URL, clones the profile from a GitHub repo and links it for updates.

The URL format is: github.com/user/repo/path/to/profile
(or just user/repo/path — github.com is assumed)

Examples:
  cvm add chiche                                         # empty profile
  cvm add chiche --from work                             # copy from "work"
  cvm add chiche github.com/chichex/cvm/profiles/chiche  # from repo
  cvm add chiche chichex/cvm/profiles/chiche              # shorthand
  cvm add chiche --local                                  # local to project`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		local, _ := cmd.Flags().GetBool("local")
		from, _ := cmd.Flags().GetString("from")

		scope := config.ScopeGlobal
		projectPath := ""
		if local {
			scope = config.ScopeLocal
			var err error
			projectPath, err = getProjectPath()
			if err != nil {
				return err
			}
		}

		// URL provided: clone from remote
		if len(args) == 2 {
			url := args[1]
			repo, repoPath := parseURL(url)

			if err := remote.Add(name, repo, repoPath, "", scope, projectPath); err != nil {
				return err
			}

			fmt.Printf("Added profile %q from %s (%s)\n", name, repo, scope)
			fmt.Printf("  activate: %s\n", useCommand(name, scope))
			fmt.Printf("  update:   cvm pull %s\n", name)
			return nil
		}

		// No URL: create empty or copy from another
		if err := profile.Init(scope, name, from, projectPath); err != nil {
			return err
		}
		fmt.Printf("Created profile %q (%s)\n", name, scope)
		fmt.Printf("  activate: %s\n", useCommand(name, scope))
		return nil
	},
}

// parseURL normalizes any GitHub URL format into ("user/repo", "path/inside/repo").
//
// Supported formats:
//
//	chichex/cvm/profiles/chiche
//	github.com/chichex/cvm/profiles/chiche
//	https://github.com/chichex/cvm/profiles/chiche
//	git@github.com:chichex/cvm.git/profiles/chiche
//	git@github.com:chichex/cvm/profiles/chiche
func parseURL(url string) (repo, repoPath string) {
	// Handle git@github.com:user/repo[.git][/path]
	if strings.HasPrefix(url, "git@") {
		// git@github.com:chichex/cvm.git/profiles/chiche
		url = strings.TrimPrefix(url, "git@")
		if idx := strings.Index(url, ":"); idx >= 0 {
			url = url[idx+1:] // strip host:
		}
	}

	// Strip common prefixes/suffixes
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "ssh://")
	url = strings.TrimPrefix(url, "github.com/")
	url = strings.TrimSuffix(url, "/")

	// Remove .git from the repo segment (could be mid-path: user/repo.git/path)
	url = strings.Replace(url, ".git/", "/", 1)
	url = strings.TrimSuffix(url, ".git")

	parts := strings.SplitN(url, "/", 3)
	if len(parts) < 2 {
		return url, ""
	}
	repo = parts[0] + "/" + parts[1]
	if len(parts) == 3 {
		repoPath = parts[2]
	}
	_ = path.Base
	return
}

func init() {
	addCmd.Flags().Bool("local", false, "Create as local profile (default: global)")
	addCmd.Flags().String("from", "", "Copy from existing profile")
}

func useCommand(name string, scope config.Scope) string {
	if scope == config.ScopeLocal {
		return fmt.Sprintf("cvm use %s --local", name)
	}
	return fmt.Sprintf("cvm use %s", name)
}
