// Package cmd provides CLI commands for CVM.
// Spec: S-017 | Req: C-006
package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chichex/cvm/internal/session"
	"github.com/spf13/cobra"
)

// sessionCmd is the root subcommand for session management.
// Spec: S-017 | Req: C-006
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage CVM sessions",
}

// sessionStartCmd starts a new CVM session.
// Spec: S-017 | Req: B-001, C-006
var sessionStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new CVM session",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session-id")
		project, _ := cmd.Flags().GetString("project")
		profileName, _ := cmd.Flags().GetString("profile")
		pid, _ := cmd.Flags().GetInt("pid")

		// Default project to cwd if not provided.
		if project == "" {
			var err error
			project, err = os.Getwd()
			if err != nil {
				project = ""
			}
		}

		return session.Start(sessionID, project, profileName, pid)
	},
}

// sessionAppendCmd appends an event to an existing session.
// Spec: S-017 | Req: B-002, B-003, B-004, C-006
var sessionAppendCmd = &cobra.Command{
	Use:   "append <uuid>",
	Short: "Append an event to a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID := args[0]
		if sessionID == "" {
			fmt.Fprintln(os.Stderr, "error: session_id is required")
			os.Exit(1)
		}

		eventType, _ := cmd.Flags().GetString("type")
		content, _ := cmd.Flags().GetString("content")
		tool, _ := cmd.Flags().GetString("tool")
		agentType, _ := cmd.Flags().GetString("agent-type")

		// Validate event type. Spec: S-017 | Errors table
		switch eventType {
		case "prompt", "tool", "agent":
		default:
			fmt.Fprintln(os.Stderr, "error: type must be one of: prompt, tool, agent")
			os.Exit(1)
		}

		// Validate required fields per type. Spec: S-017 | Errors table
		if eventType == "tool" && tool == "" {
			fmt.Fprintln(os.Stderr, "error: --tool is required when --type is tool")
			os.Exit(1)
		}
		if eventType == "agent" && agentType == "" {
			fmt.Fprintln(os.Stderr, "error: --agent-type is required when --type is agent")
			os.Exit(1)
		}

		return session.Append(sessionID, eventType, content, tool, agentType)
	},
}

// sessionEndCmd ends a session.
// Spec: S-017 | Req: B-005, C-006
var sessionEndCmd = &cobra.Command{
	Use:   "end <uuid>",
	Short: "End a session and generate summary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return session.End(args[0])
	},
}

// sessionStatusCmd lists active sessions.
// Spec: S-017 | Req: B-007, C-006
var sessionStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		return session.Status()
	},
}

// sessionListCmd lists all sessions.
// Spec: S-017 | Req: B-008, C-006
var sessionListCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List all sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		return session.List(limit)
	},
}

// sessionShowCmd shows events for a session.
// Spec: S-017 | Req: B-009, E-009, C-006
var sessionShowCmd = &cobra.Command{
	Use:   "show <uuid>",
	Short: "Show events for a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return session.Show(args[0])
	},
}

// sessionGCCmd deletes old closed session files.
// Spec: S-017 | Req: B-010, C-006
var sessionGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Delete old closed session files",
	RunE: func(cmd *cobra.Command, args []string) error {
		olderThanStr, _ := cmd.Flags().GetString("older-than")
		d, err := parseGCDuration(olderThanStr)
		if err != nil {
			return fmt.Errorf("invalid --older-than value %q: %w", olderThanStr, err)
		}
		return session.GC(d)
	},
}

// parseGCDuration parses a duration string that may include "d" for days.
// Go's time.ParseDuration does not support "d" suffix.
// Spec: S-017 | Req: C-006
func parseGCDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		daysStr := s[:len(s)-1]
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, fmt.Errorf("invalid days value %q", daysStr)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func init() {
	// Start flags. Spec: S-017 | Req: C-006
	sessionStartCmd.Flags().String("session-id", "", "UUID of the session (generated if omitted)")
	sessionStartCmd.Flags().String("project", "", "Absolute path to project dir (defaults to cwd)")
	sessionStartCmd.Flags().String("profile", "", "Active CVM profile name")
	sessionStartCmd.Flags().Int("pid", 0, "PID of claude process (uses os.Getppid() if 0)")

	// Append flags. Spec: S-017 | Req: C-006
	sessionAppendCmd.Flags().String("type", "", "Event type: prompt, tool, or agent (required)")
	sessionAppendCmd.Flags().String("content", "", "Event content")
	sessionAppendCmd.Flags().String("tool", "", "Tool name (required when --type is tool)")
	sessionAppendCmd.Flags().String("agent-type", "", "Agent type (required when --type is agent)")
	sessionAppendCmd.MarkFlagRequired("type")

	// List flags. Spec: S-017 | Req: C-006
	sessionListCmd.Flags().Int("limit", 20, "Maximum number of sessions to show (0 = no limit)")

	// GC flags. Spec: S-017 | Req: C-006
	sessionGCCmd.Flags().String("older-than", "30d", "Delete sessions older than this duration (e.g. 30d, 168h)")

	// Wire subcommands.
	sessionCmd.AddCommand(sessionStartCmd)
	sessionCmd.AddCommand(sessionAppendCmd)
	sessionCmd.AddCommand(sessionEndCmd)
	sessionCmd.AddCommand(sessionStatusCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionShowCmd)
	sessionCmd.AddCommand(sessionGCCmd)
}
