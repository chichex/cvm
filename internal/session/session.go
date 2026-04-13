// Package session implements the CVM session system.
// Spec: S-017 | Version: 0.4.0
package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/chichex/cvm/internal/automation"
	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/kb"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
)

// SessionEvent is the base struct for all JSONL lines.
// Concrete event types are discriminated by Type field.
// Additional fields use omitempty so unused fields are omitted.
// Spec: S-017 | Req: C-001, C-002, C-003
type SessionEvent struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"ts"`
	Content   string          `json:"content,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	AgentType string          `json:"agent_type,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Project   string          `json:"project,omitempty"`
	Profile   string          `json:"profile,omitempty"`
	PID       int             `json:"pid,omitempty"`
	Tools     map[string]bool `json:"tools,omitempty"`
	SummaryKey string         `json:"summary_key,omitempty"`
	Reason    string          `json:"reason,omitempty"`
}

// sessionsDir is a function variable so tests can override it.
// Spec: S-017 | Req: I-002
var sessionsDir = func() string {
	return filepath.Join(config.CvmHome(), "sessions")
}

// sessionPath returns the full .jsonl path for a UUID.
func sessionPath(uuid string) string {
	return filepath.Join(sessionsDir(), uuid+".jsonl")
}

// detectTools checks PATH for known tools.
// Spec: S-017 | Req: C-002
func detectTools() map[string]bool {
	tools := map[string]bool{}
	for _, tool := range []string{"claude", "codex", "gemini", "gh", "docker", "node", "npm", "go"} {
		_, err := exec.LookPath(tool)
		tools[tool] = err == nil
	}
	return tools
}

// truncate truncates s to maxRunes runes, appending "…" if truncated.
// Spec: S-017 | Req: C-004
func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

// withFileLock acquires an exclusive lock on the given file, runs fn, then releases.
// Spec: S-017 | Req: I-007
func withFileLock(f *os.File, fn func() error) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring file lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
	return fn()
}

// appendEvent appends a JSON-encoded event to the session file under lock.
// It first checks if the last line is an end event (E-002).
// Spec: S-017 | Req: I-007, I-011
func appendEvent(path string, event SessionEvent) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return withFileLock(f, func() error {
		// Check last line for end event (O(1): seek near EOF, scan backwards)
		// Spec: S-017 | Req: I-011, E-002
		info, err := f.Stat()
		if err != nil {
			return err
		}
		if info.Size() > 0 {
			lastLine, err := readLastLine(f, info.Size())
			if err == nil && lastLine != "" {
				var ev SessionEvent
				if json.Unmarshal([]byte(lastLine), &ev) == nil && ev.Type == "end" {
					return errAlreadyEnded
				}
			}
		}

		line, err := json.Marshal(event)
		if err != nil {
			return err
		}
		line = append(line, '\n')
		_, err = f.Write(line)
		return err
	})
}

// errAlreadyEnded is a sentinel used internally by appendEvent.
var errAlreadyEnded = fmt.Errorf("session already ended")

// readLastLine reads the last newline-terminated line from an open file.
// O(1) for typical line sizes: seeks near EOF and scans backward for a newline.
func readLastLine(f *os.File, size int64) (string, error) {
	// Seek to read up to 4096 bytes from the end.
	bufSize := int64(4096)
	if size < bufSize {
		bufSize = size
	}
	offset := size - bufSize
	buf := make([]byte, bufSize)
	n, err := f.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return "", err
	}
	buf = buf[:n]

	// Find the last newline.
	// The file ends with \n so find the second-to-last newline.
	end := len(buf)
	if end > 0 && buf[end-1] == '\n' {
		end--
	}
	start := end
	for start > 0 && buf[start-1] != '\n' {
		start--
	}
	return string(buf[start:end]), nil
}

// readStartEvent reads the first line of a session file and parses it as SessionEvent.
// Spec: S-017 | Req: C-002
func readStartEvent(path string) (*SessionEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty session file")
	}
	var ev SessionEvent
	if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
		return nil, fmt.Errorf("parsing start event: %w", err)
	}
	return &ev, nil
}

// isAlive checks if a PID is alive and its process name contains "claude".
// Spec: S-017 | Req: I-010
func isAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; send signal 0 to check existence.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return processNameContainsClaude(pid)
}

// processNameContainsClaude checks if the process name for pid contains "claude".
// Spec: S-017 | Req: I-010
func processNameContainsClaude(pid int) bool {
	var out []byte
	var err error
	if runtime.GOOS == "darwin" {
		out, err = exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=").Output()
	} else {
		commPath := fmt.Sprintf("/proc/%d/comm", pid)
		out, err = os.ReadFile(commPath)
	}
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(string(out))), "claude")
}

// isActive returns true if the session at path has no end event, a live PID, and process name "claude".
// Spec: S-017 | Req: B-007, I-010
func isActive(path string) bool {
	ev, err := readStartEvent(path)
	if err != nil {
		return false
	}
	if !isAlive(ev.PID) {
		return false
	}
	// Check no end event exists.
	return !hasEndEvent(path)
}

// hasEndEvent returns true if the session file has an "end" event as its last line.
func hasEndEvent(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return false
	}
	lastLine, err := readLastLine(f, info.Size())
	if err != nil || lastLine == "" {
		return false
	}
	var ev SessionEvent
	if json.Unmarshal([]byte(lastLine), &ev) != nil {
		return false
	}
	return ev.Type == "end"
}

// countEvents counts JSON lines in a session file.
func countEvents(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	// Use larger buffer to handle long lines.
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			count++
		}
	}
	return count
}

// readAllEvents reads all events from a session file. Invalid JSON lines are skipped.
// Spec: S-017 | Req: I-008, E-011
func readAllEvents(path string) ([]SessionEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []SessionEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev SessionEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping invalid JSON line: %v\n", err)
			continue
		}
		events = append(events, ev)
	}
	return events, scanner.Err()
}

// resolvePrefix resolves a UUID prefix to a full session UUID.
// Spec: S-017 | Req: E-009
func resolvePrefix(prefix string) (string, error) {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading sessions dir: %w", err)
	}

	var matches []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".jsonl")
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("session %s not found", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous prefix %s, matches: %s", prefix, strings.Join(matches, ", "))
	}
}

// cleanOrphans scans the sessions dir and marks orphaned sessions.
// An orphan is a session with no end event and PID dead or not "claude".
// Spec: S-017 | Req: E-007
func cleanOrphans() {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		uuid := strings.TrimSuffix(e.Name(), ".jsonl")

		// Read start event to get PID.
		ev, err := readStartEvent(path)
		if err != nil {
			continue
		}
		if ev.Type != "start" {
			continue
		}

		// Skip already-ended sessions.
		if hasEndEvent(path) {
			continue
		}

		// If PID alive and process is "claude", it's not an orphan.
		if isAlive(ev.PID) {
			continue
		}

		// Orphan: acquire lock, verify no end event (check-under-lock), write end event.
		// Spec: S-017 | Req: E-007
		f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			continue
		}
		_ = withFileLock(f, func() error {
			info, _ := f.Stat()
			if info != nil && info.Size() > 0 {
				lastLine, _ := readLastLine(f, info.Size())
				var check SessionEvent
				if json.Unmarshal([]byte(lastLine), &check) == nil && check.Type == "end" {
					return nil // already ended under lock
				}
			}
			endEv := SessionEvent{
				Type:       "end",
				Timestamp:  time.Now().Format(time.RFC3339),
				SummaryKey: "",
				Reason:     "orphan",
			}
			line, _ := json.Marshal(endEv)
			line = append(line, '\n')
			_, err := f.Write(line)
			if err == nil {
				fmt.Fprintf(os.Stderr, "warning: cleaned up orphan session %s (PID %d dead)\n", uuid, ev.PID)
			}
			return err
		})
		f.Close()
	}
}

// generateUUID generates a UUID v4 using os.ReadFile of /dev/urandom.
func generateUUID() string {
	f, err := os.Open("/dev/urandom")
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	defer f.Close()
	b := make([]byte, 16)
	io.ReadFull(f, b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Start creates a new session file with a start event.
// Spec: S-017 | Req: B-001
func Start(sessionID, project, profile string, pid int) error {
	// Ensure sessions dir exists. Spec: S-017 | Req: I-008
	if err := os.MkdirAll(sessionsDir(), 0755); err != nil {
		return fmt.Errorf("creating sessions dir: %w", err)
	}

	// Run orphan cleanup before creating new session. Spec: S-017 | Req: E-007, B-001
	cleanOrphans()

	if sessionID == "" {
		sessionID = generateUUID()
	}
	if pid == 0 {
		pid = os.Getppid()
	}

	tools := detectTools()

	ev := SessionEvent{
		Type:      "start",
		Timestamp: time.Now().Format(time.RFC3339),
		SessionID: sessionID,
		Project:   project,
		Profile:   profile,
		PID:       pid,
		Tools:     tools,
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	path := sessionPath(sessionID)
	if err := os.WriteFile(path, line, 0644); err != nil {
		return fmt.Errorf("creating session file: %w", err)
	}

	// Print UUID to stdout (for programmatic capture by hooks/tests)
	fmt.Println(sessionID)

	// Print session info to stderr (same output as legacy lifecycle.Start)
	// UUID goes to stdout for programmatic capture; stats go to stderr for human info
	fmt.Fprintf(os.Stderr, "cvm session started\n")
	st, _ := state.Load()
	if st != nil {
		globalName := st.Global.Active
		localName := st.GetLocal(project)
		fmt.Fprintf(os.Stderr, "  global: %s\n", globalName)
		if localName != "" {
			fmt.Fprintf(os.Stderr, "  local:  %s\n", localName)
		}
	}
	globalTotal, globalEnabled, globalStale, _ := kb.Stats(config.ScopeGlobal, project)
	localTotal, localEnabled, localStale, _ := kb.Stats(config.ScopeLocal, project)
	fmt.Fprintf(os.Stderr, "  kb:     %d global (%d enabled, %d stale), %d local (%d enabled, %d stale)\n",
		globalTotal, globalEnabled, globalStale, localTotal, localEnabled, localStale)
	toolNames := make([]string, 0, len(tools))
	for t, available := range tools {
		if available {
			toolNames = append(toolNames, t)
		}
	}
	sort.Strings(toolNames)
	fmt.Fprintf(os.Stderr, "  tools:  [%s]\n", strings.Join(toolNames, " "))

	return nil
}

// Append appends an event to an existing session file.
// Spec: S-017 | Req: B-002, B-003, B-004, C-004, E-001, E-002, I-007, I-011
func Append(sessionID, eventType, content, tool, agentType string) error {
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "error: session_id is required")
		os.Exit(1)
	}

	// Apply truncation limits. Spec: S-017 | Req: C-004
	switch eventType {
	case "prompt":
		content = truncate(content, 300)
	case "tool":
		content = truncate(content, 200)
	case "agent":
		content = truncate(content, 300)
	}

	path := sessionPath(sessionID)

	// E-001: file doesn't exist → no-op with warning
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "warning: session %s not found, skipping append\n", sessionID)
		return nil
	}

	ev := SessionEvent{
		Type:      eventType,
		Timestamp: time.Now().Format(time.RFC3339),
		Content:   content,
		Tool:      tool,
		AgentType: agentType,
	}

	err := appendEvent(path, ev)
	if err == errAlreadyEnded {
		// E-002: session already ended → no-op with warning
		fmt.Fprintf(os.Stderr, "warning: session %s already ended, ignoring append\n", sessionID)
		return nil
	}
	return err
}

// End closes a session, optionally generating a summary.
// Spec: S-017 | Req: B-005, B-006, E-003, E-004, E-010
func End(sessionID string) error {
	path := sessionPath(sessionID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: session %s not found\n", sessionID)
		os.Exit(1)
	}

	// Read events snapshot WITHOUT holding lock during LLM call. Spec: S-017 | Req: B-005
	events, err := readAllEvents(path)
	if err != nil {
		return fmt.Errorf("reading session events: %w", err)
	}

	// Compaction for summarization only (read-time, file not rewritten). Spec: S-017 | Req: E-006, C-005
	summaryEvents := events
	if len(events) > 1000 {
		summaryEvents = append(events[:1], events[len(events)-999:]...)
	}

	// Determine whether to generate summary. Spec: S-017 | Req: B-005, B-006, E-003, I-009
	autosummaryEnabled := os.Getenv("CVM_AUTOSUMMARY_ENABLED") != "false"
	model := os.Getenv("CVM_AUTOSUMMARY_MODEL")
	if model == "" {
		model = "haiku"
	}

	summaryKey := ""
	reason := "normal"

	// E-003: skip summary on short sessions (< 3 events)
	if autosummaryEnabled && len(events) >= 3 {
		key, genErr := generateSummary(sessionID, summaryEvents, model)
		if genErr != nil {
			// E-004: summary failure → warn, continue
			fmt.Fprintf(os.Stderr, "warning: summary generation failed: %v\n", genErr)
			reason = "error"
		} else {
			summaryKey = key
		}
	}

	// Append end event under lock. Check for concurrent end (E-010).
	endEv := SessionEvent{
		Type:       "end",
		Timestamp:  time.Now().Format(time.RFC3339),
		SummaryKey: summaryKey,
		Reason:     reason,
	}
	err = appendEvent(path, endEv)
	if err == errAlreadyEnded {
		fmt.Fprintf(os.Stderr, "warning: session %s already ended\n", sessionID)
		return nil
	}
	if err != nil {
		return err
	}

	// Cleanup learning-pulse. Spec: S-017 | Req: B-005
	_ = os.Remove(filepath.Join(config.CvmHome(), "learning-pulse"))

	// Automation integration (reused from lifecycle.End). Spec: S-017 | Req: B-005
	projectPath := ""
	if len(events) > 0 && events[0].Type == "start" {
		projectPath = events[0].Project
	}
	if err := runEndAutomation(projectPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: automation integration failed: %v\n", err)
	}

	return nil
}

// generateSummary calls claude CLI to summarize the session and stores result in global KB.
// Returns the KB key. Spec: S-017 | Req: B-005, C-008
func generateSummary(sessionID string, events []SessionEvent, model string) (string, error) {
	// Build events text for prompt. Spec: S-017 | Req: C-008
	var sb strings.Builder
	for _, ev := range events {
		line, _ := json.Marshal(ev)
		sb.Write(line)
		sb.WriteByte('\n')
	}
	eventsText := sb.String()

	promptText := fmt.Sprintf(`Summarize this coding session from the captured events.
Generate JSON: {"request": "...", "accomplished": "...", "discovered": "...", "next_steps": "..."}
Max 1-2 sentences per field. Output ONLY the JSON.

<events>
%s
</events>`, eventsText)

	cmd := exec.Command("claude", "-p", "--model", model)
	cmd.Stdin = strings.NewReader(promptText)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude invocation failed: %w", err)
	}

	summaryBody := strings.TrimSpace(string(out))
	if summaryBody == "" {
		return "", fmt.Errorf("empty summary output")
	}

	// Key format: session-summary-<YYYYMMDD-HHMMSS>-<uuid8>. Spec: S-017 | Req: B-005
	uuid8 := sessionID
	if len(uuid8) > 8 {
		uuid8 = uuid8[:8]
	}
	key := fmt.Sprintf("session-summary-%s-%s", time.Now().Format("20060102-150405"), uuid8)

	if err := kb.Put(config.ScopeGlobal, "", key, summaryBody, []string{"session", "summary"}); err != nil {
		return "", fmt.Errorf("storing summary in KB: %w", err)
	}

	return key, nil
}

// runEndAutomation replicates lifecycle.End automation integration.
// Spec: S-017 | Req: B-005
func runEndAutomation(projectPath string) error {
	if err := saveActiveProfiles(projectPath); err != nil {
		return err
	}

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

// saveActiveProfiles saves the currently active profiles (reused from lifecycle).
func saveActiveProfiles(projectPath string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	if st.Global.Active != "" {
		if err := profile.Save(config.ScopeGlobal, st.Global.Active, ""); err != nil {
			return fmt.Errorf("saving active global profile %q: %w", st.Global.Active, err)
		}
	}
	localProfile := st.GetLocal(projectPath)
	if localProfile != "" {
		if err := profile.Save(config.ScopeLocal, localProfile, projectPath); err != nil {
			return fmt.Errorf("saving active local profile %q: %w", localProfile, err)
		}
	}
	return nil
}

// queueAutomationRunner starts the automation runner in background (reused from lifecycle).
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

// taggedEntryCount counts KB entries tagged as learning/gotcha/decision.
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

// sessionFileInfo holds summary info for listing.
type sessionFileInfo struct {
	uuid      string
	path      string
	startTime time.Time
	startEv   *SessionEvent
	active    bool
	eventCount int
}

// listSessionFiles returns all session files sorted by start time descending.
func listSessionFiles(limit int) ([]sessionFileInfo, error) {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var infos []sessionFileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		uuid := strings.TrimSuffix(e.Name(), ".jsonl")
		path := filepath.Join(dir, e.Name())

		ev, err := readStartEvent(path)
		if err != nil || ev.Type != "start" {
			continue
		}
		ts, err := time.Parse(time.RFC3339, ev.Timestamp)
		if err != nil {
			ts = time.Time{}
		}

		active := !hasEndEvent(path) && isAlive(ev.PID)
		count := countEvents(path)

		infos = append(infos, sessionFileInfo{
			uuid:       uuid,
			path:       path,
			startTime:  ts,
			startEv:    ev,
			active:     active,
			eventCount: count,
		})
	}

	// Sort by start time descending.
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].startTime.After(infos[j].startTime)
	})

	if limit > 0 && len(infos) > limit {
		infos = infos[:limit]
	}
	return infos, nil
}

// Status lists active sessions.
// Spec: S-017 | Req: B-007
func Status() error {
	infos, err := listSessionFiles(0)
	if err != nil {
		return err
	}

	activeCount := 0
	for _, info := range infos {
		if !info.active {
			continue
		}
		activeCount++
		shortUUID := info.uuid
		if len(shortUUID) > 8 {
			shortUUID = shortUUID[:8]
		}
		fmt.Printf("%s  project=%-40s  profile=%s  started=%s  events=%d\n",
			shortUUID,
			info.startEv.Project,
			info.startEv.Profile,
			info.startTime.Format("2006-01-02 15:04:05"),
			info.eventCount,
		)
	}

	if activeCount == 0 {
		fmt.Println("No active sessions")
	}
	return nil
}

// List lists all sessions ordered by start time descending.
// Spec: S-017 | Req: B-008
func List(limit int) error {
	infos, err := listSessionFiles(limit)
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	for _, info := range infos {
		status := "closed"
		if info.active {
			status = "active"
		}
		shortUUID := info.uuid
		if len(shortUUID) > 8 {
			shortUUID = shortUUID[:8]
		}
		fmt.Printf("%s  %-7s  %-40s  %s  events=%d\n",
			shortUUID,
			status,
			info.startEv.Project,
			info.startTime.Format("2006-01-02 15:04:05"),
			info.eventCount,
		)
	}
	return nil
}

// Show prints all events for a session. Supports UUID prefix matching.
// Spec: S-017 | Req: B-009, E-009, I-008
func Show(sessionID string) error {
	// Resolve prefix if needed. Spec: S-017 | Req: E-009
	uuid, err := resolvePrefix(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	path := sessionPath(uuid)
	events, err := readAllEvents(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: session %s not found\n", sessionID)
		os.Exit(1)
	}

	for _, ev := range events {
		ts := ev.Timestamp
		switch ev.Type {
		case "start":
			fmt.Printf("[%s] START  session=%s project=%s profile=%s pid=%d\n",
				ts, ev.SessionID, ev.Project, ev.Profile, ev.PID)
		case "end":
			fmt.Printf("[%s] END    summary_key=%s reason=%s\n", ts, ev.SummaryKey, ev.Reason)
		case "prompt":
			fmt.Printf("[%s] PROMPT %s\n", ts, ev.Content)
		case "tool":
			fmt.Printf("[%s] TOOL   %s: %s\n", ts, ev.Tool, ev.Content)
		case "agent":
			fmt.Printf("[%s] AGENT  %s: %s\n", ts, ev.AgentType, ev.Content)
		default:
			line, _ := json.Marshal(ev)
			fmt.Printf("[%s] %s\n", ts, string(line))
		}
	}
	return nil
}

// GC deletes closed session files with mtime older than olderThan.
// Spec: S-017 | Req: B-010
func GC(olderThan time.Duration) error {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("deleted 0 session(s)")
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-olderThan)
	deleted := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, e.Name())

		// Never delete active sessions. Spec: S-017 | Req: B-010
		if isActive(path) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err == nil {
				deleted++
			}
		}
	}

	fmt.Printf("deleted %d session(s)\n", deleted)
	return nil
}
