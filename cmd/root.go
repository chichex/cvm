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
	Long: `cvm manages Claude Code configuration profiles at global (~/.claude/)
and local (.claude/) levels. Switch configs instantly, nuke everything,
restore to vanilla. Like nvm but for your Claude Code setup.`,
	Version: Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Primary commands
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(useCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(nukeCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(bypassCmd)

	// Subsystems
	rootCmd.AddCommand(remoteCmd)
	rootCmd.AddCommand(overrideCmd)

	// Legacy (still work, but simplified API is preferred)
	rootCmd.AddCommand(globalCmd)
	rootCmd.AddCommand(localCmd)
}
