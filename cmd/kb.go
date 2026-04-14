package cmd

import (
	"fmt"
	"strings"
	"strconv"
	"time"

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

// Spec: S-010 | Req: B-008, B-013 | Spec: S-017 | Req: C-010
var kbPutCmd = &cobra.Command{
	Use:   "put <key>",
	Short: "Create or update a KB entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		key := args[0]
		body, _ := cmd.Flags().GetString("body")
		tagsStr, _ := cmd.Flags().GetString("tag")
		typeTag, _ := cmd.Flags().GetString("type")
		sessionID, _ := cmd.Flags().GetString("session-id")
		var tags []string
		if tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ",") {
				tags = append(tags, strings.TrimSpace(t))
			}
		}
		if typeTag != "" {
			if err := kb.ValidateType(typeTag); err != nil {
				return err
			}
			tags = append(tags, typeTag)
		}
		// If session-id provided, skip dedup and write directly with session linking.
		// Otherwise use dedup as before. Spec: S-017 | Req: C-010, B-015
		if sessionID != "" {
			if err := kb.Put(scope, projectPath, key, body, tags, sessionID); err != nil {
				return err
			}
			fmt.Printf("Saved KB entry %q (%s)\n", key, scope)
		} else {
			skipped, err := kb.PutWithDedup(scope, projectPath, key, body, tags)
			if err != nil {
				return err
			}
			if skipped {
				fmt.Printf("Skipped KB entry %q (identical content)\n", key)
			} else {
				fmt.Printf("Saved KB entry %q (%s)\n", key, scope)
			}
		}
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

// Spec: S-010 | Req: B-009, B-010
var kbSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search KB entries",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)

		opts := kb.SearchOptions{}
		opts.Tag, _ = cmd.Flags().GetString("tag")
		opts.TypeTag, _ = cmd.Flags().GetString("type")
		opts.Sort, _ = cmd.Flags().GetString("sort")

		sinceStr, _ := cmd.Flags().GetString("since")
		if sinceStr != "" {
			d, err := parseDuration(sinceStr)
			if err != nil {
				return fmt.Errorf("invalid --since value %q: %w", sinceStr, err)
			}
			opts.Since = d
		}

		results, err := kb.SearchWithOptions(scope, projectPath, args[0], opts)
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

// parseDuration parses duration strings like "7d", "30d", "24h"
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

var kbCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all KB entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		force, _ := cmd.Flags().GetBool("force")

		entries, err := kb.List(scope, projectPath, "")
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("KB is already empty")
			return nil
		}

		if !force {
			fmt.Printf("This will remove all %d entries from %s KB. Continue? [y/N] ", len(entries), scope)
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				fmt.Println("Aborted")
				return nil
			}
		}

		removed, err := kb.Clean(scope, projectPath)
		if err != nil {
			return err
		}
		fmt.Printf("Removed %d entries from %s KB\n", removed, scope)
		return nil
	},
}

// Spec: S-010 | Req: B-011
var kbTimelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Show entries grouped by day",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		days, _ := cmd.Flags().GetInt("days")
		timeline, err := kb.Timeline(scope, projectPath, days)
		if err != nil {
			return err
		}
		if len(timeline) == 0 {
			fmt.Println("No entries in the last", days, "days")
			return nil
		}
		for _, day := range timeline {
			fmt.Printf("=== %s ===\n", day.Date.Format("2006-01-02"))
			for _, e := range day.Entries {
				tags := ""
				if len(e.Tags) > 0 {
					tags = fmt.Sprintf(" [%s]", strings.Join(e.Tags, ", "))
				}
				fmt.Printf("  - %s%s\n", e.Key, tags)
			}
			fmt.Println()
		}
		return nil
	},
}

// Spec: S-010 | Req: B-012
var kbStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show KB statistics and token estimates",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		stats, err := kb.StatsDetailed(scope, projectPath)
		if err != nil {
			return err
		}
		fmt.Printf("KB Statistics (%s)\n", scope)
		fmt.Printf("  Total entries:   %d\n", stats.Total)
		fmt.Printf("  Enabled:         %d\n", stats.Enabled)
		fmt.Printf("  Stale (>30d):    %d\n", stats.Stale)
		fmt.Printf("  Total tokens:    ~%d (estimated)\n", stats.TotalTokens)
		if stats.TotalTokens > 50000 {
			fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: total tokens exceed 50K — consider pruning stale entries\n")
		}
		if len(stats.PerEntry) > 0 {
			fmt.Println("\n  Top entries by tokens:")
			// Sort by tokens desc
			type kv struct {
				Key    string
				Tokens int
			}
			var sorted []kv
			for k, v := range stats.PerEntry {
				sorted = append(sorted, kv{k, v})
			}
			for i := 0; i < len(sorted); i++ {
				for j := i + 1; j < len(sorted); j++ {
					if sorted[j].Tokens > sorted[i].Tokens {
						sorted[i], sorted[j] = sorted[j], sorted[i]
					}
				}
			}
			limit := 10
			if len(sorted) < limit {
				limit = len(sorted)
			}
			for _, kv := range sorted[:limit] {
				fmt.Printf("    %-35s ~%d tokens\n", kv.Key, kv.Tokens)
			}
		}
		return nil
	},
}

// Spec: S-010 | Req: B-014
var kbCompactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Show compact index for context injection",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		entries, err := kb.Compact(scope, projectPath)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No entries")
			return nil
		}
		fmt.Printf("%-30s %-20s %-12s %s\n", "KEY", "TAGS", "UPDATED", "FIRST-LINE")
		fmt.Printf("%-30s %-20s %-12s %s\n", "---", "----", "-------", "----------")
		for _, e := range entries {
			tags := strings.Join(e.Tags, ",")
			if len(tags) > 18 {
				tags = tags[:18] + ".."
			}
			updated := e.UpdatedAt.Format("2006-01-02")
			fmt.Printf("%-30s %-20s %-12s %s\n", e.Key, tags, updated, e.FirstLine)
		}
		return nil
	},
}

// Spec: S-019 | Req: C-005
var kbMigrateTagsCmd = &cobra.Command{
	Use:   "migrate-tags",
	Short: "Clean up KB: remove entries without a type tag and session-buffer residues",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, projectPath := kbScope(cmd)
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		b, err := kb.NewBackend(scope, projectPath)
		if err != nil {
			return err
		}
		defer b.Close()

		entries, err := b.List("")
		if err != nil {
			return err
		}

		deletedUntyped := 0
		deletedBuffer := 0

		for _, e := range entries {
			// Delete session-buffer-* residues
			if strings.HasPrefix(e.Key, "session-buffer-") {
				if dryRun {
					fmt.Printf("[dry-run] would delete session-buffer entry: %s\n", e.Key)
				} else {
					if err := b.Remove(e.Key); err != nil {
						fmt.Printf("warning: failed to delete %s: %v\n", e.Key, err)
						continue
					}
				}
				deletedBuffer++
				continue
			}

			// Check if entry has at least one type tag
			hasType := false
			for _, tag := range e.Tags {
				if kb.ClassifyTag(tag) == "type" {
					hasType = true
					break
				}
			}
			if !hasType {
				if dryRun {
					fmt.Printf("[dry-run] would delete untyped entry: %s (tags: %v)\n", e.Key, e.Tags)
				} else {
					if err := b.Remove(e.Key); err != nil {
						fmt.Printf("warning: failed to delete %s: %v\n", e.Key, err)
						continue
					}
				}
				deletedUntyped++
			}
		}

		if deletedUntyped == 0 && deletedBuffer == 0 {
			fmt.Println("No changes needed")
		} else {
			prefix := ""
			if dryRun {
				prefix = "[dry-run] "
			}
			fmt.Printf("%sDeleted %d untyped entries, removed %d session-buffer entries\n", prefix, deletedUntyped, deletedBuffer)
		}
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{kbPutCmd, kbLsCmd, kbRmCmd, kbShowCmd, kbEnableCmd, kbDisableCmd, kbSearchCmd, kbCleanCmd, kbTimelineCmd, kbStatsCmd, kbCompactCmd, kbMigrateTagsCmd} {
		c.Flags().Bool("local", false, "Use local KB (default: global)")
	}
	kbPutCmd.Flags().String("body", "", "Entry body content")
	kbPutCmd.Flags().String("tag", "", "Comma-separated tags")
	kbPutCmd.Flags().String("type", "", "Entry type: "+strings.Join(kb.ValidTypes, "|"))
	kbPutCmd.Flags().String("session-id", "", "Link entry to a session UUID (Spec: S-017 | Req: C-010)")
	kbLsCmd.Flags().String("tag", "", "Filter by tag")
	kbCleanCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	// Search flags (B-009, B-010)
	kbSearchCmd.Flags().String("sort", "relevance", "Sort order: relevance|recent")
	kbSearchCmd.Flags().String("tag", "", "Filter by tag")
	kbSearchCmd.Flags().String("since", "", "Filter by age, e.g. 7d or 30d")
	kbSearchCmd.Flags().String("type", "", "Filter by type tag")

	// Timeline flags (B-011)
	kbTimelineCmd.Flags().Int("days", 7, "Number of days to show")

	kbCmd.AddCommand(kbPutCmd)
	kbCmd.AddCommand(kbLsCmd)
	kbCmd.AddCommand(kbRmCmd)
	kbCmd.AddCommand(kbShowCmd)
	kbCmd.AddCommand(kbEnableCmd)
	kbCmd.AddCommand(kbDisableCmd)
	kbCmd.AddCommand(kbSearchCmd)
	kbCmd.AddCommand(kbCleanCmd)
	kbCmd.AddCommand(kbTimelineCmd)
	kbCmd.AddCommand(kbStatsCmd)
	kbCmd.AddCommand(kbCompactCmd)
	kbMigrateTagsCmd.Flags().Bool("dry-run", false, "Print changes without applying them")
	kbCmd.AddCommand(kbMigrateTagsCmd)
}
