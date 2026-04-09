package cmd

import (
	"fmt"

	"github.com/ayrtonmarini/cvm/internal/config"
	"github.com/ayrtonmarini/cvm/internal/profile"
	"github.com/ayrtonmarini/cvm/internal/state"
	"github.com/spf13/cobra"
)

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Remove all managed config from Claude Code",
	Long: `Removes all cvm-managed configuration files from ~/.claude/ and .claude/.
Runtime files (sessions, cache, history) are never touched.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		globalOnly, _ := cmd.Flags().GetBool("global")
		localOnly, _ := cmd.Flags().GetBool("local")
		force, _ := cmd.Flags().GetBool("force")

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
			if err := profile.Nuke(config.ScopeGlobal, ""); err != nil {
				return fmt.Errorf("nuking global: %w", err)
			}
			st.SetGlobal("")
			fmt.Println("Nuked global config (~/.claude/)")
		}

		if localOnly {
			projectPath, err := getProjectPath()
			if err != nil {
				return err
			}
			if err := profile.Nuke(config.ScopeLocal, projectPath); err != nil {
				return fmt.Errorf("nuking local: %w", err)
			}
			st.ClearLocal(projectPath)
			fmt.Println("Nuked local config (.claude/)")
		}

		return st.Save()
	},
}

func init() {
	nukeCmd.Flags().Bool("global", false, "Only nuke global config")
	nukeCmd.Flags().Bool("local", false, "Only nuke local config")
	nukeCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
}
