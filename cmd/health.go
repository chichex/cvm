package cmd

import (
	"fmt"
	"os"

	"github.com/ayrtonmarini/cvm/internal/config"
	"github.com/ayrtonmarini/cvm/internal/kb"
	"github.com/ayrtonmarini/cvm/internal/profile"
	"github.com/ayrtonmarini/cvm/internal/state"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Show system health and diagnostics",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.Load()
		if err != nil {
			return err
		}
		cwd, _ := os.Getwd()

		fmt.Println("=== cvm health ===")
		fmt.Println()

		if _, err := os.Stat(config.CvmHome()); err == nil {
			fmt.Printf("cvm home:       %s  ✓\n", config.CvmHome())
		} else {
			fmt.Printf("cvm home:       %s  ✗ (not initialized)\n", config.CvmHome())
		}

		globalName := st.Global.Active
		if globalName == "" {
			fmt.Println("global profile: (vanilla)")
		} else {
			dir := profile.ProfileDir(config.ScopeGlobal, globalName)
			if _, err := os.Stat(dir); err == nil {
				fmt.Printf("global profile: %s  ✓\n", globalName)
			} else {
				fmt.Printf("global profile: %s  ✗ (missing!)\n", globalName)
			}
		}

		localName := st.GetLocal(cwd)
		if localName == "" {
			fmt.Println("local profile:  (vanilla)")
		} else {
			dir := profile.ProfileDir(config.ScopeLocal, localName)
			if _, err := os.Stat(dir); err == nil {
				fmt.Printf("local profile:  %s  ✓\n", localName)
			} else {
				fmt.Printf("local profile:  %s  ✗ (missing!)\n", localName)
			}
		}

		fmt.Println()
		if profile.HasVanilla(config.ScopeGlobal, "") {
			fmt.Println("vanilla global: ✓  (can restore)")
		} else {
			fmt.Println("vanilla global: -  (no backup yet)")
		}
		if profile.HasVanilla(config.ScopeLocal, cwd) {
			fmt.Println("vanilla local:  ✓  (can restore)")
		} else {
			fmt.Println("vanilla local:  -  (no backup yet)")
		}

		fmt.Println()
		globalTotal, globalEnabled, globalStale, _ := kb.Stats(config.ScopeGlobal, cwd)
		localTotal, localEnabled, localStale, _ := kb.Stats(config.ScopeLocal, cwd)

		icon := func(total, stale int) string {
			if total == 0 {
				return "-"
			}
			if stale > 0 {
				return "⚠"
			}
			return "✓"
		}

		fmt.Printf("kb global:      %d entries (%d enabled, %d stale)  %s\n",
			globalTotal, globalEnabled, globalStale, icon(globalTotal, globalStale))
		fmt.Printf("kb local:       %d entries (%d enabled, %d stale)  %s\n",
			localTotal, localEnabled, localStale, icon(localTotal, localStale))

		fmt.Println()
		globalProfiles, _ := profile.List(config.ScopeGlobal, "")
		localProfiles, _ := profile.List(config.ScopeLocal, cwd)
		fmt.Printf("profiles:       %d global, %d local\n", len(globalProfiles), len(localProfiles))

		return nil
	},
}
