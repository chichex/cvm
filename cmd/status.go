package cmd

import (
	"fmt"
	"os"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/harness"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active profiles (global + local)",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.Load()
		if err != nil {
			return err
		}

		// Global
		globalProfile := st.Global.Active
		if globalProfile == "" {
			globalProfile = "(vanilla)"
		}
		fmt.Printf("Global:  %-20s  → %s\n", globalProfile, harness.Claude().TargetDir(config.ScopeGlobal, ""))

		// Local
		cwd, _ := os.Getwd()
		localProfile := st.GetLocal(cwd)
		if localProfile == "" {
			localProfile = "(vanilla)"
		}
		fmt.Printf("Local:   %-20s  → %s\n", localProfile, harness.Claude().TargetDir(config.ScopeLocal, cwd))

		return nil
	},
}
