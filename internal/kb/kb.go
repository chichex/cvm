package kb

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func Put(scope config.Scope, projectPath, key, body string, tags []string) error {
	dir := entriesDir(scope, projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return err
	}

	now := time.Now()
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

func LoadDocuments(scope config.Scope, projectPath string) ([]Document, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return nil, err
	}

	docs := make([]Document, 0, len(idx.Entries))
	for _, entry := range idx.Entries {
		body, err := readBody(scope, projectPath, entry.Key)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		docs = append(docs, Document{
			Entry: entry,
			Body:  body,
		})
	}
	return docs, nil
}

func SaveDocument(scope config.Scope, projectPath string, doc Document) error {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return err
	}

	found := false
	for i, entry := range idx.Entries {
		if entry.Key == doc.Entry.Key {
			idx.Entries[i] = doc.Entry
			found = true
			break
		}
	}
	if !found {
		idx.Entries = append(idx.Entries, doc.Entry)
	}

	if err := os.MkdirAll(entriesDir(scope, projectPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(entryPath(scope, projectPath, doc.Entry.Key), []byte(renderDocument(doc.Entry.Key, doc.Entry.Tags, doc.Body)), 0644); err != nil {
		return err
	}

	return saveIndex(scope, projectPath, idx)
}

func List(scope config.Scope, projectPath, tag string) ([]Entry, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return nil, err
	}
	if tag == "" {
		return idx.Entries, nil
	}
	var filtered []Entry
	for _, e := range idx.Entries {
		for _, t := range e.Tags {
			if t == tag {
				filtered = append(filtered, e)
				break
			}
		}
	}
	return filtered, nil
}

func Remove(scope config.Scope, projectPath, key string) error {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return err
	}
	found := false
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries = append(idx.Entries[:i], idx.Entries[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("entry %q not found", key)
	}
	os.Remove(entryPath(scope, projectPath, key))
	return saveIndex(scope, projectPath, idx)
}

func Show(scope config.Scope, projectPath, key string) (string, error) {
	data, err := os.ReadFile(entryPath(scope, projectPath, key))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("entry %q not found", key)
		}
		return "", err
	}
	idx, _ := loadIndex(scope, projectPath)
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries[i].LastReferenced = time.Now()
			saveIndex(scope, projectPath, idx)
			break
		}
	}
	return string(data), nil
}

func SetEnabled(scope config.Scope, projectPath, key string, enabled bool) error {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return err
	}
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries[i].Enabled = enabled
			idx.Entries[i].UpdatedAt = time.Now()
			return saveIndex(scope, projectPath, idx)
		}
	}
	return fmt.Errorf("entry %q not found", key)
}

func Search(scope config.Scope, projectPath, query string) ([]SearchResult, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(query)
	var results []SearchResult
	for _, e := range idx.Entries {
		data, err := os.ReadFile(entryPath(scope, projectPath, e.Key))
		if err != nil {
			continue
		}
		content := strings.ToLower(string(data))
		if strings.Contains(content, query) || strings.Contains(strings.ToLower(e.Key), query) {
			results = append(results, SearchResult{
				Entry:   e,
				Snippet: extractSnippet(string(data), query),
			})
		}
	}
	return results, nil
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

func Clean(scope config.Scope, projectPath string) (int, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return 0, err
	}
	count := len(idx.Entries)
	if count == 0 {
		return 0, nil
	}

	// Remove all entry files
	for _, e := range idx.Entries {
		os.Remove(entryPath(scope, projectPath, e.Key))
	}

	// Reset index
	idx.Entries = nil
	if err := saveIndex(scope, projectPath, idx); err != nil {
		return 0, err
	}
	return count, nil
}

func Stats(scope config.Scope, projectPath string) (total, enabled, stale int, err error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return 0, 0, 0, err
	}
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	for _, e := range idx.Entries {
		total++
		if e.Enabled {
			enabled++
		}
		if !e.LastReferenced.IsZero() && e.LastReferenced.Before(thirtyDaysAgo) {
			stale++
		} else if e.LastReferenced.IsZero() && e.CreatedAt.Before(thirtyDaysAgo) {
			stale++
		}
	}
	return total, enabled, stale, nil
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

// Spec: S-010 | Req: B-008
func PutWithOptions(scope config.Scope, projectPath, key, body string, tags []string, typeTag string) error {
	if typeTag != "" {
		if err := ValidateType(typeTag); err != nil {
			return err
		}
		tags = append(tags, "type:"+typeTag)
	}
	return Put(scope, projectPath, key, body, tags)
}

// Spec: S-010 | Req: B-013 — content-hash dedup logic in Put
func PutWithDedup(scope config.Scope, projectPath, key, body string, tags []string) (skipped bool, err error) {
	dir := entriesDir(scope, projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, err
	}

	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return false, err
	}

	newHash := bodyHash(body)

	// Check for duplicate content
	for _, e := range idx.Entries {
		existingBody, readErr := readBody(scope, projectPath, e.Key)
		if readErr != nil {
			continue
		}
		existingHash := bodyHash(existingBody)

		if existingHash == newHash {
			if e.Key == key {
				// Same key, same body — check if tags differ
				tagsChanged := !tagsEqual(e.Tags, tags)
				if tagsChanged {
					// Update tags only
					now := time.Now()
					for i, entry := range idx.Entries {
						if entry.Key == key {
							idx.Entries[i].Tags = tags
							idx.Entries[i].UpdatedAt = now
							break
						}
					}
					content := renderDocument(key, tags, body)
					if writeErr := os.WriteFile(entryPath(scope, projectPath, key), []byte(content), 0644); writeErr != nil {
						return false, writeErr
					}
					return false, saveIndex(scope, projectPath, idx)
				}
				// Same key, same body, same tags — skip entirely
				return true, nil
			}
			// Different key, same body — warn
			fmt.Fprintf(os.Stderr, "warning: duplicate content (matches %q)\n", e.Key)
		}
	}

	return false, Put(scope, projectPath, key, body, tags)
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

// Spec: S-010 | Req: B-009, B-010
func SearchWithOptions(scope config.Scope, projectPath, query string, opts SearchOptions) ([]SearchResult, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(query)
	var results []SearchResult

	now := time.Now()

	for _, e := range idx.Entries {
		// Apply filters
		if opts.Tag != "" {
			hasTag := false
			for _, t := range e.Tags {
				if t == opts.Tag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		if opts.TypeTag != "" {
			typeTag := "type:" + opts.TypeTag
			hasType := false
			for _, t := range e.Tags {
				if t == typeTag {
					hasType = true
					break
				}
			}
			if !hasType {
				continue
			}
		}

		if opts.Since > 0 {
			cutoff := now.Add(-opts.Since)
			if e.UpdatedAt.Before(cutoff) {
				continue
			}
		}

		// Check match + compute rank
		lowerKey := strings.ToLower(e.Key)
		rank := -1

		if lowerKey == lowerQuery {
			rank = 0 // exact key match
		} else if strings.Contains(lowerKey, lowerQuery) {
			rank = 1 // key contains
		} else {
			data, readErr := os.ReadFile(entryPath(scope, projectPath, e.Key))
			if readErr != nil {
				continue
			}
			content := strings.ToLower(string(data))
			if strings.Contains(content, lowerQuery) {
				rank = 2 // body contains
			}
		}

		if rank >= 0 {
			snippet := ""
			if rank <= 1 {
				// For key matches, read body for snippet
				data, readErr := os.ReadFile(entryPath(scope, projectPath, e.Key))
				if readErr == nil {
					snippet = extractSnippet(string(data), query)
				}
			} else {
				data, _ := os.ReadFile(entryPath(scope, projectPath, e.Key))
				snippet = extractSnippet(string(data), query)
			}

			results = append(results, SearchResult{
				Entry:   e,
				Snippet: snippet,
				Rank:    rank,
			})
		}
	}

	// Sort results
	if opts.Sort == "recent" {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Entry.UpdatedAt.After(results[j].Entry.UpdatedAt)
		})
	} else {
		// Default: relevance (by rank, then by UpdatedAt within same rank)
		sort.Slice(results, func(i, j int) bool {
			if results[i].Rank != results[j].Rank {
				return results[i].Rank < results[j].Rank
			}
			return results[i].Entry.UpdatedAt.After(results[j].Entry.UpdatedAt)
		})
	}

	return results, nil
}

// Spec: S-010 | Req: B-011
type TimelineDay struct {
	Date    time.Time
	Entries []Entry
}

func Timeline(scope config.Scope, projectPath string, days int) ([]TimelineDay, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	// Group by day
	dayMap := make(map[string][]Entry)
	for _, e := range idx.Entries {
		if e.UpdatedAt.Before(cutoff) {
			continue
		}
		dayKey := e.UpdatedAt.Format("2006-01-02")
		dayMap[dayKey] = append(dayMap[dayKey], e)
	}

	// Sort days descending
	var dayKeys []string
	for k := range dayMap {
		dayKeys = append(dayKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dayKeys)))

	var result []TimelineDay
	for _, k := range dayKeys {
		t, _ := time.Parse("2006-01-02", k)
		entries := dayMap[k]
		// Sort entries within day by UpdatedAt desc
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
		})
		result = append(result, TimelineDay{Date: t, Entries: entries})
	}

	return result, nil
}

// Spec: S-010 | Req: B-012
func StatsDetailed(scope config.Scope, projectPath string) (StatsResult, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return StatsResult{}, err
	}

	result := StatsResult{
		PerEntry: make(map[string]int),
	}
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	for _, e := range idx.Entries {
		result.Total++
		if e.Enabled {
			result.Enabled++
		}
		if !e.LastReferenced.IsZero() && e.LastReferenced.Before(thirtyDaysAgo) {
			result.Stale++
		} else if e.LastReferenced.IsZero() && e.CreatedAt.Before(thirtyDaysAgo) {
			result.Stale++
		}

		// Token estimation: chars/4
		body, readErr := readBody(scope, projectPath, e.Key)
		if readErr != nil {
			continue
		}
		tokens := len(body) / 4
		result.PerEntry[e.Key] = tokens
		result.TotalTokens += tokens
	}

	return result, nil
}

// Spec: S-010 | Req: B-014
func Compact(scope config.Scope, projectPath string) ([]CompactEntry, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return nil, err
	}

	var entries []CompactEntry
	for _, e := range idx.Entries {
		body, readErr := readBody(scope, projectPath, e.Key)
		firstLine := ""
		if readErr == nil && body != "" {
			lines := strings.SplitN(body, "\n", 2)
			firstLine = strings.TrimSpace(lines[0])
			if len(firstLine) > 80 {
				firstLine = firstLine[:80] + "..."
			}
		}
		entries = append(entries, CompactEntry{
			Key:       e.Key,
			Tags:      e.Tags,
			FirstLine: firstLine,
			UpdatedAt: e.UpdatedAt,
		})
	}

	// Sort by UpdatedAt desc
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
	})

	return entries, nil
}
