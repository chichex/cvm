package cmd

import (
	"fmt"

	"github.com/ayrtonmarini/cvm/internal/config"
	"github.com/ayrtonmarini/cvm/internal/profile"
	"github.com/ayrtonmarini/cvm/internal/state"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore vanilla backup (pre-cvm state)",
	RunE: func(cmd *cobra.Command, args []string) error {
		globalOnly, _ := cmd.Flags().GetBool("global")
		localOnly, _ := cmd.Flags().GetBool("local")

		if !globalOnly && !localOnly {
			globalOnly = true
			localOnly = true
		}

		st, err := state.Load()
		if err != nil {
			return err
		}

		if globalOnly {
			if !profile.HasVanilla(config.ScopeGlobal, "") {
				fmt.Println("No vanilla backup found for global config")
			} else {
				profile.Nuke(config.ScopeGlobal, "")
				if err := profile.RestoreVanilla(config.ScopeGlobal, ""); err != nil {
					return fmt.Errorf("restoring global: %w", err)
				}
				st.SetGlobal("")
				fmt.Println("Restored global config to vanilla")
			}
		}

		if localOnly {
			projectPath, err := getProjectPath()
			if err != nil {
				return err
			}
			if !profile.HasVanilla(config.ScopeLocal, projectPath) {
				fmt.Println("No vanilla backup found for local config")
			} else {
				profile.Nuke(config.ScopeLocal, projectPath)
				if err := profile.RestoreVanilla(config.ScopeLocal, projectPath); err != nil {
					return fmt.Errorf("restoring local: %w", err)
				}
				st.ClearLocal(projectPath)
				fmt.Println("Restored local config to vanilla")
			}
		}

		return st.Save()
	},
}

func init() {
	restoreCmd.Flags().Bool("global", false, "Only restore global")
	restoreCmd.Flags().Bool("local", false, "Only restore local")
}
