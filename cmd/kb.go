package cmd

import (
	"fmt"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/kb"
	"github.com/spf13/cobra"
)

func kbScope(cmd *cobra.Command) (config.Scope, string) {
	isLocal, _ := cmd.Flags().GetBool("local")
	if isLocal {
		projectPath, _ := getProjectPath()
		return config.ScopeLocal, projectPath
	}
	return config.ScopeGlobal, ""
}

var kbCmd = &cobra.Command{
	Use:   "kb",
	Short: "Manage knowledge base entries",
}

var kbPutCmd = &cobra.Command{
	Use:   "put <key>",
	Short: "Create or update a KB entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		key := args[0]
		body, _ := cmd.Flags().GetString("body")
		tagsStr, _ := cmd.Flags().GetString("tag")
		var tags []string
		if tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ",") {
				tags = append(tags, strings.TrimSpace(t))
			}
		}
		if err := kb.Put(scope, projectPath, key, body, tags); err != nil {
			return err
		}
		fmt.Printf("Saved KB entry %q (%s)\n", key, scope)
		return nil
	},
}

var kbLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List KB entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		tag, _ := cmd.Flags().GetString("tag")
		entries, err := kb.List(scope, projectPath, tag)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No KB entries")
			return nil
		}
		for _, e := range entries {
			status := "✓"
			if !e.Enabled {
				status = "✗"
			}
			tags := ""
			if len(e.Tags) > 0 {
				tags = fmt.Sprintf(" [%s]", strings.Join(e.Tags, ", "))
			}
			fmt.Printf("  %s %-30s%s\n", status, e.Key, tags)
		}
		return nil
	},
}

var kbRmCmd = &cobra.Command{
	Use:   "rm <key>",
	Short: "Remove a KB entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		if err := kb.Remove(scope, projectPath, args[0]); err != nil {
			return err
		}
		fmt.Printf("Removed KB entry %q\n", args[0])
		return nil
	},
}

var kbShowCmd = &cobra.Command{
	Use:   "show <key>",
	Short: "Show a KB entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		content, err := kb.Show(scope, projectPath, args[0])
		if err != nil {
			return err
		}
		fmt.Print(content)
		return nil
	},
}

var kbEnableCmd = &cobra.Command{
	Use:   "enable <key>",
	Short: "Enable a KB entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		if err := kb.SetEnabled(scope, projectPath, args[0], true); err != nil {
			return err
		}
		fmt.Printf("Enabled KB entry %q\n", args[0])
		return nil
	},
}

var kbDisableCmd = &cobra.Command{
	Use:   "disable <key>",
	Short: "Disable a KB entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		if err := kb.SetEnabled(scope, projectPath, args[0], false); err != nil {
			return err
		}
		fmt.Printf("Disabled KB entry %q\n", args[0])
		return nil
	},
}

var kbSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search KB entries",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		results, err := kb.Search(scope, projectPath, args[0])
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Println("No matches")
			return nil
		}
		for _, r := range results {
			tags := ""
			if len(r.Entry.Tags) > 0 {
				tags = fmt.Sprintf(" [%s]", strings.Join(r.Entry.Tags, ", "))
			}
			fmt.Printf("  %-25s%s\n", r.Entry.Key, tags)
			fmt.Printf("    %s\n", r.Snippet)
		}
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{kbPutCmd, kbLsCmd, kbRmCmd, kbShowCmd, kbEnableCmd, kbDisableCmd, kbSearchCmd} {
		c.Flags().Bool("local", false, "Use local KB (default: global)")
	}
	kbPutCmd.Flags().String("body", "", "Entry body content")
	kbPutCmd.Flags().String("tag", "", "Comma-separated tags")
	kbLsCmd.Flags().String("tag", "", "Filter by tag")

	kbCmd.AddCommand(kbPutCmd)
	kbCmd.AddCommand(kbLsCmd)
	kbCmd.AddCommand(kbRmCmd)
	kbCmd.AddCommand(kbShowCmd)
	kbCmd.AddCommand(kbEnableCmd)
	kbCmd.AddCommand(kbDisableCmd)
	kbCmd.AddCommand(kbSearchCmd)
}
