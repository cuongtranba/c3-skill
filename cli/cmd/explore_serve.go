package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// ExploreServeOptions holds parameters for the live explorer server.
type ExploreServeOptions struct {
	Store      *store.Store
	C3Dir      string
	IncludeADR bool
	Port       int
}

/* ─── SSE hub ─────────────────────────────────────────────────────── */

type sseHub struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
	latest  string // last valid payload JSON
}

func newSSEHub(latest string) *sseHub {
	return &sseHub{clients: map[chan string]struct{}{}, latest: latest}
}

func (h *sseHub) add() chan string {
	ch := make(chan string, 8)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *sseHub) remove(ch chan string) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *sseHub) setLatest(data string) {
	h.mu.Lock()
	h.latest = data
	h.mu.Unlock()
}

func (h *sseHub) getLatest() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.latest
}

// broadcast sends one SSE frame to every client; a slow client's full buffer
// drops the frame rather than blocking the hub (reconnect converges anyway).
func (h *sseHub) broadcast(event, data string) {
	frame := sseFrame(event, data)
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- frame:
		default:
		}
	}
}

func sseFrame(event, data string) string {
	return "event: " + event + "\ndata: " + data + "\n\n"
}

/* ─── server ──────────────────────────────────────────────────────── */

type exploreServer struct {
	c3Dir      string
	includeADR bool
	shell      string
	hub        *sseHub
	w          io.Writer

	rebuildMu    sync.Mutex
	rebuildTimer *time.Timer
}

// RunExploreServe serves the explorer live: the .c3/activity.jsonl trail —
// written by every CLI command — is the single source of truth. Mutating
// entries trigger a payload rebuild; every entry streams to the browser as an
// action event.
func RunExploreServe(opts ExploreServeOptions, w io.Writer) error {
	payload, err := buildExplorePayload(opts.Store, opts.C3Dir, opts.IncludeADR)
	if err != nil {
		return err
	}
	if issues := validateExplorePayload(payload); len(issues) > 0 {
		return fmt.Errorf("explore: payload failed schema validation (%d issue(s)) — refusing to serve:\n  - %s\nhint: fix the issues above; `c3x explore --schema` prints the payload contract",
			len(issues), strings.Join(issues, "\n  - "))
	}
	initial, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("explore: marshal payload: %w", err)
	}

	shell, err := loadExplorerShell(w)
	if err != nil {
		return err
	}

	srv := &exploreServer{
		c3Dir:      opts.C3Dir,
		includeADR: opts.IncludeADR,
		shell:      shell,
		hub:        newSSEHub(string(initial)),
		w:          w,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleRoot)
	mux.HandleFunc("/events", srv.handleEvents)

	go srv.tailActivity()

	addr := fmt.Sprintf("127.0.0.1:%d", opts.Port)
	fmt.Fprintf(w, "Live explorer serving %d nodes at http://%s — every c3x command updates it (Ctrl-C to stop)\n",
		len(payload.Nodes), addr)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
		// SSE connections are long-lived; a write timeout would kill them.
		WriteTimeout:      0,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return server.ListenAndServe()
}

func (s *exploreServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	var payload explorePayload
	if err := json.Unmarshal([]byte(s.hub.getLatest()), &payload); err != nil {
		http.Error(w, "explore: cached payload corrupt: "+err.Error(), http.StatusInternalServerError)
		return
	}
	html, err := injectExplorerPayload(s.shell, payload, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.WriteString(w, html)
}

func (s *exploreServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.hub.add()
	defer s.hub.remove(ch)

	// First frame is always the current payload so a (re)connecting client
	// converges regardless of what it missed.
	_, _ = io.WriteString(w, sseFrame("payload", s.hub.getLatest()))
	fl.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case frame := <-ch:
			if _, err := io.WriteString(w, frame); err != nil {
				return
			}
			fl.Flush()
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

/* ─── activity tail → action events + rebuild trigger ─────────────── */

func (s *exploreServer) tailActivity() {
	path := filepath.Join(s.c3Dir, activityFileName)
	// Start at the current end: history before the server started is not replayed.
	var offset int64
	if info, err := os.Stat(path); err == nil {
		offset = info.Size()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		entries, newOffset := readNewActivity(path, offset)
		offset = newOffset
		rebuild := false
		for _, e := range entries {
			if data, err := json.Marshal(e); err == nil {
				s.hub.broadcast("action", string(data))
			}
			if e.Mutating && e.Success {
				rebuild = true
			}
		}
		if rebuild {
			s.scheduleRebuild()
		}
	}
}

func (s *exploreServer) scheduleRebuild() {
	s.rebuildMu.Lock()
	defer s.rebuildMu.Unlock()
	if s.rebuildTimer != nil {
		s.rebuildTimer.Stop()
	}
	s.rebuildTimer = time.AfterFunc(300*time.Millisecond, s.rebuildAndBroadcast)
}

func (s *exploreServer) rebuildAndBroadcast() {
	payload, err := s.rebuildPayload()
	if err != nil {
		issues, _ := json.Marshal(map[string][]string{"issues": {err.Error()}})
		s.hub.broadcast("invalid", string(issues))
		return
	}
	if problems := validateExplorePayload(payload); len(problems) > 0 {
		issues, _ := json.Marshal(map[string][]string{"issues": problems})
		s.hub.broadcast("invalid", string(issues))
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.hub.setLatest(string(data))
	s.hub.broadcast("payload", string(data))
}

func (s *exploreServer) rebuildPayload() (explorePayload, error) {
	if err := EnsureLocalCache(s.c3Dir, s.includeADR, nil, io.Discard); err != nil {
		return explorePayload{}, fmt.Errorf("refresh cache: %w", err)
	}
	st, err := store.Open(filepath.Join(s.c3Dir, "c3.db"))
	if err != nil {
		return explorePayload{}, fmt.Errorf("open store: %w", err)
	}
	defer st.Close()
	return buildExplorePayload(st, s.c3Dir, s.includeADR)
}
