// Spec: S-016
// API handlers for the CVM dashboard HTTP endpoints.
package dashboard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/kb"
	"github.com/chichex/cvm/internal/session"
)

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

// jsonOK writes a JSON success response.
func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// parseScope reads and validates the "scope" query param.
// Spec: S-016 | Req: I-002b, E-005
func parseScope(r *http.Request, def string) (string, error) {
	s := r.URL.Query().Get("scope")
	if s == "" {
		s = def
	}
	switch s {
	case "global", "local", "both":
		return s, nil
	default:
		return "", nil
	}
}

// parseLimit reads and validates the "limit" query param.
// Spec: S-016 | Req: I-002c, E-004
func parseLimit(r *http.Request, def, min, max int) (int, error) {
	s := r.URL.Query().Get("limit")
	if s == "" {
		return def, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < min || v > max {
		return 0, nil
	}
	return v, nil
}

// --- Timeline ---

type timelineEntryJSON struct {
	Key          string   `json:"key"`
	Tags         []string `json:"tags"`
	Scope        string   `json:"scope"`
	UpdatedAt    string   `json:"updated_at"`
	FirstLine    string   `json:"first_line"`
	TokenEstimate int     `json:"token_estimate"`
}

type timelineDayJSON struct {
	Date    string              `json:"date"`
	Entries []timelineEntryJSON `json:"entries"`
}

type timelineResponse struct {
	Days  []timelineDayJSON `json:"days"`
	Total int               `json:"total"`
}

// handleTimeline serves GET /api/timeline
// Spec: S-016 | Req: I-002a, I-002b, I-002c, I-002d, B-004, B-005, B-013
func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse scope
	scopeStr := r.URL.Query().Get("scope")
	if scopeStr == "" {
		scopeStr = "both"
	}
	if scopeStr != "global" && scopeStr != "local" && scopeStr != "both" {
		jsonError(w, "scope must be global, local, or both", http.StatusBadRequest)
		return
	}

	// Parse limit
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 1 || v > 500 {
			jsonError(w, "limit must be between 1 and 500", http.StatusBadRequest)
			return
		}
		limit = v
	}

	// Parse days
	daysStr := r.URL.Query().Get("days")
	days := 7
	if daysStr != "" {
		v, err := strconv.Atoi(daysStr)
		if err != nil || v < 1 {
			jsonError(w, "days must be a positive integer", http.StatusBadRequest)
			return
		}
		days = v
	}

	// Collect compact entries from selected scopes
	type scopedEntry struct {
		kb.CompactEntry
		scope string
	}

	var all []scopedEntry

	collectCompact := func(b kb.Backend, scopeName string) {
		if b == nil {
			return
		}
		entries, err := b.Compact()
		if err != nil {
			return
		}
		cutoff := time.Now().AddDate(0, 0, -days)
		for _, e := range entries {
			if e.UpdatedAt.After(cutoff) {
				all = append(all, scopedEntry{e, scopeName})
			}
		}
	}

	if scopeStr == "global" || scopeStr == "both" {
		collectCompact(s.globalBack, "global")
	}
	if scopeStr == "local" || scopeStr == "both" {
		collectCompact(s.localBack, "local")
	}

	// Sort all by UpdatedAt desc
	sort.Slice(all, func(i, j int) bool {
		return all[i].UpdatedAt.After(all[j].UpdatedAt)
	})

	// Apply limit
	if len(all) > limit {
		all = all[:limit]
	}

	// Group by day
	dayMap := make(map[string]*timelineDayJSON)
	var dayOrder []string

	for _, e := range all {
		dayKey := e.UpdatedAt.UTC().Format("2006-01-02")
		if _, ok := dayMap[dayKey]; !ok {
			dayMap[dayKey] = &timelineDayJSON{Date: dayKey}
			dayOrder = append(dayOrder, dayKey)
		}
		tags := e.Tags
		if tags == nil {
			tags = []string{}
		}
		dayMap[dayKey].Entries = append(dayMap[dayKey].Entries, timelineEntryJSON{
			Key:           e.Key,
			Tags:          tags,
			Scope:         e.scope,
			UpdatedAt:     e.UpdatedAt.UTC().Format(time.RFC3339),
			FirstLine:     e.FirstLine,
			TokenEstimate: len(e.FirstLine) / 4,
		})
	}

	var resultDays []timelineDayJSON
	for _, d := range dayOrder {
		resultDays = append(resultDays, *dayMap[d])
	}
	if resultDays == nil {
		resultDays = []timelineDayJSON{}
	}

	jsonOK(w, timelineResponse{Days: resultDays, Total: len(all)})
}

// --- Session ---


// sessionsDir returns the path to ~/.cvm/sessions/.
// Spec: S-017 | Req: I-002, I-004
func sessionsDir() string {
	return filepath.Join(config.CvmHome(), "sessions")
}

// dashboardSessionEvent mirrors session.SessionEvent for JSON parsing within the dashboard.
// We duplicate the struct to avoid circular import from session → dashboard.
// Spec: S-017 | Req: C-001, C-002, C-003
type dashboardSessionEvent = session.SessionEvent

// readSessionStartEvent reads the first JSONL line of a session file and parses it.
// Spec: S-017 | Req: C-002
func readSessionStartEvent(path string) (*dashboardSessionEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	if !scanner.Scan() {
		return nil, os.ErrInvalid
	}
	var ev dashboardSessionEvent
	if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
		return nil, err
	}
	return &ev, nil
}

// readAllSessionEvents reads all JSONL lines from a session file, skipping invalid lines.
// Spec: S-017 | Req: I-008, E-011
func readAllSessionEvents(path string) []dashboardSessionEvent {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var events []dashboardSessionEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev dashboardSessionEvent
		if json.Unmarshal([]byte(line), &ev) == nil {
			events = append(events, ev)
		}
	}
	return events
}

// sessionHasEndEvent checks if the last valid JSON line of a session file is an end event.
func sessionHasEndEvent(events []dashboardSessionEvent) bool {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == "end" {
			return true
		}
	}
	return false
}

// sessionIsPIDAlive checks if a PID is alive and the process name contains "claude".
// Spec: S-017 | Req: I-010
func sessionIsPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Send signal 0 to check process existence (Unix only)
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	// Check process name contains "claude". Spec: S-017 | Req: I-010
	psOut, psErr := execPsComm(pid)
	if psErr != nil {
		// Fallback: Linux /proc/<pid>/comm
		out, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			return false
		}
		return strings.Contains(strings.ToLower(strings.TrimSpace(string(out))), "claude")
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(psOut)), "claude")
}

// execPsComm runs `ps -p <pid> -o comm=` and returns stdout.
func execPsComm(pid int) (string, error) {
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=")
	out, err := cmd.Output()
	return string(out), err
}

type sessionDetailResponse struct {
	SessionID  string                   `json:"session_id"`
	Key        string                   `json:"key"`
	Events     []dashboardSessionEvent  `json:"events"`
	EventCount int                      `json:"event_count"`
	Found      bool                     `json:"found"`
	StartedAt  string                   `json:"started_at,omitempty"`
	ProjectDir string                   `json:"project_dir,omitempty"`
}

// handleSession serves GET /api/session
// Reads from ~/.cvm/sessions/ JSONL files instead of local KB.
// Spec: S-017 | Req: B-011, B-012, C-007
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	dir := sessionsDir()

	if id != "" {
		// Specific session file — try exact match first, then prefix
		path := filepath.Join(dir, id+".jsonl")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Try prefix resolution
			entries, rdErr := os.ReadDir(dir)
			if rdErr != nil {
				jsonOK(w, sessionDetailResponse{SessionID: id, Found: false, Events: []dashboardSessionEvent{}})
				return
			}
			var matches []string
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
					name := strings.TrimSuffix(e.Name(), ".jsonl")
					if strings.HasPrefix(name, id) {
						matches = append(matches, name)
					}
				}
			}
			if len(matches) == 1 {
				path = filepath.Join(dir, matches[0]+".jsonl")
				id = matches[0]
			} else {
				jsonOK(w, sessionDetailResponse{SessionID: id, Found: false, Events: []dashboardSessionEvent{}})
				return
			}
		}
		events := readAllSessionEvents(path)
		startedAt := ""
		projectDir := ""
		if len(events) > 0 && events[0].Type == "start" {
			startedAt = events[0].Timestamp
			projectDir = events[0].Project
		}
		if events == nil {
			events = []dashboardSessionEvent{}
		}
		jsonOK(w, sessionDetailResponse{
			SessionID:  id,
			Key:        id + ".jsonl",
			Events:     events,
			EventCount: len(events),
			Found:      true,
			StartedAt:  startedAt,
			ProjectDir: projectDir,
		})
		return
	}

	// List all session files
	entries, err := os.ReadDir(dir)
	if err != nil {
		jsonOK(w, map[string]interface{}{"sessions": []interface{}{}, "project_dir": s.cfg.ProjectPath})
		return
	}
	type sessionSummary struct {
		SessionID  string `json:"session_id"`
		Key        string `json:"key"`
		EventCount int    `json:"event_count"`
		StartedAt  string `json:"started_at"`
		ProjectDir string `json:"project_dir"`
		Active     bool   `json:"active"`
	}
	var result []sessionSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		uuid := strings.TrimSuffix(e.Name(), ".jsonl")
		path := filepath.Join(dir, e.Name())
		events := readAllSessionEvents(path)
		startedAt := ""
		projectDir := ""
		if len(events) > 0 && events[0].Type == "start" {
			startedAt = events[0].Timestamp
			projectDir = events[0].Project
		}
		active := len(events) > 0 && !sessionHasEndEvent(events)
		if active && len(events) > 0 {
			active = sessionIsPIDAlive(events[0].PID)
		}
		result = append(result, sessionSummary{
			SessionID:  uuid,
			Key:        e.Name(),
			EventCount: len(events),
			StartedAt:  startedAt,
			ProjectDir: projectDir,
			Active:     active,
		})
	}
	if result == nil {
		result = []sessionSummary{}
	}
	jsonOK(w, map[string]interface{}{"sessions": result, "project_dir": s.cfg.ProjectPath})
}

// countLines counts non-empty lines in a string.
func countLines(s string) int {
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// --- Entries (KB Browser) ---

type entryJSON struct {
	Key           string   `json:"key"`
	Tags          []string `json:"tags"`
	Scope         string   `json:"scope"`
	Enabled       bool     `json:"enabled"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	Body          string   `json:"body"`
	TokenEstimate int      `json:"token_estimate"`
}

type entriesResponse struct {
	Entries []entryJSON `json:"entries"`
	Total   int         `json:"total"`
	Offset  int         `json:"offset"`
	Limit   int         `json:"limit"`
}

// handleEntries serves GET /api/entries
// Spec: S-016 | Req: I-002h, I-002i, I-002j, I-002k, B-008
func (s *Server) handleEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse scope
	scopeStr := r.URL.Query().Get("scope")
	if scopeStr == "" {
		scopeStr = "both"
	}
	if scopeStr != "global" && scopeStr != "local" && scopeStr != "both" {
		jsonError(w, "scope must be global, local, or both", http.StatusBadRequest)
		return
	}

	// Parse limit
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 1 || v > 500 {
			jsonError(w, "limit must be between 1 and 500", http.StatusBadRequest)
			return
		}
		limit = v
	}

	// Parse offset
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		v, err := strconv.Atoi(offsetStr)
		if err != nil || v < 0 {
			jsonError(w, "offset must be a non-negative integer", http.StatusBadRequest)
			return
		}
		offset = v
	}

	query := r.URL.Query().Get("q")
	tag := r.URL.Query().Get("tag")

	type scopedDoc struct {
		doc   kb.Document
		scope string
	}

	var allDocs []scopedDoc

	fetchDocs := func(b kb.Backend, scopeName string) {
		if b == nil {
			return
		}
		if query != "" {
			// Use FTS search
			opts := kb.SearchOptions{}
			if tag != "" {
				opts.Tag = tag
			}
			results, err := b.Search(query, opts)
			if err != nil {
				return
			}
			for _, res := range results {
				// Fetch full body
				doc, err := b.Get(res.Entry.Key)
				if err != nil {
					doc = kb.Document{Entry: res.Entry, Body: res.Snippet}
				}
				allDocs = append(allDocs, scopedDoc{doc, scopeName})
			}
		} else {
			// Use list
			entries, err := b.List(tag)
			if err != nil {
				return
			}
			for _, e := range entries {
				doc, err := b.Get(e.Key)
				if err != nil {
					doc = kb.Document{Entry: e, Body: ""}
				}
				allDocs = append(allDocs, scopedDoc{doc, scopeName})
			}
		}
	}

	if scopeStr == "global" || scopeStr == "both" {
		fetchDocs(s.globalBack, "global")
	}
	if scopeStr == "local" || scopeStr == "both" {
		fetchDocs(s.localBack, "local")
	}

	// Sort by UpdatedAt desc
	sort.Slice(allDocs, func(i, j int) bool {
		return allDocs[i].doc.Entry.UpdatedAt.After(allDocs[j].doc.Entry.UpdatedAt)
	})

	total := len(allDocs)

	// Apply offset + limit (post-fetch for flat backend compat)
	// Spec: S-016 | Req: I-002k
	if offset >= len(allDocs) {
		allDocs = nil
	} else {
		allDocs = allDocs[offset:]
	}
	if len(allDocs) > limit {
		allDocs = allDocs[:limit]
	}

	result := make([]entryJSON, 0, len(allDocs))
	for _, sd := range allDocs {
		e := sd.doc.Entry
		tags := e.Tags
		if tags == nil {
			tags = []string{}
		}
		result = append(result, entryJSON{
			Key:           e.Key,
			Tags:          tags,
			Scope:         sd.scope,
			Enabled:       e.Enabled,
			CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:     e.UpdatedAt.UTC().Format(time.RFC3339),
			Body:          sd.doc.Body,
			TokenEstimate: len(sd.doc.Body) / 4,
		})
	}

	jsonOK(w, entriesResponse{
		Entries: result,
		Total:   total,
		Offset:  offset,
		Limit:   limit,
	})
}

// --- Sessions (combined active buffers + completed summaries) ---

type knowledgeEntryJSON struct {
	Key           string   `json:"key"`
	Tags          []string `json:"tags"`
	Scope         string   `json:"scope"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	Body          string   `json:"body"`
	TokenEstimate int      `json:"token_estimate"`
}

type sessionMetaJSON struct {
	ProjectDir string `json:"project_dir,omitempty"`
	EventCount string `json:"event_count,omitempty"`
	EstTokens  string `json:"est_tokens,omitempty"`
	TimeRange  string `json:"time_range,omitempty"`
}

type sessionCardJSON struct {
	// Common fields
	ID         string               `json:"id"`
	Key        string               `json:"key"`
	Status     string               `json:"status"`    // "active", "stale", or "summarized"
	Scope      string               `json:"scope"`     // "local" (active) or "global" (summarized)
	CreatedAt  string               `json:"created_at"`
	UpdatedAt  string               `json:"updated_at"`
	ProjectDir string               `json:"project_dir,omitempty"`
	Meta       *sessionMetaJSON     `json:"meta,omitempty"`
	Knowledge  []knowledgeEntryJSON `json:"knowledge"`

	// Active-only fields
	LineCount int `json:"line_count,omitempty"`

	// Summarized-only fields
	SummaryBody string `json:"summary_body,omitempty"`
}

type sessionsResponse struct {
	Sessions   []sessionCardJSON `json:"sessions"`
	ProjectDir string            `json:"project_dir"`
}

// handleSessions serves GET /api/sessions
// Returns active sessions from ~/.cvm/sessions/*.jsonl + completed summaries from global KB.
// Spec: S-017 | Req: B-012, C-007, I-004, I-005
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cards []sessionCardJSON

	// --- Active sessions from ~/.cvm/sessions/*.jsonl ---
	// Spec: S-017 | Req: B-012, C-007, I-004
	dir := sessionsDir()
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			uuid := strings.TrimSuffix(e.Name(), ".jsonl")
			path := filepath.Join(dir, e.Name())

			events := readAllSessionEvents(path)
			if len(events) == 0 || events[0].Type != "start" {
				continue
			}
			startEv := events[0]

			// Only include sessions without an end event and with a live PID
			if sessionHasEndEvent(events) {
				continue
			}
			if !sessionIsPIDAlive(startEv.PID) {
				continue
			}

			startedAt := startEv.Timestamp
			updatedAt := startedAt
			// Use last event timestamp as "updated_at"
			if len(events) > 0 {
				updatedAt = events[len(events)-1].Timestamp
			}

			finfo, ferr := e.Info()
			if ferr == nil {
				updatedAt = finfo.ModTime().UTC().Format(time.RFC3339)
			}

			card := sessionCardJSON{
				ID:         uuid,
				Key:        e.Name(),
				Status:     "active",
				Scope:      "local",
				CreatedAt:  startedAt,
				UpdatedAt:  updatedAt,
				ProjectDir: startEv.Project,
				LineCount:  len(events),
				Knowledge:  []knowledgeEntryJSON{},
			}
			cards = append(cards, card)
		}
	}

	// --- Completed summaries from global KB ---
	// Recognize both "session-summary-*" (new, S-017) and "session-*" (legacy) key patterns.
	// Spec: S-017 | Req: I-005
	if s.globalBack != nil {
		entries, err := s.globalBack.List("")
		if err == nil {
			for _, e := range entries {
				// Recognize new "session-summary-*" keys and legacy "session-*" keys.
				// Exclude "session-buffer-*" which was the old active buffer pattern.
				isNewSummary := strings.HasPrefix(e.Key, "session-summary-")
				isLegacySummary := strings.HasPrefix(e.Key, "session-") &&
					!strings.HasPrefix(e.Key, "session-summary-") &&
					!strings.HasPrefix(e.Key, "session-buffer-")
				if !isNewSummary && !isLegacySummary {
					continue
				}
				// Must have "summary" in tags to be a completed session
				hasSummaryTag := false
				for _, t := range e.Tags {
					if t == "summary" {
						hasSummaryTag = true
						break
					}
				}
				if !hasSummaryTag {
					continue
				}
				doc, err := s.globalBack.Get(e.Key)
				summaryBody := ""
				if err == nil {
					summaryBody = doc.Body
				}
				tags := e.Tags
				if tags == nil {
					tags = []string{}
				}
				// Parse [meta] line if present
				var meta *sessionMetaJSON
				displayBody := summaryBody
				if strings.HasPrefix(summaryBody, "[meta]") {
					parts := strings.SplitN(summaryBody, "\n", 2)
					metaLine := parts[0]
					if len(parts) > 1 {
						displayBody = parts[1]
					}
					meta = parseMetaLine(metaLine)
				}

				card := sessionCardJSON{
					ID:          e.Key,
					Key:         e.Key,
					Status:      "summarized",
					Scope:       "global",
					CreatedAt:   e.CreatedAt.UTC().Format(time.RFC3339),
					UpdatedAt:   e.UpdatedAt.UTC().Format(time.RFC3339),
					ProjectDir:  metaProjectDir(meta),
					Meta:        meta,
					SummaryBody: displayBody,
					Knowledge:   []knowledgeEntryJSON{},
				}
				cards = append(cards, card)
			}
		}
	}

	// --- Correlate knowledge entries to sessions ---
	// Collect all non-session knowledge entries from both scopes
	type scopedKB struct {
		entry knowledgeEntryJSON
		ts    time.Time
	}
	var allKB []scopedKB

	collectKB := func(b kb.Backend, scopeName string) {
		if b == nil {
			return
		}
		entries, err := b.List("")
		if err != nil {
			return
		}
		for _, e := range entries {
			// Skip session keys
			if strings.HasPrefix(e.Key, "session-") {
				continue
			}
			doc, err := b.Get(e.Key)
			body := ""
			if err == nil {
				body = doc.Body
			}
			tags := e.Tags
			if tags == nil {
				tags = []string{}
			}
			allKB = append(allKB, scopedKB{
				entry: knowledgeEntryJSON{
					Key:           e.Key,
					Tags:          tags,
					Scope:         scopeName,
					CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339),
					UpdatedAt:     e.UpdatedAt.UTC().Format(time.RFC3339),
					Body:          body,
					TokenEstimate: len(body) / 4,
				},
				ts: e.CreatedAt,
			})
		}
	}

	collectKB(s.globalBack, "global")
	collectKB(s.localBack, "local")

	// For each session card, find knowledge entries whose CreatedAt falls within the session timeframe
	for i, card := range cards {
		start, errStart := time.Parse(time.RFC3339, card.CreatedAt)
		end, errEnd := time.Parse(time.RFC3339, card.UpdatedAt)
		if errStart != nil || errEnd != nil {
			continue
		}
		// Add a small buffer after the session ended (5 min) to capture entries written just after
		end = end.Add(5 * time.Minute)

		for _, kb := range allKB {
			if !kb.ts.Before(start) && !kb.ts.After(end) {
				cards[i].Knowledge = append(cards[i].Knowledge, kb.entry)
			}
		}
	}

	// Sort: active first, stale second, summarized last, then by UpdatedAt desc
	statusOrder := map[string]int{"active": 0, "stale": 1, "summarized": 2}
	sort.Slice(cards, func(i, j int) bool {
		oi, oj := statusOrder[cards[i].Status], statusOrder[cards[j].Status]
		if oi != oj {
			return oi < oj
		}
		ti, _ := time.Parse(time.RFC3339, cards[i].UpdatedAt)
		tj, _ := time.Parse(time.RFC3339, cards[j].UpdatedAt)
		return ti.After(tj)
	})

	if cards == nil {
		cards = []sessionCardJSON{}
	}

	jsonOK(w, sessionsResponse{
		Sessions:   cards,
		ProjectDir: s.cfg.ProjectPath,
	})
}

// --- Stats ---

type scopeStatsJSON struct {
	Total       int            `json:"total"`
	Enabled     int            `json:"enabled"`
	Stale       int            `json:"stale"`
	TotalTokens int            `json:"total_tokens"`
	ByTag       map[string]int `json:"by_tag"`
}

type statsResponse struct {
	Global         scopeStatsJSON `json:"global"`
	Local          scopeStatsJSON `json:"local"`
	ActiveSessions int            `json:"active_sessions"`
}

// handleStats serves GET /api/stats
// Spec: S-016 | Req: I-002l, I-002m, I-002n, I-002o, B-009
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	buildScopeStats := func(b kb.Backend) scopeStatsJSON {
		zero := scopeStatsJSON{ByTag: map[string]int{}}
		if b == nil {
			return zero
		}
		stats, err := b.Stats()
		if err != nil {
			return zero
		}

		// Build by_tag from all entries
		byTag := make(map[string]int)
		entries, err := b.List("")
		if err == nil {
			for _, e := range entries {
				for _, t := range e.Tags {
					byTag[t]++
				}
			}
		}

		return scopeStatsJSON{
			Total:       stats.Total,
			Enabled:     stats.Enabled,
			Stale:       stats.Stale,
			TotalTokens: stats.TotalTokens,
			ByTag:       byTag,
		}
	}

	globalStats := buildScopeStats(s.globalBack)
	localStats := buildScopeStats(s.localBack)

	// Count active sessions from ~/.cvm/sessions/*.jsonl (open files with live PID).
	// Spec: S-017 | Req: B-011, C-007, I-004, I-006
	activeSessions := 0
	if sessEntries, err := os.ReadDir(sessionsDir()); err == nil {
		for _, e := range sessEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			path := filepath.Join(sessionsDir(), e.Name())
			events := readAllSessionEvents(path)
			if len(events) == 0 || events[0].Type != "start" {
				continue
			}
			if sessionHasEndEvent(events) {
				continue
			}
			if sessionIsPIDAlive(events[0].PID) {
				activeSessions++
			}
		}
	}

	jsonOK(w, statsResponse{
		Global:         globalStats,
		Local:          localStats,
		ActiveSessions: activeSessions,
	})
}

// parseMetaLine parses "[meta] key=val | key=val | ..." into sessionMetaJSON.
func parseMetaLine(line string) *sessionMetaJSON {
	meta := &sessionMetaJSON{}
	line = strings.TrimPrefix(line, "[meta] ")
	for _, part := range strings.Split(line, " | ") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "project":
			meta.ProjectDir = kv[1]
		case "events":
			meta.EventCount = kv[1]
		case "est_tokens":
			meta.EstTokens = kv[1]
		case "time_range":
			meta.TimeRange = kv[1]
		}
	}
	return meta
}

func metaProjectDir(m *sessionMetaJSON) string {
	if m != nil {
		return m.ProjectDir
	}
	return ""
}
