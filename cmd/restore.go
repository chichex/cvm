package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore vanilla backup (pre-cvm state)",
	RunE: func(cmd *cobra.Command, args []string) error {
		globalOnly, _ := cmd.Flags().GetBool("global")
		localOnly, _ := cmd.Flags().GetBool("local")
		harnessName, _ := cmd.Flags().GetString("harness")
		harnesses, err := selectedHarnesses(cmd)
		if err != nil {
			return err
		}

		if !globalOnly && !localOnly {
			globalOnly = true
			localOnly = true
		}

		st, err := state.Load()
		if err != nil {
			return err
		}

		if globalOnly {
			for _, h := range harnesses {
				if !profile.HasVanillaWithHarness(config.ScopeGlobal, "", h) {
					fmt.Printf("No vanilla backup found for global %s config\n", h.Name())
				} else {
					if err := profile.NukeWithHarness(config.ScopeGlobal, "", h); err != nil {
						return fmt.Errorf("nuking global %s: %w", h.Name(), err)
					}
					if err := profile.RestoreVanillaWithHarness(config.ScopeGlobal, "", h); err != nil {
						return fmt.Errorf("restoring global %s: %w", h.Name(), err)
					}
					st.ClearGlobalHarness(h.Name())
					fmt.Printf("Restored global config to vanilla (%s harness)\n", h.Name())
				}
			}
		}

		if localOnly {
			projectPath, err := getProjectPath()
			if err != nil {
				return err
			}
			for _, h := range harnesses {
				if !h.SupportsScope(config.ScopeLocal) {
					if harnessName != "" {
						return fmt.Errorf("%s harness does not support local scope", h.Name())
					}
					continue
				}
				if !profile.HasVanillaWithHarness(config.ScopeLocal, projectPath, h) {
					fmt.Printf("No vanilla backup found for local %s config\n", h.Name())
				} else {
					if err := profile.NukeWithHarness(config.ScopeLocal, projectPath, h); err != nil {
						return fmt.Errorf("nuking local %s: %w", h.Name(), err)
					}
					if err := profile.RestoreVanillaWithHarness(config.ScopeLocal, projectPath, h); err != nil {
						return fmt.Errorf("restoring local %s: %w", h.Name(), err)
					}
					st.ClearLocalHarness(projectPath, h.Name())
					fmt.Printf("Restored local config to vanilla (%s harness)\n", h.Name())
				}
			}
		}

		return st.Save()
	},
}

func init() {
	restoreCmd.Flags().Bool("global", false, "Only restore global")
	restoreCmd.Flags().Bool("local", false, "Only restore local")
	restoreCmd.Flags().String("harness", "", "Only restore one harness")
}
