package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/profile"
	"github.com/spf13/cobra"
)

var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Switch to vanilla (remove profile symlinks)",
	Long: `Remove the symlinks created by the active profile and restore the
vanilla stash (the pre-cvm files moved aside on first use).

By default every harness with an active profile is reverted. Use --harness
to target a single harness. This is equivalent to 'cvm use --none'.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		harnessFlagSet := cmd.Flags().Changed("harness")

		harnesses, err := harnessesForUseNone(cmd, harnessFlagSet)
		if err != nil {
			return err
		}
		if len(harnesses) == 0 {
			fmt.Println("Already vanilla")
			return nil
		}
		for _, h := range harnesses {
			if err := profile.UseNoneWithHarness(h); err != nil {
				return err
			}
			fmt.Printf("Switched %s harness to vanilla\n", h.Name())
		}
		return nil
	},
}

func init() {
	offCmd.Flags().String("harness", "", "Harness to target")
}
