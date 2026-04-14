// Spec: S-013 | Req: I-001
// Backend interface abstracting flat-file and SQLite KB storage.
package kb

import "time"

// Backend abstracts all KB storage operations. Both FlatBackend and SQLiteBackend
// implement this interface, making the storage layer transparent to callers.
// Spec: S-013 | Req: I-001
type Backend interface {
	// Put inserts or updates an entry. now is injectable for deterministic tests.
	// sessionID links the entry to a session; empty string means no session link.
	// Spec: S-017 | Req: C-010
	Put(key, body string, tags []string, now time.Time, sessionID string) error

	// Get returns the Document for the given key, or error if not found.
	Get(key string) (Document, error)

	// Show returns the full rendered content of the entry and updates LastReferenced.
	// Spec: S-013 | Fix: Backend wiring
	Show(key string) (string, error)

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

	// Clean removes all entries and returns the count removed.
	// Spec: S-013 | Fix: Backend wiring
	Clean() (int, error)

	// PutWithDedup inserts or updates an entry only if the content hash differs.
	// Returns skipped=true if content and tags are identical to the existing entry.
	// Spec: S-013 | Fix: Backend wiring
	PutWithDedup(key, body string, tags []string, now time.Time) (skipped bool, err error)

	// Close releases any held resources. Must be idempotent.
	Close() error
}
