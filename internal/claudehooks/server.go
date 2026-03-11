package claudehooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/dshills/witness/internal/events"
)

// Server receives Claude Code hook HTTP callbacks and converts them
// to Witness events via the provided EventSink.
type Server struct {
	sink   events.EventSink
	runID  string
	server *http.Server
	addr   string
}

// NewServer creates a hook server bound to a random port on localhost.
func NewServer(sink events.EventSink, runID string) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	s := &Server{
		sink:  sink,
		runID: runID,
		addr:  ln.Addr().String(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/hook", s.handleHook)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("claudehooks: server error: %v", err)
		}
	}()

	return s, nil
}

// Addr returns the listener address (e.g., "127.0.0.1:54321").
func (s *Server) Addr() string {
	return s.addr
}

// URL returns the base URL of the hook server.
func (s *Server) URL() string {
	return "http://" + s.addr
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var payload HookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	evts := Convert(s.runID, payload)
	for _, evt := range evts {
		if err := s.sink.Append(r.Context(), evt); err != nil {
			log.Printf("claudehooks: append event: %v", err)
		}
	}

	// Return empty JSON (Claude Code expects valid JSON response for HTTP hooks).
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{}`))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// HooksSettingsForAddr returns the Claude Code settings JSON that configures
// hooks to point at the given address.
func HooksSettingsForAddr(addr string) string {
	url := "http://" + addr + "/hook"
	return hooksSettingsJSON(url)
}

// HooksSettings returns the Claude Code settings JSON that configures
// hooks to point at this server.
func (s *Server) HooksSettings() string {
	url := s.URL() + "/hook"
	return hooksSettingsJSON(url)
}

func hooksSettingsJSON(url string) string {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []map[string]any{
				{
					"matcher": ".*",
					"hooks": []map[string]any{
						{
							"type":    "http",
							"url":     url,
							"timeout": 5,
						},
					},
				},
			},
			"PostToolUse": []map[string]any{
				{
					"matcher": ".*",
					"hooks": []map[string]any{
						{
							"type":    "http",
							"url":     url,
							"timeout": 5,
						},
					},
				},
			},
			"SubagentStart": []map[string]any{
				{
					"matcher": ".*",
					"hooks": []map[string]any{
						{
							"type":    "http",
							"url":     url,
							"timeout": 5,
						},
					},
				},
			},
			"SubagentStop": []map[string]any{
				{
					"matcher": ".*",
					"hooks": []map[string]any{
						{
							"type":    "http",
							"url":     url,
							"timeout": 5,
						},
					},
				},
			},
			"UserPromptSubmit": []map[string]any{
				{
					"hooks": []map[string]any{
						{
							"type":    "http",
							"url":     url,
							"timeout": 5,
						},
					},
				},
			},
			"Stop": []map[string]any{
				{
					"hooks": []map[string]any{
						{
							"type":    "http",
							"url":     url,
							"timeout": 5,
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(settings)
	return string(data)
}
