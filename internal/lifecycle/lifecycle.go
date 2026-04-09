package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ayrtonmarini/cvm/internal/config"
	"github.com/ayrtonmarini/cvm/internal/kb"
	"github.com/ayrtonmarini/cvm/internal/state"
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

	return nil
}

func End(projectPath string) error {
	os.Remove(sessionPath())
	fmt.Println("cvm session ended")
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
	return nil
}

func detectTools() map[string]bool {
	tools := map[string]bool{}
	for _, tool := range []string{"codex", "aider", "gh", "docker", "node", "npm", "go"} {
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
