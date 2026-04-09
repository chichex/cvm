package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Inspect active profile contents",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		globalName, err := profile.Current(config.ScopeGlobal, "")
		if err != nil {
			return err
		}
		localName, err := profile.Current(config.ScopeLocal, cwd)
		if err != nil {
			return err
		}

		if globalName == "" && localName == "" {
			fmt.Println("No active profiles")
			return nil
		}

		if globalName != "" {
			if err := printInventory(config.ScopeGlobal, globalName, ""); err != nil {
				return err
			}
		} else {
			fmt.Println("Global profile: (vanilla)")
		}

		fmt.Println()

		if localName != "" {
			if err := printInventory(config.ScopeLocal, localName, cwd); err != nil {
				return err
			}
		} else {
			fmt.Println("Local profile: (vanilla)")
		}

		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Inspect a specific stored profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		isLocal, _ := cmd.Flags().GetBool("local")
		scope := config.ScopeGlobal
		projectPath := ""
		if isLocal {
			scope = config.ScopeLocal
			var err error
			projectPath, err = getProjectPath()
			if err != nil {
				return err
			}
		}

		return printInventory(scope, args[0], projectPath)
	},
}

func init() {
	profileShowCmd.Flags().Bool("local", false, "Inspect a local profile (default: global)")
	profileCmd.AddCommand(profileShowCmd)
	rootCmd.AddCommand(profileCmd)
}

func printInventory(scope config.Scope, name, projectPath string) error {
	inv, err := profile.Inspect(scope, name, projectPath)
	if err != nil {
		return err
	}
	label := titleWord(string(scope))
	if !inv.Exists {
		fmt.Printf("%s profile %q not found\n", label, name)
		return nil
	}

	fmt.Printf("%s profile: %s\n", label, inv.Name)
	fmt.Printf("Path: %s\n", inv.Path)

	if len(inv.Files) == 0 && len(inv.Dirs) == 0 {
		fmt.Println("Contents: empty")
		return nil
	}

	if len(inv.Files) > 0 {
		fmt.Printf("Files: %s\n", strings.Join(inv.Files, ", "))
	}

	keys := make([]string, 0, len(inv.Dirs))
	for key := range inv.Dirs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		children := inv.Dirs[key]
		fmt.Printf("%s (%d): %s\n", titleWord(key), len(children), strings.Join(children, ", "))
	}

	if len(inv.MCPServers) > 0 {
		fmt.Printf("MCP servers (%d): %s\n", len(inv.MCPServers), strings.Join(inv.MCPServers, ", "))
	}

	return nil
}

func titleWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
