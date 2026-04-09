package cmd

import (
	"fmt"
	"os"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all profiles (global + local)",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.Load()
		if err != nil {
			return err
		}

		cwd, _ := os.Getwd()

		// Global profiles
		globalProfiles, _ := profile.List(config.ScopeGlobal, "")
		if len(globalProfiles) > 0 {
			fmt.Println("Global profiles:")
			for _, p := range globalProfiles {
				status := "idle"
				if p.Active {
					status = "IN USE"
				}
				source := "(local)"
				if r, ok := st.FindRemote(config.ScopeGlobal, p.Name, ""); ok {
					source = r.Repo
				}
				fmt.Printf("  %-18s %-8s %-35s %d items\n", p.Name, status, source, p.Items)
			}
		} else {
			fmt.Println("Global profiles: none")
		}

		// Local profiles
		localProfiles, _ := profile.List(config.ScopeLocal, cwd)
		if len(localProfiles) > 0 {
			fmt.Println("\nLocal profiles:")
			for _, p := range localProfiles {
				status := "idle"
				if p.Active {
					status = "IN USE"
				}
				source := "(local)"
				if r, ok := st.FindRemote(config.ScopeLocal, p.Name, cwd); ok {
					source = r.Repo
				}
				fmt.Printf("  %-18s %-8s %-35s %d items\n", p.Name, status, source, p.Items)
			}
		}

		if len(globalProfiles) == 0 && len(localProfiles) == 0 {
			fmt.Println("\nNo profiles yet. Create one with: cvm add <name>")
		}

		return nil
	},
}
