package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "cvm",
	Short: "Claude Version Manager - profile manager for Claude Code",
	Long: `cvm manages user-level agent harness configuration profiles.
Switch configs instantly, restore to vanilla.
Like nvm but for your agent harness setup.`,
	Version: Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Core profile management
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(useCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(rmCmd)

	// Core operations
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(bypassCmd)
	rootCmd.AddCommand(upgradeCmd)

}
