package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
}

type bypassTarget struct {
	profileName string
	overrideDir string
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
		fmt.Printf("profile %q: %s\n", target.profileName, mode)
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

		if err := profile.Reapply(target.profileName); err != nil {
			return err
		}
		fmt.Printf("profile %q: bypassPermissions\n", target.profileName)
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

		if err := profile.Reapply(target.profileName); err != nil {
			return err
		}
		fmt.Printf("profile %q: default\n", target.profileName)
	}

	return nil
}

func activeBypassTargets(cmd *cobra.Command) ([]bypassTarget, error) {
	name, err := profile.Current()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, nil
	}

	return []bypassTarget{{profileName: name, overrideDir: profile.OverrideDir(name)}}, nil
}
