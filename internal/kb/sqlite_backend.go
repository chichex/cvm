// Spec: S-013 | Req: I-001a (SQLiteBackend)
// SQLiteBackend implements the Backend interface using SQLite + FTS5.
package kb

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chichex/cvm/internal/config"
	_ "modernc.org/sqlite"
)

const schema = `
-- Spec: S-013 | Req: I-003
CREATE TABLE IF NOT EXISTS entries (
    key             TEXT    PRIMARY KEY,
    body            TEXT    NOT NULL DEFAULT '',
    tags            TEXT    NOT NULL DEFAULT '[]',
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL,
    last_referenced TEXT
);

-- Spec: S-017 | Req: C-001 — sessions table (single source of truth for session state)
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

-- Spec: S-013 | Req: I-004
CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
    key,
    body,
    tags,
    content='entries',
    content_rowid='rowid',
    tokenize='porter ascii'
);

-- Spec: S-013 | Req: I-005
CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(rowid, key, body, tags)
    VALUES (new.rowid, new.key, new.body, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, key, body, tags)
    VALUES ('delete', old.rowid, old.key, old.body, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, key, body, tags)
    VALUES ('delete', old.rowid, old.key, old.body, old.tags);
    INSERT INTO entries_fts(rowid, key, body, tags)
    VALUES (new.rowid, new.key, new.body, new.tags);
END;
`

// SQLiteBackend implements Backend using SQLite + FTS5.
// Spec: S-013 | Req: I-001a
type SQLiteBackend struct {
	db     *sql.DB
	closed bool
}

// dbPath returns the path to the kb.db file for the given scope and projectPath.
// Spec: S-013 | Req: I-007
func dbPath(scope config.Scope, projectPath string) string {
	var dir string
	if scope == config.ScopeGlobal {
		dir = config.GlobalKBDir()
	} else {
		dir = config.LocalKBDir(projectPath)
	}
	return filepath.Join(dir, "kb.db")
}

// NewSQLiteBackend opens or creates the SQLite KB database.
// Handles corruption (E-001) and permission errors (E-002) by returning error
// so the factory can fall back to FlatBackend.
// Triggers automatic migration from flat files when kb.db doesn't exist.
// Spec: S-013 | Req: I-002c, I-006, I-007, B-001, B-006, I-012
func NewSQLiteBackend(scope config.Scope, projectPath string) (*SQLiteBackend, error) {
	path := dbPath(scope, projectPath)

	// Ensure parent directory exists (Spec: S-013 | Req: I-007c)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create KB dir: %w", err)
	}

	// Track whether DB is new (for migration decision)
	_, statErr := os.Stat(path)
	dbIsNew := os.IsNotExist(statErr)

	// Check if DB exists to detect corruption / permission issues before opening
	if !dbIsNew {
		// File exists — check permissions
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
		f.Close()
	}

	// Open DB (creates it if not exists)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Spec: S-013 | Req: I-006 — WAL mode + busy_timeout before anything else
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma setup: %w", err)
		}
	}

	// Spec: S-013 | Req: E-001 — integrity check on existing DB (skip for new DB)
	if !dbIsNew {
		row := db.QueryRow("PRAGMA integrity_check")
		var integrityResult string
		if err := row.Scan(&integrityResult); err != nil || integrityResult != "ok" {
			db.Close()
			// Rename corrupt DB for diagnostics (Spec: S-013 | Req: E-001)
			ts := time.Now().Format("20060102T150405")
			corruptPath := path + ".corrupt." + ts
			os.Rename(path, corruptPath)
			return nil, fmt.Errorf("DB integrity check failed: %s (renamed to %s)", integrityResult, corruptPath)
		}
	}

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	// Spec: S-017 | Req: C-002, C-002a — idempotent migration: add session_id column to entries
	var sessionIDExists int
	err = db.QueryRow(`SELECT 1 FROM pragma_table_info('entries') WHERE name='session_id'`).Scan(&sessionIDExists)
	if err != nil {
		// Column not present — add it
		if _, alterErr := db.Exec(`ALTER TABLE entries ADD COLUMN session_id TEXT`); alterErr != nil {
			db.Close()
			return nil, fmt.Errorf("add session_id column: %w", alterErr)
		}
		if _, idxErr := db.Exec(`CREATE INDEX IF NOT EXISTS idx_entries_session_id ON entries(session_id)`); idxErr != nil {
			db.Close()
			return nil, fmt.Errorf("create session_id index: %w", idxErr)
		}
	}

	// Spec: S-013 | Req: I-007c, NF-007 — set file permissions to 0600
	if err := os.Chmod(path, 0600); err != nil {
		// Non-fatal: log but continue
		fmt.Fprintf(os.Stderr, "[cvm] warning: could not set DB permissions: %v\n", err)
	}

	sb := &SQLiteBackend{db: db}

	// Spec: S-013 | Req: B-001, I-012 — migrate from flat files only if DB is new
	// and a .index.json exists with entries.
	if dbIsNew {
		indexFile := filepath.Join(kbDir(scope, projectPath), ".index.json")
		if _, indexErr := os.Stat(indexFile); indexErr == nil {
			// Flat index exists — attempt migration
			if migErr := migrateFromFlat(sb, scope, projectPath); migErr != nil {
				// Migration failure is non-fatal: DB was created but may be empty
				fmt.Fprintf(os.Stderr, "[cvm] migration warning: %v\n", migErr)
			}
		}
	}

	return sb, nil
}

// Put inserts or updates an entry.
// sessionID links the entry to a session; empty string stores NULL (no session link).
// Spec: S-013 | Req: B-007 | Spec: S-017 | Req: C-010
func (s *SQLiteBackend) Put(key, body string, tags []string, now time.Time, sessionID string) error {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	nowStr := now.UTC().Format(time.RFC3339Nano)

	// sessionID: empty string → NULL; non-empty → store value
	var sessionIDVal interface{}
	if sessionID != "" {
		sessionIDVal = sessionID
	}

	// Upsert: preserve created_at on update (Spec: S-013 | Req: I-009)
	_, err = s.db.Exec(`
		INSERT INTO entries (key, body, tags, enabled, created_at, updated_at, session_id)
		VALUES (?, ?, ?, 1, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			body = excluded.body,
			tags = excluded.tags,
			updated_at = excluded.updated_at,
			session_id = excluded.session_id
	`, key, body, string(tagsJSON), nowStr, nowStr, sessionIDVal)
	if err != nil {
		if strings.Contains(err.Error(), "disk") || strings.Contains(err.Error(), "full") {
			return fmt.Errorf("kb: disk full")
		}
		return fmt.Errorf("put entry: %w", err)
	}
	return nil
}

// Get returns the Document for a key.
func (s *SQLiteBackend) Get(key string) (Document, error) {
	row := s.db.QueryRow(`
		SELECT key, body, tags, enabled, created_at, updated_at, last_referenced
		FROM entries WHERE key = ?`, key)
	doc, err := scanDocument(row)
	if err == sql.ErrNoRows {
		return Document{}, errNotFound(key)
	}
	return doc, err
}

// List returns all entries optionally filtered by tag.
func (s *SQLiteBackend) List(tag string) ([]Entry, error) {
	var rows *sql.Rows
	var err error

	if tag == "" {
		rows, err = s.db.Query(`SELECT key, body, tags, enabled, created_at, updated_at, last_referenced FROM entries`)
	} else {
		// Filter by tag via JSON array containment
		rows, err = s.db.Query(`
			SELECT e.key, e.body, e.tags, e.enabled, e.created_at, e.updated_at, e.last_referenced
			FROM entries e, json_each(e.tags) je
			WHERE je.value = ?
			GROUP BY e.key`, tag)
	}
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// Remove deletes an entry.
// Spec: S-013 | Req: B-010
func (s *SQLiteBackend) Remove(key string) error {
	res, err := s.db.Exec(`DELETE FROM entries WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("remove entry: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errNotFound(key)
	}
	return nil
}

// Search performs FTS5 full-text search with ranking.
// Spec: S-013 | Req: B-008, B-009, E-004
func (s *SQLiteBackend) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	// Spec: S-013 | Req: E-004 — empty query returns all entries
	if strings.TrimSpace(query) == "" {
		entries, err := s.List(opts.Tag)
		if err != nil {
			return nil, err
		}
		var results []SearchResult
		for _, e := range entries {
			results = append(results, SearchResult{Entry: e, Snippet: "", Rank: 2})
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].Entry.UpdatedAt.After(results[j].Entry.UpdatedAt)
		})
		return results, nil
	}

	// Build FTS5 query — join with entries for filtering
	// Spec: S-013 | Req: B-008 — FTS5 MATCH with BM25 ranking
	baseQuery := `
		SELECT e.key, e.body, e.tags, e.enabled, e.created_at, e.updated_at, e.last_referenced,
		       rank
		FROM entries_fts f
		JOIN entries e ON e.rowid = f.rowid
		WHERE entries_fts MATCH ?`

	args := []interface{}{query}

	// Spec: S-013 | Req: B-009 — tag filter
	if opts.Tag != "" {
		baseQuery += ` AND EXISTS (SELECT 1 FROM json_each(e.tags) WHERE value = ?)`
		args = append(args, opts.Tag)
	}
	if opts.TypeTag != "" {
		baseQuery += ` AND EXISTS (SELECT 1 FROM json_each(e.tags) WHERE value = ?)`
		args = append(args, opts.TypeTag)
	}
	// Spec: S-013 | Req: B-009 — since filter
	if opts.Since > 0 {
		cutoff := time.Now().Add(-opts.Since).UTC().Format(time.RFC3339Nano)
		baseQuery += ` AND e.updated_at >= ?`
		args = append(args, cutoff)
	}

	baseQuery += ` ORDER BY rank` // rank is negative in FTS5, ASC = most relevant first

	rows, err := s.db.Query(baseQuery, args...)
	if err != nil {
		// FTS5 syntax error (e.g. query contains '-' operator) — fall back to LIKE scan
		// Spec: S-013 | Fix: Backend wiring (graceful fallback for user queries with special chars)
		return s.searchLike(query, opts)
	}
	defer rows.Close()

	lowerQuery := strings.ToLower(query)
	var results []SearchResult
	for rows.Next() {
		var (
			key, body, tagsJSON     string
			enabledInt              int
			createdStr, updatedStr  string
			lastRefStr              sql.NullString
			rank                    float64
		)
		if err := rows.Scan(&key, &body, &tagsJSON, &enabledInt, &createdStr, &updatedStr, &lastRefStr, &rank); err != nil {
			return nil, err
		}
		e, err := parseEntry(key, tagsJSON, enabledInt, createdStr, updatedStr, lastRefStr)
		if err != nil {
			continue
		}

		// Compute rank category: 0=exact key, 1=key contains, 2=body/tags
		// Spec: S-013 | Req: B-008
		lowerKey := strings.ToLower(key)
		var rankCat int
		if lowerKey == lowerQuery {
			rankCat = 0
		} else if strings.Contains(lowerKey, lowerQuery) {
			rankCat = 1
		} else {
			rankCat = 2
		}

		snippet := extractSnippet(body, query)
		results = append(results, SearchResult{
			Entry:   e,
			Snippet: snippet,
			Rank:    rankCat,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort: by rank category, then by UpdatedAt desc within same rank
	// Unless Sort == "recent"
	if opts.Sort == "recent" {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Entry.UpdatedAt.After(results[j].Entry.UpdatedAt)
		})
	} else {
		sort.Slice(results, func(i, j int) bool {
			if results[i].Rank != results[j].Rank {
				return results[i].Rank < results[j].Rank
			}
			return results[i].Entry.UpdatedAt.After(results[j].Entry.UpdatedAt)
		})
	}

	return results, nil
}

// searchLike performs a case-insensitive substring scan as a fallback when FTS5 query parsing fails.
// Spec: S-013 | Fix: Backend wiring (graceful FTS5 fallback)
func (s *SQLiteBackend) searchLike(query string, opts SearchOptions) ([]SearchResult, error) {
	likePattern := "%" + strings.ReplaceAll(query, "%", "\\%") + "%"

	likeQuery := `SELECT e.key, e.body, e.tags, e.enabled, e.created_at, e.updated_at, e.last_referenced
		FROM entries e
		WHERE (LOWER(e.key) LIKE LOWER(?) OR LOWER(e.body) LIKE LOWER(?))`
	args := []interface{}{likePattern, likePattern}

	if opts.Tag != "" {
		likeQuery += ` AND EXISTS (SELECT 1 FROM json_each(e.tags) WHERE value = ?)`
		args = append(args, opts.Tag)
	}
	if opts.TypeTag != "" {
		likeQuery += ` AND EXISTS (SELECT 1 FROM json_each(e.tags) WHERE value = ?)`
		args = append(args, opts.TypeTag)
	}
	if opts.Since > 0 {
		cutoff := time.Now().Add(-opts.Since).UTC().Format(time.RFC3339Nano)
		likeQuery += ` AND e.updated_at >= ?`
		args = append(args, cutoff)
	}

	rows, err := s.db.Query(likeQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("like search: %w", err)
	}
	defer rows.Close()

	lowerQuery := strings.ToLower(query)
	var results []SearchResult
	for rows.Next() {
		var key, body, tagsJSON string
		var enabledInt int
		var createdStr, updatedStr string
		var lastRefStr sql.NullString

		if err := rows.Scan(&key, &body, &tagsJSON, &enabledInt, &createdStr, &updatedStr, &lastRefStr); err != nil {
			return nil, err
		}
		e, err := parseEntry(key, tagsJSON, enabledInt, createdStr, updatedStr, lastRefStr)
		if err != nil {
			continue
		}

		lowerKey := strings.ToLower(key)
		var rankCat int
		if lowerKey == lowerQuery {
			rankCat = 0
		} else if strings.Contains(lowerKey, lowerQuery) {
			rankCat = 1
		} else {
			rankCat = 2
		}

		snippet := extractSnippet(body, query)
		results = append(results, SearchResult{Entry: e, Snippet: snippet, Rank: rankCat})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if opts.Sort == "recent" {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Entry.UpdatedAt.After(results[j].Entry.UpdatedAt)
		})
	} else {
		sort.Slice(results, func(i, j int) bool {
			if results[i].Rank != results[j].Rank {
				return results[i].Rank < results[j].Rank
			}
			return results[i].Entry.UpdatedAt.After(results[j].Entry.UpdatedAt)
		})
	}
	return results, nil
}

// Timeline returns entries grouped by day for the last N days.
// Spec: S-013 | Req: B-013
func (s *SQLiteBackend) Timeline(days int) ([]TimelineDay, error) {
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339Nano)

	rows, err := s.db.Query(`
		SELECT key, body, tags, enabled, created_at, updated_at, last_referenced
		FROM entries
		WHERE updated_at >= ?
		ORDER BY updated_at DESC`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("timeline query: %w", err)
	}
	defer rows.Close()

	entries, err := scanEntries(rows)
	if err != nil {
		return nil, err
	}

	// Group by day
	dayMap := make(map[string][]Entry)
	for _, e := range entries {
		dayKey := e.UpdatedAt.Format("2006-01-02")
		dayMap[dayKey] = append(dayMap[dayKey], e)
	}

	var dayKeys []string
	for k := range dayMap {
		dayKeys = append(dayKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dayKeys)))

	var result []TimelineDay
	for _, k := range dayKeys {
		t, _ := time.Parse("2006-01-02", k)
		result = append(result, TimelineDay{Date: t, Entries: dayMap[k]})
	}
	return result, nil
}

// Stats returns aggregate statistics.
// Spec: S-013 | Req: B-012
func (s *SQLiteBackend) Stats() (StatsResult, error) {
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339Nano)

	var total, enabled, stale int
	err := s.db.QueryRow(`
		SELECT
			COUNT(*),
			SUM(CASE WHEN enabled = 1 THEN 1 ELSE 0 END),
			SUM(CASE WHEN
				(last_referenced IS NOT NULL AND last_referenced < ?) OR
				(last_referenced IS NULL AND created_at < ?)
			THEN 1 ELSE 0 END)
		FROM entries`, thirtyDaysAgo, thirtyDaysAgo).Scan(&total, &enabled, &stale)
	if err != nil {
		return StatsResult{}, fmt.Errorf("stats query: %w", err)
	}

	// Token estimation (chars/4) per entry
	rows, err := s.db.Query(`SELECT key, body FROM entries`)
	if err != nil {
		return StatsResult{}, fmt.Errorf("stats tokens query: %w", err)
	}
	defer rows.Close()

	perEntry := make(map[string]int)
	totalTokens := 0
	for rows.Next() {
		var key, body string
		if err := rows.Scan(&key, &body); err != nil {
			continue
		}
		tokens := len(body) / 4
		perEntry[key] = tokens
		totalTokens += tokens
	}

	return StatsResult{
		Total:       total,
		Enabled:     enabled,
		Stale:       stale,
		TotalTokens: totalTokens,
		PerEntry:    perEntry,
	}, nil
}

// Compact returns a condensed view of all entries sorted by UpdatedAt desc.
// Spec: S-013 | Req: B-014
func (s *SQLiteBackend) Compact() ([]CompactEntry, error) {
	rows, err := s.db.Query(`
		SELECT key, tags, body, updated_at
		FROM entries
		ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("compact query: %w", err)
	}
	defer rows.Close()

	var entries []CompactEntry
	for rows.Next() {
		var key, tagsJSON, body, updatedStr string
		if err := rows.Scan(&key, &tagsJSON, &body, &updatedStr); err != nil {
			return nil, err
		}
		var tags []string
		json.Unmarshal([]byte(tagsJSON), &tags)

		updatedAt, _ := time.Parse(time.RFC3339Nano, updatedStr)

		firstLine := ""
		if body != "" {
			lines := strings.SplitN(body, "\n", 2)
			firstLine = strings.TrimSpace(lines[0])
			if len(firstLine) > 80 {
				firstLine = firstLine[:80] + "..."
			}
		}

		entries = append(entries, CompactEntry{
			Key:       key,
			Tags:      tags,
			FirstLine: firstLine,
			UpdatedAt: updatedAt,
		})
	}
	return entries, rows.Err()
}

// SetEnabled toggles the enabled flag.
// Spec: S-013 | Req: B-016
func (s *SQLiteBackend) SetEnabled(key string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE entries SET enabled = ?, updated_at = ? WHERE key = ?`, enabledInt, now, key)
	if err != nil {
		return fmt.Errorf("set enabled: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errNotFound(key)
	}
	return nil
}

// LoadDocuments returns all Documents.
// Spec: S-013 | Req: B-017
func (s *SQLiteBackend) LoadDocuments() ([]Document, error) {
	rows, err := s.db.Query(`
		SELECT key, body, tags, enabled, created_at, updated_at, last_referenced FROM entries`)
	if err != nil {
		return nil, fmt.Errorf("load documents: %w", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// SaveDocument upserts a Document preserving created_at.
// Spec: S-013 | Req: B-017
func (s *SQLiteBackend) SaveDocument(doc Document) error {
	tagsJSON, err := json.Marshal(doc.Entry.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	enabledInt := 0
	if doc.Entry.Enabled {
		enabledInt = 1
	}
	createdStr := doc.Entry.CreatedAt.UTC().Format(time.RFC3339Nano)
	updatedStr := doc.Entry.UpdatedAt.UTC().Format(time.RFC3339Nano)

	var lastRefStr *string
	if !doc.Entry.LastReferenced.IsZero() {
		s := doc.Entry.LastReferenced.UTC().Format(time.RFC3339Nano)
		lastRefStr = &s
	}

	_, err = s.db.Exec(`
		INSERT INTO entries (key, body, tags, enabled, created_at, updated_at, last_referenced)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			body = excluded.body,
			tags = excluded.tags,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at,
			last_referenced = excluded.last_referenced
	`, doc.Entry.Key, doc.Body, string(tagsJSON), enabledInt, createdStr, updatedStr, lastRefStr)
	return err
}

// Close closes the underlying DB connection. Idempotent.
// Spec: S-013 | Req: I-001c
func (s *SQLiteBackend) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

// Show returns the body for a key and updates LastReferenced.
// Spec: S-013 | Req: B-011
func (s *SQLiteBackend) Show(key string) (string, error) {
	var body string
	err := s.db.QueryRow(`SELECT body FROM entries WHERE key = ?`, key).Scan(&body)
	if err == sql.ErrNoRows {
		return "", errNotFound(key)
	}
	if err != nil {
		return "", err
	}
	// Update last_referenced
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.db.Exec(`UPDATE entries SET last_referenced = ? WHERE key = ?`, now, key)
	return body, nil
}

// Clean deletes all entries.
// Spec: S-013 | Req: B-015
func (s *SQLiteBackend) Clean() (int, error) {
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&count)
	if count == 0 {
		return 0, nil
	}
	if _, err := s.db.Exec(`DELETE FROM entries`); err != nil {
		return 0, fmt.Errorf("clean: %w", err)
	}
	return count, nil
}

// bodyHashSQLite computes SHA256 hash of a body for dedup (Spec: S-013 | Req: B-018)
func bodyHashSQLite(body string) string {
	h := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", h[:8])
}

// PutWithDedup inserts or updates only if the content hash differs.
// Returns skipped=true if the entry already exists with identical body and tags.
// Spec: S-013 | Fix: Backend wiring
func (s *SQLiteBackend) PutWithDedup(key, body string, tags []string, now time.Time) (skipped bool, err error) {
	newHash := bodyHashSQLite(body)

	// Scan all entries to find duplicates
	rows, err := s.db.Query(`SELECT key, body, tags FROM entries`)
	if err != nil {
		return false, fmt.Errorf("dedup scan: %w", err)
	}
	defer rows.Close()

	type existingEntry struct {
		key  string
		body string
		tags []string
	}
	var existing []existingEntry

	for rows.Next() {
		var eKey, eBody, eTagsJSON string
		if err := rows.Scan(&eKey, &eBody, &eTagsJSON); err != nil {
			continue
		}
		var eTags []string
		json.Unmarshal([]byte(eTagsJSON), &eTags)
		existing = append(existing, existingEntry{eKey, eBody, eTags})
	}
	rows.Close()

	for _, e := range existing {
		if bodyHashSQLite(e.body) == newHash {
			if e.key == key {
				tagsChanged := !tagsEqual(e.tags, tags)
				if tagsChanged {
					// Same key, same body, different tags — update tags only
					tagsJSON, _ := json.Marshal(tags)
					nowStr := now.UTC().Format(time.RFC3339Nano)
					_, err = s.db.Exec(`UPDATE entries SET tags = ?, updated_at = ? WHERE key = ?`, string(tagsJSON), nowStr, key)
					return false, err
				}
				// Same key, same body, same tags — skip
				return true, nil
			}
			// Different key, same body — warn but continue
			fmt.Fprintf(os.Stderr, "warning: duplicate content (matches %q)\n", e.key)
		}
	}

	return false, s.Put(key, body, tags, now, "")
}

// --- helpers ---

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanDocument(s scanner) (Document, error) {
	var key, body, tagsJSON string
	var enabledInt int
	var createdStr, updatedStr string
	var lastRefStr sql.NullString

	if err := s.Scan(&key, &body, &tagsJSON, &enabledInt, &createdStr, &updatedStr, &lastRefStr); err != nil {
		return Document{}, err
	}

	e, err := parseEntry(key, tagsJSON, enabledInt, createdStr, updatedStr, lastRefStr)
	if err != nil {
		return Document{}, err
	}
	return Document{Entry: e, Body: body}, nil
}

func scanEntries(rows *sql.Rows) ([]Entry, error) {
	var entries []Entry
	for rows.Next() {
		var key, body, tagsJSON string
		var enabledInt int
		var createdStr, updatedStr string
		var lastRefStr sql.NullString

		if err := rows.Scan(&key, &body, &tagsJSON, &enabledInt, &createdStr, &updatedStr, &lastRefStr); err != nil {
			return nil, err
		}
		e, err := parseEntry(key, tagsJSON, enabledInt, createdStr, updatedStr, lastRefStr)
		if err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func parseEntry(key, tagsJSON string, enabledInt int, createdStr, updatedStr string, lastRefStr sql.NullString) (Entry, error) {
	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		tags = nil
	}

	createdAt, err := time.Parse(time.RFC3339Nano, createdStr)
	if err != nil {
		createdAt, _ = time.Parse(time.RFC3339, createdStr)
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedStr)
	if err != nil {
		updatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	}

	var lastReferenced time.Time
	if lastRefStr.Valid && lastRefStr.String != "" {
		lastReferenced, _ = time.Parse(time.RFC3339Nano, lastRefStr.String)
		if lastReferenced.IsZero() {
			lastReferenced, _ = time.Parse(time.RFC3339, lastRefStr.String)
		}
	}

	return Entry{
		Key:            key,
		Tags:           tags,
		Enabled:        enabledInt == 1,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		LastReferenced: lastReferenced,
	}, nil
}
