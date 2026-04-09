package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chichex/cvm/internal/automation"
	"github.com/spf13/cobra"
)

var automationCmd = &cobra.Command{
	Use:   "automation",
	Short: "Inspect low-latency automation candidates and artifacts",
}

var automationStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show automation summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := automation.Load()
		if err != nil {
			return err
		}

		fmt.Printf("Pending candidates: %d\n", state.PendingCount())
		if !state.LastRetroQueuedAt.IsZero() {
			fmt.Printf("Last retro queued: %s\n", state.LastRetroQueuedAt.Format("2006-01-02 15:04:05"))
		}
		if !state.LastAutomationRunAt.IsZero() {
			fmt.Printf("Last automation run: %s\n", state.LastAutomationRunAt.Format("2006-01-02 15:04:05"))
		}
		if state.PendingCount() == 0 {
			if runs := state.RecentRuns(3); len(runs) > 0 {
				fmt.Println("Recent runs:")
				for _, run := range runs {
					fmt.Printf("  %-30s %-8s %s\n", run.ID, run.Status, run.Summary)
				}
			}
			return nil
		}

		for _, candidate := range sortedCandidates(state.Pending) {
			fmt.Printf("  %-22s %-8s %s\n", candidate.ID, candidate.Kind, candidate.Reason)
		}
		if runs := state.RecentRuns(3); len(runs) > 0 {
			fmt.Println("Recent runs:")
			for _, run := range runs {
				fmt.Printf("  %-30s %-8s %s\n", run.ID, run.Status, run.Summary)
			}
		}
		return nil
	},
}

var automationLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List queued automation candidates",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := automation.Load()
		if err != nil {
			return err
		}
		if state.PendingCount() == 0 {
			fmt.Println("No automation candidates")
			return nil
		}

		for _, candidate := range sortedCandidates(state.Pending) {
			scope := candidate.Scope
			if candidate.ProjectPath != "" {
				scope = fmt.Sprintf("%s:%s", candidate.Scope, candidate.ProjectPath)
			}
			fmt.Printf("  %-22s %-8s %-12s hits=%d %s\n", candidate.ID, candidate.Kind, scope, candidate.Hits, candidate.Reason)
		}
		return nil
	},
}

var automationShowCmd = &cobra.Command{
	Use:   "show <candidate-id>",
	Short: "Show a materialized automation artifact",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := automation.ReadArtifact(args[0])
		if err != nil {
			return err
		}
		fmt.Print(content)
		return nil
	},
}

var automationRunLimit int

var automationRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Process pending automation candidates now",
	RunE: func(cmd *cobra.Command, args []string) error {
		runs, err := automation.RunPending(automationRunLimit)
		if err != nil {
			return err
		}
		if len(runs) == 0 {
			fmt.Println("No automation candidates to run")
			return nil
		}
		for _, run := range runs {
			fmt.Printf("  %-30s %-8s %s\n", run.ID, run.Status, run.Summary)
		}
		return nil
	},
}

var automationHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show recent automation runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := automation.Load()
		if err != nil {
			return err
		}
		runs := state.RecentRuns(10)
		if len(runs) == 0 {
			fmt.Println("No automation runs")
			return nil
		}
		for _, run := range runs {
			fmt.Printf("  %-30s %-8s %-8s %s\n", run.ID, run.Status, run.Kind, run.Summary)
		}
		return nil
	},
}

var automationShowRunCmd = &cobra.Command{
	Use:   "show-run <run-id>",
	Short: "Show a recorded automation run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		run, err := automation.ReadRun(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Run: %s\n", run.ID)
		fmt.Printf("Status: %s\n", run.Status)
		fmt.Printf("Kind: %s\n", run.Kind)
		fmt.Printf("Candidate: %s\n", run.CandidateID)
		if run.ProjectPath != "" {
			fmt.Printf("Project: %s\n", run.ProjectPath)
		}
		fmt.Printf("Summary: %s\n", run.Summary)
		if run.Artifact != "" {
			fmt.Printf("Artifact: %s\n", run.Artifact)
		}
		if run.Error != "" {
			fmt.Printf("Error: %s\n", run.Error)
		}
		if len(run.Actions) > 0 {
			fmt.Println("Actions:")
			for _, action := range run.Actions {
				fmt.Printf("  - %s %s [%s] %s\n", action.Type, action.Target, action.Status, action.Detail)
			}
		}
		return nil
	},
}

func init() {
	automationRunCmd.Flags().IntVar(&automationRunLimit, "limit", 0, "Maximum candidates to process")
	automationCmd.AddCommand(automationStatusCmd)
	automationCmd.AddCommand(automationLsCmd)
	automationCmd.AddCommand(automationShowCmd)
	automationCmd.AddCommand(automationRunCmd)
	automationCmd.AddCommand(automationHistoryCmd)
	automationCmd.AddCommand(automationShowRunCmd)
	rootCmd.AddCommand(automationCmd)
}

func sortedCandidates(candidates []automation.Candidate) []automation.Candidate {
	out := append([]automation.Candidate(nil), candidates...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return strings.Compare(out[i].ID, out[j].ID) < 0
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}
