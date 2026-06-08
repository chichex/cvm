package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/harness"
)

// writeProfileSource lays out a minimal claude profile source repo under
// ~/.cvm/profiles/<name> with a single managed item.
func writeProfileSource(t *testing.T, name, item, content string) string {
	t.Helper()
	dir := ProfileDir(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir profile source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, item), []byte(content), 0644); err != nil {
		t.Fatalf("write profile item: %v", err)
	}
	return dir
}

func TestUseSymlinksManagedItems(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := writeProfileSource(t, "work", "CLAUDE.md", "from profile")

	if err := UseWithHarness("work", harness.Claude()); err != nil {
		t.Fatalf("use: %v", err)
	}

	tgt := filepath.Join(home, config.ClaudeDirName, "CLAUDE.md")
	info, err := os.Lstat(tgt)
	if err != nil {
		t.Fatalf("lstat target: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink", tgt)
	}
	resolved, err := filepath.EvalSymlinks(tgt)
	if err != nil {
		t.Fatalf("eval symlink: %v", err)
	}
	wantSrc, _ := filepath.EvalSymlinks(filepath.Join(src, "CLAUDE.md"))
	if resolved != wantSrc {
		t.Fatalf("symlink points to %q, want %q", resolved, wantSrc)
	}

	// Editing through the symlink writes into the source repo.
	if err := os.WriteFile(tgt, []byte("edited live"), 0644); err != nil {
		t.Fatalf("write through symlink: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(src, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(got) != "edited live" {
		t.Fatalf("source not updated through symlink: %q", string(got))
	}
}

func TestVanillaStashRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Pre-existing real vanilla file the user already had.
	claudeDir := filepath.Join(home, config.ClaudeDirName)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir claude dir: %v", err)
	}
	vanillaPath := filepath.Join(claudeDir, "CLAUDE.md")
	if err := os.WriteFile(vanillaPath, []byte("vanilla"), 0644); err != nil {
		t.Fatalf("write vanilla: %v", err)
	}

	writeProfileSource(t, "work", "CLAUDE.md", "from profile")

	if err := UseWithHarness("work", harness.Claude()); err != nil {
		t.Fatalf("use: %v", err)
	}

	// Vanilla file was stashed exactly once.
	stash := filepath.Join(config.VanillaDirForHarness("claude"), "CLAUDE.md")
	if data, err := os.ReadFile(stash); err != nil || string(data) != "vanilla" {
		t.Fatalf("expected stashed vanilla, got data=%q err=%v", string(data), err)
	}

	// off restores the stash and drops the symlink.
	if err := UseNoneWithHarness(harness.Claude()); err != nil {
		t.Fatalf("off: %v", err)
	}
	info, err := os.Lstat(vanillaPath)
	if err != nil {
		t.Fatalf("lstat restored: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected restored real file, got symlink")
	}
	if data, _ := os.ReadFile(vanillaPath); string(data) != "vanilla" {
		t.Fatalf("vanilla not restored: %q", string(data))
	}
	if _, err := os.Lstat(stash); !os.IsNotExist(err) {
		t.Fatalf("expected stash to be consumed, err=%v", err)
	}
}
