package lifecycle

import (
	"testing"

	"github.com/chichex/cvm/internal/automation"
)

func TestEndDoesNotQueueRetro(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	t.Setenv("HOME", home)

	if err := End(project); err != nil {
		t.Fatalf("End() error = %v", err)
	}

	state, err := automation.Load()
	if err != nil {
		t.Fatalf("loading automation state: %v", err)
	}
	if state.PendingCount() != 0 {
		t.Fatalf("expected no pending automation, got %d", state.PendingCount())
	}
}
