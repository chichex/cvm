// Spec: S-016
// API handlers for the CVM dashboard HTTP endpoints.
package dashboard

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/kb"
	"github.com/chichex/cvm/internal/session"
	_ "modernc.org/sqlite"
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

// openGlobalDB opens the global KB SQLite database for direct querying.
// The caller must close the returned *sql.DB.
// Spec: S-017 | Req: C-001, B-011, B-012
func openGlobalDB() (*sql.DB, error) {
	path := filepath.Join(config.GlobalKBDir(), "kb.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, err
	}
	// Ensure sessions table + entries.session_id exist (idempotent). Spec: S-017 | Req: C-002a
	db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'active',
		project TEXT NOT NULL, profile TEXT NOT NULL DEFAULT '',
		started_at TEXT NOT NULL, ended_at TEXT,
		jsonl_path TEXT NOT NULL, event_count INTEGER NOT NULL DEFAULT 0,
		parent_session_id TEXT
	)`)
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions(parent_session_id)")
	var hasCol int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('entries') WHERE name='session_id'").Scan(&hasCol); err == nil && hasCol == 0 {
		db.Exec("ALTER TABLE entries ADD COLUMN session_id TEXT")
		db.Exec("CREATE INDEX IF NOT EXISTS idx_entries_session_id ON entries(session_id)")
	}
	// Spec: S-018 | Req: C-001, E-004 — idempotent migration for parent_session_id
	var hasParentCol int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name='parent_session_id'").Scan(&hasParentCol); err == nil && hasParentCol == 0 {
		if _, err := db.Exec("ALTER TABLE sessions ADD COLUMN parent_session_id TEXT"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to add parent_session_id column: %v\n", err)
		}
		db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions(parent_session_id)")
	}
	return db, nil
}

// querySessionStatus returns the status of a session from SQLite. Spec: S-017 | Req: I-005
func querySessionStatus(sessionID string) (string, error) {
	db, err := openGlobalDB()
	if err != nil {
		return "", err
	}
	defer db.Close()
	var status string
	err = db.QueryRow("SELECT status FROM sessions WHERE id = ?", sessionID).Scan(&status)
	return status, err
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
		// Active: check SQLite first, fallback to JSONL. Spec: S-017 | Req: I-005
		active := false
		if dbStatus, err := querySessionStatus(uuid); err == nil {
			active = dbStatus == "active"
		} else {
			active = len(events) > 0 && !sessionHasEndEvent(events)
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

// Spec: S-020 | Req: C-002
type entryJSON struct {
	Key           string   `json:"key"`
	Tags          []string `json:"tags"`
	Scope         string   `json:"scope"`
	Enabled       bool     `json:"enabled"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	Body          string   `json:"body"`
	TokenEstimate int      `json:"token_estimate"`
	SessionID     string   `json:"session_id,omitempty"` // Spec: S-020 | Req: C-002a
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
			SessionID:     e.SessionID, // Spec: S-020 | Req: C-002c
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

// Spec: S-018 | Req: C-006
type retroSessionJSON struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at,omitempty"`
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
	LineCount  int `json:"line_count,omitempty"`
	KBEntries  int `json:"kb_entries,omitempty"` // Spec: S-017 | Req: C-008

	// Summarized-only fields
	SummaryBody string `json:"summary_body,omitempty"`

	// Spec: S-018 | Req: C-006 — nested retro session
	RetroSession *retroSessionJSON `json:"retro_session,omitempty"`
}

type sessionsResponse struct {
	Sessions   []sessionCardJSON `json:"sessions"`
	ProjectDir string            `json:"project_dir"`
}

// handleSessions serves GET /api/sessions
// Reads from SQLite sessions table + legacy KB summaries (E-009).
// Spec: S-017 | Req: B-011, B-012, C-008, E-009, I-005
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cards []sessionCardJSON

	// --- Sessions from SQLite sessions table (C-008 query) ---
	// Spec: S-017 | Req: B-011, C-008, I-005
	// Spec: S-018 | Req: C-006, B-002, B-003, I-001
	db, dbErr := openGlobalDB()
	if dbErr == nil {
		// Query top-level sessions only (parent_session_id IS NULL). Spec: S-018 | Req: I-001
		rows, err := db.Query(`
			SELECT s.id, s.status, s.project, s.profile, s.started_at, s.ended_at,
			       s.jsonl_path, s.event_count, COUNT(e.key) as kb_entries
			FROM sessions s
			LEFT JOIN entries e ON e.session_id = s.id
			WHERE s.parent_session_id IS NULL
			GROUP BY s.id
			ORDER BY s.started_at DESC
		`)
		if err == nil {
			for rows.Next() {
				var (
					id, status, project, profile, startedAt string
					endedAt                                  sql.NullString
					jsonlPath                                string
					eventCount, kbEntries                    int
				)
				if rows.Scan(&id, &status, &project, &profile, &startedAt, &endedAt, &jsonlPath, &eventCount, &kbEntries) != nil {
					continue
				}

				updatedAt := startedAt
				if endedAt.Valid {
					updatedAt = endedAt.String
				} else if jsonlPath != "" {
					if fi, err := os.Stat(jsonlPath); err == nil {
						updatedAt = fi.ModTime().UTC().Format(time.RFC3339)
					}
				}

				card := sessionCardJSON{
					ID:         id,
					Key:        id + ".jsonl",
					Status:     status,
					Scope:      "global",
					CreatedAt:  startedAt,
					UpdatedAt:  updatedAt,
					ProjectDir: project,
					LineCount:  eventCount,
					KBEntries:  kbEntries,
					Knowledge:  []knowledgeEntryJSON{},
					Meta: &sessionMetaJSON{
						ProjectDir: project,
						EventCount: fmt.Sprintf("%d", eventCount),
					},
				}
				cards = append(cards, card)
			}
			rows.Close()
		}

		// Spec: S-018 | Req: B-002, C-006, E-003 — attach child retro sessions to parents
		childRows, childErr := db.Query(`
			SELECT id, status, started_at, ended_at, parent_session_id
			FROM sessions
			WHERE parent_session_id IS NOT NULL
			ORDER BY started_at DESC
		`)
		if childErr == nil {
			// Build map: parent_session_id → most recent child (E-003)
			retroMap := map[string]*retroSessionJSON{}
			for childRows.Next() {
				var childID, childStatus, childStartedAt, parentID string
				var childEndedAt sql.NullString
				if childRows.Scan(&childID, &childStatus, &childStartedAt, &childEndedAt, &parentID) != nil {
					continue
				}
				if _, exists := retroMap[parentID]; exists {
					continue // keep only the most recent (first row due to ORDER BY DESC)
				}
				retro := &retroSessionJSON{
					ID:        childID,
					Status:    childStatus,
					StartedAt: childStartedAt,
				}
				if childEndedAt.Valid {
					retro.EndedAt = childEndedAt.String
				}
				retroMap[parentID] = retro
			}
			childRows.Close()
			for i := range cards {
				if retro, ok := retroMap[cards[i].ID]; ok {
					cards[i].RetroSession = retro
				}
			}
		}

		// Spec: S-018 | Req: B-003, I-003 — populate knowledge entries per session
		kbRows, kbErr := db.Query(`
			SELECT key, body, tags, created_at, updated_at, session_id
			FROM entries
			WHERE session_id IS NOT NULL AND session_id != ''
		`)
		if kbErr == nil {
			knowledgeMap := map[string][]knowledgeEntryJSON{}
			for kbRows.Next() {
				var key, body, tagsJSON, createdAt, updatedAt, sessionID string
				if kbRows.Scan(&key, &body, &tagsJSON, &createdAt, &updatedAt, &sessionID) != nil {
					continue
				}
				var tags []string
				if json.Unmarshal([]byte(tagsJSON), &tags) != nil {
					tags = []string{}
				}
				knowledgeMap[sessionID] = append(knowledgeMap[sessionID], knowledgeEntryJSON{
					Key:           key,
					Tags:          tags,
					Scope:         "global",
					CreatedAt:     createdAt,
					UpdatedAt:     updatedAt,
					Body:          body,
					TokenEstimate: len(body) / 4,
				})
			}
			kbRows.Close()
			for i := range cards {
				if entries, ok := knowledgeMap[cards[i].ID]; ok {
					cards[i].Knowledge = entries
					cards[i].KBEntries = len(entries)
				}
			}
		}

		db.Close()
	}

	// --- Legacy KB summaries (E-009: backward compatibility) ---
	// Show existing session-summary-* entries from KB with status "legacy".
	// Spec: S-017 | Req: E-009
	if s.globalBack != nil {
		entries, err := s.globalBack.List("")
		if err == nil {
			for _, e := range entries {
				isLegacySummary := strings.HasPrefix(e.Key, "session-")
				if !isLegacySummary {
					continue
				}
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
				doc, getErr := s.globalBack.Get(e.Key)
				summaryBody := ""
				if getErr == nil {
					summaryBody = doc.Body
				}
				tags := e.Tags
				if tags == nil {
					tags = []string{}
				}
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
					Status:      "legacy",
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

	// Sort: active first, ended second, legacy last, then by UpdatedAt desc.
	statusOrder := map[string]int{"active": 0, "ended": 1, "legacy": 2}
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

// Spec: S-019 | Req: C-002
type scopeStatsJSON struct {
	Total       int            `json:"total"`
	Enabled     int            `json:"enabled"`
	Stale       int            `json:"stale"`
	TotalTokens int            `json:"total_tokens"`
	ByType      map[string]int `json:"by_type"`
	ByTopic     map[string]int `json:"by_topic"`
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

	// Spec: S-019 | Req: C-002
	buildScopeStats := func(b kb.Backend) scopeStatsJSON {
		zero := scopeStatsJSON{ByType: map[string]int{}, ByTopic: map[string]int{}}
		if b == nil {
			return zero
		}
		stats, err := b.Stats()
		if err != nil {
			return zero
		}

		byType := make(map[string]int)
		byTopic := make(map[string]int)
		entries, err := b.List("")
		if err == nil {
			for _, e := range entries {
				for _, t := range e.Tags {
					switch kb.ClassifyTag(t) {
					case "type":
						byType[t]++
					case "topic":
						byTopic[t]++
					// "internal" tags are excluded
					}
				}
			}
		}

		return scopeStatsJSON{
			Total:       stats.Total,
			Enabled:     stats.Enabled,
			Stale:       stats.Stale,
			TotalTokens: stats.TotalTokens,
			ByType:      byType,
			ByTopic:     byTopic,
		}
	}

	globalStats := buildScopeStats(s.globalBack)
	localStats := buildScopeStats(s.localBack)

	// Count active sessions from SQLite sessions table. Spec: S-017 | Req: B-012, C-008, I-005
	activeSessions := 0
	db, dbErr := openGlobalDB()
	if dbErr == nil {
		row := db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE status = 'active'`)
		row.Scan(&activeSessions) //nolint:errcheck
		db.Close()
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
