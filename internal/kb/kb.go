package kb

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chichex/cvm/internal/config"
)

// Spec: S-010 | Req: B-008
var ValidTypes = []string{"decision", "learning", "gotcha", "discovery", "session"}

func ValidateType(t string) error {
	for _, v := range ValidTypes {
		if t == v {
			return nil
		}
	}
	return fmt.Errorf("invalid type %q: must be one of %s", t, strings.Join(ValidTypes, ", "))
}

// Spec: S-019 | Req: B-001, B-002, B-003
var TypeTags = ValidTypes

var InternalTags = []string{"auto-captured", "session-buffer"}

func ClassifyTag(tag string) string {
	for _, t := range InternalTags {
		if tag == t || strings.HasPrefix(tag, t+"-") {
			return "internal"
		}
	}
	if strings.HasPrefix(tag, "type:") {
		return "internal"
	}
	if matched, _ := filepath.Match("s[0-9][0-9][0-9]", tag); matched {
		return "internal"
	}
	for _, t := range TypeTags {
		if tag == t {
			return "type"
		}
	}
	return "topic"
}

// Spec: S-010 | Req: B-009, B-010
type SearchOptions struct {
	Tag     string
	Since   time.Duration
	TypeTag string
	Sort    string // "relevance" or "recent"
}

// Spec: S-010 | Req: B-014
type CompactEntry struct {
	Key       string
	Tags      []string
	FirstLine string
	UpdatedAt time.Time
}

// Spec: S-010 | Req: B-012
type StatsResult struct {
	Total       int
	Enabled     int
	Stale       int
	TotalTokens int
	PerEntry    map[string]int
}

type Entry struct {
	Key            string    `json:"key"`
	Tags           []string  `json:"tags"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	LastReferenced time.Time `json:"last_referenced,omitempty"`
}

type Document struct {
	Entry Entry
	Body  string
}

type Index struct {
	Entries []Entry `json:"entries"`
}

func kbDir(scope config.Scope, projectPath string) string {
	if scope == config.ScopeGlobal {
		return config.GlobalKBDir()
	}
	return config.LocalKBDir(projectPath)
}

func indexPath(scope config.Scope, projectPath string) string {
	return filepath.Join(kbDir(scope, projectPath), ".index.json")
}

func entriesDir(scope config.Scope, projectPath string) string {
	return filepath.Join(kbDir(scope, projectPath), "entries")
}

func entryPath(scope config.Scope, projectPath, key string) string {
	return filepath.Join(entriesDir(scope, projectPath), key+".md")
}

func loadIndex(scope config.Scope, projectPath string) (*Index, error) {
	idx := &Index{}
	data, err := os.ReadFile(indexPath(scope, projectPath))
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func saveIndex(scope config.Scope, projectPath string, idx *Index) error {
	dir := kbDir(scope, projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath(scope, projectPath), data, 0644)
}

// Put inserts or updates an entry. Delegates through Backend.
// sessionID links the entry to a session; pass "" for no session link.
// Spec: S-013 | Fix: Backend wiring | Spec: S-017 | Req: C-010
func Put(scope config.Scope, projectPath, key, body string, tags []string, sessionID string) error {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return err
	}
	defer b.Close()
	return b.Put(key, body, tags, time.Now(), sessionID)
}

// LoadDocuments returns all Documents. Delegates through Backend.
// Spec: S-013 | Fix: Backend wiring
func LoadDocuments(scope config.Scope, projectPath string) ([]Document, error) {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return nil, err
	}
	defer b.Close()
	return b.LoadDocuments()
}

// SaveDocument upserts a Document. Delegates through Backend.
// Spec: S-013 | Fix: Backend wiring
func SaveDocument(scope config.Scope, projectPath string, doc Document) error {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return err
	}
	defer b.Close()
	return b.SaveDocument(doc)
}

// List returns all entries optionally filtered by tag. Delegates through Backend.
// Spec: S-013 | Fix: Backend wiring
func List(scope config.Scope, projectPath, tag string) ([]Entry, error) {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return nil, err
	}
	defer b.Close()
	return b.List(tag)
}

// Remove deletes an entry. Delegates through Backend.
// Spec: S-013 | Fix: Backend wiring
func Remove(scope config.Scope, projectPath, key string) error {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return err
	}
	defer b.Close()
	return b.Remove(key)
}

// Show returns the raw content of the entry and updates LastReferenced. Delegates through Backend.
// Spec: S-013 | Fix: Backend wiring
func Show(scope config.Scope, projectPath, key string) (string, error) {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return "", err
	}
	defer b.Close()
	return b.Show(key)
}

// SetEnabled toggles the enabled flag. Delegates through Backend.
// Spec: S-013 | Fix: Backend wiring
func SetEnabled(scope config.Scope, projectPath, key string, enabled bool) error {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return err
	}
	defer b.Close()
	return b.SetEnabled(key, enabled)
}

// Search performs full-text search. Delegates through Backend.
// Spec: S-013 | Fix: Backend wiring
func Search(scope config.Scope, projectPath, query string) ([]SearchResult, error) {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return nil, err
	}
	defer b.Close()
	return b.Search(query, SearchOptions{})
}

type SearchResult struct {
	Entry   Entry
	Snippet string
	Rank    int // 0=exact key, 1=key contains, 2=body match (Spec: S-010 | Req: B-009)
}

func extractSnippet(content, query string) string {
	lower := strings.ToLower(content)
	idx := strings.Index(lower, strings.ToLower(query))
	if idx == -1 {
		if len(content) > 100 {
			return content[:100] + "..."
		}
		return content
	}
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 40
	if end > len(content) {
		end = len(content)
	}
	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return strings.TrimSpace(snippet)
}

// Clean removes all entries. Delegates through Backend.
// Spec: S-013 | Fix: Backend wiring
func Clean(scope config.Scope, projectPath string) (int, error) {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return 0, err
	}
	defer b.Close()
	return b.Clean()
}

// Stats returns basic statistics. Delegates through Backend.
// Spec: S-013 | Fix: Backend wiring
func Stats(scope config.Scope, projectPath string) (total, enabled, stale int, err error) {
	b, bErr := NewBackend(scope, projectPath)
	if bErr != nil {
		return 0, 0, 0, bErr
	}
	defer b.Close()
	result, sErr := b.Stats()
	if sErr != nil {
		return 0, 0, 0, sErr
	}
	return result.Total, result.Enabled, result.Stale, nil
}

// putWithTime is like Put but accepts an explicit timestamp for testability.
// Spec: S-013 | Req: I-001b
func putWithTime(scope config.Scope, projectPath, key, body string, tags []string, now time.Time) error {
	dir := entriesDir(scope, projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return err
	}

	found := false
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries[i].Tags = tags
			idx.Entries[i].UpdatedAt = now
			found = true
			break
		}
	}

	if !found {
		idx.Entries = append(idx.Entries, Entry{
			Key:       key,
			Tags:      tags,
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	content := fmt.Sprintf("---\nkey: %s\ntags: [%s]\n---\n\n%s\n", key, strings.Join(tags, ", "), body)
	if err := os.WriteFile(entryPath(scope, projectPath, key), []byte(content), 0644); err != nil {
		return err
	}

	return saveIndex(scope, projectPath, idx)
}

// errNotFound returns the canonical "not found" error for a key.
func errNotFound(key string) error {
	return fmt.Errorf("entry %q not found", key)
}

func renderDocument(key string, tags []string, body string) string {
	return fmt.Sprintf("---\nkey: %s\ntags: [%s]\n---\n\n%s\n", key, strings.Join(tags, ", "), body)
}

func readBody(scope config.Scope, projectPath, key string) (string, error) {
	data, err := os.ReadFile(entryPath(scope, projectPath, key))
	if err != nil {
		return "", err
	}
	content := string(data)

	const frontmatterEnd = "\n---\n\n"
	if idx := strings.Index(content, frontmatterEnd); idx >= 0 {
		return strings.TrimSpace(content[idx+len(frontmatterEnd):]), nil
	}

	return strings.TrimSpace(content), nil
}

// Spec: S-010 | Req: B-013
func bodyHash(body string) string {
	h := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", h[:8])
}

// PutWithDedup inserts or updates only if the content hash differs. Delegates through Backend.
// Spec: S-010 | Req: B-013 | Fix: Backend wiring
func PutWithDedup(scope config.Scope, projectPath, key, body string, tags []string) (skipped bool, err error) {
	b, bErr := NewBackend(scope, projectPath)
	if bErr != nil {
		return false, bErr
	}
	defer b.Close()
	return b.PutWithDedup(key, body, tags, time.Now())
}

func tagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SearchWithOptions performs filtered full-text search. Delegates through Backend.
// Spec: S-010 | Req: B-009, B-010 | Fix: Backend wiring
func SearchWithOptions(scope config.Scope, projectPath, query string, opts SearchOptions) ([]SearchResult, error) {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return nil, err
	}
	defer b.Close()
	return b.Search(query, opts)
}

// Spec: S-010 | Req: B-011
type TimelineDay struct {
	Date    time.Time
	Entries []Entry
}

// Timeline returns entries grouped by day. Delegates through Backend.
// Spec: S-010 | Req: B-011 | Fix: Backend wiring
func Timeline(scope config.Scope, projectPath string, days int) ([]TimelineDay, error) {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return nil, err
	}
	defer b.Close()
	return b.Timeline(days)
}

// StatsDetailed returns aggregate statistics. Delegates through Backend.
// Spec: S-010 | Req: B-012 | Fix: Backend wiring
func StatsDetailed(scope config.Scope, projectPath string) (StatsResult, error) {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return StatsResult{}, err
	}
	defer b.Close()
	return b.Stats()
}

// Compact returns a condensed view of all entries. Delegates through Backend.
// Spec: S-010 | Req: B-014 | Fix: Backend wiring
func Compact(scope config.Scope, projectPath string) ([]CompactEntry, error) {
	b, err := NewBackend(scope, projectPath)
	if err != nil {
		return nil, err
	}
	defer b.Close()
	return b.Compact()
}
