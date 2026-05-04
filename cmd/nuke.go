package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Remove managed config from harness targets",
	Long: `Removes cvm-managed configuration files from harness target directories.
	Runtime files (sessions, cache, history) are never touched.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		harnesses, err := selectedHarnesses(cmd)
		if err != nil {
			return err
		}

		if !force {
			fmt.Print("This will remove all cvm-managed config. Continue? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				fmt.Println("Aborted")
				return nil
			}
		}

		st, err := state.Load()
		if err != nil {
			return err
		}

		for _, h := range harnesses {
			if err := profile.NukeWithHarness(h); err != nil {
				return fmt.Errorf("nuking %s: %w", h.Name(), err)
			}
			st.ClearGlobalHarness(h.Name())
			fmt.Printf("Nuked config (%s harness: %s)\n", h.Name(), h.TargetDir())
		}

		return st.Save()
	},
}

func init() {
	nukeCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	nukeCmd.Flags().String("harness", "", "Only nuke one harness")
}
