// Spec: S-013 | Req: I-001
// Backend interface abstracting flat-file and SQLite KB storage.
package kb

import "time"

// Backend abstracts all KB storage operations. Both FlatBackend and SQLiteBackend
// implement this interface, making the storage layer transparent to callers.
// Spec: S-013 | Req: I-001
type Backend interface {
	// Put inserts or updates an entry. now is injectable for deterministic tests.
	Put(key, body string, tags []string, now time.Time) error

	// Get returns the Document for the given key, or error if not found.
	Get(key string) (Document, error)

	// List returns all entries optionally filtered by tag. Empty tag = all entries.
	List(tag string) ([]Entry, error)

	// Remove deletes the entry with the given key. Returns error if not found.
	Remove(key string) error

	// Search performs full-text search and returns ranked results.
	Search(query string, opts SearchOptions) ([]SearchResult, error)

	// Timeline returns entries grouped by day for the last N days, descending.
	Timeline(days int) ([]TimelineDay, error)

	// Stats returns aggregate statistics.
	Stats() (StatsResult, error)

	// Compact returns a condensed view of all entries, sorted by UpdatedAt desc.
	Compact() ([]CompactEntry, error)

	// SetEnabled toggles the enabled flag for an entry.
	SetEnabled(key string, enabled bool) error

	// LoadDocuments returns all Documents (entry metadata + body).
	LoadDocuments() ([]Document, error)

	// SaveDocument upserts a Document, preserving created_at on update.
	SaveDocument(doc Document) error

	// Close releases any held resources. Must be idempotent.
	Close() error
}
