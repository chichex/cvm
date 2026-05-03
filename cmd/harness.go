package cmd

import (
	"fmt"

	"github.com/chichex/cvm/internal/harness"
	"github.com/spf13/cobra"
)

const defaultHarnessName = "claude"

func harnessFromFlag(cmd *cobra.Command) (harness.Harness, error) {
	name, _ := cmd.Flags().GetString("harness")
	if name == "" {
		name = defaultHarnessName
	}
	h, ok := harness.ByName(name)
	if !ok {
		return nil, fmt.Errorf("unknown harness %q", name)
	}
	return h, nil
}

func selectedHarnesses(cmd *cobra.Command) ([]harness.Harness, error) {
	name, _ := cmd.Flags().GetString("harness")
	if name == "" {
		return harness.All(), nil
	}
	h, ok := harness.ByName(name)
	if !ok {
		return nil, fmt.Errorf("unknown harness %q", name)
	}
	return []harness.Harness{h}, nil
}
