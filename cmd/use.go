package cmd

import (
	"fmt"
	"sort"

	"github.com/chichex/cvm/internal/harness"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a profile",
	Long: `Activate a profile for every harness declared in its manifest.
Use --harness to target a specific harness only.
Use --none to go back to vanilla.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		none, _ := cmd.Flags().GetBool("none")
		harnessFlagSet := cmd.Flags().Changed("harness")

		if none {
			harnesses, err := harnessesForUseNone(cmd, harnessFlagSet)
			if err != nil {
				return err
			}
			if len(harnesses) == 0 {
				fmt.Println("Already vanilla")
				return nil
			}
			for _, h := range harnesses {
				if err := profile.UseNoneWithHarness(h); err != nil {
					return err
				}
				fmt.Printf("Switched %s harness to vanilla\n", h.Name())
			}
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("provide a profile name or use --none")
		}

		name := args[0]
		harnesses, err := harnessesForProfileUse(cmd, name, harnessFlagSet)
		if err != nil {
			return err
		}
		for _, h := range harnesses {
			if err := profile.UseWithHarness(name, h); err != nil {
				return err
			}
			fmt.Printf("Switched %s harness to %q\n", h.Name(), name)
		}
		return nil
	},
}

func init() {
	useCmd.Flags().Bool("none", false, "Switch to vanilla (no profile)")
	useCmd.Flags().String("harness", "", "Harness to target")
}

func harnessesForProfileUse(cmd *cobra.Command, name string, harnessFlagSet bool) ([]harness.Harness, error) {
	if harnessFlagSet {
		h, err := harnessFromFlag(cmd)
		if err != nil {
			return nil, err
		}
		return []harness.Harness{h}, nil
	}

	manifest, err := profile.LoadManifest(profile.ProfileDir(name))
	if err != nil {
		return nil, err
	}
	return harnessesByName(manifest.Harnesses)
}

func harnessesForUseNone(cmd *cobra.Command, harnessFlagSet bool) ([]harness.Harness, error) {
	if harnessFlagSet {
		h, err := harnessFromFlag(cmd)
		if err != nil {
			return nil, err
		}
		return []harness.Harness{h}, nil
	}

	st, err := state.Load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(st.Global.Harnesses))
	for name := range st.Global.Harnesses {
		names = append(names, name)
	}
	sort.Strings(names)
	return harnessesByName(names)
}

func harnessesByName(names []string) ([]harness.Harness, error) {
	harnesses := make([]harness.Harness, 0, len(names))
	for _, name := range names {
		h, ok := harness.ByName(name)
		if !ok {
			return nil, fmt.Errorf("unknown harness %q", name)
		}
		harnesses = append(harnesses, h)
	}
	return harnesses, nil
}
