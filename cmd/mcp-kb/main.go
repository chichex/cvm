// Spec: S-014
// Package main implements a minimal MCP server that exposes the CVM Knowledge Base
// via two tools: kb_search and kb_get. Transport: stdio JSON-RPC 2.0.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chichex/cvm/internal/config"
	"github.com/chichex/cvm/internal/kb"
)

// ---- JSON-RPC 2.0 types ----

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // null, number, or string
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---- MCP protocol types ----

type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type initializeResult struct {
	ProtocolVersion string      `json:"protocolVersion"`
	ServerInfo      serverInfo  `json:"serverInfo"`
	Capabilities    interface{} `json:"capabilities"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type toolsListResult struct {
	Tools []toolDef `json:"tools"`
}

type toolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolsCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ---- Tool input types ----

type kbSearchInput struct {
	Query string `json:"query"`
	Tags  string `json:"tags"`
	Type  string `json:"type"`
	Limit int    `json:"limit"`
	Scope string `json:"scope"`
}

type kbGetInput struct {
	Key   string `json:"key"`
	Scope string `json:"scope"`
}

// ---- Tool output types (Spec: S-014 | Req: B-004) ----

type searchOutput struct {
	Results []searchResultItem `json:"results"`
	Total   int                `json:"total"`
	Query   string             `json:"query"`
	Scope   string             `json:"scope"`
}

type searchResultItem struct {
	Key       string   `json:"key"`
	Tags      []string `json:"tags"`
	Snippet   string   `json:"snippet"`
	Rank      int      `json:"rank"`
	UpdatedAt string   `json:"updated_at"`
}

type getOutput struct {
	Key       string   `json:"key"`
	Tags      []string `json:"tags"`
	Body      string   `json:"body"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Scope     string   `json:"scope"`
}

type errorOutput struct {
	Error      string   `json:"error"`
	Key        string   `json:"key,omitempty"`
	Scope      string   `json:"scope,omitempty"`
	Message    string   `json:"message,omitempty"`
	ValidTypes []string `json:"valid_types,omitempty"`
}

// ---- Scope detection (Spec: S-014 | Req: B-006) ----

// resolveScope returns the config.Scope and projectPath for a given scope string.
// For "local", it walks up from cwd looking for a .cvm/ directory.
// Returns an error string if local scope cannot be resolved.
func resolveScope(scopeStr string) (config.Scope, string, string) {
	if scopeStr == "" || scopeStr == "global" {
		return config.ScopeGlobal, "", ""
	}
	// Local scope: find project root by walking up from cwd
	cwd, err := os.Getwd()
	if err != nil {
		return config.ScopeLocal, "", "local_kb_not_found"
	}
	dir := cwd
	for {
		if fi, statErr := os.Stat(filepath.Join(dir, ".cvm")); statErr == nil && fi.IsDir() {
			return config.ScopeLocal, dir, ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return config.ScopeLocal, "", "local_kb_not_found"
}

// ---- Tool definitions ----

var kbSearchDef = toolDef{
	Name: "kb_search",
	Description: "Search the CVM Knowledge Base for entries matching a query. " +
		"Returns keys, tags, and snippet context for each match.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query (case-insensitive substring match against key and body)",
			},
			"tags": map[string]interface{}{
				"type":        "string",
				"description": "Filter by tag (exact match). Optional.",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"decision", "learning", "gotcha", "discovery", "session"},
				"description": "Filter by type tag. Optional.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"minimum":     1,
				"maximum":     100,
				"default":     20,
				"description": "Maximum number of results to return.",
			},
			"scope": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"global", "local"},
				"default":     "global",
				"description": "KB scope to search. 'local' requires the server to detect the project path.",
			},
		},
		"required": []string{"query"},
	},
}

var kbGetDef = toolDef{
	Name:        "kb_get",
	Description: "Retrieve the full content of a Knowledge Base entry by key.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"key": map[string]interface{}{
				"type":        "string",
				"description": "The exact key of the KB entry to retrieve.",
			},
			"scope": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"global", "local"},
				"default":     "global",
				"description": "KB scope. 'local' requires project path detection.",
			},
		},
		"required": []string{"key"},
	},
}

// ---- Tool handlers ----

// handleKbSearch implements the kb_search tool (Spec: S-014 | Req: B-002)
// Spec: S-013 | Fix: Backend wiring — uses Backend instead of package-level SearchWithOptions
func handleKbSearch(raw json.RawMessage) toolsCallResult {
	var input kbSearchInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResult("invalid_input", fmt.Sprintf("failed to parse arguments: %v", err), "", "", nil)
	}

	// Validate type (Spec: S-014 | Req: E-002)
	if input.Type != "" {
		if err := kb.ValidateType(input.Type); err != nil {
			return errorResult("invalid_type", "", "", "", kb.ValidTypes)
		}
	}

	// Validate limit (Spec: S-014 | Req: E-003)
	if input.Limit < 0 {
		input.Limit = 20
	}
	if input.Limit > 100 {
		return errorResult("invalid_input", "limit must be between 1 and 100 (maximum is 100)", "", "", nil)
	}
	if input.Limit == 0 {
		input.Limit = 20
	}

	// Resolve scope (Spec: S-014 | Req: B-006)
	scope, projectPath, scopeErr := resolveScope(input.Scope)
	if scopeErr != "" {
		return errorResult(scopeErr, "", "", input.Scope, nil)
	}

	scopeLabel := string(scope)

	// Use Backend directly (Spec: S-013 | Fix: Backend wiring)
	b, err := kb.NewBackend(scope, projectPath)
	if err != nil {
		return errorResult("storage_error", err.Error(), "", scopeLabel, nil)
	}
	defer b.Close()

	opts := kb.SearchOptions{
		Tag:     input.Tags,
		TypeTag: input.Type,
	}

	results, err := b.Search(input.Query, opts)
	if err != nil {
		// KB not initialized is treated as empty, not error (Spec: S-014 | Req: E-001)
		if os.IsNotExist(err) {
			results = nil
		} else {
			return errorResult("storage_error", err.Error(), "", scopeLabel, nil)
		}
	}

	// Apply limit
	if len(results) > input.Limit {
		results = results[:input.Limit]
	}

	items := make([]searchResultItem, 0, len(results))
	for _, r := range results {
		tags := r.Entry.Tags
		if tags == nil {
			tags = []string{}
		}
		items = append(items, searchResultItem{
			Key:       r.Entry.Key,
			Tags:      tags,
			Snippet:   r.Snippet,
			Rank:      r.Rank,
			UpdatedAt: r.Entry.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	out := searchOutput{
		Results: items,
		Total:   len(items),
		Query:   input.Query,
		Scope:   scopeLabel,
	}

	return jsonResult(out)
}

// handleKbGet implements the kb_get tool (Spec: S-014 | Req: B-003)
// Spec: S-013 | Fix: Backend wiring — uses b.Get() (read-only) instead of kb.Show() (mutates LastReferenced)
func handleKbGet(raw json.RawMessage) toolsCallResult {
	var input kbGetInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResult("invalid_input", fmt.Sprintf("failed to parse arguments: %v", err), "", "", nil)
	}

	if input.Key == "" {
		return errorResult("invalid_input", "key is required", "", "", nil)
	}

	scope, projectPath, scopeErr := resolveScope(input.Scope)
	if scopeErr != "" {
		return errorResult(scopeErr, "", input.Key, input.Scope, nil)
	}

	scopeLabel := string(scope)

	// Use Backend.Get (read-only, does not mutate LastReferenced)
	// This fixes the read-only invariant violation from using kb.Show() (Spec: S-014 | Req: I-001)
	b, err := kb.NewBackend(scope, projectPath)
	if err != nil {
		return errorResult("storage_error", err.Error(), input.Key, scopeLabel, nil)
	}
	defer b.Close()

	doc, err := b.Get(input.Key)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errorResultNotFound(input.Key, scopeLabel)
		}
		return errorResult("storage_error", err.Error(), input.Key, scopeLabel, nil)
	}

	tags := doc.Entry.Tags
	if tags == nil {
		tags = []string{}
	}

	out := getOutput{
		Key:       input.Key,
		Tags:      tags,
		Body:      doc.Body,
		CreatedAt: doc.Entry.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: doc.Entry.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Scope:     scopeLabel,
	}

	return jsonResult(out)
}

// stripFrontmatter removes the YAML frontmatter block from markdown content.
// Frontmatter format: ---\nkey: ...\ntags: [...]\n---\n\n<body>
// Spec: S-014 | Req: I-007
func stripFrontmatter(content string) string {
	const frontmatterEnd = "\n---\n\n"
	if idx := strings.Index(content, frontmatterEnd); idx >= 0 {
		return strings.TrimSpace(content[idx+len(frontmatterEnd):])
	}
	return strings.TrimSpace(content)
}

// ---- Result helpers ----

func jsonResult(v interface{}) toolsCallResult {
	data, err := json.Marshal(v)
	if err != nil {
		return errorResult("internal_error", err.Error(), "", "", nil)
	}
	return toolsCallResult{
		Content: []contentItem{{Type: "text", Text: string(data)}},
	}
}

func errorResult(errCode, message, key, scope string, validTypes []string) toolsCallResult {
	out := errorOutput{
		Error:      errCode,
		Key:        key,
		Scope:      scope,
		Message:    message,
		ValidTypes: validTypes,
	}
	data, _ := json.Marshal(out)
	return toolsCallResult{
		Content: []contentItem{{Type: "text", Text: string(data)}},
		IsError: true,
	}
}

func errorResultNotFound(key, scope string) toolsCallResult {
	out := errorOutput{
		Error: "key_not_found",
		Key:   key,
		Scope: scope,
	}
	data, _ := json.Marshal(out)
	return toolsCallResult{
		Content: []contentItem{{Type: "text", Text: string(data)}},
		IsError: true,
	}
}

// ---- MCP handlers ----

func handleInitialize(req rpcRequest) rpcResponse {
	result := initializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: serverInfo{
			Name:    "cvm-kb",
			Version: "1.0",
		},
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	}
	return rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func handleToolsList(req rpcRequest) rpcResponse {
	return rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: toolsListResult{
			Tools: []toolDef{kbSearchDef, kbGetDef},
		},
	}
}

func handleToolsCall(req rpcRequest) rpcResponse {
	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "invalid params: " + err.Error()},
		}
	}

	var result toolsCallResult
	switch params.Name {
	case "kb_search":
		result = handleKbSearch(params.Arguments)
	case "kb_get":
		result = handleKbGet(params.Arguments)
	default:
		result = toolsCallResult{
			Content: []contentItem{{Type: "text", Text: fmt.Sprintf(`{"error":"unknown_tool","tool":%q}`, params.Name)}},
			IsError: true,
		}
	}

	return rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// ---- Main loop (Spec: S-014 | Req: B-001) ----

func main() {
	// I-002: Never write to stdout except for JSON-RPC responses
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(os.Stderr, "mcp-kb: parse error: %v\n", err)
			// Only respond if we can produce an id-less error
			resp := rpcResponse{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`null`),
				Error:   &rpcError{Code: -32700, Message: "parse error: " + err.Error()},
			}
			if encErr := encoder.Encode(resp); encErr != nil {
				fmt.Fprintf(os.Stderr, "mcp-kb: encode error: %v\n", encErr)
				os.Exit(1)
			}
			continue
		}

		// Notifications (no id) — no response needed
		if req.ID == nil {
			// notifications/initialized and similar
			continue
		}

		var resp rpcResponse
		switch req.Method {
		case "initialize":
			resp = handleInitialize(req)
		case "tools/list":
			resp = handleToolsList(req)
		case "tools/call":
			resp = handleToolsCall(req)
		default:
			resp = rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32601, Message: "method not found: " + req.Method},
			}
		}

		if err := encoder.Encode(resp); err != nil {
			fmt.Fprintf(os.Stderr, "mcp-kb: encode error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-kb: stdin error: %v\n", err)
		os.Exit(1)
	}

	// EOF — clean exit (Spec: S-014 | Req: I-006)
	os.Exit(0)
}
