package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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

var editCmd = &cobra.Command{
	Use:   "edit [name]",
	Short: "Open a profile directory in your editor",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		} else {
			var err error
			name, err = profile.Current()
			if err != nil || name == "" {
				return fmt.Errorf("no active profile, specify a name")
			}
		}

		dir := profile.ProfileDir(name)
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
