package harness

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Codex bypass keys, sourced from
// https://developers.openai.com/codex/config-reference. Together they put
// Codex into "no approvals + unrestricted sandbox" mode — the closest
// equivalent to Claude's bypassPermissions / OpenCode's permission=allow.
const (
	codexConfigFile        = "config.toml"
	codexApprovalKey       = "approval_policy"
	codexApprovalBypass    = "never"
	codexSandboxKey        = "sandbox_mode"
	codexSandboxBypass     = "danger-full-access"
	codexBypassStatusValue = "danger-full-access/never"
)

type codexHarness struct{}

var managedCodexDirItems = []string{
	"AGENTS.md",
}

func Codex() Harness {
	return codexHarness{}
}

func (codexHarness) Name() string {
	return "codex"
}

func (codexHarness) TargetDir() string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

func (h codexHarness) DefaultAssetDir(profileDir string) string {
	return h.Name()
}

func (h codexHarness) ScaffoldAsset(kind, name string) (ScaffoldAsset, error) {
	switch kind {
	case "instructions":
		return ScaffoldAsset{ProfilePath: h.MarkdownInstructionsFile(), Content: "# Profile Instructions\n\n", Mode: 0644}, nil
	default:
		return ScaffoldAsset{}, fmt.Errorf("codex does not support %s scaffolding", kind)
	}
}

func (codexHarness) ManagedDirItems() []string {
	return append([]string{}, managedCodexDirItems...)
}

func (codexHarness) ExternalManagedPath() (ManagedPath, bool) {
	return ManagedPath{}, false
}

func (h codexHarness) ProfileDiscoveryItems() []string {
	return h.ManagedDirItems()
}

func (codexHarness) MarkdownInstructionsFile() string {
	return "AGENTS.md"
}

func (codexHarness) SupportsPortableSkills() bool {
	return false
}

func (codexHarness) SupportsPortableAgents() bool {
	return false
}

func (codexHarness) IsUserMCPPath(profilePath string) bool {
	// Codex MCP support is not managed by cvm yet; config.toml stays user-owned
	// and outside ManagedDirItems until cvm has explicit TOML merge semantics.
	return false
}

func (codexHarness) IsMCPPath(profilePath string) bool {
	// Revisit this if Codex gains a managed MCP asset so additive merge rules apply.
	return false
}

// EnableBypass writes Codex's bypass keys directly to the live config.toml.
// config.toml is not part of the managed item set (cvm has no TOML merge
// semantics yet), so the override mechanism is bypassed and we mutate the
// live file in place — preserving any unrelated keys/sections the user has.
//
// profileName is accepted for interface symmetry but unused: Codex bypass
// state lives on the live config, not in per-profile overrides.
func (h codexHarness) EnableBypass(_ string) error {
	path := filepath.Join(h.TargetDir(), codexConfigFile)
	return setCodexTopLevelKeys(path, map[string]string{
		codexApprovalKey: quoteTOMLString(codexApprovalBypass),
		codexSandboxKey:  quoteTOMLString(codexSandboxBypass),
	})
}

// DisableBypass strips the two bypass keys from the live config.toml.
func (h codexHarness) DisableBypass(_ string) error {
	path := filepath.Join(h.TargetDir(), codexConfigFile)
	return removeCodexTopLevelKeys(path, []string{codexApprovalKey, codexSandboxKey})
}

// BypassStatus returns a non-empty status string if both bypass keys are
// present with their bypass values; otherwise "".
func (h codexHarness) BypassStatus(_ string) (string, error) {
	path := filepath.Join(h.TargetDir(), codexConfigFile)
	values, err := readCodexTopLevelKeys(path, []string{codexApprovalKey, codexSandboxKey})
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if values[codexApprovalKey] == codexApprovalBypass && values[codexSandboxKey] == codexSandboxBypass {
		return codexBypassStatusValue, nil
	}
	return "", nil
}

// --- minimal TOML helpers ---
//
// cvm only needs to set/get a handful of top-level scalar keys in
// config.toml. Pulling in BurntSushi/toml just for that would be overkill,
// so we operate line-by-line and only touch the top-level table (i.e. the
// region before the first [section] header). All other content — comments,
// whitespace, sub-tables — is preserved verbatim.

// setCodexTopLevelKeys ensures each key in `pairs` is set to the given raw
// TOML value within the top-level table of `path`. Existing top-level keys
// are replaced in place; missing ones are appended just before the first
// section header (or at end-of-file). Values must already be TOML-encoded
// (e.g. quoted strings).
func setCodexTopLevelKeys(path string, pairs map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	lines := splitLines(data)
	remaining := map[string]string{}
	for k, v := range pairs {
		remaining[k] = v
	}

	sectionStart := -1 // index of first line that opens a section header

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if sectionStart == -1 && isTOMLSectionHeader(trimmed) {
			sectionStart = i
			break
		}
		key, ok := topLevelKeyName(trimmed)
		if !ok {
			continue
		}
		if val, want := remaining[key]; want {
			lines[i] = fmt.Sprintf("%s = %s", key, val)
			delete(remaining, key)
		}
	}

	if len(remaining) > 0 {
		// Append in deterministic order so test output is stable.
		ordered := orderedKeys(remaining)
		insert := make([]string, 0, len(ordered))
		for _, k := range ordered {
			insert = append(insert, fmt.Sprintf("%s = %s", k, remaining[k]))
		}

		if sectionStart == -1 {
			// No sections — append at end, ensuring a trailing newline.
			lines = append(lines, insert...)
		} else {
			// Insert just before the section header, with a separating blank
			// line if the preceding line is non-empty.
			prefix := lines[:sectionStart]
			if len(prefix) > 0 && strings.TrimSpace(prefix[len(prefix)-1]) != "" {
				insert = append([]string{""}, insert...)
			}
			insert = append(insert, "")
			combined := append([]string{}, prefix...)
			combined = append(combined, insert...)
			combined = append(combined, lines[sectionStart:]...)
			lines = combined
		}
	}

	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0644)
}

// removeCodexTopLevelKeys removes any top-level lines whose key matches one
// of `keys`. If the file becomes empty (or only whitespace) it is deleted.
func removeCodexTopLevelKeys(path string, keys []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	wanted := map[string]struct{}{}
	for _, k := range keys {
		wanted[k] = struct{}{}
	}

	lines := splitLines(data)
	out := make([]string, 0, len(lines))
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isTOMLSectionHeader(trimmed) {
			inSection = true
			out = append(out, line)
			continue
		}
		if !inSection {
			if key, ok := topLevelKeyName(trimmed); ok {
				if _, drop := wanted[key]; drop {
					continue
				}
			}
		}
		out = append(out, line)
	}

	joined := strings.Join(out, "\n")
	if strings.TrimSpace(joined) == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if !strings.HasSuffix(joined, "\n") {
		joined += "\n"
	}
	return os.WriteFile(path, []byte(joined), 0644)
}

// readCodexTopLevelKeys returns the raw decoded string values for the given
// top-level keys. Keys that aren't present, aren't strings, or live inside a
// section header are simply absent from the result map.
func readCodexTopLevelKeys(path string, keys []string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	wanted := map[string]struct{}{}
	for _, k := range keys {
		wanted[k] = struct{}{}
	}

	out := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if isTOMLSectionHeader(trimmed) {
			break
		}
		key, ok := topLevelKeyName(trimmed)
		if !ok {
			continue
		}
		if _, want := wanted[key]; !want {
			continue
		}
		eq := strings.Index(trimmed, "=")
		if eq < 0 {
			continue
		}
		raw := strings.TrimSpace(trimmed[eq+1:])
		raw = stripInlineComment(raw)
		if v, ok := unquoteTOMLString(raw); ok {
			out[key] = v
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// isTOMLSectionHeader detects a `[section]` or `[[array.of.tables]]` line.
func isTOMLSectionHeader(trimmed string) bool {
	if strings.HasPrefix(trimmed, "#") {
		return false
	}
	return strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
}

// topLevelKeyName returns the key name of a `key = value` line, or false if
// the line is empty, a comment, or doesn't look like an assignment. Quoted
// keys are returned with surrounding quotes stripped.
func topLevelKeyName(trimmed string) (string, bool) {
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	eq := strings.Index(trimmed, "=")
	if eq <= 0 {
		return "", false
	}
	key := strings.TrimSpace(trimmed[:eq])
	if key == "" {
		return "", false
	}
	if strings.ContainsAny(key, ".[]") {
		return "", false // dotted/bracketed keys are out of scope here
	}
	if (strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"")) ||
		(strings.HasPrefix(key, "'") && strings.HasSuffix(key, "'")) {
		key = key[1 : len(key)-1]
	}
	return key, true
}

// quoteTOMLString returns a TOML basic-string literal for the given value.
// It only handles the small subset of escapes we actually emit (none of our
// bypass values contain quotes or backslashes), so the implementation stays
// compact and audit-friendly.
func quoteTOMLString(s string) string {
	return "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(s) + "\""
}

// unquoteTOMLString accepts a basic ("...") or literal ('...') string and
// returns its contents. Returns ok=false for any other shape (numbers,
// arrays, inline tables, multiline strings).
func unquoteTOMLString(raw string) (string, bool) {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		// Reverse the simple escapes we emit.
		inner := raw[1 : len(raw)-1]
		inner = strings.NewReplacer("\\\"", "\"", "\\\\", "\\").Replace(inner)
		return inner, true
	}
	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return raw[1 : len(raw)-1], true
	}
	return "", false
}

// stripInlineComment removes a trailing `# comment` from a TOML value, but
// only when the `#` is outside of any string literal we recognize.
func stripInlineComment(raw string) string {
	inDouble, inSingle := false, false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == '\\' && inDouble && i+1 < len(raw):
			i++ // skip escaped char inside basic string
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '#' && !inDouble && !inSingle:
			return strings.TrimSpace(raw[:i])
		}
	}
	return raw
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	s := string(data)
	// Strip a single trailing newline so we don't introduce an empty trailing
	// line when joining back; we'll re-add it on write.
	s = strings.TrimRight(s, "\n")
	return strings.Split(s, "\n")
}

func orderedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Simple deterministic order: bypass writes only ever pass two known
	// keys, but sort anyway so tests don't depend on map iteration.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
