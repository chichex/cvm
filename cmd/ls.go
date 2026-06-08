package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List profiles and the active one per harness",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.Load()
		if err != nil {
			return err
		}

		// Collect every known profile name: clone-backed (live on disk) plus
		// path-registered ones recorded only in state.
		names := map[string]bool{}
		cloneItems := map[string]int{}
		clones, _ := profile.List()
		for _, p := range clones {
			names[p.Name] = true
			cloneItems[p.Name] = p.Items
		}
		for name := range st.Sources {
			names[name] = true
		}

		if len(names) == 0 {
			fmt.Println("No profiles yet. Create one with: cvm add <name>")
			return nil
		}

		// active[profileName] = sorted list of harnesses it's active for.
		active := map[string][]string{}
		harnessNames := make([]string, 0, len(st.Global.Harnesses))
		for h := range st.Global.Harnesses {
			harnessNames = append(harnessNames, h)
		}
		sort.Strings(harnessNames)
		for _, h := range harnessNames {
			if p := st.GetGlobalHarness(h); p != "" {
				active[p] = append(active[p], h)
			}
		}

		sorted := make([]string, 0, len(names))
		for name := range names {
			sorted = append(sorted, name)
		}
		sort.Strings(sorted)

		fmt.Println("Profiles:")
		for _, name := range sorted {
			status := "idle"
			if hs := active[name]; len(hs) > 0 {
				status = "IN USE: " + strings.Join(hs, ",")
			}

			source := "(local)"
			if r, ok := st.FindRemote(name); ok {
				source = r.Repo
			} else if dir, ok := st.Sources[name]; ok {
				source = dir
			}

			items := ""
			if n, ok := cloneItems[name]; ok {
				items = fmt.Sprintf("%d items", n)
			}

			fmt.Printf("  %-18s %-18s %-35s %s\n", name, status, source, items)
		}

		return nil
	},
}
