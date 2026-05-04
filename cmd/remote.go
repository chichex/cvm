package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/remote"
	"github.com/spf13/cobra"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote profile sources (GitHub repos)",
}

var remoteAddCmd = &cobra.Command{
	Use:   "add <profile-name> <repo> [path]",
	Short: "Link a profile to a GitHub repo",
	Long: `Clone a profile from a git repo and link it for updates.

Examples:
  cvm remote add chiche chichex/cvm profiles/chiche
  cvm remote add myconfig github.com/user/claude-profiles profiles/work
  cvm remote add dotfiles git@github.com:user/dotfiles.git claude-profile`,
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		repo := args[1]
		path := ""
		if len(args) > 2 {
			path = args[2]
		}
		branch, _ := cmd.Flags().GetString("branch")

		if err := remote.Add(name, repo, path, branch); err != nil {
			return err
		}

		fmt.Printf("Linked profile %q to %s (path: %s)\n", name, repo, path)
		fmt.Printf("Use: %s\n", useCommand(name))
		fmt.Printf("Update: cvm pull %s\n", name)
		return nil
	},
}

var remoteLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List remote-linked profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		remotes, err := remote.List()
		if err != nil {
			return err
		}
		if len(remotes) == 0 {
			fmt.Println("No remote-linked profiles")
			return nil
		}
		for name, r := range remotes {
			fmt.Printf("  %-20s %s -> %s\n", name, r.Repo, r.Path)
		}
		return nil
	},
}

var remoteRmCmd = &cobra.Command{
	Use:   "rm <profile-name>",
	Short: "Unlink a profile from its remote (keeps local copy)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := remote.Remove(args[0]); err != nil {
			return err
		}
		fmt.Printf("Unlinked profile %q from remote (local copy kept)\n", args[0])
		return nil
	},
}

func init() {
	remoteAddCmd.Flags().String("branch", "main", "Git branch")

	remoteCmd.AddCommand(remoteAddCmd)
	remoteCmd.AddCommand(remoteLsCmd)
	remoteCmd.AddCommand(remoteRmCmd)
}
