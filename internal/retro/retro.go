package retro

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxConversationChars = 15000

// transcriptEntry represents a single entry in a Claude Code JSONL transcript.
type transcriptEntry struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}

type messageContent struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// claudeProjectDir returns the path where Claude Code stores project data.
// Claude encodes paths by replacing "/" with "-".
func claudeProjectDir(projectPath string) string {
	encoded := strings.ReplaceAll(projectPath, "/", "-")
	return filepath.Join(os.Getenv("HOME"), ".claude", "projects", encoded)
}

// FindLatestTranscript finds the most recently modified .jsonl transcript
// in the Claude project directory for the given project path.
func FindLatestTranscript(projectPath string) (string, error) {
	dir := claudeProjectDir(projectPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("cannot read Claude project dir %s: %w", dir, err)
	}

	var latest string
	var latestTime int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().UnixNano() > latestTime {
			latestTime = info.ModTime().UnixNano()
			latest = filepath.Join(dir, e.Name())
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no transcript found in %s", dir)
	}
	return latest, nil
}

// ExtractConversation reads a JSONL transcript and extracts user/assistant
// text into a clean conversation format, truncated to maxConversationChars.
func ExtractConversation(transcriptPath string) (string, error) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer

	for scanner.Scan() {
		var entry transcriptEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		if entry.Type != "user" && entry.Type != "assistant" {
			continue
		}

		var msg messageContent
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		text := extractText(msg.Content)
		if text == "" {
			continue
		}

		role := strings.ToUpper(entry.Type[:1]) + entry.Type[1:]
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", role, text))

		if sb.Len() > maxConversationChars {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return sb.String(), nil
}

// extractText pulls plain text from a message content field.
// Content can be a string (user messages) or an array of blocks (assistant messages).
func extractText(raw json.RawMessage) string {
	// Try as string first (user messages)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}

	// Try as array of blocks (assistant messages)
	var blocks []textBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}

	return ""
}
