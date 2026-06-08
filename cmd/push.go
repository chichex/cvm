package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/remote"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [profile-name]",
	Short: "Push local profile changes to its git remote",
	Long: `Push commits in the profile's source repo to its git remote.
Without arguments, pushes the active profile's source repo.

cvm is a transparent pass-through: git's output (including non-fast-forward
rejections) is surfaced verbatim. cvm never merges on your behalf.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		if name == "" {
			resolved, err := activeProfileName(cmd)
			if err != nil {
				return err
			}
			if resolved == "" {
				return fmt.Errorf("no active profile; pass a profile name")
			}
			name = resolved
		}

		return remote.Push(name)
	},
}

func init() {
	pushCmd.Flags().String("harness", "", "Harness whose active profile to push (default: claude)")
}

// activeProfileName resolves the active profile for the selected harness.
func activeProfileName(cmd *cobra.Command) (string, error) {
	h, err := harnessFromFlag(cmd)
	if err != nil {
		return "", err
	}
	st, err := state.Load()
	if err != nil {
		return "", err
	}
	return st.GetGlobalHarness(h.Name()), nil
}
