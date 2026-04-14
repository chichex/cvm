// Spec: S-013 | Req: I-001a
// FlatBackend implements Backend using the existing flat-file (.index.json + entries/*.md) storage.
// It uses internal helpers from kb.go directly — never calls public package-level functions
// to avoid infinite recursion (public functions → NewBackend() → FlatBackend → public function).
package kb

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chichex/cvm/internal/config"
)

// FlatBackend is the flat-file KB backend. It satisfies the Backend interface by
// operating directly on .index.json + entries/*.md via internal helpers.
// Spec: S-013 | Req: I-001a
type FlatBackend struct {
	scope       config.Scope
	projectPath string
}

// NewFlatBackend creates a new FlatBackend for the given scope and projectPath.
func NewFlatBackend(scope config.Scope, projectPath string) *FlatBackend {
	return &FlatBackend{scope: scope, projectPath: projectPath}
}

// Put inserts or updates an entry. The now parameter is used as the timestamp.
// sessionID is accepted for interface compatibility but silently ignored — flat files have no session_id column.
// Spec: S-013 | Req: I-001b | Spec: S-017 | Req: C-010
func (f *FlatBackend) Put(key, body string, tags []string, now time.Time, sessionID string) error {
	return putWithTime(f.scope, f.projectPath, key, body, tags, now)
}

// Get returns the Document for the given key.
func (f *FlatBackend) Get(key string) (Document, error) {
	body, err := readBody(f.scope, f.projectPath, key)
	if err != nil {
		return Document{}, err
	}
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return Document{}, err
	}
	for _, e := range idx.Entries {
		if e.Key == key {
			return Document{Entry: e, Body: body}, nil
		}
	}
	return Document{}, errNotFound(key)
}

// List returns all entries, optionally filtered by tag.
// Spec: S-013 | Req: I-001a
func (f *FlatBackend) List(tag string) ([]Entry, error) {
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return nil, err
	}
	if tag == "" {
		return idx.Entries, nil
	}
	var result []Entry
	for _, e := range idx.Entries {
		for _, t := range e.Tags {
			if t == tag {
				result = append(result, e)
				break
			}
		}
	}
	return result, nil
}

// Remove deletes the entry with the given key.
// Spec: S-013 | Req: I-001a
func (f *FlatBackend) Remove(key string) error {
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return err
	}
	found := false
	var remaining []Entry
	for _, e := range idx.Entries {
		if e.Key == key {
			found = true
		} else {
			remaining = append(remaining, e)
		}
	}
	if !found {
		return errNotFound(key)
	}
	os.Remove(entryPath(f.scope, f.projectPath, key))
	idx.Entries = remaining
	return saveIndex(f.scope, f.projectPath, idx)
}

// Search performs full-text search.
// Spec: S-013 | Req: I-001a
func (f *FlatBackend) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(strings.TrimSpace(query))

	// Empty query: return all entries (optionally filtered by tag), sorted by UpdatedAt desc
	if lowerQuery == "" {
		var results []SearchResult
		for _, e := range idx.Entries {
			if opts.Tag != "" {
				found := false
				for _, t := range e.Tags {
					if t == opts.Tag {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			results = append(results, SearchResult{Entry: e, Snippet: "", Rank: 2})
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].Entry.UpdatedAt.After(results[j].Entry.UpdatedAt)
		})
		return results, nil
	}

	var cutoff time.Time
	if opts.Since > 0 {
		cutoff = time.Now().Add(-opts.Since)
	}

	var results []SearchResult
	for _, e := range idx.Entries {
		// Tag filter
		if opts.Tag != "" {
			found := false
			for _, t := range e.Tags {
				if t == opts.Tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		// TypeTag filter
		if opts.TypeTag != "" {
			found := false
			for _, t := range e.Tags {
				if t == "type:"+opts.TypeTag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		// Since filter
		if !cutoff.IsZero() && !e.UpdatedAt.After(cutoff) {
			continue
		}

		lowerKey := strings.ToLower(e.Key)

		// Check key match
		keyMatch := strings.Contains(lowerKey, lowerQuery)

		// Read body for content match
		body, readErr := readBody(f.scope, f.projectPath, e.Key)
		bodyMatch := readErr == nil && strings.Contains(strings.ToLower(body), lowerQuery)

		if !keyMatch && !bodyMatch {
			continue
		}

		// Rank: 0=exact key, 1=key contains, 2=body/tags match
		var rankCat int
		if lowerKey == lowerQuery {
			rankCat = 0
		} else if keyMatch {
			rankCat = 1
		} else {
			rankCat = 2
		}

		snippet := ""
		if readErr == nil {
			snippet = extractSnippet(body, query)
		}

		results = append(results, SearchResult{
			Entry:   e,
			Snippet: snippet,
			Rank:    rankCat,
		})
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

// Timeline returns entries grouped by day.
// Spec: S-013 | Req: I-001a
func (f *FlatBackend) Timeline(days int) ([]TimelineDay, error) {
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	// Group by day
	dayMap := make(map[string][]Entry)
	for _, e := range idx.Entries {
		if !e.UpdatedAt.IsZero() && e.UpdatedAt.Before(cutoff) {
			continue
		}
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
// Spec: S-013 | Req: I-001a
func (f *FlatBackend) Stats() (StatsResult, error) {
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return StatsResult{}, err
	}

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	total := len(idx.Entries)
	enabled := 0
	stale := 0
	perEntry := make(map[string]int)
	totalTokens := 0

	for _, e := range idx.Entries {
		if e.Enabled {
			enabled++
		}
		// Stale: last_referenced older than 30 days (or if zero, created_at older than 30 days)
		if !e.LastReferenced.IsZero() {
			if e.LastReferenced.Before(thirtyDaysAgo) {
				stale++
			}
		} else if e.CreatedAt.Before(thirtyDaysAgo) {
			stale++
		}

		body, readErr := readBody(f.scope, f.projectPath, e.Key)
		if readErr == nil {
			tokens := len(body) / 4
			perEntry[e.Key] = tokens
			totalTokens += tokens
		}
	}

	return StatsResult{
		Total:       total,
		Enabled:     enabled,
		Stale:       stale,
		TotalTokens: totalTokens,
		PerEntry:    perEntry,
	}, nil
}

// Compact returns a condensed view of all entries.
// Spec: S-013 | Req: I-001a
func (f *FlatBackend) Compact() ([]CompactEntry, error) {
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return nil, err
	}

	entries := make([]CompactEntry, 0, len(idx.Entries))
	for _, e := range idx.Entries {
		body, _ := readBody(f.scope, f.projectPath, e.Key)
		firstLine := ""
		if body != "" {
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

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
	})

	return entries, nil
}

// SetEnabled toggles the enabled flag.
// Spec: S-013 | Req: I-001a
func (f *FlatBackend) SetEnabled(key string, enabled bool) error {
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return err
	}
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries[i].Enabled = enabled
			idx.Entries[i].UpdatedAt = time.Now()
			return saveIndex(f.scope, f.projectPath, idx)
		}
	}
	return errNotFound(key)
}

// LoadDocuments returns all Documents.
// Spec: S-013 | Req: I-001a
func (f *FlatBackend) LoadDocuments() ([]Document, error) {
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return nil, err
	}
	docs := make([]Document, 0, len(idx.Entries))
	for _, e := range idx.Entries {
		body, readErr := readBody(f.scope, f.projectPath, e.Key)
		if readErr != nil {
			// Skip entries whose file is missing
			continue
		}
		docs = append(docs, Document{Entry: e, Body: body})
	}
	return docs, nil
}

// SaveDocument upserts a Document.
// Spec: S-013 | Req: I-001a
func (f *FlatBackend) SaveDocument(doc Document) error {
	dir := entriesDir(f.scope, f.projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return err
	}

	found := false
	for i, e := range idx.Entries {
		if e.Key == doc.Entry.Key {
			// Preserve created_at on update
			idx.Entries[i].Tags = doc.Entry.Tags
			idx.Entries[i].Enabled = doc.Entry.Enabled
			idx.Entries[i].UpdatedAt = doc.Entry.UpdatedAt
			idx.Entries[i].LastReferenced = doc.Entry.LastReferenced
			found = true
			break
		}
	}
	if !found {
		idx.Entries = append(idx.Entries, doc.Entry)
	}

	content := renderDocument(doc.Entry.Key, doc.Entry.Tags, doc.Body)
	if err := os.WriteFile(entryPath(f.scope, f.projectPath, doc.Entry.Key), []byte(content), 0644); err != nil {
		return err
	}

	return saveIndex(f.scope, f.projectPath, idx)
}

// Show returns the body of the entry (without frontmatter) and updates LastReferenced.
// Matches SQLiteBackend.Show() behavior: returns body only, no frontmatter.
// Spec: S-013 | Fix: Backend wiring
func (f *FlatBackend) Show(key string) (string, error) {
	body, err := readBody(f.scope, f.projectPath, key)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("entry %q not found", key)
		}
		return "", err
	}
	idx, _ := loadIndex(f.scope, f.projectPath)
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries[i].LastReferenced = time.Now()
			saveIndex(f.scope, f.projectPath, idx)
			break
		}
	}
	return body, nil
}

// Clean removes all entries and returns the count removed.
// Spec: S-013 | Fix: Backend wiring
func (f *FlatBackend) Clean() (int, error) {
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return 0, err
	}
	count := len(idx.Entries)
	if count == 0 {
		return 0, nil
	}
	for _, e := range idx.Entries {
		os.Remove(entryPath(f.scope, f.projectPath, e.Key))
	}
	idx.Entries = nil
	if err := saveIndex(f.scope, f.projectPath, idx); err != nil {
		return 0, err
	}
	return count, nil
}

// PutWithDedup inserts or updates only if the content hash differs.
// Spec: S-013 | Fix: Backend wiring
func (f *FlatBackend) PutWithDedup(key, body string, tags []string, now time.Time) (skipped bool, err error) {
	dir := entriesDir(f.scope, f.projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, err
	}
	idx, err := loadIndex(f.scope, f.projectPath)
	if err != nil {
		return false, err
	}
	newHash := bodyHash(body)
	for _, e := range idx.Entries {
		existingBody, readErr := readBody(f.scope, f.projectPath, e.Key)
		if readErr != nil {
			continue
		}
		if bodyHash(existingBody) == newHash {
			if e.Key == key {
				tagsChanged := !tagsEqual(e.Tags, tags)
				if tagsChanged {
					for i, entry := range idx.Entries {
						if entry.Key == key {
							idx.Entries[i].Tags = tags
							idx.Entries[i].UpdatedAt = now
							break
						}
					}
					content := renderDocument(key, tags, body)
					if writeErr := os.WriteFile(entryPath(f.scope, f.projectPath, key), []byte(content), 0644); writeErr != nil {
						return false, writeErr
					}
					return false, saveIndex(f.scope, f.projectPath, idx)
				}
				return true, nil
			}
			fmt.Fprintf(os.Stderr, "warning: duplicate content (matches %q)\n", e.Key)
		}
	}
	// No duplicate — delegate to Put
	return false, f.Put(key, body, tags, now, "")
}

// Close is a no-op for the flat-file backend.
// Spec: S-013 | Req: I-001c
func (f *FlatBackend) Close() error {
	return nil
}
