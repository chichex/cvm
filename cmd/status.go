package cmd

import (
	"fmt"
	"os"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active profiles by harness (global + local)",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.Load()
		if err != nil {
			return err
		}
		harnesses, err := selectedHarnesses(cmd)
		if err != nil {
			return err
		}

		cwd, _ := os.Getwd()
		for _, h := range harnesses {
			globalProfile := st.GetGlobalHarness(h.Name())
			if globalProfile == "" {
				globalProfile = "(vanilla)"
			}
			localProfile := st.GetLocalHarness(cwd, h.Name())
			if localProfile == "" {
				localProfile = "(vanilla)"
			}

			fmt.Printf("%s harness:\n", h.Name())
			fmt.Printf("  Global: %-20s  -> %s\n", globalProfile, h.TargetDir(config.ScopeGlobal, ""))
			fmt.Printf("  Local:  %-20s  -> %s\n", localProfile, h.TargetDir(config.ScopeLocal, cwd))
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().String("harness", "", "Only show one harness")
}
