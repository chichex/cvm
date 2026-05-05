package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/profile"
	"github.com/spf13/cobra"
)

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := profile.Current()
		if err != nil {
			return err
		}
		if name == "" {
			fmt.Println("(vanilla)")
		} else {
			fmt.Println(name)
		}
		return nil
	},
}

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save current harness state to the active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := profile.Current()
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("no active profile, nothing to save")
		}
		if err := profile.Save(name); err != nil {
			return err
		}
		fmt.Printf("Saved current state to profile %q\n", name)
		return nil
	},
}
