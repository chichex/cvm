package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/settings"
	"github.com/spf13/cobra"
)

var bypassCmd = &cobra.Command{
	Use:   "bypass [on|off|status]",
	Short: "Inspect or change bypass permissions for active profiles",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := "status"
		if len(args) > 0 {
			mode = args[0]
		}

		switch mode {
		case "status":
			return showBypassStatus(cmd)
		case "on":
			return setBypassMode(cmd, settings.BypassPermissionsMode)
		case "off":
			return setBypassMode(cmd, settings.DefaultMode)
		default:
			return fmt.Errorf("use 'on', 'off', or 'status'")
		}
	},
}

func init() {
	bypassCmd.Flags().Bool("global", false, "Affect the active global profile")
	bypassCmd.Flags().Bool("local", false, "Affect the active local profile")
}

func showBypassStatus(cmd *cobra.Command) error {
	targets, err := activeBypassTargets(cmd)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Println("No active profiles")
		return nil
	}

	for _, target := range targets {
		mode, err := settings.GetPermissionsMode(filepath.Join(target.targetDir, "settings.json"))
		if err != nil {
			return err
		}
		if mode == "" {
			mode = "(unset)"
		}
		fmt.Printf("%s profile %q: %s\n", target.scope, target.profileName, mode)
	}
	return nil
}

func setBypassMode(cmd *cobra.Command, mode string) error {
	targets, err := activeBypassTargets(cmd)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no active profiles")
	}

	for _, target := range targets {
		profileSettings := filepath.Join(target.profileDir, "settings.json")
		targetSettings := filepath.Join(target.targetDir, "settings.json")
		if err := settings.SetPermissionsMode(profileSettings, mode); err != nil {
			return err
		}
		if err := settings.SetPermissionsMode(targetSettings, mode); err != nil {
			return err
		}
		fmt.Printf("%s profile %q: %s\n", target.scope, target.profileName, mode)
	}

	return nil
}

type bypassTarget struct {
	scope       config.Scope
	profileName string
	profileDir  string
	targetDir   string
}

func activeBypassTargets(cmd *cobra.Command) ([]bypassTarget, error) {
	useGlobal, _ := cmd.Flags().GetBool("global")
	useLocal, _ := cmd.Flags().GetBool("local")

	projectPath, err := getProjectPath()
	if err != nil {
		return nil, err
	}

	type scopeSelection struct {
		scope       config.Scope
		projectPath string
	}

	var selections []scopeSelection
	switch {
	case useGlobal && useLocal:
		selections = []scopeSelection{
			{scope: config.ScopeGlobal, projectPath: ""},
			{scope: config.ScopeLocal, projectPath: projectPath},
		}
	case useGlobal:
		selections = []scopeSelection{{scope: config.ScopeGlobal, projectPath: ""}}
	case useLocal:
		selections = []scopeSelection{{scope: config.ScopeLocal, projectPath: projectPath}}
	default:
		globalName, err := profile.Current(config.ScopeGlobal, "")
		if err != nil {
			return nil, err
		}
		localName, err := profile.Current(config.ScopeLocal, projectPath)
		if err != nil {
			return nil, err
		}

		if globalName != "" {
			selections = append(selections, scopeSelection{scope: config.ScopeGlobal, projectPath: ""})
		}
		if localName != "" {
			selections = append(selections, scopeSelection{scope: config.ScopeLocal, projectPath: projectPath})
		}
	}

	var targets []bypassTarget
	for _, selection := range selections {
		name, err := profile.Current(selection.scope, selection.projectPath)
		if err != nil {
			return nil, err
		}
		if name == "" {
			continue
		}
		targets = append(targets, bypassTarget{
			scope:       selection.scope,
			profileName: name,
			profileDir:  profile.ProfileDir(selection.scope, name),
			targetDir:   targetDirForScope(selection.scope, selection.projectPath),
		})
	}

	return targets, nil
}

func targetDirForScope(scope config.Scope, projectPath string) string {
	if scope == config.ScopeGlobal {
		return config.ClaudeHome()
	}
	return config.ProjectClaudeDir(projectPath)
}
