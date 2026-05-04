package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chichex/cvm/internal/profile"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Inspect and author profile contents",
	RunE: func(cmd *cobra.Command, args []string) error {
		active, err := profile.Current()
		if err != nil {
			return err
		}
		if active == "" {
			fmt.Println("No active profiles")
			return nil
		}
		return printInventory(active)
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Inspect a specific stored profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return printInventory(args[0])
	},
}

var profileAddCmd = &cobra.Command{
	Use:   "add <skill|hook|agent|instructions> [name]",
	Short: "Scaffold a profile asset",
	Long: `Scaffold profile assets without editing cvm's internal layout directly.

Portable assets are cvm-owned concepts written under portable/: instructions,
skills, and agents. Passing --harness writes a harness-specific override instead.
Hooks are always harness-specific and require --harness.

Portable assets are authoring inputs for the portable contract. Harness rendering
is planned separately, so use --harness when you need an asset applied by today's
activation flow.`,
	Example: `  cvm profile add instructions --profile work
  cvm profile add skill deploy --profile work
  cvm profile add agent reviewer --profile work
  cvm profile add hook post --profile work --harness claude
  cvm profile add skill deploy --profile work --harness opencode --from-file ./deploy.md`,
	Args: validateProfileAddArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")
		if profileName == "" {
			current, err := profile.Current()
			if err != nil {
				return err
			}
			if current == "" {
				return fmt.Errorf("no active profile; pass --profile <name>")
			}
			profileName = current
		}

		harnessName, _ := cmd.Flags().GetString("harness")
		fromFile, _ := cmd.Flags().GetString("from-file")
		open, _ := cmd.Flags().GetBool("open")

		name := ""
		if len(args) > 1 {
			name = args[1]
		}

		asset, err := profile.ScaffoldAsset(profile.ScaffoldAssetOptions{
			ProfileName: profileName,
			Kind:        args[0],
			Name:        name,
			HarnessName: harnessName,
			FromFile:    fromFile,
		})
		if err != nil {
			return err
		}

		if asset.Created {
			fmt.Printf("Created %s %s: %s\n", asset.Layer, asset.Kind, asset.Path)
		} else {
			fmt.Printf("Profile asset already exists: %s\n", asset.Path)
		}
		if asset.ManifestCreated {
			fmt.Printf("Created manifest: %s\n", filepath.Join(profile.ProfileDir(profileName), "cvm.profile.toml"))
		}
		if asset.Portable {
			fmt.Println("Note: portable assets are authored now; harness rendering is planned separately.")
		}
		if open {
			return openAssetEditor(asset.Path)
		}
		return nil
	},
}

func init() {
	profileAddCmd.Flags().String("profile", "", "Profile to edit (default: active profile in selected scope)")
	profileAddCmd.Flags().String("harness", "", "Write a harness-specific asset instead of a portable asset")
	profileAddCmd.Flags().String("from-file", "", "Seed the asset from an existing file")
	profileAddCmd.Flags().Bool("open", false, "Open the scaffolded asset in $EDITOR")
	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileShowCmd)
	rootCmd.AddCommand(profileCmd)
}

func validateProfileAddArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("provide an asset type: skill, hook, agent, or instructions")
	}
	switch args[0] {
	case "instructions":
		if len(args) != 1 {
			return fmt.Errorf("instructions does not take a name")
		}
	case "skill", "hook", "agent":
		if len(args) != 2 {
			return fmt.Errorf("%s requires a name", args[0])
		}
	default:
		return fmt.Errorf("unknown type %q: must be one of skill, hook, agent, instructions", args[0])
	}
	return nil
}

func printInventory(name string) error {
	inv, err := profile.Inspect(name)
	if err != nil {
		return err
	}
	if !inv.Exists {
		fmt.Printf("Profile %q not found\n", name)
		return nil
	}

	fmt.Printf("Profile: %s\n", inv.Name)
	fmt.Printf("Path: %s\n", inv.Path)

	if len(inv.Files) == 0 && len(inv.Dirs) == 0 {
		fmt.Println("Contents: empty")
		return nil
	}

	if len(inv.Files) > 0 {
		fmt.Printf("Files: %s\n", strings.Join(inv.Files, ", "))
	}

	keys := make([]string, 0, len(inv.Dirs))
	for key := range inv.Dirs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		children := inv.Dirs[key]
		fmt.Printf("%s (%d): %s\n", titleWord(key), len(children), strings.Join(children, ", "))
	}

	if len(inv.MCPServers) > 0 {
		fmt.Printf("MCP servers (%d): %s\n", len(inv.MCPServers), strings.Join(inv.MCPServers, ", "))
	}

	return nil
}

func titleWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
