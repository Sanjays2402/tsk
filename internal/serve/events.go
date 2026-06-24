package serve

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// fileSig is a lightweight fingerprint of the .tsk.md used to detect external
// edits (CLI / TUI / hand-edit / another tab) without a full parse. mtime + size
// + existence together catch every real write store.Save performs: its atomic
// replace bumps mtime, content edits move size, and whole-file create/delete
// flips existence. Cheap enough to poll once a second forever.
type fileSig struct {
	mtimeUnixNano int64
	size          int64
	exists        bool
}

// statSig fingerprints the file at path. A missing file is a valid state
// (exists=false) so creating or deleting the whole store also registers as a
// change rather than erroring the stream.
func statSig(path string) fileSig {
	fi, err := os.Stat(path)
	if err != nil {
		return fileSig{exists: false}
	}
	return fileSig{mtimeUnixNano: fi.ModTime().UnixNano(), size: fi.Size(), exists: true}
}

// sigChanged reports whether two fingerprints differ in any field.
func sigChanged(a, b fileSig) bool { return a != b }

// eventsPollInterval is how often the SSE handler restats the file. Small
// enough to feel live, large enough to be effectively free. Package-level so
// tests can shrink it; production never touches it.
var eventsPollInterval = 1 * time.Second

// heartbeatEvery is how many idle poll ticks pass between keep-alive comments.
// At the 1s default that is a comment roughly every 15s, which keeps browsers
// and any intermediary proxies from idling the connection out.
const heartbeatEvery = 15

// handleEvents streams Server-Sent Events whenever the underlying .tsk.md
// changes on disk (F21), so a connected web client refreshes itself when the
// file is edited by the CLI, TUI, a second browser tab, or a text editor.
//
// It holds no lock and never writes: each tick it restats the file and, when
// the fingerprint moves, emits a "change" event carrying the new mtime/size.
// The design is deliberately local-first and dependency-free — no fsnotify,
// just a cheap os.Stat poll. EventSource on the client reconnects on its own,
// so a server restart is self-healing.
//
// Auth: this route sits behind requireAuth like every other /api/* endpoint.
// EventSource cannot set an Authorization header, but it sends the same-origin
// tsk_token cookie the ?token= bootstrap installs, so token-gated servers work
// transparently once the browser session is established.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // tell any proxy not to buffer the stream

	path := s.resolveFile()
	last := statSig(path)

	// Initial hello so the client can flip its indicator to "live" at once and
	// learn the baseline fingerprint without waiting a whole poll interval.
	fmt.Fprintf(w, "event: ready\ndata: {\"mtime\":%d,\"size\":%d}\n\n", last.mtimeUnixNano, last.size)
	flusher.Flush()

	ticker := time.NewTicker(eventsPollInterval)
	defer ticker.Stop()
	ctx := r.Context()
	idle := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cur := statSig(path)
			if sigChanged(last, cur) {
				last = cur
				idle = 0
				fmt.Fprintf(w, "event: change\ndata: {\"mtime\":%d,\"size\":%d}\n\n", cur.mtimeUnixNano, cur.size)
				flusher.Flush()
				continue
			}
			idle++
			if idle >= heartbeatEvery {
				idle = 0
				fmt.Fprint(w, ": keep-alive\n\n")
				flusher.Flush()
			}
		}
	}
}
