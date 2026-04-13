// Spec: S-014 | Type: unit
// Tests for the MCP KB server: tools/list, kb_search, kb_get behaviors.
package mcpkb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/kb"
)

// ---- helpers ----

// setupGlobalKB creates a temporary global KB directory with the given entries,
// overrides CVM_HOME-equivalent by setting HOME to a temp dir, and returns cleanup.
func setupTempKB(t *testing.T) (globalKBDir string, cleanup func()) {
	t.Helper()
	tmpHome := t.TempDir()

	// Override the home dir used by config.GlobalKBDir via HOME env var.
	// config.CvmHome() uses os.UserHomeDir() which reads HOME.
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)

	globalKBDir = filepath.Join(tmpHome, ".cvm", "global", "kb")
	if err := os.MkdirAll(globalKBDir, 0755); err != nil {
		t.Fatal(err)
	}

	cleanup = func() {
		os.Setenv("HOME", origHome)
	}
	return globalKBDir, cleanup
}

// putEntry writes an entry to the global KB.
func putEntry(t *testing.T, key, body string, tags []string) {
	t.Helper()
	if err := kb.Put(config.ScopeGlobal, "", key, body, tags); err != nil {
		t.Fatalf("put entry %q: %v", key, err)
	}
}

// decodeText unmarshals content[0].text as JSON into dst.
func decodeText(t *testing.T, result toolsCallResult, dst interface{}) {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected content, got empty")
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), dst); err != nil {
		t.Fatalf("unmarshal content text: %v\ntext: %s", err, result.Content[0].Text)
	}
}

// rawArgs encodes v as JSON for use as tool arguments.
func rawArgs(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return json.RawMessage(b)
}

// ---- tools/list ----

// Spec: S-014 | Req: B-001 | Type: happy
func TestToolsList(t *testing.T) {
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	}
	resp := handleToolsList(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatal(err)
	}
	var list toolsListResult
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}

	if len(list.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(list.Tools))
	}

	names := map[string]bool{}
	for _, tool := range list.Tools {
		names[tool.Name] = true
	}
	if !names["kb_search"] {
		t.Error("missing tool kb_search")
	}
	if !names["kb_get"] {
		t.Error("missing tool kb_get")
	}
}

// ---- kb_search ----

// Spec: S-014 | Req: B-002-happy | Type: happy
func TestKbSearch_Happy(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	putEntry(t, "arch-decision-api", "Chose REST over gRPC for API design", []string{"decision", "type:decision"})
	putEntry(t, "api-gateway-notes", "Notes about the API gateway setup", []string{"learning"})
	putEntry(t, "unrelated-entry", "Nothing about apis here", []string{"gotcha"})

	result := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "api"}))

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	var out searchOutput
	decodeText(t, result, &out)

	if out.Total < 2 {
		t.Errorf("expected at least 2 results, got %d", out.Total)
	}
	if out.Query != "api" {
		t.Errorf("expected query 'api', got %q", out.Query)
	}
	if out.Scope != "global" {
		t.Errorf("expected scope 'global', got %q", out.Scope)
	}

	// Verify result fields are populated
	for _, r := range out.Results {
		if r.Key == "" {
			t.Error("result has empty key")
		}
		if r.UpdatedAt == "" {
			t.Error("result has empty updated_at")
		}
	}
}

// Spec: S-014 | Req: B-002-filter | Type: edge
func TestKbSearch_FilterByType(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	putEntry(t, "bug-fix-2026", "Found a nasty bug", []string{"type:gotcha"})
	putEntry(t, "arch-v2", "Architecture decision v2", []string{"type:decision"})

	result := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "a", "type": "gotcha"}))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	var out searchOutput
	decodeText(t, result, &out)

	if out.Total != 1 {
		t.Errorf("expected 1 result, got %d", out.Total)
	}
	if out.Total > 0 && out.Results[0].Key != "bug-fix-2026" {
		t.Errorf("expected key 'bug-fix-2026', got %q", out.Results[0].Key)
	}
}

// Spec: S-014 | Req: B-002-limit | Type: edge
func TestKbSearch_Limit(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	// Create 10 entries with "common" in body
	for i := 0; i < 10; i++ {
		key := "entry-common-" + string(rune('a'+i))
		putEntry(t, key, "common body content", []string{"learning"})
	}

	result := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "common", "limit": 5}))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	var out searchOutput
	decodeText(t, result, &out)

	if out.Total != 5 {
		t.Errorf("expected 5 results, got %d", out.Total)
	}
	if len(out.Results) != 5 {
		t.Errorf("expected 5 result items, got %d", len(out.Results))
	}
}

// Spec: S-014 | Req: B-002-empty | Type: edge
func TestKbSearch_Empty(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	result := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "xyznotfound"}))
	if result.IsError {
		t.Fatalf("expected no error, got: %s", result.Content[0].Text)
	}

	var out searchOutput
	decodeText(t, result, &out)

	if out.Total != 0 {
		t.Errorf("expected total 0, got %d", out.Total)
	}
	if len(out.Results) != 0 {
		t.Errorf("expected empty results, got %d items", len(out.Results))
	}
}

// Spec: S-014 | Req: E-001 | Type: edge
func TestKbSearch_KBNotInitialized(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()
	// Don't create any entries — KB is empty / uninitialized

	result := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "anything"}))
	if result.IsError {
		t.Fatalf("empty KB should not return error, got: %s", result.Content[0].Text)
	}

	var out searchOutput
	decodeText(t, result, &out)

	if out.Total != 0 {
		t.Errorf("expected 0, got %d", out.Total)
	}
}

// Spec: S-014 | Req: E-002 | Type: error
func TestKbSearch_InvalidType(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	result := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "x", "type": "invalid-type"}))
	if !result.IsError {
		t.Fatal("expected isError=true for invalid type")
	}

	var out errorOutput
	decodeText(t, result, &out)

	if out.Error != "invalid_type" {
		t.Errorf("expected error 'invalid_type', got %q", out.Error)
	}
	if len(out.ValidTypes) == 0 {
		t.Error("expected valid_types to be populated")
	}
}

// Spec: S-014 | Req: E-003 | Type: error
func TestKbSearch_LimitOutOfRange(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	result := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "x", "limit": 200}))
	if !result.IsError {
		t.Fatal("expected isError=true for limit > 100")
	}

	var out errorOutput
	decodeText(t, result, &out)

	if out.Error != "invalid_input" {
		t.Errorf("expected error 'invalid_input', got %q", out.Error)
	}
}

// Spec: S-014 | Req: B-002-happy — rank ordering | Type: happy
func TestKbSearch_RankOrder(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	// Exact key match (rank 0), key contains (rank 1), body match (rank 2)
	putEntry(t, "golang", "Some unrelated content", []string{"learning"})
	putEntry(t, "golang-tips", "More Go tips", []string{"learning"})
	putEntry(t, "my-notes", "I love golang programming", []string{"learning"})

	result := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "golang"}))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	var out searchOutput
	decodeText(t, result, &out)

	if len(out.Results) < 3 {
		t.Fatalf("expected 3 results, got %d", len(out.Results))
	}

	// Verify rank ascending order
	for i := 1; i < len(out.Results); i++ {
		if out.Results[i].Rank < out.Results[i-1].Rank {
			t.Errorf("results not sorted by rank: index %d (rank %d) < index %d (rank %d)",
				i, out.Results[i].Rank, i-1, out.Results[i-1].Rank)
		}
	}
}

// ---- kb_get ----

// Spec: S-014 | Req: B-003-happy | Type: happy
func TestKbGet_Happy(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	putEntry(t, "my-decision", "Usamos flat files por simplicidad", []string{"decision", "type:decision"})

	result := handleKbGet(rawArgs(t, map[string]interface{}{"key": "my-decision"}))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	var out getOutput
	decodeText(t, result, &out)

	if out.Key != "my-decision" {
		t.Errorf("expected key 'my-decision', got %q", out.Key)
	}
	if out.Body != "Usamos flat files por simplicidad" {
		t.Errorf("expected body without frontmatter, got %q", out.Body)
	}
	if out.Scope != "global" {
		t.Errorf("expected scope 'global', got %q", out.Scope)
	}
	if len(out.Tags) == 0 {
		t.Error("expected tags to be populated")
	}
	if out.CreatedAt == "" {
		t.Error("expected created_at to be set")
	}
	if out.UpdatedAt == "" {
		t.Error("expected updated_at to be set")
	}
}

// Spec: S-014 | Req: B-003-notfound | Type: error
func TestKbGet_NotFound(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	result := handleKbGet(rawArgs(t, map[string]interface{}{"key": "does-not-exist"}))
	if !result.IsError {
		t.Fatal("expected isError=true for missing key")
	}

	var out errorOutput
	decodeText(t, result, &out)

	if out.Error != "key_not_found" {
		t.Errorf("expected 'key_not_found', got %q", out.Error)
	}
	if out.Key != "does-not-exist" {
		t.Errorf("expected key 'does-not-exist', got %q", out.Key)
	}
}

// Spec: S-014 | Req: I-007 | Type: happy — frontmatter stripped
func TestKbGet_FrontmatterStripped(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	putEntry(t, "frontmatter-test", "Body content only", []string{"learning"})

	result := handleKbGet(rawArgs(t, map[string]interface{}{"key": "frontmatter-test"}))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	var out getOutput
	decodeText(t, result, &out)

	if out.Body != "Body content only" {
		t.Errorf("frontmatter not stripped, body is: %q", out.Body)
	}
	// Ensure frontmatter markers are not in body
	if len(out.Body) > 3 && out.Body[:3] == "---" {
		t.Error("body still contains frontmatter delimiter")
	}
}

// Spec: S-014 | Req: B-004 | Type: happy — JSON validity
func TestToolOutputIsValidJSON(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	putEntry(t, "json-test", "JSON test entry", []string{"learning"})

	// kb_search output must be valid JSON
	searchResult := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "json-test"}))
	if len(searchResult.Content) == 0 {
		t.Fatal("no content in kb_search result")
	}
	if searchResult.Content[0].Type != "text" {
		t.Errorf("expected content type 'text', got %q", searchResult.Content[0].Type)
	}
	var v1 interface{}
	if err := json.Unmarshal([]byte(searchResult.Content[0].Text), &v1); err != nil {
		t.Errorf("kb_search output is not valid JSON: %v\ntext: %s", err, searchResult.Content[0].Text)
	}

	// kb_get output must be valid JSON
	getResult := handleKbGet(rawArgs(t, map[string]interface{}{"key": "json-test"}))
	if len(getResult.Content) == 0 {
		t.Fatal("no content in kb_get result")
	}
	if getResult.Content[0].Type != "text" {
		t.Errorf("expected content type 'text', got %q", getResult.Content[0].Type)
	}
	var v2 interface{}
	if err := json.Unmarshal([]byte(getResult.Content[0].Text), &v2); err != nil {
		t.Errorf("kb_get output is not valid JSON: %v\ntext: %s", err, getResult.Content[0].Text)
	}
}

// Spec: S-014 | Req: B-006-local-notfound | Type: error
func TestKbSearch_LocalScope_NotFound(t *testing.T) {
	// Run from a temp dir without .cvm/
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	result := handleKbSearch(rawArgs(t, map[string]interface{}{"query": "anything", "scope": "local"}))
	if !result.IsError {
		t.Fatal("expected isError=true when local .cvm/ not found")
	}

	var out errorOutput
	decodeText(t, result, &out)

	if out.Error != "local_kb_not_found" {
		t.Errorf("expected 'local_kb_not_found', got %q", out.Error)
	}
}

// Spec: S-014 | Req: B-006-local-notfound | Type: error
func TestKbGet_LocalScope_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	result := handleKbGet(rawArgs(t, map[string]interface{}{"key": "anything", "scope": "local"}))
	if !result.IsError {
		t.Fatal("expected isError=true when local .cvm/ not found")
	}

	var out errorOutput
	decodeText(t, result, &out)

	if out.Error != "local_kb_not_found" {
		t.Errorf("expected 'local_kb_not_found', got %q", out.Error)
	}
}

// Spec: S-014 | Req: B-001 | Type: happy — initialize handler
func TestHandleInitialize(t *testing.T) {
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}`),
	}
	resp := handleInitialize(req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result initializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version '2024-11-05', got %q", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "cvm-kb" {
		t.Errorf("expected server name 'cvm-kb', got %q", result.ServerInfo.Name)
	}
	if result.ServerInfo.Version == "" {
		t.Error("expected server version to be set")
	}
}

// ---- stripFrontmatter ----

// Spec: S-014 | Req: I-007 | Type: unit
func TestStripFrontmatter(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with frontmatter",
			input:    "---\nkey: foo\ntags: [bar]\n---\n\nBody content here",
			expected: "Body content here",
		},
		{
			name:     "without frontmatter",
			input:    "Just plain body",
			expected: "Just plain body",
		},
		{
			name:     "multiline body",
			input:    "---\nkey: foo\n---\n\nLine 1\nLine 2\nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripFrontmatter(tc.input)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

// ---- timestamp format ----

// Spec: S-014 | Req: B-004 — RFC3339 timestamps | Type: unit
func TestTimestampsAreRFC3339(t *testing.T) {
	_, cleanup := setupTempKB(t)
	defer cleanup()

	putEntry(t, "ts-test", "Timestamp test", []string{"learning"})

	result := handleKbGet(rawArgs(t, map[string]interface{}{"key": "ts-test"}))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	var out getOutput
	decodeText(t, result, &out)

	for _, ts := range []string{out.CreatedAt, out.UpdatedAt} {
		if _, err := time.Parse("2006-01-02T15:04:05Z", ts); err != nil {
			t.Errorf("timestamp %q is not RFC3339: %v", ts, err)
		}
	}
}
