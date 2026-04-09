package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/remote"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull [profile-name]",
	Short: "Pull latest updates for remote-linked profiles",
	Long: `Pull the latest version from the linked git repo.
Without arguments, pulls all remote-linked profiles.
With a name, pulls only that profile.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		updated, err := remote.Pull(name)
		if err != nil {
			return err
		}

		if len(updated) == 0 {
			fmt.Println("No profiles were updated")
		} else {
			fmt.Printf("Updated %d profile(s): %v\n", len(updated), updated)
		}
		return nil
	},
}
