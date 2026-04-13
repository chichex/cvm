// Spec: S-013 | Req: B-001 through B-007, E-007, I-012, I-013
// Migration from flat-file KB to SQLite.
package kb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chichex/cvm/internal/config"
)

// migrateFromFlat migrates flat-file KB entries into the SQLite backend.
// It is called by NewSQLiteBackend when kb.db does not exist but .index.json does.
// The migration is atomic: all entries are inserted in a single transaction.
// On failure the transaction is rolled back and the DB is empty (no partial state).
// Flat files are preserved post-migration (Spec: S-013 | Req: I-013).
// Spec: S-013 | Req: B-001, B-003, E-007, I-012, I-013
func migrateFromFlat(sb *SQLiteBackend, scope config.Scope, projectPath string) error {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return fmt.Errorf("migration: load index: %w", err)
	}

	// Spec: S-013 | Req: B-002 — 0 entries: create DB but don't print migration messages
	if len(idx.Entries) == 0 {
		return nil
	}

	// Spec: S-013 | Req: B-001 — print migration start message
	fmt.Fprintf(os.Stderr, "[cvm] migrating KB to SQLite...\n")

	tx, err := sb.db.Begin()
	if err != nil {
		return fmt.Errorf("migration: begin transaction: %w", err)
	}

	imported := 0
	for _, entry := range idx.Entries {
		// Read body from flat file using the key as literal filename stem.
		// Spec: S-013 | Req: E-007 — missing file: use empty body with warning
		body, readErr := readBodyForMigration(scope, projectPath, entry.Key)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "[cvm] migration warning: body not found for key %s, importing with empty body\n", entry.Key)
			body = ""
		}

		tagsData, jsonErr := json.Marshal(nilSafeTags(entry.Tags))
		if jsonErr != nil {
			tx.Rollback()
			return fmt.Errorf("migration: marshal tags for %q: %w", entry.Key, jsonErr)
		}

		enabledInt := 0
		if entry.Enabled {
			enabledInt = 1
		}

		createdStr := entry.CreatedAt.UTC().Format(time.RFC3339Nano)
		updatedStr := entry.UpdatedAt.UTC().Format(time.RFC3339Nano)
		var lastRefStr *string
		if !entry.LastReferenced.IsZero() {
			s := entry.LastReferenced.UTC().Format(time.RFC3339Nano)
			lastRefStr = &s
		}

		_, execErr := tx.Exec(`
			INSERT INTO entries (key, body, tags, enabled, created_at, updated_at, last_referenced)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(key) DO NOTHING
		`, entry.Key, body, string(tagsData), enabledInt, createdStr, updatedStr, lastRefStr)
		if execErr != nil {
			tx.Rollback()
			return fmt.Errorf("migration: insert %q: %w", entry.Key, execErr)
		}
		imported++
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return fmt.Errorf("migration: commit: %w", err)
	}

	// Spec: S-013 | Req: B-001 — print completion message
	fmt.Fprintf(os.Stderr, "[cvm] migration complete: %d entries imported\n", imported)
	return nil
}

// readBodyForMigration reads the body from the flat-file for a given key.
// It uses the key literally as the filename stem (special chars like '/' are not treated
// as path separators — we join them directly so the path may not exist, which is OK).
// Spec: S-013 | Req: B-004, E-007
func readBodyForMigration(scope config.Scope, projectPath, key string) (string, error) {
	// Use filepath.Join so the key is used as-is (without interpreting slashes as dirs)
	// We build the path manually to avoid filepath.Join collapsing the key
	path := kbDir(scope, projectPath) + "/entries/" + key + ".md"
	data, err := os.ReadFile(path)
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

// nilSafeTags returns an empty slice if tags is nil.
func nilSafeTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

// flatIndexPath returns the path to the .index.json file for migration detection.
func flatIndexPath(scope config.Scope, projectPath string) string {
	return filepath.Join(kbDir(scope, projectPath), ".index.json")
}
