package cmd

import (
	"github.com/ayrtonmarini/cvm/internal/lifecycle"
	"github.com/spf13/cobra"
)

var lifecycleCmd = &cobra.Command{
	Use:   "lifecycle",
	Short: "Session lifecycle management (used by hooks)",
}

var lifecycleStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Called on session start - loads context and detects tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := getProjectPath()
		if err != nil {
			return err
		}
		return lifecycle.Start(projectPath)
	},
}

var lifecycleEndCmd = &cobra.Command{
	Use:   "end",
	Short: "Called on session end - cleanup",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := getProjectPath()
		if err != nil {
			return err
		}
		return lifecycle.End(projectPath)
	},
}

var lifecycleStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current session info",
	RunE: func(cmd *cobra.Command, args []string) error {
		return lifecycle.Status()
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleStartCmd)
	lifecycleCmd.AddCommand(lifecycleEndCmd)
	lifecycleCmd.AddCommand(lifecycleStatusCmd)
}
