// Spec: S-013 | Req: I-001a
// FlatBackend implements Backend using the existing flat-file (.index.json + entries/*.md) storage.
// It delegates to the existing package-level functions in kb.go.
package kb

import (
	"fmt"
	"os"
	"time"

	"github.com/chichex/cvm/internal/config"
)

// FlatBackend is the flat-file KB backend. It satisfies the Backend interface by
// delegating to the existing package-level functions that operate on .index.json + entries/*.md.
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
// Spec: S-013 | Req: I-001b
func (f *FlatBackend) Put(key, body string, tags []string, now time.Time) error {
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
func (f *FlatBackend) List(tag string) ([]Entry, error) {
	return List(f.scope, f.projectPath, tag)
}

// Remove deletes the entry with the given key.
func (f *FlatBackend) Remove(key string) error {
	return Remove(f.scope, f.projectPath, key)
}

// Search performs full-text search.
func (f *FlatBackend) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	return SearchWithOptions(f.scope, f.projectPath, query, opts)
}

// Timeline returns entries grouped by day.
func (f *FlatBackend) Timeline(days int) ([]TimelineDay, error) {
	return Timeline(f.scope, f.projectPath, days)
}

// Stats returns aggregate statistics.
func (f *FlatBackend) Stats() (StatsResult, error) {
	return StatsDetailed(f.scope, f.projectPath)
}

// Compact returns a condensed view of all entries.
func (f *FlatBackend) Compact() ([]CompactEntry, error) {
	return Compact(f.scope, f.projectPath)
}

// SetEnabled toggles the enabled flag.
func (f *FlatBackend) SetEnabled(key string, enabled bool) error {
	return SetEnabled(f.scope, f.projectPath, key, enabled)
}

// LoadDocuments returns all Documents.
func (f *FlatBackend) LoadDocuments() ([]Document, error) {
	return LoadDocuments(f.scope, f.projectPath)
}

// SaveDocument upserts a Document.
func (f *FlatBackend) SaveDocument(doc Document) error {
	return SaveDocument(f.scope, f.projectPath, doc)
}

// Show returns the raw rendered content of the entry (including frontmatter) and updates LastReferenced.
// Spec: S-013 | Fix: Backend wiring
func (f *FlatBackend) Show(key string) (string, error) {
	data, err := os.ReadFile(entryPath(f.scope, f.projectPath, key))
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
	return string(data), nil
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
	return false, f.Put(key, body, tags, now)
}

// Close is a no-op for the flat-file backend.
// Spec: S-013 | Req: I-001c
func (f *FlatBackend) Close() error {
	return nil
}
