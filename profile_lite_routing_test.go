package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests guard the documented routing contract of the lite profile's
// subagent skills (/o, /c, /g). They are structural assertions against the
// SKILL.md files — the files are interpreted by Claude at runtime, so we
// cannot run the behavior end-to-end, but regressions in the contract show
// up as missing strings in the markdown.

func readLiteSkill(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("profiles", "lite", "skills", name, "SKILL.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func assertAll(t *testing.T, skill, body string, needles []string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(body, n) {
			t.Errorf("skill %s: missing required routing clause %q", skill, n)
		}
	}
}

func TestLiteRouting_OIsUnifiedWithOpusDefault(t *testing.T) {
	body := readLiteSkill(t, "o")
	assertAll(t, "/o", body, []string{
		"--codex",
		"--gemini",
		"--opus",
		"default",
		"Opus",
	})
	// /o must reject more than one provider flag per invocation.
	if !strings.Contains(body, "mas de uno") && !strings.Contains(body, "No aceptar mas de un flag") {
		t.Errorf("/o: missing rejection clause for multiple provider flags")
	}
}

func TestLiteRouting_CDeprecatedShimDelegatesToCodex(t *testing.T) {
	body := readLiteSkill(t, "c")
	assertAll(t, "/c", body, []string{
		"DEPRECATED",
		"--codex",
		"/o",
	})
	// /c must sanitize pre-existing provider flags to avoid the conflict
	// guard in /o when the user passes a different provider flag.
	if !strings.Contains(body, "Sanear") && !strings.Contains(body, "sanear") {
		t.Errorf("/c: missing sanitization step for pre-existing provider flags")
	}
	for _, flag := range []string{"--codex", "--gemini", "--opus"} {
		if !strings.Contains(body, flag) {
			t.Errorf("/c: sanitization must mention flag %s", flag)
		}
	}
}

func TestLiteRouting_GDeprecatedShimDelegatesToGemini(t *testing.T) {
	body := readLiteSkill(t, "g")
	assertAll(t, "/g", body, []string{
		"DEPRECATED",
		"--gemini",
		"/o",
	})
	if !strings.Contains(body, "Sanear") && !strings.Contains(body, "sanear") {
		t.Errorf("/g: missing sanitization step for pre-existing provider flags")
	}
	for _, flag := range []string{"--codex", "--gemini", "--opus"} {
		if !strings.Contains(body, flag) {
			t.Errorf("/g: sanitization must mention flag %s", flag)
		}
	}
}

func TestLiteRouting_CLAUDEmdDocumentsUnifiedRouting(t *testing.T) {
	path := filepath.Join("profiles", "lite", "CLAUDE.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(b)
	for _, needle := range []string{
		"`/o`",
		"--codex",
		"--gemini",
		"DEPRECATED",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("CLAUDE.md: missing routing doc clause %q", needle)
		}
	}
}
