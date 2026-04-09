package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/spf13/cobra"
)

var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Manage local profiles (.claude/ in current project)",
}

func getProjectPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine project path: %w", err)
	}
	return cwd, nil
}

func getProjectName() string {
	cwd, _ := os.Getwd()
	return filepath.Base(cwd)
}

var localInitCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Create a new local profile for the current project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := getProjectPath()
		if err != nil {
			return err
		}
		name := getProjectName()
		if len(args) > 0 {
			name = args[0]
		}
		nameFlag, _ := cmd.Flags().GetString("name")
		if nameFlag != "" {
			name = nameFlag
		}
		from, _ := cmd.Flags().GetString("from")
		if err := profile.Init(config.ScopeLocal, name, from, projectPath); err != nil {
			return err
		}
		fmt.Printf("Created local profile %q\n", name)
		fmt.Printf("  edit at: %s\n", profile.ProfileDir(config.ScopeLocal, name))
		return nil
	},
}

var localUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a local profile for the current project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := getProjectPath()
		if err != nil {
			return err
		}
		none, _ := cmd.Flags().GetBool("none")
		if none {
			if err := profile.UseNone(config.ScopeLocal, projectPath); err != nil {
				return err
			}
			fmt.Println("Switched to vanilla (no local profile)")
			return nil
		}
		if len(args) == 0 {
			return fmt.Errorf("provide a profile name or use --none")
		}
		if err := profile.Use(config.ScopeLocal, args[0], projectPath); err != nil {
			return err
		}
		fmt.Printf("Switched to local profile %q\n", args[0])
		return nil
	},
}

var localLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List local profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := getProjectPath()
		if err != nil {
			return err
		}
		profiles, err := profile.List(config.ScopeLocal, projectPath)
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			fmt.Println("No local profiles. Create one with: cvm local init [name]")
			return nil
		}
		for _, p := range profiles {
			marker := "  "
			if p.Active {
				marker = "* "
			}
			fmt.Printf("%s%-20s (%d items)\n", marker, p.Name, p.Items)
		}
		return nil
	},
}

var localCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show active local profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := getProjectPath()
		if err != nil {
			return err
		}
		name, err := profile.Current(config.ScopeLocal, projectPath)
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

var localSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save current .claude/ state to the active local profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := getProjectPath()
		if err != nil {
			return err
		}
		name, err := profile.Current(config.ScopeLocal, projectPath)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("no active local profile, nothing to save")
		}
		if err := profile.Save(config.ScopeLocal, name, projectPath); err != nil {
			return err
		}
		fmt.Printf("Saved current state to local profile %q\n", name)
		return nil
	},
}

var localRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a local profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := getProjectPath()
		if err != nil {
			return err
		}
		if err := profile.Remove(config.ScopeLocal, args[0], projectPath); err != nil {
			return err
		}
		fmt.Printf("Removed local profile %q\n", args[0])
		return nil
	},
}

func init() {
	localInitCmd.Flags().String("from", "", "Copy from existing profile")
	localInitCmd.Flags().String("name", "", "Profile name (default: project directory name)")
	localUseCmd.Flags().Bool("none", false, "Switch to vanilla (no profile)")

	localCmd.AddCommand(localInitCmd)
	localCmd.AddCommand(localUseCmd)
	localCmd.AddCommand(localLsCmd)
	localCmd.AddCommand(localCurrentCmd)
	localCmd.AddCommand(localSaveCmd)
	localCmd.AddCommand(localRmCmd)
}
