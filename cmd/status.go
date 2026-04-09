package cmd

import (
	"fmt"
	"os"

	"github.com/ayrtonmarini/cvm/internal/config"
	"github.com/ayrtonmarini/cvm/internal/state"
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
		fmt.Printf("Global:  %-20s  → %s\n", globalProfile, config.ClaudeHome())

		// Local
		cwd, _ := os.Getwd()
		localProfile := st.GetLocal(cwd)
		if localProfile == "" {
			localProfile = "(vanilla)"
		}
		fmt.Printf("Local:   %-20s  → %s/.claude\n", localProfile, cwd)

		return nil
	},
}
