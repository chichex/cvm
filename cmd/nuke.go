package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Remove managed config from harness targets",
	Long: `Removes cvm-managed configuration files from harness target directories.
Runtime files (sessions, cache, history) are never touched.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		globalOnly, _ := cmd.Flags().GetBool("global")
		localOnly, _ := cmd.Flags().GetBool("local")
		force, _ := cmd.Flags().GetBool("force")
		harnessName, _ := cmd.Flags().GetString("harness")
		harnesses, err := selectedHarnesses(cmd)
		if err != nil {
			return err
		}

		if !globalOnly && !localOnly {
			globalOnly = true
			localOnly = true
		}

		if !force {
			fmt.Print("This will remove all cvm-managed config. Continue? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				fmt.Println("Aborted")
				return nil
			}
		}

		st, err := state.Load()
		if err != nil {
			return err
		}

		if globalOnly {
			for _, h := range harnesses {
				if err := profile.NukeWithHarness(config.ScopeGlobal, "", h); err != nil {
					return fmt.Errorf("nuking global %s: %w", h.Name(), err)
				}
				st.ClearGlobalHarness(h.Name())
				fmt.Printf("Nuked global config (%s harness: %s)\n", h.Name(), h.TargetDir(config.ScopeGlobal, ""))
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
				if err := profile.NukeWithHarness(config.ScopeLocal, projectPath, h); err != nil {
					return fmt.Errorf("nuking local %s: %w", h.Name(), err)
				}
				st.ClearLocalHarness(projectPath, h.Name())
				fmt.Printf("Nuked local config (%s harness: %s)\n", h.Name(), h.TargetDir(config.ScopeLocal, projectPath))
			}
		}

		return st.Save()
	},
}

func init() {
	nukeCmd.Flags().Bool("global", false, "Only nuke global config")
	nukeCmd.Flags().Bool("local", false, "Only nuke local config")
	nukeCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	nukeCmd.Flags().String("harness", "", "Only nuke one harness")
}
