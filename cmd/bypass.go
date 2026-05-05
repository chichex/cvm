package cmd

import (
	"fmt"
	"sort"

	"github.com/chichex/cvm/internal/harness"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	"github.com/spf13/cobra"
)

var bypassCmd = &cobra.Command{
	Use:   "bypass [on|off|status]",
	Short: "Inspect or change bypass permissions for active profiles",
	Long: `Bypass permissions are persisted as overrides where possible, so they
survive 'cvm pull'. Codex stores bypass directly in ~/.codex/config.toml
because cvm has no TOML-level override merge yet.

By default the command targets every harness with an active profile. Use
--harness to scope to one of: claude, opencode, codex.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := "status"
		if len(args) > 0 {
			mode = args[0]
		}

		switch mode {
		case "status":
			return showBypassStatus(cmd)
		case "on":
			return enableBypass(cmd)
		case "off":
			return disableBypass(cmd)
		default:
			return fmt.Errorf("use 'on', 'off', or 'status'")
		}
	},
}

func init() {
	bypassCmd.Flags().String("harness", "", "Harness to target (claude, opencode, codex). Default: every active harness")
}

type bypassTarget struct {
	harness     harness.Harness
	profileName string
}

func showBypassStatus(cmd *cobra.Command) error {
	targets, err := activeBypassTargets(cmd)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Println("No active profiles")
		return nil
	}

	for _, target := range targets {
		mode, err := target.harness.BypassStatus(target.profileName)
		if err != nil {
			mode = ""
		}
		if mode == "" {
			mode = "(default)"
		}
		fmt.Printf("%s profile %q: %s\n", target.harness.Name(), target.profileName, mode)
	}
	return nil
}

func enableBypass(cmd *cobra.Command) error {
	targets, err := activeBypassTargets(cmd)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no active profiles")
	}

	for _, target := range targets {
		if err := target.harness.EnableBypass(target.profileName); err != nil {
			return fmt.Errorf("%s: %w", target.harness.Name(), err)
		}
		if err := profile.ReapplyWithHarness(target.profileName, target.harness); err != nil {
			return fmt.Errorf("%s: %w", target.harness.Name(), err)
		}
		mode, err := target.harness.BypassStatus(target.profileName)
		if err != nil || mode == "" {
			mode = "bypassPermissions"
		}
		fmt.Printf("%s profile %q: %s\n", target.harness.Name(), target.profileName, mode)
	}

	return nil
}

func disableBypass(cmd *cobra.Command) error {
	targets, err := activeBypassTargets(cmd)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no active profiles")
	}

	for _, target := range targets {
		if err := target.harness.DisableBypass(target.profileName); err != nil {
			return fmt.Errorf("%s: %w", target.harness.Name(), err)
		}
		if err := profile.ReapplyWithHarness(target.profileName, target.harness); err != nil {
			return fmt.Errorf("%s: %w", target.harness.Name(), err)
		}
		fmt.Printf("%s profile %q: default\n", target.harness.Name(), target.profileName)
	}

	return nil
}

// activeBypassTargets resolves the list of (harness, active profile) pairs to
// operate on. With --harness, only that harness is considered; otherwise we
// iterate every harness that currently has an active profile.
func activeBypassTargets(cmd *cobra.Command) ([]bypassTarget, error) {
	st, err := state.Load()
	if err != nil {
		return nil, err
	}

	flagSet := cmd.Flags().Changed("harness")
	if flagSet {
		h, err := harnessFromFlag(cmd)
		if err != nil {
			return nil, err
		}
		name := st.GetGlobalHarness(h.Name())
		if name == "" {
			return nil, nil
		}
		return []bypassTarget{{harness: h, profileName: name}}, nil
	}

	// No flag: every active harness.
	names := make([]string, 0, len(st.Global.Harnesses))
	for n := range st.Global.Harnesses {
		names = append(names, n)
	}
	sort.Strings(names)

	targets := make([]bypassTarget, 0, len(names))
	for _, hName := range names {
		h, ok := harness.ByName(hName)
		if !ok {
			continue
		}
		profileName := st.GetGlobalHarness(hName)
		if profileName == "" {
			continue
		}
		targets = append(targets, bypassTarget{harness: h, profileName: profileName})
	}
	return targets, nil
}
