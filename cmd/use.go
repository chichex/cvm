package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/profile"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a profile",
	Long: `Activate a profile for a harness.
Use --harness to target a specific harness.
Use --none to go back to vanilla.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		none, _ := cmd.Flags().GetBool("none")
		h, err := harnessFromFlag(cmd)
		if err != nil {
			return err
		}

		if none {
			if err := profile.UseNoneWithHarness(h); err != nil {
				return err
			}
			fmt.Printf("Switched %s harness to vanilla\n", h.Name())
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("provide a profile name or use --none")
		}

		name := args[0]
		if err := profile.UseWithHarness(name, h); err != nil {
			return err
		}
		fmt.Printf("Switched %s harness to %q\n", h.Name(), name)
		return nil
	},
}

func init() {
	useCmd.Flags().Bool("none", false, "Switch to vanilla (no profile)")
	useCmd.Flags().String("harness", "", "Harness to target")
}
