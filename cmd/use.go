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
	Long: `Activate a profile for a harness. By default switches global Claude Code config (~/.claude/).
Use --local to switch the project-level config (.claude/).
Use --harness to target a specific harness.
Use --none to go back to vanilla.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		local, _ := cmd.Flags().GetBool("local")
		none, _ := cmd.Flags().GetBool("none")
		h, err := harnessFromFlag(cmd)
		if err != nil {
			return err
		}

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
			if err := profile.UseNoneWithHarness(scope, projectPath, h); err != nil {
				return err
			}
			fmt.Printf("Switched %s harness to vanilla (%s)\n", h.Name(), scope)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("provide a profile name or use --none")
		}

		name := args[0]
		if err := profile.UseWithHarness(scope, name, projectPath, h); err != nil {
			return err
		}
		fmt.Printf("Switched %s harness to %q (%s)\n", h.Name(), name, scope)
		return nil
	},
}

func init() {
	useCmd.Flags().Bool("local", false, "Switch local profile (default: global)")
	useCmd.Flags().Bool("none", false, "Switch to vanilla (no profile)")
	useCmd.Flags().String("harness", "", "Harness to target")
}
