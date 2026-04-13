// Spec: S-016
// API handlers for the CVM dashboard HTTP endpoints.
package dashboard

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chichex/cvm/internal/kb"
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

type sessionLineJSON struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Tool      string `json:"tool,omitempty"`
	Content   string `json:"content"`
}

type sessionDetailResponse struct {
	SessionID string            `json:"session_id"`
	Key       string            `json:"key"`
	Lines     []sessionLineJSON `json:"lines"`
	LineCount int               `json:"line_count"`
	Found     bool              `json:"found"`
}

type sessionBufferSummary struct {
	SessionID string `json:"session_id"`
	Key       string `json:"key"`
	LineCount int    `json:"line_count"`
	UpdatedAt string `json:"updated_at"`
}

type sessionListResponse struct {
	Buffers []sessionBufferSummary `json:"buffers"`
}

// handleSession serves GET /api/session
// Spec: S-016 | Req: I-002e, I-002f, I-002g, B-006, B-007
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")

	// Work with local backend (session buffers live in local KB)
	localB := s.localBack

	if id != "" {
		// Specific session buffer
		key := "session-buffer-" + id
		if localB == nil {
			jsonOK(w, sessionDetailResponse{
				SessionID: id,
				Key:       key,
				Lines:     []sessionLineJSON{},
				LineCount: 0,
				Found:     false,
			})
			return
		}
		doc, err := localB.Get(key)
		if err != nil {
			// Not found
			jsonOK(w, sessionDetailResponse{
				SessionID: id,
				Key:       key,
				Lines:     []sessionLineJSON{},
				LineCount: 0,
				Found:     false,
			})
			return
		}
		lines := ParseSessionLines(doc.Body)
		jsonOK(w, sessionDetailResponse{
			SessionID: id,
			Key:       key,
			Lines:     lines,
			LineCount: len(lines),
			Found:     true,
		})
		return
	}

	// List all active session buffers
	var buffers []sessionBufferSummary
	if localB != nil {
		entries, err := localB.List("")
		if err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Key, "session-buffer-") {
					sid := strings.TrimPrefix(e.Key, "session-buffer-")
					// Get body to count lines
					doc, err := localB.Get(e.Key)
					lineCount := 0
					if err == nil {
						lineCount = countLines(doc.Body)
					}
					buffers = append(buffers, sessionBufferSummary{
						SessionID: sid,
						Key:       e.Key,
						LineCount: lineCount,
						UpdatedAt: e.UpdatedAt.UTC().Format(time.RFC3339),
					})
				}
			}
		}
	}
	if buffers == nil {
		buffers = []sessionBufferSummary{}
	}
	jsonOK(w, sessionListResponse{Buffers: buffers})
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

	// Count active sessions — entries with key prefix "session-buffer-" in local
	// Spec: S-016 | Req: I-002n
	activeSessions := 0
	if s.localBack != nil {
		entries, err := s.localBack.List("")
		if err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Key, "session-buffer-") {
					activeSessions++
				}
			}
		}
	}

	jsonOK(w, statsResponse{
		Global:         globalStats,
		Local:          localStats,
		ActiveSessions: activeSessions,
	})
}
