package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/remote"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		local, _ := cmd.Flags().GetBool("local")

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

		if err := profile.Remove(scope, name, projectPath); err != nil {
			return err
		}

		// Also unlink remote if it exists
		remote.Remove(name)

		fmt.Printf("Removed profile %q\n", name)
		return nil
	},
}

func init() {
	rmCmd.Flags().Bool("local", false, "Remove local profile (default: global)")
}
