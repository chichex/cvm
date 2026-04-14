// Package session implements the CVM session system.
// Spec: S-017 | Version: 0.6.0
package session

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/chichex/cvm/internal/automation"
	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/kb"
	"github.com/chichex/cvm/internal/profile"
	"github.com/chichex/cvm/internal/state"
	_ "modernc.org/sqlite"
)

// SessionEvent is the base struct for all JSONL lines.
// Spec: S-017 | Req: C-003, C-004, C-005
type SessionEvent struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"ts"`
	Content   string          `json:"content,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	AgentType string          `json:"agent_type,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Project   string          `json:"project,omitempty"`
	Profile   string          `json:"profile,omitempty"`
	Tools     map[string]bool `json:"tools,omitempty"`
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
// Spec: S-017 | Req: C-004
func detectTools() map[string]bool {
	tools := map[string]bool{}
	for _, tool := range []string{"claude", "codex", "gemini", "gh", "docker", "node", "npm", "go"} {
		_, err := exec.LookPath(tool)
		tools[tool] = err == nil
	}
	return tools
}

// truncate truncates s to maxRunes runes, appending "..." if truncated.
// Spec: S-017 | Req: C-006
func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "\u2026"
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
// Spec: S-017 | Req: I-007, I-011
func appendEvent(path string, event SessionEvent) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return withFileLock(f, func() error {
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
func readLastLine(f *os.File, size int64) (string, error) {
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
// Spec: S-017 | Req: C-004
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
// Spec: S-017 | Req: I-008, E-001
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
// Tries SQLite first, falls back to JSONL file scan.
// Spec: S-017 | Req: E-007
func resolvePrefix(prefix string) (string, error) {
	db, err := openGlobalDB()
	if err == nil {
		defer db.Close()
		rows, qErr := db.Query(`SELECT id FROM sessions WHERE id LIKE ?||'%'`, prefix)
		if qErr == nil {
			defer rows.Close()
			var matches []string
			for rows.Next() {
				var id string
				if rows.Scan(&id) == nil {
					matches = append(matches, id)
				}
			}
			switch len(matches) {
			case 1:
				return matches[0], nil
			case 0:
				// Fall through to JSONL scan
			default:
				return "", fmt.Errorf("ambiguous prefix %s, matches: %s", prefix, strings.Join(matches, ", "))
			}
		}
	}

	// Fallback: scan JSONL files.
	dir := sessionsDir()
	entries, err2 := os.ReadDir(dir)
	if err2 != nil {
		return "", fmt.Errorf("reading sessions dir: %w", err2)
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

// sessionsSchema is the minimal DDL for the sessions table.
// Idempotent — uses CREATE TABLE IF NOT EXISTS.
// Spec: S-017 | Req: C-001, C-002a
const sessionsSchema = `
CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    status      TEXT NOT NULL DEFAULT 'active',
    project     TEXT NOT NULL,
    profile     TEXT NOT NULL DEFAULT '',
    started_at  TEXT NOT NULL,
    ended_at    TEXT,
    jsonl_path  TEXT NOT NULL,
    event_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
`

// openGlobalDB opens the global KB SQLite database directly and ensures
// the sessions table exists (idempotent migration). Spec: S-017 | Req: C-001, C-002a
// The caller is responsible for closing the returned *sql.DB.
func openGlobalDB() (*sql.DB, error) {
	path := filepath.Join(config.GlobalKBDir(), "kb.db")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating kb dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma: %w", err)
		}
	}
	// Ensure sessions table exists (idempotent). Spec: S-017 | Req: C-002a
	if _, err := db.Exec(sessionsSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sessions schema: %w", err)
	}
	// Ensure entries.session_id column exists (idempotent migration). Spec: S-017 | Req: C-002, C-002a
	var hasCol int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('entries') WHERE name='session_id'").Scan(&hasCol); err == nil && hasCol == 0 {
		db.Exec("ALTER TABLE entries ADD COLUMN session_id TEXT")
		db.Exec("CREATE INDEX IF NOT EXISTS idx_entries_session_id ON entries(session_id)")
	}
	return db, nil
}

// generateUUID generates a UUID v4 using /dev/urandom.
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

// Start creates a new session file with a start event and inserts into SQLite sessions table.
// Spec: S-017 | Req: B-001, C-001, C-004, I-005
func Start(sessionID, project, profileName string) error {
	// Ensure sessions dir exists. Spec: S-017 | Req: I-008
	if err := os.MkdirAll(sessionsDir(), 0755); err != nil {
		return fmt.Errorf("creating sessions dir: %w", err)
	}

	// B-001: MUST NOT run any orphan cleanup or PID checking. Spec: S-017 | Req: I-005

	if sessionID == "" {
		sessionID = generateUUID()
	}

	tools := detectTools()

	ev := SessionEvent{
		Type:      "start",
		Timestamp: time.Now().Format(time.RFC3339),
		SessionID: sessionID,
		Project:   project,
		Profile:   profileName,
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

	// INSERT into sessions table. Spec: S-017 | Req: B-001, C-001
	db, dbErr := openGlobalDB()
	if dbErr == nil {
		defer db.Close()
		_, sqlErr := db.Exec(`
			INSERT OR IGNORE INTO sessions (id, status, project, profile, started_at, jsonl_path, event_count)
			VALUES (?, 'active', ?, ?, ?, ?, 0)
		`, sessionID, project, profileName, ev.Timestamp, path)
		if sqlErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to insert session into db: %v\n", sqlErr)
		}
	}

	// Print UUID to stdout (for programmatic capture by hooks/tests).
	fmt.Println(sessionID)

	// Print session info to stderr.
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

// Append appends an event to an existing session file and updates event_count in SQLite.
// Spec: S-017 | Req: B-002, B-003, B-004, C-006, E-001, E-002, I-007, I-011
func Append(sessionID, eventType, content, tool, agentType string) error {
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "error: session_id is required")
		os.Exit(1)
	}

	// Apply truncation limits. Spec: S-017 | Req: C-006
	switch eventType {
	case "prompt":
		content = truncate(content, 300)
	case "tool":
		content = truncate(content, 200)
	case "agent":
		content = truncate(content, 300)
	}

	path := sessionPath(sessionID)

	// E-001: file doesn't exist -> no-op with warning
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
		// E-002: session already ended -> no-op with warning
		fmt.Fprintf(os.Stderr, "warning: session %s already ended, ignoring append\n", sessionID)
		return nil
	}
	if err != nil {
		return err
	}

	// Update event_count in SQLite (best-effort/advisory). Spec: S-017 | Req: B-002, B-003, B-004, I-011
	db, dbErr := openGlobalDB()
	if dbErr == nil {
		defer db.Close()
		if _, sqlErr := db.Exec(`UPDATE sessions SET event_count = event_count + 1 WHERE id = ?`, sessionID); sqlErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update event_count: %v\n", sqlErr)
		}
	}

	return nil
}

// End closes a session, optionally running a retro pass via claude -p.
// Spec: S-017 | Req: B-005, B-006, E-003, E-004, E-008
func End(sessionID string) error {
	path := sessionPath(sessionID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: session %s not found\n", sessionID)
		os.Exit(1)
	}

	// E-008: check concurrent end via SQLite status. Spec: S-017 | Req: E-008
	db, dbErr := openGlobalDB()
	if dbErr == nil {
		var status string
		err := db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sessionID).Scan(&status)
		if err == nil && status == "ended" {
			db.Close()
			fmt.Fprintf(os.Stderr, "warning: session %s already ended\n", sessionID)
			return nil
		}
		db.Close()
	}

	// Read events snapshot WITHOUT holding lock during LLM call. Spec: S-017 | Req: B-005
	events, err := readAllEvents(path)
	if err != nil {
		return fmt.Errorf("reading session events: %w", err)
	}

	// Compaction for retro only (read-time, file not rewritten). Spec: S-017 | Req: B-005
	retroEvents := events
	if len(events) > 1000 {
		retroEvents = make([]SessionEvent, 0, 1000)
		retroEvents = append(retroEvents, events[0])
		retroEvents = append(retroEvents, events[len(events)-999:]...)
	}

	// Determine whether to generate retro. Spec: S-017 | Req: B-005, B-006, E-003, I-009
	retroEnabled := os.Getenv("CVM_SESSION_RETRO_ENABLED") != "false"
	model := os.Getenv("CVM_SESSION_RETRO_MODEL")
	if model == "" {
		model = "haiku"
	}

	reason := "normal"

	// E-003: skip retro on short sessions (< 3 events)
	if retroEnabled && len(events) >= 3 {
		if retroErr := generateRetro(sessionID, retroEvents, model); retroErr != nil {
			// E-004: retro failure -> warn, continue with reason=error
			fmt.Fprintf(os.Stderr, "warning: retro pass failed: %v\n", retroErr)
			reason = "error"
		}
	}

	// Append end event under lock. Check for concurrent end (E-008).
	endEv := SessionEvent{
		Type:      "end",
		Timestamp: time.Now().Format(time.RFC3339),
		Reason:    reason,
	}
	err = appendEvent(path, endEv)
	if err == errAlreadyEnded {
		fmt.Fprintf(os.Stderr, "warning: session %s already ended\n", sessionID)
		return nil
	}
	if err != nil {
		return err
	}

	// Update SQLite: status='ended', ended_at=now. Spec: S-017 | Req: B-005
	db, dbErr = openGlobalDB()
	if dbErr == nil {
		defer db.Close()
		_, sqlErr := db.Exec(`UPDATE sessions SET status = 'ended', ended_at = ? WHERE id = ?`,
			endEv.Timestamp, sessionID)
		if sqlErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update session status: %v\n", sqlErr)
		}
	}

	// Cleanup learning-pulse. Spec: S-017 | Req: B-005
	_ = os.Remove(filepath.Join(config.CvmHome(), "learning-pulse"))

	// Automation integration. Spec: S-017 | Req: B-005
	projectPath := ""
	if len(events) > 0 && events[0].Type == "start" {
		projectPath = events[0].Project
	}
	if err := runEndAutomation(projectPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: automation integration failed: %v\n", err)
	}

	return nil
}

// generateRetro calls claude CLI with the retro prompt and persists results to KB with session_id.
// Spec: S-017 | Req: B-005, C-009, C-009a
func generateRetro(sessionID string, events []SessionEvent, model string) error {
	// Build events text for prompt. Spec: S-017 | Req: C-009a
	var sb strings.Builder
	for _, ev := range events {
		line, _ := json.Marshal(ev)
		sb.Write(line)
		sb.WriteByte('\n')
	}
	eventsText := sb.String()

	// Query existing KB entries for this session. Spec: S-017 | Req: B-005, C-009a
	existingEntries := ""
	db, dbErr := openGlobalDB()
	if dbErr == nil {
		rows, qErr := db.Query(`SELECT key, body, tags FROM entries WHERE session_id = ?`, sessionID)
		if qErr == nil {
			var entriesBuilder strings.Builder
			for rows.Next() {
				var key, body, tagsJSON string
				if rows.Scan(&key, &body, &tagsJSON) == nil {
					entriesBuilder.WriteString(fmt.Sprintf("key: %s\nbody: %s\ntags: %s\n\n", key, body, tagsJSON))
				}
			}
			rows.Close()
			existingEntries = entriesBuilder.String()
		}
		db.Close()
	}

	promptText := fmt.Sprintf(`Analyze this coding session's events and identify learnings, decisions, or gotchas
that were NOT already captured in the existing KB entries listed below.

Output ONLY a JSON array. Each element: {"key": "...", "body": "...", "tags": ["learning"|"decision"|"gotcha"]}
If nothing new to capture, output: []

<events>
%s
</events>

<already_captured>
%s
</already_captured>`, eventsText, existingEntries)

	cmd := exec.Command("claude", "-p", "--model", model)
	cmd.Stdin = strings.NewReader(promptText)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("claude invocation failed: %w", err)
	}

	outputStr := strings.TrimSpace(string(out))
	if outputStr == "" {
		return nil
	}

	// Parse JSON array output. Spec: S-017 | Req: B-005
	type retroEntry struct {
		Key  string   `json:"key"`
		Body string   `json:"body"`
		Tags []string `json:"tags"`
	}
	var entries []retroEntry
	if err := json.Unmarshal([]byte(outputStr), &entries); err != nil {
		return fmt.Errorf("parsing retro output: %w", err)
	}

	// Persist each entry with session_id. Spec: S-017 | Req: B-005, C-010
	for _, entry := range entries {
		if entry.Key == "" {
			continue
		}
		if putErr := kb.Put(config.ScopeGlobal, "", entry.Key, entry.Body, entry.Tags, sessionID); putErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to put retro entry %q: %v\n", entry.Key, putErr)
		}
	}

	return nil
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

// saveActiveProfiles saves the currently active profiles.
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

// queueAutomationRunner starts the automation runner in background.
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

// sessionRow holds data from the sessions SQLite table.
// Spec: S-017 | Req: C-001
type sessionRow struct {
	ID         string
	Status     string
	Project    string
	Profile    string
	StartedAt  string
	EndedAt    sql.NullString
	JSONLPath  string
	EventCount int
}

// queryActiveSessions returns all active sessions from SQLite.
// Spec: S-017 | Req: B-007
func queryActiveSessions() ([]sessionRow, error) {
	db, err := openGlobalDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, status, project, profile, started_at, ended_at, jsonl_path, event_count
		FROM sessions WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionRows(rows)
}

// queryAllSessions returns sessions ordered by started_at desc with optional limit.
// Spec: S-017 | Req: B-008
func queryAllSessions(limit int) ([]sessionRow, error) {
	db, err := openGlobalDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var (
		rows *sql.Rows
		qErr error
	)
	if limit > 0 {
		rows, qErr = db.Query(`SELECT id, status, project, profile, started_at, ended_at, jsonl_path, event_count
			FROM sessions ORDER BY started_at DESC LIMIT ?`, limit)
	} else {
		rows, qErr = db.Query(`SELECT id, status, project, profile, started_at, ended_at, jsonl_path, event_count
			FROM sessions ORDER BY started_at DESC`)
	}
	if qErr != nil {
		return nil, qErr
	}
	defer rows.Close()
	return scanSessionRows(rows)
}

// scanSessionRows scans sql.Rows into []sessionRow.
func scanSessionRows(rows *sql.Rows) ([]sessionRow, error) {
	var result []sessionRow
	for rows.Next() {
		var r sessionRow
		if err := rows.Scan(&r.ID, &r.Status, &r.Project, &r.Profile, &r.StartedAt, &r.EndedAt, &r.JSONLPath, &r.EventCount); err != nil {
			continue
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// Status lists active sessions from SQLite.
// Spec: S-017 | Req: B-007
func Status() error {
	rows, err := queryActiveSessions()
	if err != nil {
		return statusFallback()
	}

	if len(rows) == 0 {
		fmt.Println("No active sessions")
		return nil
	}

	for _, r := range rows {
		shortID := r.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		fmt.Printf("%s  project=%-40s  profile=%s  started=%s  events=%d\n",
			shortID, r.Project, r.Profile, r.StartedAt, r.EventCount)
	}
	return nil
}

// statusFallback scans JSONL files when SQLite is unavailable.
func statusFallback() error {
	infos, err := listSessionFiles(0)
	if err != nil {
		return err
	}
	activeCount := 0
	for _, info := range infos {
		if !hasEndEvent(info.path) {
			activeCount++
			shortUUID := info.uuid
			if len(shortUUID) > 8 {
				shortUUID = shortUUID[:8]
			}
			fmt.Printf("%s  project=%-40s  profile=%s  started=%s  events=%d\n",
				shortUUID, info.startEv.Project, info.startEv.Profile,
				info.startTime.Format("2006-01-02 15:04:05"), info.eventCount)
		}
	}
	if activeCount == 0 {
		fmt.Println("No active sessions")
	}
	return nil
}

// List lists all sessions from SQLite ordered by start time descending.
// Spec: S-017 | Req: B-008
func List(limit int) error {
	rows, err := queryAllSessions(limit)
	if err != nil {
		return listFallback(limit)
	}
	if len(rows) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	for _, r := range rows {
		shortID := r.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		fmt.Printf("%s  %-7s  %-40s  %s  events=%d\n",
			shortID, r.Status, r.Project, r.StartedAt, r.EventCount)
	}
	return nil
}

// listFallback scans JSONL files when SQLite is unavailable.
func listFallback(limit int) error {
	infos, err := listSessionFiles(limit)
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		fmt.Println("No sessions found")
		return nil
	}
	for _, info := range infos {
		status := "ended"
		if !hasEndEvent(info.path) {
			status = "active"
		}
		shortUUID := info.uuid
		if len(shortUUID) > 8 {
			shortUUID = shortUUID[:8]
		}
		fmt.Printf("%s  %-7s  %-40s  %s  events=%d\n",
			shortUUID, status, info.startEv.Project,
			info.startTime.Format("2006-01-02 15:04:05"), info.eventCount)
	}
	return nil
}

// sessionFileInfo holds summary info for listing (used in fallback).
type sessionFileInfo struct {
	uuid       string
	path       string
	startTime  time.Time
	startEv    *SessionEvent
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

		count := countEvents(path)

		infos = append(infos, sessionFileInfo{
			uuid:       uuid,
			path:       path,
			startTime:  ts,
			startEv:    ev,
			eventCount: count,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].startTime.After(infos[j].startTime)
	})

	if limit > 0 && len(infos) > limit {
		infos = infos[:limit]
	}
	return infos, nil
}

// Show prints all events for a session and lists linked KB entries.
// Spec: S-017 | Req: B-009, E-007, I-008
func Show(sessionID string) error {
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
			fmt.Printf("[%s] START  session=%s project=%s profile=%s\n",
				ts, ev.SessionID, ev.Project, ev.Profile)
		case "end":
			fmt.Printf("[%s] END    reason=%s\n", ts, ev.Reason)
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

	// List KB entries linked to this session. Spec: S-017 | Req: B-009
	db, dbErr := openGlobalDB()
	if dbErr == nil {
		defer db.Close()
		rows, qErr := db.Query(`SELECT key, tags FROM entries WHERE session_id = ?`, uuid)
		if qErr == nil {
			defer rows.Close()
			var linked []string
			for rows.Next() {
				var key, tagsJSON string
				if rows.Scan(&key, &tagsJSON) == nil {
					linked = append(linked, fmt.Sprintf("  %s  tags=%s", key, tagsJSON))
				}
			}
			if len(linked) > 0 {
				fmt.Printf("\n--- KB Entries for this session (%d) ---\n", len(linked))
				for _, l := range linked {
					fmt.Println(l)
				}
			}
		}
	}

	return nil
}

// GC deletes ended sessions with ended_at older than olderThan.
// Spec: S-017 | Req: B-010
func GC(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	db, dbErr := openGlobalDB()
	if dbErr != nil {
		return gcFallback(olderThan)
	}
	defer db.Close()

	// Query ended sessions with ended_at before cutoff. Spec: S-017 | Req: B-010
	rows, err := db.Query(`SELECT id, jsonl_path FROM sessions WHERE status = 'ended' AND ended_at < ?`,
		cutoff.Format(time.RFC3339))
	if err != nil {
		return err
	}
	defer rows.Close()

	type gcRow struct {
		id        string
		jsonlPath string
	}
	var toDelete []gcRow
	for rows.Next() {
		var r gcRow
		if rows.Scan(&r.id, &r.jsonlPath) == nil {
			toDelete = append(toDelete, r)
		}
	}
	rows.Close()

	deleted := 0
	for _, r := range toDelete {
		// Unlink KB entries from this session. Spec: S-017 | Req: B-010
		if _, err := db.Exec(`UPDATE entries SET session_id = NULL WHERE session_id = ?`, r.id); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to unlink entries for session %s: %v\n", r.id, err)
		}
		// Delete from sessions table. Spec: S-017 | Req: B-010
		if _, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, r.id); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete session %s from db: %v\n", r.id, err)
			continue
		}
		// Remove JSONL file.
		if r.jsonlPath != "" {
			_ = os.Remove(r.jsonlPath)
		}
		deleted++
	}

	fmt.Printf("deleted %d session(s)\n", deleted)
	return nil
}

// gcFallback uses JSONL file mtime when SQLite is unavailable.
func gcFallback(olderThan time.Duration) error {
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

		if !hasEndEvent(path) {
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
