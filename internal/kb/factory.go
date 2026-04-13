// Spec: S-013 | Req: I-002
// NewBackend factory: selects the backend based on CVM_KB_BACKEND env var.
package kb

import (
	"fmt"
	"os"

	"github.com/chichex/cvm/internal/config"
)

// NewBackend creates the appropriate KB backend based on CVM_KB_BACKEND env var.
// - "flat"         → FlatBackend (flat-file storage)
// - "sqlite" or "" → SQLiteBackend (default; falls back to FlatBackend on init error)
// - anything else  → error
// Spec: S-013 | Req: I-002
func NewBackend(scope config.Scope, projectPath string) (Backend, error) {
	backendEnv := os.Getenv("CVM_KB_BACKEND")

	switch backendEnv {
	case "flat":
		// Spec: S-013 | Req: I-002b
		return NewFlatBackend(scope, projectPath), nil

	case "sqlite", "":
		// Spec: S-013 | Req: I-002c — default is SQLite
		b, err := NewSQLiteBackend(scope, projectPath)
		if err != nil {
			// Spec: S-013 | Req: I-002d — fallback to flat on init failure
			fmt.Fprintf(os.Stderr, "[cvm] warning: SQLite backend failed (%v), falling back to flat files\n", err)
			return NewFlatBackend(scope, projectPath), nil
		}
		return b, nil

	default:
		// Spec: S-013 | Req: I-002e
		return nil, fmt.Errorf("unknown backend %q: valid values are \"sqlite\" and \"flat\"", backendEnv)
	}
}
