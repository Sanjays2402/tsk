package serve

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// flushRecorder is a minimal http.ResponseWriter + http.Flusher whose buffer is
// guarded by a mutex, so an SSE handler streaming from one goroutine and a test
// reading the accumulated body from another never race.
type flushRecorder struct {
	mu     sync.Mutex
	hdr    http.Header
	buf    bytes.Buffer
	status int
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{hdr: make(http.Header), status: http.StatusOK}
}

func (f *flushRecorder) Header() http.Header { return f.hdr }

func (f *flushRecorder) WriteHeader(code int) {
	f.mu.Lock()
	f.status = code
	f.mu.Unlock()
}

func (f *flushRecorder) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.Write(p)
}

func (f *flushRecorder) Flush() {}

func (f *flushRecorder) body() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.String()
}

func TestStatSig(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".tsk.md")

	// Missing file: exists=false, zero fields.
	if sig := statSig(file); sig.exists {
		t.Fatalf("missing file should report exists=false, got %+v", sig)
	}

	if err := store.AtomicWriteFile(file, []byte("# t\n\n- [ ] a\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	a := statSig(file)
	if !a.exists {
		t.Fatalf("present file should report exists=true")
	}
	if a.size == 0 {
		t.Fatalf("present file should report non-zero size")
	}

	// Same content restatted: identical signature, sigChanged=false.
	if b := statSig(file); sigChanged(a, b) {
		t.Fatalf("unchanged file should not report a change: %+v vs %+v", a, b)
	}

	// Grow the file: size moves, sigChanged=true.
	if err := store.AtomicWriteFile(file, []byte("# t\n\n- [ ] a\n- [ ] b\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	c := statSig(file)
	if !sigChanged(a, c) {
		t.Fatalf("grown file should report a change: %+v vs %+v", a, c)
	}
}

func TestSigChanged(t *testing.T) {
	base := fileSig{mtimeUnixNano: 100, size: 10, exists: true}
	cases := []struct {
		name string
		othr fileSig
		want bool
	}{
		{"identical", fileSig{100, 10, true}, false},
		{"mtime moved", fileSig{200, 10, true}, true},
		{"size moved", fileSig{100, 20, true}, true},
		{"deleted", fileSig{0, 0, false}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sigChanged(base, c.othr); got != c.want {
				t.Fatalf("sigChanged(%+v,%+v) = %v, want %v", base, c.othr, got, c.want)
			}
		})
	}
}

func TestEventsRejectsNonGet(t *testing.T) {
	s, _ := newTestServer(t)
	rec, _ := do(t, s.Handler(), http.MethodPost, "/api/events", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/events = %d, want 405", rec.Code)
	}
}

// TestEventsStreamsReadyThenChange drives the SSE handler with a fast poll and
// a real file mutation, asserting it emits a ready frame immediately and a
// change frame once the .tsk.md fingerprint moves.
func TestEventsStreamsReadyThenChange(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".tsk.md")
	if err := store.AtomicWriteFile(file, []byte("# t\n\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s := New(Options{File: file, Now: func() time.Time { return time.Unix(0, 0) }, TZ: time.UTC})

	// Shrink the poll interval for the test, restore after.
	prev := eventsPollInterval
	eventsPollInterval = 5 * time.Millisecond
	defer func() { eventsPollInterval = prev }()

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := newFlushRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Handler().ServeHTTP(rec, req)
	}()

	// Give the handler a moment to emit "ready", then mutate the file so the
	// next poll sees a changed fingerprint.
	time.Sleep(20 * time.Millisecond)
	if err := store.AtomicWriteFile(file, []byte("# t\n\n- [ ] new\n"), 0o644); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	// Wait for the change to be observed and streamed.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.body(), "event: change") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Stop the handler and join it before the final read so nothing races.
	cancel()
	<-done

	body := rec.body()
	if !strings.Contains(body, "event: ready") {
		t.Fatalf("missing ready frame in stream:\n%s", body)
	}
	if !strings.Contains(body, "event: change") {
		t.Fatalf("missing change frame after file mutation:\n%s", body)
	}
	// The frames must be well-formed SSE (event:/data: pairs terminated blankly).
	sc := bufio.NewScanner(strings.NewReader(body))
	var sawData bool
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "data: {") {
			sawData = true
		}
	}
	if !sawData {
		t.Fatalf("no data: frame found in stream:\n%s", body)
	}
}

func TestEventsContentTypeHeaders(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".tsk.md")
	if err := store.AtomicWriteFile(file, []byte("# t\n\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s := New(Options{File: file, Now: func() time.Time { return time.Unix(0, 0) }, TZ: time.UTC})

	prev := eventsPollInterval
	eventsPollInterval = 5 * time.Millisecond
	defer func() { eventsPollInterval = prev }()

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := newFlushRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Handler().ServeHTTP(rec, req)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", cc)
	}
}
