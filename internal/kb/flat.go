// Spec: S-013 | Req: I-001a
// FlatBackend implements Backend using the existing flat-file (.index.json + entries/*.md) storage.
// It delegates to the existing package-level functions in kb.go.
package kb

import (
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

// Close is a no-op for the flat-file backend.
// Spec: S-013 | Req: I-001c
func (f *FlatBackend) Close() error {
	return nil
}
