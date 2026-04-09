package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chichex/cvm/internal/automation"
	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/kb"
	"github.com/chichex/cvm/internal/state"
)

type SessionInfo struct {
	StartedAt   time.Time       `json:"started_at"`
	ProjectPath string          `json:"project_path"`
	Profile     string          `json:"profile"`
	Tools       map[string]bool `json:"tools"`
}

func sessionPath() string {
	return filepath.Join(config.CvmHome(), "session.json")
}

func Start(projectPath string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}

	globalProfile := st.Global.Active
	localProfile := st.GetLocal(projectPath)
	tools := detectTools()

	session := SessionInfo{
		StartedAt:   time.Now(),
		ProjectPath: projectPath,
		Profile:     globalProfile,
		Tools:       tools,
	}
	if localProfile != "" {
		session.Profile = localProfile + " (local) + " + globalProfile + " (global)"
	}

	data, _ := json.MarshalIndent(session, "", "  ")
	os.MkdirAll(config.CvmHome(), 0755)
	os.WriteFile(sessionPath(), data, 0644)

	fmt.Printf("cvm session started\n")
	fmt.Printf("  global: %s\n", displayProfile(globalProfile))
	if localProfile != "" {
		fmt.Printf("  local:  %s\n", localProfile)
	}

	globalTotal, globalEnabled, globalStale, _ := kb.Stats(config.ScopeGlobal, projectPath)
	localTotal, localEnabled, localStale, _ := kb.Stats(config.ScopeLocal, projectPath)

	if globalTotal > 0 || localTotal > 0 {
		fmt.Printf("  kb:     %d global (%d enabled, %d stale), %d local (%d enabled, %d stale)\n",
			globalTotal, globalEnabled, globalStale, localTotal, localEnabled, localStale)
	}

	var available []string
	for tool, ok := range tools {
		if ok {
			available = append(available, tool)
		}
	}
	if len(available) > 0 {
		fmt.Printf("  tools:  %v\n", available)
	}

	autoState, err := automation.Load()
	if err == nil && autoState.PendingCount() > 0 {
		fmt.Printf("  automation: %d pending candidate(s)\n", autoState.PendingCount())
	}

	return nil
}

func End(projectPath string) error {
	_ = os.Remove(filepath.Join(config.CvmHome(), "learning-pulse"))

	globalTotal, _, globalStale, _ := kb.Stats(config.ScopeGlobal, projectPath)
	localTotal, _, localStale, _ := kb.Stats(config.ScopeLocal, projectPath)

	autoState, err := automation.Load()
	if err != nil {
		return err
	}

	queued := autoState.RecordSessionEnd(
		automation.Snapshot{
			Scope:  config.ScopeGlobal,
			Total:  globalTotal,
			Stale:  globalStale,
			Tagged: taggedEntryCount(config.ScopeGlobal, projectPath),
		},
		automation.Snapshot{
			Scope:       config.ScopeLocal,
			ProjectPath: projectPath,
			Total:       localTotal,
			Stale:       localStale,
			Tagged:      taggedEntryCount(config.ScopeLocal, projectPath),
		},
	)
	if err := automation.MaterializePending(autoState); err != nil {
		return err
	}
	if err := autoState.Save(); err != nil {
		return err
	}

	_ = os.Remove(sessionPath())
	fmt.Println("cvm session ended")
	if len(queued) > 0 {
		fmt.Printf("  automation: %d candidate(s) queued\n", len(queued))
	}
	if autoState.PendingCount() > 0 {
		if runnerQueued, err := queueAutomationRunner(); runnerQueued {
			fmt.Println("  automation: runner queued in background")
		} else if err != nil {
			fmt.Printf("  automation: runner skipped (%v)\n", err)
		}
	}
	return nil
}

func Status() error {
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No active cvm session")
			return nil
		}
		return err
	}

	var session SessionInfo
	json.Unmarshal(data, &session)

	fmt.Printf("Session active since: %s\n", session.StartedAt.Format(time.RFC3339))
	fmt.Printf("Project: %s\n", session.ProjectPath)
	fmt.Printf("Profile: %s\n", session.Profile)

	var tools []string
	for tool, ok := range session.Tools {
		if ok {
			tools = append(tools, tool)
		}
	}
	if len(tools) > 0 {
		fmt.Printf("Tools: %v\n", tools)
	}

	autoState, err := automation.Load()
	if err == nil {
		if autoState.PendingCount() == 0 {
			fmt.Println("Automation: no pending candidates")
		} else {
			fmt.Printf("Automation: %d pending candidate(s)\n", autoState.PendingCount())
			for _, candidate := range autoState.Pending {
				scope := candidate.Scope
				if candidate.ProjectPath != "" {
					scope = fmt.Sprintf("%s:%s", scope, candidate.ProjectPath)
				}
				fmt.Printf("  - %s [%s] %s\n", candidate.Kind, scope, candidate.Reason)
			}
		}
	}
	return nil
}

func detectTools() map[string]bool {
	tools := map[string]bool{}
	for _, tool := range []string{"claude", "codex", "aider", "gh", "docker", "node", "npm", "go"} {
		_, err := exec.LookPath(tool)
		tools[tool] = err == nil
	}
	return tools
}

func displayProfile(name string) string {
	if name == "" {
		return "(vanilla)"
	}
	return name
}

func queueAutomationRunner() (bool, error) {
	binPath, err := os.Executable()
	if err != nil {
		return false, err
	}

	logDir := filepath.Join(config.CvmHome(), "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return false, err
	}

	stamp := time.Now().Format("20060102-150405")
	logFile, err := os.OpenFile(filepath.Join(logDir, "automation-"+stamp+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return false, err
	}

	cmd := exec.Command(binPath, "automation", "run")
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return false, err
	}

	return true, logFile.Close()
}

func taggedEntryCount(scope config.Scope, projectPath string) int {
	entries, err := kb.List(scope, projectPath, "")
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		for _, tag := range entry.Tags {
			switch strings.ToLower(tag) {
			case "learning", "gotcha", "decision":
				count++
				goto nextEntry
			}
		}
	nextEntry:
	}

	return count
}
