package cmd

import (
	"fmt"

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

		if err := profile.Remove(name); err != nil {
			return err
		}

		// Also unlink remote if it exists
		remote.Remove(name)

		fmt.Printf("Removed profile %q\n", name)
		return nil
	},
}
