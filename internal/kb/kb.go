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

type Entry struct {
	Key            string    `json:"key"`
	Tags           []string  `json:"tags"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	LastReferenced time.Time `json:"last_referenced,omitempty"`
}

type Document struct {
	Entry Entry
	Body  string
}

type Index struct {
	Entries []Entry `json:"entries"`
}

func kbDir(scope config.Scope, projectPath string) string {
	if scope == config.ScopeGlobal {
		return config.GlobalKBDir()
	}
	return config.LocalKBDir(projectPath)
}

func indexPath(scope config.Scope, projectPath string) string {
	return filepath.Join(kbDir(scope, projectPath), ".index.json")
}

func entriesDir(scope config.Scope, projectPath string) string {
	return filepath.Join(kbDir(scope, projectPath), "entries")
}

func entryPath(scope config.Scope, projectPath, key string) string {
	return filepath.Join(entriesDir(scope, projectPath), key+".md")
}

func loadIndex(scope config.Scope, projectPath string) (*Index, error) {
	idx := &Index{}
	data, err := os.ReadFile(indexPath(scope, projectPath))
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func saveIndex(scope config.Scope, projectPath string, idx *Index) error {
	dir := kbDir(scope, projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath(scope, projectPath), data, 0644)
}

func Put(scope config.Scope, projectPath, key, body string, tags []string) error {
	dir := entriesDir(scope, projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return err
	}

	now := time.Now()
	found := false
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries[i].Tags = tags
			idx.Entries[i].UpdatedAt = now
			found = true
			break
		}
	}

	if !found {
		idx.Entries = append(idx.Entries, Entry{
			Key:       key,
			Tags:      tags,
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	content := fmt.Sprintf("---\nkey: %s\ntags: [%s]\n---\n\n%s\n", key, strings.Join(tags, ", "), body)
	if err := os.WriteFile(entryPath(scope, projectPath, key), []byte(content), 0644); err != nil {
		return err
	}

	return saveIndex(scope, projectPath, idx)
}

func LoadDocuments(scope config.Scope, projectPath string) ([]Document, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return nil, err
	}

	docs := make([]Document, 0, len(idx.Entries))
	for _, entry := range idx.Entries {
		body, err := readBody(scope, projectPath, entry.Key)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		docs = append(docs, Document{
			Entry: entry,
			Body:  body,
		})
	}
	return docs, nil
}

func SaveDocument(scope config.Scope, projectPath string, doc Document) error {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return err
	}

	found := false
	for i, entry := range idx.Entries {
		if entry.Key == doc.Entry.Key {
			idx.Entries[i] = doc.Entry
			found = true
			break
		}
	}
	if !found {
		idx.Entries = append(idx.Entries, doc.Entry)
	}

	if err := os.MkdirAll(entriesDir(scope, projectPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(entryPath(scope, projectPath, doc.Entry.Key), []byte(renderDocument(doc.Entry.Key, doc.Entry.Tags, doc.Body)), 0644); err != nil {
		return err
	}

	return saveIndex(scope, projectPath, idx)
}

func List(scope config.Scope, projectPath, tag string) ([]Entry, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return nil, err
	}
	if tag == "" {
		return idx.Entries, nil
	}
	var filtered []Entry
	for _, e := range idx.Entries {
		for _, t := range e.Tags {
			if t == tag {
				filtered = append(filtered, e)
				break
			}
		}
	}
	return filtered, nil
}

func Remove(scope config.Scope, projectPath, key string) error {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return err
	}
	found := false
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries = append(idx.Entries[:i], idx.Entries[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("entry %q not found", key)
	}
	os.Remove(entryPath(scope, projectPath, key))
	return saveIndex(scope, projectPath, idx)
}

func Show(scope config.Scope, projectPath, key string) (string, error) {
	data, err := os.ReadFile(entryPath(scope, projectPath, key))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("entry %q not found", key)
		}
		return "", err
	}
	idx, _ := loadIndex(scope, projectPath)
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries[i].LastReferenced = time.Now()
			saveIndex(scope, projectPath, idx)
			break
		}
	}
	return string(data), nil
}

func SetEnabled(scope config.Scope, projectPath, key string, enabled bool) error {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return err
	}
	for i, e := range idx.Entries {
		if e.Key == key {
			idx.Entries[i].Enabled = enabled
			idx.Entries[i].UpdatedAt = time.Now()
			return saveIndex(scope, projectPath, idx)
		}
	}
	return fmt.Errorf("entry %q not found", key)
}

func Search(scope config.Scope, projectPath, query string) ([]SearchResult, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(query)
	var results []SearchResult
	for _, e := range idx.Entries {
		data, err := os.ReadFile(entryPath(scope, projectPath, e.Key))
		if err != nil {
			continue
		}
		content := strings.ToLower(string(data))
		if strings.Contains(content, query) || strings.Contains(strings.ToLower(e.Key), query) {
			results = append(results, SearchResult{
				Entry:   e,
				Snippet: extractSnippet(string(data), query),
			})
		}
	}
	return results, nil
}

type SearchResult struct {
	Entry   Entry
	Snippet string
}

func extractSnippet(content, query string) string {
	lower := strings.ToLower(content)
	idx := strings.Index(lower, strings.ToLower(query))
	if idx == -1 {
		if len(content) > 100 {
			return content[:100] + "..."
		}
		return content
	}
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 40
	if end > len(content) {
		end = len(content)
	}
	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return strings.TrimSpace(snippet)
}

func Clean(scope config.Scope, projectPath string) (int, error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return 0, err
	}
	count := len(idx.Entries)
	if count == 0 {
		return 0, nil
	}

	// Remove all entry files
	for _, e := range idx.Entries {
		os.Remove(entryPath(scope, projectPath, e.Key))
	}

	// Reset index
	idx.Entries = nil
	if err := saveIndex(scope, projectPath, idx); err != nil {
		return 0, err
	}
	return count, nil
}

func Stats(scope config.Scope, projectPath string) (total, enabled, stale int, err error) {
	idx, err := loadIndex(scope, projectPath)
	if err != nil {
		return 0, 0, 0, err
	}
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	for _, e := range idx.Entries {
		total++
		if e.Enabled {
			enabled++
		}
		if !e.LastReferenced.IsZero() && e.LastReferenced.Before(thirtyDaysAgo) {
			stale++
		} else if e.LastReferenced.IsZero() && e.CreatedAt.Before(thirtyDaysAgo) {
			stale++
		}
	}
	return total, enabled, stale, nil
}

func renderDocument(key string, tags []string, body string) string {
	return fmt.Sprintf("---\nkey: %s\ntags: [%s]\n---\n\n%s\n", key, strings.Join(tags, ", "), body)
}

func readBody(scope config.Scope, projectPath, key string) (string, error) {
	data, err := os.ReadFile(entryPath(scope, projectPath, key))
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
