package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chichex/cvm/internal/harness"
)

func TestLoadManifestDefaultsToClaudeRoot(t *testing.T) {
	dir := t.TempDir()

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if !manifest.SupportsHarness("claude") {
		t.Fatal("default manifest should support claude")
	}

	assetDir, err := manifest.AssetDir(dir, harness.Claude())
	if err != nil {
		t.Fatalf("AssetDir: %v", err)
	}
	if assetDir != dir {
		t.Fatalf("unexpected asset dir: got %q want %q", assetDir, dir)
	}
}

func TestLoadManifestSupportsHarnessSpecificAssetDir(t *testing.T) {
	dir := t.TempDir()
	body := []byte("name = \"lite\"\nharnesses = [\"claude\", \"codex\"]\n\n[assets]\nclaude = \"claude\"\ncodex = \"codex\"\n")
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), body, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if !manifest.SupportsHarness("codex") {
		t.Fatal("manifest should support codex")
	}

	assetDir, err := manifest.AssetDir(dir, harness.Claude())
	if err != nil {
		t.Fatalf("AssetDir: %v", err)
	}
	want := filepath.Join(dir, "claude")
	if assetDir != want {
		t.Fatalf("unexpected asset dir: got %q want %q", assetDir, want)
	}
}

func TestManifestRejectsEscapingAssetDir(t *testing.T) {
	dir := t.TempDir()
	body := []byte("harnesses = [\"claude\"]\n\n[assets]\nclaude = \"../escape\"\n")
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), body, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if _, err := manifest.AssetDir(dir, harness.Claude()); err == nil {
		t.Fatal("expected AssetDir to reject escaping path")
	}
}
