// Spec: S-017 | Req: B-001..B-010, E-001..E-010, I-001..I-011
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// overrideSessionsDir temporarily overrides the sessions directory for tests.
// Tests call this to isolate from real ~/.cvm/sessions.
func overrideSessionsDirForTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Monkey-patch sessionsDir by using a test-specific env var approach:
	// we override at test level by writing directly to our temp dir.
	return dir
}

// writeSession writes a start event to a session file in the given dir.
func writeSession(t *testing.T, dir, uuid string, ev SessionEvent) string {
	t.Helper()
	path := filepath.Join(dir, uuid+".jsonl")
	line, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshaling start event: %v", err)
	}
	line = append(line, '\n')
	if err := os.WriteFile(path, line, 0644); err != nil {
		t.Fatalf("writing session file: %v", err)
	}
	return path
}

// appendLineToFile appends a raw JSON line to a session file.
func appendLineToFile(t *testing.T, path string, ev SessionEvent) {
	t.Helper()
	line, _ := json.Marshal(ev)
	line = append(line, '\n')
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("opening file for append: %v", err)
	}
	defer f.Close()
	f.Write(line)
}

// TestTruncate verifies content truncation by type. Spec: S-017 | Req: C-004 | Type: happy
func TestTruncate(t *testing.T) {
	// prompt: 300 runes
	longPrompt := strings.Repeat("あ", 310)
	result := truncate(longPrompt, 300)
	runes := []rune(result)
	if len(runes) != 301 { // 300 + "…"
		t.Errorf("expected 301 runes (300 + ellipsis), got %d", len(runes))
	}
	if !strings.HasSuffix(result, "…") {
		t.Error("expected ellipsis suffix")
	}

	// short content unchanged
	short := "hello"
	if truncate(short, 300) != short {
		t.Error("short content should not be truncated")
	}

	// tool: 200 runes
	longTool := strings.Repeat("x", 205)
	result = truncate(longTool, 200)
	runes = []rune(result)
	if len(runes) != 201 {
		t.Errorf("tool truncation: expected 201 runes, got %d", len(runes))
	}
}

// TestReadLastLine verifies reading last line from a file. Spec: S-017 | Req: I-011 | Type: happy
func TestReadLastLine(t *testing.T) {
	f, err := os.CreateTemp("", "lastline-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	content := `{"type":"start","ts":"2026-01-01T00:00:00Z"}` + "\n" +
		`{"type":"prompt","ts":"2026-01-01T00:01:00Z","content":"hello"}` + "\n"
	f.WriteString(content)

	info, _ := f.Stat()
	last, err := readLastLine(f, info.Size())
	if err != nil {
		t.Fatalf("readLastLine: %v", err)
	}
	if !strings.Contains(last, "prompt") {
		t.Errorf("expected last line to contain 'prompt', got: %q", last)
	}
}

// TestStartCreatesFile verifies Start creates the session file with correct start event.
// Spec: S-017 | Req: B-001 | Type: happy
func TestStartCreatesFile(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	uuid := "test-start-uuid-0001"
	err := Start(uuid, "/projects/myapp", "sdd-mem", 99999)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	path := filepath.Join(dir, uuid+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("session file not created: %v", err)
	}

	var ev SessionEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &ev); err != nil {
		t.Fatalf("parsing start event: %v", err)
	}
	if ev.Type != "start" {
		t.Errorf("expected type=start, got %q", ev.Type)
	}
	if ev.SessionID != uuid {
		t.Errorf("expected session_id=%q, got %q", uuid, ev.SessionID)
	}
	if ev.Project != "/projects/myapp" {
		t.Errorf("expected project=/projects/myapp, got %q", ev.Project)
	}
	if ev.PID != 99999 {
		t.Errorf("expected pid=99999, got %d", ev.PID)
	}
	if ev.Tools == nil {
		t.Error("expected tools to be set")
	}
}

// TestStartUsesGetppidWhenPIDZero verifies os.Getppid() is used when pid=0.
// Spec: S-017 | Req: B-001, C-002 | Type: happy
func TestStartUsesGetppidWhenPIDZero(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	uuid := "test-ppid-uuid-0001"
	err := Start(uuid, "/proj", "default", 0)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	ev, err := readStartEvent(filepath.Join(dir, uuid+".jsonl"))
	if err != nil {
		t.Fatalf("readStartEvent: %v", err)
	}
	if ev.PID != os.Getppid() {
		t.Errorf("expected pid=%d (getppid), got %d", os.Getppid(), ev.PID)
	}
}

// TestStartGeneratesUUIDWhenEmpty verifies UUID is generated when sessionID is empty.
// Spec: S-017 | Req: C-006 | Type: happy
func TestStartGeneratesUUIDWhenEmpty(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	err := Start("", "/proj", "default", 12345)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 session file, got %d", len(entries))
	}
	name := entries[0].Name()
	if !strings.HasSuffix(name, ".jsonl") {
		t.Errorf("expected .jsonl file, got %q", name)
	}
}

// TestAppendAddsEvents verifies events are appended with correct truncation.
// Spec: S-017 | Req: B-002, B-003, B-004, C-004 | Type: happy
func TestAppendAddsEvents(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	uuid := "test-append-uuid-0001"
	path := writeSession(t, dir, uuid, SessionEvent{
		Type: "start", Timestamp: time.Now().Format(time.RFC3339),
		SessionID: uuid, Project: "/proj", PID: 12345,
	})

	// Prompt event
	err := Append(uuid, "prompt", "hello world", "", "")
	if err != nil {
		t.Fatalf("Append prompt: %v", err)
	}

	// Tool event
	err = Append(uuid, "tool", "ls -la", "Bash", "")
	if err != nil {
		t.Fatalf("Append tool: %v", err)
	}

	// Agent event
	err = Append(uuid, "agent", "Research done", "", "haiku")
	if err != nil {
		t.Fatalf("Append agent: %v", err)
	}

	events, err := readAllEvents(path)
	if err != nil {
		t.Fatalf("readAllEvents: %v", err)
	}
	if len(events) != 4 { // start + 3 appends
		t.Errorf("expected 4 events, got %d", len(events))
	}

	if events[1].Type != "prompt" || events[1].Content != "hello world" {
		t.Errorf("unexpected prompt event: %+v", events[1])
	}
	if events[2].Type != "tool" || events[2].Tool != "Bash" {
		t.Errorf("unexpected tool event: %+v", events[2])
	}
	if events[3].Type != "agent" || events[3].AgentType != "haiku" {
		t.Errorf("unexpected agent event: %+v", events[3])
	}
}

// TestAppendTruncation verifies truncation limits are enforced.
// Spec: S-017 | Req: C-004 | Type: edge
func TestAppendTruncation(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	uuid := "test-trunc-uuid-0001"
	path := writeSession(t, dir, uuid, SessionEvent{
		Type: "start", Timestamp: time.Now().Format(time.RFC3339), SessionID: uuid, PID: 12345,
	})

	longContent := strings.Repeat("a", 400)
	if err := Append(uuid, "prompt", longContent, "", ""); err != nil {
		t.Fatalf("Append: %v", err)
	}

	events, _ := readAllEvents(path)
	if len(events) < 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	runes := []rune(events[1].Content)
	if len(runes) > 301 { // 300 + ellipsis
		t.Errorf("prompt content not truncated: %d runes", len(runes))
	}
}

// TestAppendNoopOnMissingFile verifies E-001: no-op with exit 0 when file doesn't exist.
// Spec: S-017 | Req: E-001 | Type: error
func TestAppendNoopOnMissingFile(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := Append("nonexistent-uuid", "prompt", "hello", "", "")

	w.Close()
	os.Stderr = old
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderrOutput := string(buf[:n])

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !strings.Contains(stderrOutput, "not found") {
		t.Errorf("expected 'not found' warning, got: %q", stderrOutput)
	}
}

// TestAppendNoopOnEndedSession verifies E-002: no-op when session already ended.
// Spec: S-017 | Req: E-002 | Type: error
func TestAppendNoopOnEndedSession(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	uuid := "test-ended-uuid-0001"
	path := writeSession(t, dir, uuid, SessionEvent{
		Type: "start", Timestamp: time.Now().Format(time.RFC3339), SessionID: uuid, PID: 12345,
	})
	appendLineToFile(t, path, SessionEvent{
		Type: "end", Timestamp: time.Now().Format(time.RFC3339), Reason: "normal",
	})

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := Append(uuid, "prompt", "hello", "", "")

	w.Close()
	os.Stderr = old
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderrOutput := string(buf[:n])

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !strings.Contains(stderrOutput, "already ended") {
		t.Errorf("expected 'already ended' warning, got: %q", stderrOutput)
	}

	// File should not have grown.
	events, _ := readAllEvents(path)
	if len(events) != 2 { // start + end
		t.Errorf("expected 2 events (no new append), got %d", len(events))
	}
}

// TestEndAppendsEndEvent verifies End appends an end event.
// Spec: S-017 | Req: B-005 | Type: happy
func TestEndAppendsEndEvent(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	uuid := "test-end-uuid-0001"
	path := writeSession(t, dir, uuid, SessionEvent{
		Type: "start", Timestamp: time.Now().Format(time.RFC3339), SessionID: uuid, Project: "/proj", PID: 12345,
	})
	// Add some events to meet the threshold (we'll skip actual LLM by disabling autosummary).
	_ = path

	// Disable autosummary for this test.
	t.Setenv("CVM_AUTOSUMMARY_ENABLED", "false")

	err := End(uuid)
	if err != nil {
		t.Fatalf("End: %v", err)
	}

	events, _ := readAllEvents(path)
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	last := events[len(events)-1]
	if last.Type != "end" {
		t.Errorf("expected last event type=end, got %q", last.Type)
	}
	if last.Reason != "normal" {
		t.Errorf("expected reason=normal, got %q", last.Reason)
	}
}

// TestEndSkipsSummaryOnShortSession verifies E-003: < 3 events → no summary.
// Spec: S-017 | Req: E-003 | Type: edge
func TestEndSkipsSummaryOnShortSession(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	uuid := "test-short-uuid-0001"
	path := writeSession(t, dir, uuid, SessionEvent{
		Type: "start", Timestamp: time.Now().Format(time.RFC3339), SessionID: uuid, PID: 12345,
	})
	// Only 2 events: start + 1 append = 2 events total (< 3, no summary)
	appendLineToFile(t, path, SessionEvent{
		Type: "prompt", Timestamp: time.Now().Format(time.RFC3339), Content: "hi",
	})

	t.Setenv("CVM_AUTOSUMMARY_ENABLED", "true")

	err := End(uuid)
	if err != nil {
		t.Fatalf("End: %v", err)
	}

	events, _ := readAllEvents(path)
	last := events[len(events)-1]
	if last.Type != "end" {
		t.Errorf("expected end event, got %q", last.Type)
	}
	if last.SummaryKey != "" {
		t.Errorf("expected empty summary_key (short session), got %q", last.SummaryKey)
	}
}

// TestGCDeletesOldClosedFiles verifies GC removes old closed sessions, keeps active ones.
// Spec: S-017 | Req: B-010 | Type: happy
func TestGCDeletesOldClosedFiles(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	// Create 3 old closed sessions.
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	for i := 0; i < 3; i++ {
		uuid := fmt.Sprintf("old-closed-%04d", i)
		path := filepath.Join(dir, uuid+".jsonl")
		ev := SessionEvent{Type: "start", Timestamp: oldTime.Format(time.RFC3339), SessionID: uuid, PID: 1}
		endEv := SessionEvent{Type: "end", Timestamp: oldTime.Format(time.RFC3339), Reason: "normal"}
		line1, _ := json.Marshal(ev)
		line2, _ := json.Marshal(endEv)
		os.WriteFile(path, append(append(line1, '\n'), append(line2, '\n')...), 0644)
		// Set mtime to 31 days ago.
		os.Chtimes(path, oldTime, oldTime)
	}

	// Create 1 recent closed session (should not be deleted).
	recentUUID := "recent-closed-0001"
	recentPath := filepath.Join(dir, recentUUID+".jsonl")
	ev := SessionEvent{Type: "start", Timestamp: time.Now().Format(time.RFC3339), SessionID: recentUUID, PID: 1}
	endEv := SessionEvent{Type: "end", Timestamp: time.Now().Format(time.RFC3339), Reason: "normal"}
	line1, _ := json.Marshal(ev)
	line2, _ := json.Marshal(endEv)
	os.WriteFile(recentPath, append(append(line1, '\n'), append(line2, '\n')...), 0644)

	err := GC(30 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}

	// 3 old should be deleted.
	for i := 0; i < 3; i++ {
		uuid := fmt.Sprintf("old-closed-%04d", i)
		path := filepath.Join(dir, uuid+".jsonl")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected old session %s to be deleted", uuid)
		}
	}
	// Recent should remain.
	if _, err := os.Stat(recentPath); err != nil {
		t.Errorf("expected recent session to remain: %v", err)
	}
}

// TestResolvePrefix verifies UUID prefix matching. Spec: S-017 | Req: E-009 | Type: happy
func TestResolvePrefix(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	uuid := "778a7b24-509f-4f79-a99e-cd01e631ef82"
	path := filepath.Join(dir, uuid+".jsonl")
	os.WriteFile(path, []byte(`{"type":"start","ts":"2026-01-01T00:00:00Z"}`+"\n"), 0644)

	resolved, err := resolvePrefix("778a7b24")
	if err != nil {
		t.Fatalf("resolvePrefix: %v", err)
	}
	if resolved != uuid {
		t.Errorf("expected %q, got %q", uuid, resolved)
	}
}

// TestResolvePrefixAmbiguous verifies ambiguous prefix returns error.
// Spec: S-017 | Req: E-009 | Type: error
func TestResolvePrefixAmbiguous(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	for i := 0; i < 2; i++ {
		uuid := fmt.Sprintf("778a7b24-%04d", i)
		path := filepath.Join(dir, uuid+".jsonl")
		os.WriteFile(path, []byte(`{"type":"start","ts":"2026-01-01T00:00:00Z"}`+"\n"), 0644)
	}

	_, err := resolvePrefix("778a7b24")
	if err == nil {
		t.Error("expected error for ambiguous prefix")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
}

// TestConcurrentAppends verifies file locking prevents corruption under concurrent writes.
// Spec: S-017 | Req: E-008, I-007 | Type: edge
func TestConcurrentAppends(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	uuid := "test-concurrent-uuid-0001"
	path := writeSession(t, dir, uuid, SessionEvent{
		Type: "start", Timestamp: time.Now().Format(time.RFC3339), SessionID: uuid, PID: 12345,
	})

	const goroutines = 20
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			Append(uuid, "prompt", fmt.Sprintf("message %d", i), "", "")
		}(i)
	}
	wg.Wait()

	events, err := readAllEvents(path)
	if err != nil {
		t.Fatalf("readAllEvents: %v", err)
	}
	// All goroutines should have written: start + 20 prompts = 21 events
	if len(events) != goroutines+1 {
		t.Errorf("expected %d events, got %d (possible corruption)", goroutines+1, len(events))
	}
	// All events must be valid JSON (already validated by readAllEvents).
	for i, ev := range events {
		if ev.Type == "" {
			t.Errorf("event %d has empty type (possible corruption)", i)
		}
	}
}

// TestOrphanDetection verifies that sessions with dead PIDs are detected as orphans.
// Spec: S-017 | Req: E-007 | Type: edge
func TestOrphanDetection(t *testing.T) {
	dir := overrideSessionsDirForTest(t)
	origDir := sessionsDir
	sessionsDir = func() string { return dir }
	defer func() { sessionsDir = origDir }()

	// Create a session with a dead PID (PID 1 won't be named "claude" in tests).
	uuid := "test-orphan-uuid-0001"
	path := writeSession(t, dir, uuid, SessionEvent{
		Type: "start", Timestamp: time.Now().Format(time.RFC3339), SessionID: uuid, PID: 99999999,
	})

	cleanOrphans()

	// The orphan session should now have an end event.
	if !hasEndEvent(path) {
		t.Error("expected orphan session to have an end event after cleanOrphans")
	}

	events, _ := readAllEvents(path)
	last := events[len(events)-1]
	if last.Type != "end" {
		t.Errorf("expected end event, got %q", last.Type)
	}
	if last.Reason != "orphan" {
		t.Errorf("expected reason=orphan, got %q", last.Reason)
	}
}

// TestDetectTools verifies detectTools returns a non-nil map with known tools.
// Spec: S-017 | Req: C-002 | Type: happy
func TestDetectTools(t *testing.T) {
	tools := detectTools()
	if tools == nil {
		t.Fatal("expected non-nil tools map")
	}
	expected := []string{"claude", "codex", "gemini", "gh", "docker", "node", "npm", "go"}
	for _, tool := range expected {
		if _, ok := tools[tool]; !ok {
			t.Errorf("expected key %q in tools map", tool)
		}
	}
}

// TestReadAllEventsSkipsInvalidLines verifies I-008: invalid JSON lines are skipped.
// Spec: S-017 | Req: I-008, E-011 | Type: edge
func TestReadAllEventsSkipsInvalidLines(t *testing.T) {
	f, err := os.CreateTemp("", "session-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	content := `{"type":"start","ts":"2026-01-01T00:00:00Z"}` + "\n" +
		`invalid json here` + "\n" +
		`{"type":"prompt","ts":"2026-01-01T00:01:00Z","content":"hi"}` + "\n"
	f.WriteString(content)

	events, err := readAllEvents(f.Name())
	if err != nil {
		t.Fatalf("readAllEvents: %v", err)
	}
	// Should have 2 valid events (start + prompt), skipping invalid line.
	if len(events) != 2 {
		t.Errorf("expected 2 valid events, got %d", len(events))
	}
}
