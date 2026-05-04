package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active profiles by harness",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.Load()
		if err != nil {
			return err
		}
		harnesses, err := selectedHarnesses(cmd)
		if err != nil {
			return err
		}

		for _, h := range harnesses {
			active := st.GetGlobalHarness(h.Name())
			if active == "" {
				active = "(vanilla)"
			}

			fmt.Printf("%s harness: %-20s -> %s\n", h.Name(), active, h.TargetDir())
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().String("harness", "", "Only show one harness")
}
