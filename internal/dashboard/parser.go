// Spec: S-016
// parser.go — Parses session buffer lines into structured events.
// Formats defined in S-011 and S-015.
package dashboard

import (
	"regexp"
	"strings"
)

// reToolLine matches: [HH:MM] [TOOL:Name] content
// Spec: S-016 | Req: I-002f (S-015 format)
var reToolLine = regexp.MustCompile(`^\[(\d{2}:\d{2})\]\s+\[TOOL:([^\]]+)\]\s*(.*)$`)

// reUserLine matches: [HH:MM] USER: content
// Spec: S-016 | Req: I-002f (S-011 format)
var reUserLine = regexp.MustCompile(`^\[(\d{2}:\d{2})\]\s+USER:\s*(.*)$`)

// ParseSessionLines parses the raw body of a session buffer entry into structured lines.
// Spec: S-016 | Req: I-002e, I-002f, E-003
func ParseSessionLines(body string) []sessionLineJSON {
	var lines []sessionLineJSON
	for _, raw := range strings.Split(body, "\n") {
		raw = strings.TrimRight(raw, "\r")
		if raw == "" {
			continue
		}
		line := parseLine(raw)
		lines = append(lines, line)
	}
	if lines == nil {
		lines = []sessionLineJSON{}
	}
	return lines
}

// parseLine classifies a single raw line.
// Spec: S-016 | Req: I-002e, E-003
func parseLine(raw string) sessionLineJSON {
	// Try TOOL format first
	if m := reToolLine.FindStringSubmatch(raw); m != nil {
		return sessionLineJSON{
			Timestamp: m[1],
			Type:      "TOOL",
			Tool:      m[2],
			Content:   m[3],
		}
	}
	// Try USER format
	if m := reUserLine.FindStringSubmatch(raw); m != nil {
		return sessionLineJSON{
			Timestamp: m[1],
			Type:      "USER",
			Content:   m[2],
		}
	}
	// Fallback: RAW — Spec: S-016 | Req: E-003
	return sessionLineJSON{
		Type:    "RAW",
		Content: raw,
	}
}
