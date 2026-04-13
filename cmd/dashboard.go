// Spec: S-016
// dashboard.go — cobra subcommand for the CVM realtime observability web dashboard.
package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/chichex/cvm/internal/dashboard"
	"github.com/spf13/cobra"
)

// dashboardCmd is the `cvm dashboard` subcommand.
// Spec: S-016 | Req: I-001, B-001, B-002, B-003
var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Start the CVM realtime observability web dashboard",
	Long: `Start a local HTTP server that lets you observe CVM activity in your browser.

The dashboard is read-only and shows:
  - Timeline: recent KB entries in reverse-chronological order
  - Session: live view of the active session buffer
  - Browser: full-text search over all KB entries
  - Stats: token counts, tag breakdown, active sessions

The server stops when you press Ctrl-C.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		projectPath, _ := cmd.Flags().GetString("project")

		// Resolve port: flag > env var > default
		// Spec: S-016 | Req: I-001f
		if !cmd.Flags().Changed("port") {
			if envPort := os.Getenv("CVM_DASHBOARD_PORT"); envPort != "" {
				p, err := strconv.Atoi(envPort)
				if err == nil {
					port = p
				}
			}
		}

		// Validate port — Spec: S-016 | Req: I-001a
		if port < 1024 || port > 65535 {
			fmt.Fprintf(os.Stderr, "port must be between 1024 and 65535\n")
			os.Exit(1)
		}

		// Resolve project path
		if projectPath == "" {
			var err error
			projectPath, err = getProjectPath()
			if err != nil {
				projectPath, _ = os.Getwd()
			}
		}

		cfg := dashboard.Config{
			Port:        port,
			ProjectPath: projectPath,
		}

		srv, err := dashboard.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize dashboard: %w", err)
		}

		if err := srv.Run(); err != nil {
			// Port-in-use error: print to stderr and exit 1
			// Spec: S-016 | Req: I-001c, B-003
			if isPortInUseErr(err, port) {
				fmt.Fprintf(os.Stderr, "port %d already in use\n", port)
				os.Exit(1)
			}
			return err
		}
		return nil
	},
}

// isPortInUseErr detects port-in-use errors.
func isPortInUseErr(err error, port int) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) > 0 && (contains(msg, "already in use") || contains(msg, "bind"))
}

// contains is a simple substring check to avoid importing strings in this file.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}

func init() {
	dashboardCmd.Flags().Int("port", 3333, "Port to listen on (also: CVM_DASHBOARD_PORT env var)")
	dashboardCmd.Flags().String("project", "", "Project directory for local KB (default: current directory)")
}
