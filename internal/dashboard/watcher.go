// Spec: S-016
// watcher.go — Background goroutine that polls the KB and session files, emitting change events.
package dashboard

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chichex/cvm/internal/kb"
)

// EventType represents a type of SSE event.
// Spec: S-016 | Req: I-002r, I-002s
type EventType string

const (
	EventTick           EventType = "tick"
	EventEntryAdded     EventType = "entry_added"
	EventSessionUpdated EventType = "session_updated"
)

// Event is a single SSE event to broadcast.
type Event struct {
	Type EventType
	Data map[string]string
}

// Watcher polls the KB and ~/.cvm/sessions/ for changes and broadcasts events.
// Spec: S-016 | Req: I-002q, I-002r, I-002s, I-002u, I-INV-004
// Spec: S-017 | Req: B-013
type Watcher struct {
	globalBack kb.Backend
	localBack  kb.Backend

	subscribers map[chan Event]struct{}
	mu          sync.RWMutex

	stopCh chan struct{}

	// Snapshots for change detection: key -> updated_at
	// Spec: S-016 | Req: I-002u
	globalSnapshot map[string]time.Time
	localSnapshot  map[string]time.Time

	// sessionSnapshot tracks session file mtimes: uuid -> mtime
	// Spec: S-017 | Req: B-013
	sessionSnapshot map[string]time.Time
}

// NewWatcher creates a new Watcher.
func NewWatcher(global, local kb.Backend) *Watcher {
	return &Watcher{
		globalBack:      global,
		localBack:       local,
		subscribers:     make(map[chan Event]struct{}),
		stopCh:          make(chan struct{}),
		globalSnapshot:  make(map[string]time.Time),
		localSnapshot:   make(map[string]time.Time),
		sessionSnapshot: make(map[string]time.Time),
	}
}

// Subscribe returns a channel that receives events. The caller must call Unsubscribe when done.
// Spec: S-016 | Req: I-INV-004
func (w *Watcher) Subscribe() chan Event {
	ch := make(chan Event, 32)
	w.mu.Lock()
	w.subscribers[ch] = struct{}{}
	w.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber.
// Spec: S-016 | Req: E-006, I-INV-004
func (w *Watcher) Unsubscribe(ch chan Event) {
	w.mu.Lock()
	delete(w.subscribers, ch)
	w.mu.Unlock()
	// Drain the channel to prevent goroutine leak in broadcast
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// broadcast sends an event to all subscribers.
// Spec: S-016 | Req: I-INV-004
func (w *Watcher) broadcast(e Event) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for ch := range w.subscribers {
		select {
		case ch <- e:
		default:
			// Drop event if subscriber is slow — non-blocking
		}
	}
}

// Run starts the polling loop. Blocks until Stop() is called.
// Spec: S-016 | Req: I-002q, I-002r, I-002s, I-002t
func (w *Watcher) Run() {
	ticker := time.NewTicker(2 * time.Second)
	keepalive := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	defer keepalive.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-keepalive.C:
			// Keepalive comment — handled per-connection in SSE handler
		case t := <-ticker.C:
			// Emit tick event
			w.broadcast(Event{
				Type: EventTick,
				Data: map[string]string{"ts": t.UTC().Format(time.RFC3339)},
			})
			// Check for KB changes
			w.checkChanges()
		}
	}
}

// Stop signals the watcher to stop.
func (w *Watcher) Stop() {
	close(w.stopCh)
}

// checkChanges polls backends and session files, broadcasting change events.
// Spec: S-016 | Req: I-002r, I-002s, I-002u
// Spec: S-017 | Req: B-013
func (w *Watcher) checkChanges() {
	if w.globalBack != nil {
		w.checkBackend(w.globalBack, "global", w.globalSnapshot)
	}
	if w.localBack != nil {
		w.checkBackend(w.localBack, "local", w.localSnapshot)
	}
	// Poll ~/.cvm/sessions/ for session file mtime changes.
	// Spec: S-017 | Req: B-013
	w.checkSessionFiles()
}

// checkBackend compares the current snapshot with the stored one and emits events.
// Spec: S-016 | Req: I-002u (compare updated_at only, not full bodies)
func (w *Watcher) checkBackend(b kb.Backend, scopeName string, snapshot map[string]time.Time) {
	entries, err := b.List("")
	if err != nil {
		return
	}

	for _, e := range entries {
		prev, seen := snapshot[e.Key]
		if !seen {
			// New entry — always emit entry_added for KB entries
			w.broadcast(Event{
				Type: EventEntryAdded,
				Data: map[string]string{"key": e.Key, "scope": scopeName},
			})
			snapshot[e.Key] = e.UpdatedAt
		} else if e.UpdatedAt.After(prev) {
			// Updated entry
			w.broadcast(Event{
				Type: EventEntryAdded,
				Data: map[string]string{"key": e.Key, "scope": scopeName},
			})
			snapshot[e.Key] = e.UpdatedAt
		}
	}

	// Remove stale keys from snapshot
	current := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		current[e.Key] = struct{}{}
	}
	for key := range snapshot {
		if _, ok := current[key]; !ok {
			delete(snapshot, key)
		}
	}
}

// checkSessionFiles polls ~/.cvm/sessions/ for mtime changes and emits EventSessionUpdated.
// Spec: S-017 | Req: B-013
func (w *Watcher) checkSessionFiles() {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Directory may not exist yet — not an error
		return
	}

	current := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		uuid := strings.TrimSuffix(e.Name(), ".jsonl")
		current[uuid] = struct{}{}

		info, err := e.Info()
		if err != nil {
			continue
		}
		mtime := info.ModTime()
		prev, seen := w.sessionSnapshot[uuid]
		if !seen || mtime.After(prev) {
			w.broadcast(Event{
				Type: EventSessionUpdated,
				Data: map[string]string{"session_id": uuid, "file": e.Name()},
			})
			w.sessionSnapshot[uuid] = mtime
		}
	}

	// Remove stale entries from snapshot
	for uuid := range w.sessionSnapshot {
		if _, ok := current[uuid]; !ok {
			delete(w.sessionSnapshot, uuid)
		}
	}
}

