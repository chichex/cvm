package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/remote"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [profile-name]",
	Short: "Commit pending profile changes and push them to its git remote",
	Long: `Commit any pending changes in the profile's source repo and push them
to its git remote, in one step. Without arguments, uses the active profile.

If the working tree is dirty, cvm stages everything (git add -A) and commits
with -m's message (or a default "cvm: update <profile> profile") before pushing.
git's output — including non-fast-forward rejections — is surfaced verbatim;
cvm never merges on your behalf.`,
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

		message, _ := cmd.Flags().GetString("message")
		return remote.Push(name, message)
	},
}

func init() {
	pushCmd.Flags().String("harness", "", "Harness whose active profile to push (default: claude)")
	pushCmd.Flags().StringP("message", "m", "", "Commit message for pending changes (default: \"cvm: update <profile> profile\")")
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
