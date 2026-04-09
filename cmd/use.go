package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a profile",
	Long: `Activate a profile. By default switches global (~/.claude/).
Use --local to switch the project-level profile (.claude/).
Use --none to go back to vanilla.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		local, _ := cmd.Flags().GetBool("local")
		none, _ := cmd.Flags().GetBool("none")

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

		if none {
			if err := profile.UseNone(scope, projectPath); err != nil {
				return err
			}
			fmt.Printf("Switched to vanilla (%s)\n", scope)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("provide a profile name or use --none")
		}

		name := args[0]
		if err := profile.Use(scope, name, projectPath); err != nil {
			return err
		}
		fmt.Printf("Switched to %q (%s)\n", name, scope)
		return nil
	},
}

func init() {
	useCmd.Flags().Bool("local", false, "Switch local profile (default: global)")
	useCmd.Flags().Bool("none", false, "Switch to vanilla (no profile)")
}
