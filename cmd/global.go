package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ayrtonmarini/cvm/internal/config"
	"github.com/ayrtonmarini/cvm/internal/profile"
	"github.com/spf13/cobra"
)

var globalCmd = &cobra.Command{
	Use:   "global",
	Short: "Manage global profiles (~/.claude/)",
}

var globalInitCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Create a new global profile",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := "default"
		if len(args) > 0 {
			name = args[0]
		}
		from, _ := cmd.Flags().GetString("from")
		if err := profile.Init(config.ScopeGlobal, name, from, ""); err != nil {
			return err
		}
		fmt.Printf("Created global profile %q\n", name)
		fmt.Printf("  edit at: %s\n", profile.ProfileDir(config.ScopeGlobal, name))
		return nil
	},
}

var globalUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a global profile",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		none, _ := cmd.Flags().GetBool("none")
		if none {
			if err := profile.UseNone(config.ScopeGlobal, ""); err != nil {
				return err
			}
			fmt.Println("Switched to vanilla (no global profile)")
			return nil
		}
		if len(args) == 0 {
			return fmt.Errorf("provide a profile name or use --none")
		}
		if err := profile.Use(config.ScopeGlobal, args[0], ""); err != nil {
			return err
		}
		fmt.Printf("Switched to global profile %q\n", args[0])
		return nil
	},
}

var globalLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List global profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := profile.List(config.ScopeGlobal, "")
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			fmt.Println("No global profiles. Create one with: cvm global init <name>")
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

var globalCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show active global profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := profile.Current(config.ScopeGlobal, "")
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

var globalSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save current ~/.claude/ state to the active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := profile.Current(config.ScopeGlobal, "")
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("no active global profile, nothing to save")
		}
		if err := profile.Save(config.ScopeGlobal, name, ""); err != nil {
			return err
		}
		fmt.Printf("Saved current state to global profile %q\n", name)
		return nil
	},
}

var globalRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a global profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := profile.Remove(config.ScopeGlobal, args[0], ""); err != nil {
			return err
		}
		fmt.Printf("Removed global profile %q\n", args[0])
		return nil
	},
}

var globalEditCmd = &cobra.Command{
	Use:   "edit [name]",
	Short: "Open a global profile directory in your editor",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		} else {
			var err error
			name, err = profile.Current(config.ScopeGlobal, "")
			if err != nil || name == "" {
				return fmt.Errorf("no active profile, specify a name")
			}
		}
		dir := profile.ProfileDir(config.ScopeGlobal, name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("profile %q not found", name)
		}
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}
		fmt.Printf("Opening %s with %s\n", dir, filepath.Base(editor))
		return runEditor(editor, dir)
	},
}

func init() {
	globalInitCmd.Flags().String("from", "", "Copy from existing profile")
	globalUseCmd.Flags().Bool("none", false, "Switch to vanilla (no profile)")

	globalCmd.AddCommand(globalInitCmd)
	globalCmd.AddCommand(globalUseCmd)
	globalCmd.AddCommand(globalLsCmd)
	globalCmd.AddCommand(globalCurrentCmd)
	globalCmd.AddCommand(globalSaveCmd)
	globalCmd.AddCommand(globalRmCmd)
	globalCmd.AddCommand(globalEditCmd)
}

func runEditor(editor, path string) error {
	proc, err := os.StartProcess(editor, []string{editor, path}, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return fmt.Errorf("failed to start editor: %w", err)
	}
	st, err := proc.Wait()
	if err != nil {
		return err
	}
	if !st.Success() {
		return fmt.Errorf("editor exited with error")
	}
	return nil
}
