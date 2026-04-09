package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/kb"
	"github.com/chichex/cvm/internal/retro"
	"github.com/spf13/cobra"
)

var retroAuto bool

var retroCmd = &cobra.Command{
	Use:   "retro",
	Short: "Session retrospective — extract and persist learnings",
	Long:  "Analyze the most recent Claude Code session transcript and persist learnings, gotchas, and decisions to the KB.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !retroAuto {
			fmt.Println("Use /retro inside a Claude Code session for interactive mode.")
			fmt.Println("Use --auto for headless mode (called by SessionEnd hook).")
			return nil
		}
		projectPath, err := getProjectPath()
		if err != nil {
			return err
		}
		return runAutoRetro(projectPath)
	},
}

func init() {
	retroCmd.Flags().BoolVar(&retroAuto, "auto", false, "Run headless — extract and persist learnings without user confirmation")
	rootCmd.AddCommand(retroCmd)
}

func runAutoRetro(projectPath string) error {
	// 1. Find the most recent transcript
	transcriptPath, err := retro.FindLatestTranscript(projectPath)
	if err != nil {
		return fmt.Errorf("finding transcript: %w", err)
	}

	// 2. Extract conversation text
	conversation, err := retro.ExtractConversation(transcriptPath)
	if err != nil {
		return fmt.Errorf("extracting conversation: %w", err)
	}
	if len(strings.TrimSpace(conversation)) < 200 {
		// Too short to have meaningful learnings
		return nil
	}

	// 3. Get existing KB keys for dedup context
	existingKeys := collectExistingKeys(projectPath)

	// 4. Build the headless prompt
	prompt := buildRetroPrompt(conversation, existingKeys)

	// 5. Launch claude -p in background
	claude := exec.Command("claude", "-p", prompt, "--allowedTools", "Bash(cvm *)")
	claude.Stdout = os.Stdout
	claude.Stderr = os.Stderr

	if err := claude.Start(); err != nil {
		return fmt.Errorf("launching claude: %w", err)
	}

	// Detach — don't wait for completion
	go func() {
		_ = claude.Wait()
	}()

	return nil
}

func collectExistingKeys(projectPath string) []string {
	var keys []string
	for _, scope := range []config.Scope{config.ScopeGlobal, config.ScopeLocal} {
		entries, err := kb.List(scope, projectPath, "")
		if err != nil {
			continue
		}
		for _, e := range entries {
			keys = append(keys, e.Key)
		}
	}
	return keys
}

func buildRetroPrompt(conversation string, existingKeys []string) string {
	keysSection := ""
	if len(existingKeys) > 0 {
		keysSection = fmt.Sprintf(`
## Existing KB entries (DO NOT duplicate these)
%s
`, strings.Join(existingKeys, ", "))
	}

	return fmt.Sprintf(`You are a session retrospective analyzer for Claude Code.

TASK: Review this conversation and extract NON-OBVIOUS learnings worth persisting.

RULES:
- Only genuinely useful items for future sessions
- Skip trivial things, things derivable from code/git, ephemeral task details
- Skip sensitive info (tokens, passwords, env vars)
- If nothing worth saving: just say "No learnings." and stop

CATEGORIES:
- learning: unexpected behaviors, patterns that worked/failed, workarounds
- gotcha: silent failures, config traps, costly debugging
- decision: design choices, trade-offs, intentional tech debt
%s
ACTION: For each item, run this exact command:
cvm kb put "<key>" --body "<one-line description>" --tag "<category>,<area>"

Keys must be lowercase-kebab-case. Bodies must be 1 sentence max.

CONVERSATION:
%s`, keysSection, conversation)
}
