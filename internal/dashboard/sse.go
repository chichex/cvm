// Spec: S-016
// sse.go — Server-Sent Events handler for real-time updates.
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleSSE serves GET /api/events as a Server-Sent Events stream.
// Spec: S-016 | Req: I-002p, I-002q, I-002r, I-002s, I-002t, E-006, I-INV-004
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Verify SSE support
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers — Spec: S-016 | Req: I-002p
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Subscribe to watcher events — Spec: S-016 | Req: I-INV-004
	ch := s.watcher.Subscribe()
	defer s.watcher.Unsubscribe(ch)

	// Keepalive ticker — Spec: S-016 | Req: I-002t
	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected — Spec: S-016 | Req: E-006
			return
		case <-keepalive.C:
			// SSE comment for keepalive — Spec: S-016 | Req: I-002t
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case evt, ok := <-ch:
			if !ok {
				return
			}
			writeSSEEvent(w, flusher, string(evt.Type), evt.Data)
		}
	}
}

// writeSSEEvent writes a single SSE event to the response writer.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data map[string]string) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		dataJSON = []byte("{}")
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(dataJSON))
	flusher.Flush()
}
