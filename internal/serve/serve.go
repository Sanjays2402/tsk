// Package serve hosts the JSON HTTP API and embedded web SPA for tsk.
//
// It is a deliberately thin layer over internal/store: every request reads
// (and, where needed, writes) the on-disk .tsk.md so concurrent edits from
// the TUI, CLI, or a user's text editor are picked up. There is no in-memory
// cache, no background watcher, and no separate persistence layer. The .tsk.md
// remains the single source of truth.
//
// The package is split into three layers:
//
//   - server: wires routes, owns the dependencies (file path resolver, now())
//   - handlers: per-route logic, plain http.HandlerFunc methods on Server
//   - dto: JSON shapes returned to / accepted from the web client
//
// All responses use JSON with a stable schema. Errors return a JSON body of
// the form {"error": "..."} with an appropriate status code.
package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// Options configures a Server. Zero values pick sensible defaults.
type Options struct {
	// Addr is the host:port to bind. Defaults to "127.0.0.1:7878".
	Addr string
	// File is an absolute path to the .tsk.md to operate on. If empty, the
	// server resolves the nearest one at request time (same logic as the CLI).
	File string
	// Now is injectable for tests. Defaults to time.Now.
	Now func() time.Time
	// TZ is the location natural-language dates are interpreted in. Defaults
	// to time.Local when nil.
	TZ *time.Location
	// Token, when non-empty, gates every /api/* route behind bearer-token
	// auth (Authorization: Bearer <token>, a tsk_token cookie, or a one-time
	// ?token= bootstrap that sets the cookie). Empty = no auth (loopback
	// default). Only set this when binding off 127.0.0.1.
	Token string
	// StaticFS optionally serves a pre-built SPA at "/". When nil, the server
	// serves a minimal placeholder page at "/" so the API still works.
	StaticFS fs.FS
}

// Server holds the wiring for an HTTP server instance.
type Server struct {
	opts Options
	mux  *http.ServeMux
}

// New builds a Server with routes wired but not yet listening.
func New(opts Options) *Server {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:7878"
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.TZ == nil {
		opts.TZ = time.Local
	}
	s := &Server{opts: opts, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler exposes the underlying http.Handler (for tests + composition).
func (s *Server) Handler() http.Handler { return s.mux }

// Addr returns the configured bind address.
func (s *Server) Addr() string { return s.opts.Addr }

// ListenAndServe binds to opts.Addr and blocks until ctx is cancelled or the
// listener errors. Closing the context triggers a graceful shutdown with a
// short grace period.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.opts.Addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.opts.Addr, err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// routes wires the HTTP surface. Keep it small and readable.
func (s *Server) routes() {
	// API routes are gated by requireAuth (a no-op when no token is set).
	s.mux.Handle("/api/health", s.requireAuth(http.HandlerFunc(s.handleHealth)))
	s.mux.Handle("/api/tasks", s.requireAuth(http.HandlerFunc(s.handleTasks)))
	s.mux.Handle("/api/tasks/", s.requireAuth(http.HandlerFunc(s.handleTaskByID)))
	s.mux.Handle("/api/stats", s.requireAuth(http.HandlerFunc(s.handleStats)))
	s.mux.Handle("/api/parse-date", s.requireAuth(http.HandlerFunc(s.handleParseDate)))
	s.mux.Handle("/api/export", s.requireAuth(http.HandlerFunc(s.handleExport)))
	s.mux.Handle("/api/events", s.requireAuth(http.HandlerFunc(s.handleEvents)))

	// Root: serve SPA when StaticFS is provided, otherwise a tiny landing page.
	// The root handler also performs the one-time ?token= cookie bootstrap so a
	// browser can authenticate by visiting the link `tsk serve` prints.
	if s.opts.StaticFS != nil {
		s.mux.Handle("/", s.tokenBootstrap(http.FileServer(http.FS(s.opts.StaticFS))))
	} else {
		s.mux.Handle("/", s.tokenBootstrap(http.HandlerFunc(s.handlePlaceholder)))
	}
}

// --- JSON helpers --------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// resolveFile returns the .tsk.md path the server should operate on this
// request. When opts.File is set, it's used as-is. Otherwise the nearest
// .tsk.md is resolved from the working directory; if none exists, the path
// is the would-be location (so a first POST can create it).
func (s *Server) resolveFile() string {
	if s.opts.File != "" {
		return s.opts.File
	}
	return store.ResolveOrCreate("")
}

// loadStore opens the .tsk.md fresh for this request. Returning an error here
// lets handlers send a sensible 500.
func (s *Server) loadStore() (*store.Store, error) {
	return store.Load(s.resolveFile())
}

// --- handlers ------------------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"file": s.resolveFile(),
		"now":  s.opts.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listTasks(w, r)
	case http.MethodPost:
		s.createTask(w, r)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	// Path shapes:
	//   /api/tasks/123          PATCH | DELETE
	//   /api/tasks/123/toggle   POST
	rest := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if rest == "" {
		writeError(w, http.StatusNotFound, "missing task id")
		return
	}
	parts := strings.Split(rest, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	if len(parts) == 2 && parts[1] == "toggle" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		s.toggleTask(w, id)
		return
	}
	if len(parts) == 2 && parts[1] == "move" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		s.moveTask(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "pin" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		s.togglePin(w, id)
		return
	}
	if len(parts) != 1 {
		writeError(w, http.StatusNotFound, "unknown task subroute")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getTask(w, id)
	case http.MethodPatch:
		s.updateTask(w, r, id)
	case http.MethodDelete:
		s.deleteTask(w, id)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPatch, http.MethodDelete)
	}
}

func (s *Server) listTasks(w http.ResponseWriter, _ *http.Request) {
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"file":  st.Path,
		"tasks": tasksToDTO(st.Tasks),
	})
}

func (s *Server) getTask(w http.ResponseWriter, id int) {
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	t := st.ByID(id)
	if t == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("no task with id %d", id))
		return
	}
	writeJSON(w, http.StatusOK, taskToDTO(*t))
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var in taskInputDTO
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	prio, err := model.ParsePriority(strings.TrimSpace(in.Priority))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	task := model.Task{
		Title:    title,
		Priority: prio,
		Tags:     in.Tags,
		Notes:    strings.TrimSpace(in.Notes),
		Created:  s.opts.Now(),
	}
	if strings.TrimSpace(in.Due) != "" {
		t, err := dateparse.Parse(in.Due, s.opts.Now(), s.opts.TZ)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		task.Due = &t
	}
	id := st.Add(task)
	if err := st.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// F38: optional depends_on set at creation time (the composer's
	// `depends:#N` token). Validate against the now-saved store so the new
	// task's own id is known to sanitizeDeps, then persist again. A bad dep id
	// fails the whole create with 400 (the task was already added, so roll it
	// back by removing it to keep create atomic from the client's view).
	if len(in.DependsOn) > 0 {
		deps, err := sanitizeDeps(st, id, in.DependsOn)
		if err != nil {
			st.Remove(id)
			_ = st.Save()
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if t := st.ByID(id); t != nil {
			t.DependsOn = deps
			if err := st.Save(); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}
	created := st.ByID(id)
	if created == nil {
		writeError(w, http.StatusInternalServerError, "task disappeared after save")
		return
	}
	writeJSON(w, http.StatusCreated, taskToDTO(*created))
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request, id int) {
	var in taskPatchDTO
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	t := st.ByID(id)
	if t == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("no task with id %d", id))
		return
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			writeError(w, http.StatusBadRequest, "title cannot be empty")
			return
		}
		t.Title = title
	}
	if in.Priority != nil {
		p, err := model.ParsePriority(*in.Priority)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		t.Priority = p
	}
	if in.Due != nil {
		raw := strings.TrimSpace(*in.Due)
		if raw == "" {
			t.Due = nil
		} else {
			parsed, err := dateparse.Parse(raw, s.opts.Now(), s.opts.TZ)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			t.Due = &parsed
		}
	}
	if in.Tags != nil {
		t.Tags = *in.Tags
		t.NormalizeTags()
	}
	if in.Notes != nil {
		t.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.Pinned != nil {
		t.Pinned = *in.Pinned
	}
	if in.DependsOn != nil {
		deps, err := sanitizeDeps(st, id, *in.DependsOn)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		t.DependsOn = deps
	}
	if in.Done != nil {
		st.SetDone(id, *in.Done)
	}
	if err := st.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, taskToDTO(*st.ByID(id)))
}

func (s *Server) toggleTask(w http.ResponseWriter, id int) {
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	t := st.ByID(id)
	if t == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("no task with id %d", id))
		return
	}
	st.SetDone(id, !t.Done)
	if err := st.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, taskToDTO(*st.ByID(id)))
}

// togglePin flips the sticky `Pinned` flag (F27) on a single task and persists
// it as `pin:true` in the .tsk.md metadata comment — the same flag the CLI's
// `tsk pin`/`tsk unpin` and the TUI's pin toggle read. Returns the updated
// task so the client can re-render authoritatively.
func (s *Server) togglePin(w http.ResponseWriter, id int) {
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	t := st.ByID(id)
	if t == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("no task with id %d", id))
		return
	}
	t.Pinned = !t.Pinned
	if err := st.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, taskToDTO(*st.ByID(id)))
}

// sanitizeDeps validates and normalizes a proposed DependsOn set for task
// `self` (F26): it drops duplicates, refuses self-references, and rejects any
// id that doesn't exist in the store. The returned slice preserves first-seen
// order so a hand-curated chain keeps its intended reading order. A nil/empty
// input yields a nil slice (clearing the deps), which round-trips to no
// `depends:` key in the file.
func sanitizeDeps(st *store.Store, self int, raw []int) ([]int, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	seen := make(map[int]struct{}, len(raw))
	out := make([]int, 0, len(raw))
	for _, dep := range raw {
		if dep == self {
			return nil, fmt.Errorf("a task cannot depend on itself (#%d)", self)
		}
		if _, dup := seen[dep]; dup {
			continue
		}
		if st.ByID(dep) == nil {
			return nil, fmt.Errorf("no task with id %d to depend on", dep)
		}
		seen[dep] = struct{}{}
		out = append(out, dep)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// moveTask repositions task `id` to sit immediately before the task named in
// the request body's "before" field (0 = move to the end), persisting the new
// order to .tsk.md. The slice order IS the file order, so a drag-to-reorder in
// the web UI round-trips straight through here. Returns the full reordered
// list so the client can re-render authoritatively without a second fetch.
func (s *Server) moveTask(w http.ResponseWriter, r *http.Request, id int) {
	var in moveInputDTO
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !st.Move(id, in.Before) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("cannot move: unknown task id (%d before %d)", id, in.Before))
		return
	}
	if err := st.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"file":  st.Path,
		"tasks": tasksToDTO(st.Tasks),
	})
}

func (s *Server) deleteTask(w http.ResponseWriter, id int) {
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !st.Remove(id) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("no task with id %d", id))
		return
	}
	if err := st.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := s.opts.Now()
	writeJSON(w, http.StatusOK, computeStatsDTO(st.Tasks, now))
}

// handleParseDate validates a natural-language date string (today, fri, in 3d,
// eow, 2026-07-04, ...) against the same dateparse package the CLI and the
// create/update handlers use, and echoes back the resolved YYYY-MM-DD plus a
// short relative label. It is read-only (no store access) so the F12 due-date
// picker can give live "this resolves to Sat, Jul 4" feedback as you type.
func (s *Server) handleParseDate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	raw := strings.TrimSpace(r.URL.Query().Get("q"))
	if raw == "" {
		writeError(w, http.StatusBadRequest, "missing date query (?q=)")
		return
	}
	now := s.opts.Now()
	parsed, err := dateparse.Parse(raw, now, s.opts.TZ)
	if err != nil {
		// 200 with ok:false keeps this a non-exceptional "you're still typing"
		// signal rather than a console-spamming 400 on every keystroke.
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"input": raw,
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"input":    raw,
		"date":     parsed.Format(model.DateLayout),
		"weekday":  parsed.Format("Mon"),
		"pretty":   parsed.Format("Mon, Jan 2 2006"),
		"relative": relativeDayLabel(parsed, now),
	})
}

// relativeDayLabel renders a short human delta ("today", "tomorrow", "in 3d",
// "5d ago") between a target date and now, matching the web list's formatDue.
func relativeDayLabel(target, now time.Time) string {
	t0 := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, target.Location())
	n0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// Round rather than truncate so a 23h/25h DST day still lands on the right delta.
	days := int(math.Round(t0.Sub(n0).Hours() / 24))
	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	case days == -1:
		return "yesterday"
	case days < 0:
		return fmt.Sprintf("%dd ago", -days)
	default:
		return fmt.Sprintf("in %dd", days)
	}
}

// handlePlaceholder is served when no embedded SPA is wired (e.g. the binary
// was built without npm). It tells the user where to look.
func (s *Server) handlePlaceholder(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, placeholderHTML)
}

const placeholderHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>tsk serve</title>
<style>body{font-family:ui-monospace,monospace;background:#0b0a09;color:#fbbf24;padding:3rem;max-width:42rem;margin:auto;line-height:1.6}
a{color:#f59e0b}code{background:#1c1917;padding:0.1em 0.4em;border-radius:3px}</style>
</head><body>
<h1>tsk serve</h1>
<p>JSON API is live. The bundled web UI was not built into this binary
(<code>web/dist/</code> was empty at <code>go build</code> time).</p>
<p>Try:</p>
<ul>
  <li><a href="/api/health">/api/health</a></li>
  <li><a href="/api/tasks">/api/tasks</a></li>
  <li><a href="/api/stats">/api/stats</a></li>
</ul>
<p>To build the SPA: <code>npm --prefix web install &amp;&amp; npm --prefix web run build &amp;&amp; go build ./...</code></p>
</body></html>`

// decodeJSON reads a JSON body into dst with a sensible size cap.
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MiB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return nil
}

// stderr writer kept for symmetry with future logging hooks.
var _ = os.Stderr
