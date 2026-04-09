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
	rootCmd.AddCommand(globalCmd)
	rootCmd.AddCommand(localCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(nukeCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(kbCmd)
	rootCmd.AddCommand(lifecycleCmd)
	rootCmd.AddCommand(remoteCmd)
	rootCmd.AddCommand(pullCmd)
}
