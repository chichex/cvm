package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

// overrideScope returns the scope, active profile name, and project path.
// It errors if no profile is active for the given scope.
func overrideScope(cmd *cobra.Command) (config.Scope, string, string, error) {
	isLocal, _ := cmd.Flags().GetBool("local")
	scope := config.ScopeGlobal
	projectPath := ""
	if isLocal {
		scope = config.ScopeLocal
		var err error
		projectPath, err = getProjectPath()
		if err != nil {
			return "", "", "", err
		}
	}

	st, err := state.Load()
	if err != nil {
		return "", "", "", err
	}

	var active string
	if scope == config.ScopeGlobal {
		active = st.Global.Active
	} else {
		active = st.GetLocal(projectPath)
	}
	if active == "" {
		return "", "", "", fmt.Errorf("no active %s profile", scope)
	}

	return scope, active, projectPath, nil
}

func openEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		fmt.Printf("Override path: %s\n", path)
		return nil
	}
	cmd := exec.Command("sh", "-c", editor+" "+shellQuote(path))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

var overrideCmd = &cobra.Command{
	Use:   "override",
	Short: "Manage user overrides for the active profile",
	Long:  `Overrides are user customizations that persist across 'cvm pull'. They are stored separately from the base profile and merged on top when applied.`,
}

var overrideLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List overrides for the active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, active, projectPath, err := overrideScope(cmd)
		if err != nil {
			return err
		}
		dir := profile.OverrideDir(scope, active, projectPath)

		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No overrides")
				return nil
			}
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No overrides")
			return nil
		}

		for _, e := range entries {
			if e.IsDir() {
				children, err := os.ReadDir(filepath.Join(dir, e.Name()))
				if err != nil {
					return err
				}
				names := make([]string, 0, len(children))
				for _, c := range children {
					names = append(names, c.Name())
				}
				sort.Strings(names)
				fmt.Printf("  %s/\n", e.Name())
				for _, n := range names {
					fmt.Printf("    %s\n", n)
				}
			} else {
				fmt.Printf("  %s\n", e.Name())
			}
		}
		return nil
	},
}

var overrideAddCmd = &cobra.Command{
	Use:   "add <type> <name>",
	Short: "Add a scaffold override file (skill, hook, agent, rule, command)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, active, projectPath, err := overrideScope(cmd)
		if err != nil {
			return err
		}
		dir := profile.OverrideDir(scope, active, projectPath)
		kind := args[0]
		name := args[1]

		if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
			return fmt.Errorf("invalid name %q: must not contain path separators or '..'", name)
		}

		var filePath string
		var content string
		var mode os.FileMode = 0644

		switch kind {
		case "skill":
			filePath = filepath.Join(dir, "skills", name, "SKILL.md")
			content = "<!-- Skill: " + name + " -->\n"
		case "hook":
			filePath = filepath.Join(dir, "hooks", name+".sh")
			content = "#!/usr/bin/env bash\n"
			mode = 0755
		case "agent":
			filePath = filepath.Join(dir, "agents", name, "AGENT.md")
			content = "<!-- Agent: " + name + " -->\n"
		case "rule":
			filePath = filepath.Join(dir, "rules", name+".md")
			content = ""
		case "command":
			filePath = filepath.Join(dir, "commands", name+".md")
			content = ""
		default:
			return fmt.Errorf("unknown type %q: must be one of skill, hook, agent, rule, command", kind)
		}

		// Check if override already exists
		if _, err := os.Stat(filePath); err == nil {
			fmt.Printf("Override already exists: %s\n", filePath)
			return openEditor(filePath)
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filePath, []byte(content), mode); err != nil {
			return err
		}
		fmt.Printf("Created %s override: %s\n", kind, filePath)
		return openEditor(filePath)
	},
}

var overrideEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the override directory in $EDITOR",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, active, projectPath, err := overrideScope(cmd)
		if err != nil {
			return err
		}
		dir := profile.OverrideDir(scope, active, projectPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return openEditor(dir)
	},
}

var overrideRmCmd = &cobra.Command{
	Use:   "rm <type> <name>",
	Short: "Remove an override file (skill, hook, agent, rule, command)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, active, projectPath, err := overrideScope(cmd)
		if err != nil {
			return err
		}
		dir := profile.OverrideDir(scope, active, projectPath)
		kind := args[0]
		name := args[1]

		if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
			return fmt.Errorf("invalid name %q: must not contain path separators or '..'", name)
		}

		var target string
		switch kind {
		case "skill":
			target = filepath.Join(dir, "skills", name)
		case "hook":
			target = filepath.Join(dir, "hooks", name+".sh")
		case "agent":
			target = filepath.Join(dir, "agents", name)
		case "rule":
			target = filepath.Join(dir, "rules", name+".md")
		case "command":
			target = filepath.Join(dir, "commands", name+".md")
		default:
			return fmt.Errorf("unknown type %q: must be one of skill, hook, agent, rule, command", kind)
		}

		if _, err := os.Stat(target); os.IsNotExist(err) {
			return fmt.Errorf("%s override %q not found", kind, name)
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		fmt.Printf("Removed %s override: %s\n", kind, name)
		return nil
	},
}

var overrideSetCmd = &cobra.Command{
	Use:   "set <file>",
	Short: "Capture a live file into the override dir",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, active, projectPath, err := overrideScope(cmd)
		if err != nil {
			return err
		}
		filename := args[0]

		// Validate scope-file compatibility early
		if filename == ".claude.json" && scope == config.ScopeLocal {
			return fmt.Errorf(".claude.json is only valid for global profiles — use .mcp.json for local profiles")
		}
		if filename == ".mcp.json" && scope == config.ScopeGlobal {
			return fmt.Errorf(".mcp.json is only valid for local profiles — use .claude.json for global profiles")
		}

		// Determine live directory
		var liveDir string
		if scope == config.ScopeGlobal {
			liveDir = config.ClaudeHome()
		} else {
			liveDir = config.ProjectClaudeDir(projectPath)
		}

		// Find the correct source file path
		var src string
		if filename == ".claude.json" {
			// Special case: .claude.json lives at ~/.claude.json (not inside ~/.claude/)
			src = config.ClaudeUserConfigPath()
		} else if filename == ".mcp.json" {
			src = config.ProjectMCPConfigPath(projectPath)
		} else {
			src = filepath.Join(liveDir, filename)
		}

		// Validate it is a known managed item and that it's a file, not a directory
		known := false
		for _, item := range config.ManagedClaudeDirItems {
			if item == filename {
				known = true
				break
			}
		}
		if filename == ".claude.json" || filename == ".mcp.json" {
			known = true
		}
		if !known {
			return fmt.Errorf("unknown managed file %q: use 'override add' for directories", filename)
		}

		// Check that the source exists and is a file, not a directory
		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("file %q not found in live directory", filename)
		}
		if info.IsDir() {
			return fmt.Errorf("%q is a directory — use 'cvm override add' for directory-type items", filename)
		}

		overDir := profile.OverrideDir(scope, active, projectPath)
		dst := filepath.Join(overDir, filename)
		if err := profile.CopyFile(src, dst); err != nil {
			return err
		}
		fmt.Printf("Captured %s into override: %s\n", filename, dst)
		return nil
	},
}

var overrideShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show a structured inventory of the active profile's overrides",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, active, projectPath, err := overrideScope(cmd)
		if err != nil {
			return err
		}
		dir := profile.OverrideDir(scope, active, projectPath)

		fmt.Printf("Profile:  %s (%s)\n", active, scope)
		fmt.Printf("Override: %s\n", dir)

		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No overrides")
				return nil
			}
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No overrides")
			return nil
		}

		var files []string
		type dirEntry struct {
			name     string
			children []string
		}
		var dirs []dirEntry

		for _, e := range entries {
			if e.IsDir() {
				children, err := os.ReadDir(filepath.Join(dir, e.Name()))
				if err != nil {
					return err
				}
				names := make([]string, 0, len(children))
				for _, c := range children {
					names = append(names, c.Name())
				}
				sort.Strings(names)
				dirs = append(dirs, dirEntry{name: e.Name(), children: names})
			} else {
				files = append(files, e.Name())
			}
		}
		sort.Strings(files)

		if len(files) > 0 {
			fmt.Println("\nFiles:")
			for _, f := range files {
				fmt.Printf("  %s\n", f)
			}
		}
		if len(dirs) > 0 {
			fmt.Println("\nDirectories:")
			for _, d := range dirs {
				fmt.Printf("  %s/ (%d items)\n", d.name, len(d.children))
				for _, c := range d.children {
					fmt.Printf("    %s\n", c)
				}
			}
		}
		return nil
	},
}

var overrideApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Re-apply the active profile (including overrides)",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, active, projectPath, err := overrideScope(cmd)
		if err != nil {
			return err
		}
		if err := profile.Reapply(scope, active, projectPath); err != nil {
			return err
		}
		fmt.Printf("Applied profile %q (%s) with overrides\n", active, scope)
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{overrideLsCmd, overrideAddCmd, overrideEditCmd, overrideRmCmd, overrideSetCmd, overrideShowCmd, overrideApplyCmd} {
		c.Flags().Bool("local", false, "Use local profile overrides (default: global)")
	}

	overrideCmd.AddCommand(overrideLsCmd)
	overrideCmd.AddCommand(overrideAddCmd)
	overrideCmd.AddCommand(overrideEditCmd)
	overrideCmd.AddCommand(overrideRmCmd)
	overrideCmd.AddCommand(overrideSetCmd)
	overrideCmd.AddCommand(overrideShowCmd)
	overrideCmd.AddCommand(overrideApplyCmd)
}
