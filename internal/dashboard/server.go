// Spec: S-016
// Package dashboard implements the CVM realtime observability web server.
package dashboard

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chichex/cvm/internal/kb"
)

//go:embed web/index.html
var indexHTML embed.FS

//go:embed web/static/*
var staticFS embed.FS

// Config holds the dashboard server configuration.
// Spec: S-016 | Req: I-001
type Config struct {
	Port        int
	ProjectPath string
}

// Server is the CVM dashboard HTTP server.
// Spec: S-016 | Req: B-001
type Server struct {
	cfg         Config
	globalBack  kb.Backend
	localBack   kb.Backend
	mux         *http.ServeMux
	httpServer  *http.Server
	watcher     *Watcher
}

// New creates a new dashboard Server.
// Spec: S-016 | Req: B-001
func New(cfg Config) (*Server, error) {
	globalBack, err := kb.NewBackend("global", "")
	if err != nil {
		return nil, fmt.Errorf("global backend: %w", err)
	}

	// Local backend is best-effort — may not exist
	localBack, localErr := kb.NewBackend("local", cfg.ProjectPath)
	if localErr != nil {
		fmt.Fprintf(os.Stderr, "warning: local KB unavailable (%v)\n", localErr)
		localBack = nil
	}

	s := &Server{
		cfg:        cfg,
		globalBack: globalBack,
		localBack:  localBack,
	}

	s.mux = http.NewServeMux()
	s.registerRoutes()

	return s, nil
}

// registerRoutes sets up all HTTP routes.
// Spec: S-016 | Req: I-002
func (s *Server) registerRoutes() {
	// Static assets
	subFS, _ := fs.Sub(staticFS, "web/static")
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(subFS))))

	// API endpoints
	s.mux.HandleFunc("/api/timeline", s.handleTimeline)
	s.mux.HandleFunc("/api/session", s.handleSession)
	s.mux.HandleFunc("/api/sessions", s.handleSessions)
	s.mux.HandleFunc("/api/entries", s.handleEntries)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/events", s.handleSSE)

	// Root serves the SPA
	s.mux.HandleFunc("/", s.handleRoot)
}

// handleRoot serves the embedded index.html.
// Spec: S-016 | Req: I-002v
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	data, err := indexHTML.ReadFile("web/index.html") //nolint:gocritic
	if err != nil {
		jsonError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(data) //nolint:errcheck
}

// Run starts the HTTP server and blocks until SIGINT/SIGTERM.
// Spec: S-016 | Req: B-001, I-001c, I-001d, I-001e
func (s *Server) Run() error {
	addr := fmt.Sprintf("0.0.0.0:%d", s.cfg.Port)

	// Check port availability first
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") || strings.Contains(err.Error(), "bind") {
			return fmt.Errorf("port %d already in use", s.cfg.Port)
		}
		return err
	}

	// Start background watcher
	s.watcher = NewWatcher(s.globalBack, s.localBack)
	go s.watcher.Run()

	s.httpServer = &http.Server{
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE streams need unlimited write time
	}

	// Handle shutdown signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("Dashboard running at http://localhost:%d\n", s.cfg.Port)
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-quit:
		// Graceful shutdown — Ctrl+C is a clean exit, not an error
		fmt.Println("\nShutting down…")
		s.watcher.Stop()
		if s.globalBack != nil {
			s.globalBack.Close() //nolint:errcheck
		}
		if s.localBack != nil {
			s.localBack.Close() //nolint:errcheck
		}
		// Force-close the server (don't wait for SSE connections to drain)
		s.httpServer.Close() //nolint:errcheck
		return nil
	case err := <-errCh:
		return err
	}
}

// backendForScope returns the appropriate backend(s) given a scope string.
// Spec: S-016 | Req: I-002b, E-005
func (s *Server) backendForScope(scope string) (global, local kb.Backend, err error) {
	switch scope {
	case "global":
		return s.globalBack, nil, nil
	case "local":
		return nil, s.localBack, nil
	case "both":
		return s.globalBack, s.localBack, nil
	default:
		return nil, nil, fmt.Errorf("scope must be global, local, or both")
	}
}
