package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/settings"
	"github.com/spf13/cobra"
)

var bypassCmd = &cobra.Command{
	Use:   "bypass [on|off|status]",
	Short: "Inspect or change bypass permissions for active profiles",
	Long:  `Bypass permissions are persisted as overrides, so they survive 'cvm pull'.`,
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
			return enableBypass(cmd)
		case "off":
			return disableBypass(cmd)
		default:
			return fmt.Errorf("use 'on', 'off', or 'status'")
		}
	},
}

func init() {
	bypassCmd.Flags().Bool("global", false, "Affect the active global profile")
	bypassCmd.Flags().Bool("local", false, "Affect the active local profile")
}

type bypassTarget struct {
	scope       config.Scope
	profileName string
	overrideDir string
	projectPath string
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
		overrideSettings := filepath.Join(target.overrideDir, "settings.json")
		mode, err := settings.GetPermissionsMode(overrideSettings)
		if err != nil {
			mode = ""
		}
		if mode == "" {
			mode = "(default)"
		}
		fmt.Printf("%s profile %q: %s\n", target.scope, target.profileName, mode)
	}
	return nil
}

func enableBypass(cmd *cobra.Command) error {
	targets, err := activeBypassTargets(cmd)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no active profiles")
	}

	for _, target := range targets {
		overrideSettings := filepath.Join(target.overrideDir, "settings.json")

		cfg, err := settings.Read(overrideSettings)
		if err != nil {
			return err
		}

		bypassCfg := settings.BypassConfig()
		for k, v := range bypassCfg {
			cfg[k] = v
		}

		if err := os.MkdirAll(target.overrideDir, 0755); err != nil {
			return err
		}
		if err := settings.Write(overrideSettings, cfg); err != nil {
			return err
		}

		if err := profile.Reapply(target.scope, target.profileName, target.projectPath); err != nil {
			return err
		}
		fmt.Printf("%s profile %q: bypassPermissions\n", target.scope, target.profileName)
	}

	return nil
}

func disableBypass(cmd *cobra.Command) error {
	targets, err := activeBypassTargets(cmd)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no active profiles")
	}

	for _, target := range targets {
		overrideSettings := filepath.Join(target.overrideDir, "settings.json")

		cfg, err := settings.Read(overrideSettings)
		if err != nil {
			return err
		}

		if settings.RemovePermissions(cfg) {
			os.Remove(overrideSettings)
		} else {
			if err := settings.Write(overrideSettings, cfg); err != nil {
				return err
			}
		}

		if err := profile.Reapply(target.scope, target.profileName, target.projectPath); err != nil {
			return err
		}
		fmt.Printf("%s profile %q: default\n", target.scope, target.profileName)
	}

	return nil
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
			overrideDir: profile.OverrideDir(selection.scope, name, selection.projectPath),
			projectPath: selection.projectPath,
		})
	}

	return targets, nil
}
