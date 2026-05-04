package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.Load()
		if err != nil {
			return err
		}

		profiles, _ := profile.List()
		if len(profiles) > 0 {
			fmt.Println("Profiles:")
			for _, p := range profiles {
				status := "idle"
				if p.Active {
					status = "IN USE"
				}
				source := "(local)"
				if r, ok := st.FindRemote(p.Name); ok {
					source = r.Repo
				}
				fmt.Printf("  %-18s %-8s %-35s %d items\n", p.Name, status, source, p.Items)
			}
		}

		if len(profiles) == 0 {
			fmt.Println("No profiles yet. Create one with: cvm add <name>")
		}

		return nil
	},
}
