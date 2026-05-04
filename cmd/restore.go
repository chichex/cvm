package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore vanilla backup (pre-cvm state)",
	RunE: func(cmd *cobra.Command, args []string) error {
		harnesses, err := selectedHarnesses(cmd)
		if err != nil {
			return err
		}

		st, err := state.Load()
		if err != nil {
			return err
		}

		for _, h := range harnesses {
			if !profile.HasVanillaWithHarness(h) {
				fmt.Printf("No vanilla backup found for %s config\n", h.Name())
			} else {
				if err := profile.NukeWithHarness(h); err != nil {
					return fmt.Errorf("nuking %s: %w", h.Name(), err)
				}
				if err := profile.RestoreVanillaWithHarness(h); err != nil {
					return fmt.Errorf("restoring %s: %w", h.Name(), err)
				}
				st.ClearGlobalHarness(h.Name())
				fmt.Printf("Restored config to vanilla (%s harness)\n", h.Name())
			}
		}

		return st.Save()
	},
}

func init() {
	restoreCmd.Flags().String("harness", "", "Only restore one harness")
}
