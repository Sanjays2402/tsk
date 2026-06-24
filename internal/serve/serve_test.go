package serve

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// newTestServer wires a Server backed by a fresh .tsk.md in a temp dir.
// Returns the server, the file path, and a stable "now" anchored at
// 2026-06-24T12:00 UTC.
func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, ".tsk.md")
	if err := store.AtomicWriteFile(file, []byte("# test\n\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	s := New(Options{
		Addr: "127.0.0.1:0",
		File: file,
		Now:  func() time.Time { return now },
		TZ:   time.UTC,
	})
	return s, file
}

func do(t *testing.T, h http.Handler, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := map[string]any{}
	if rec.Body.Len() > 0 && strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			// list endpoint returns object too, but array fields land under keys.
			// Try array fallback for endpoints that return raw arrays.
			out = map[string]any{"_raw": string(rec.Body.Bytes())}
		}
	}
	return rec, out
}

func TestHealth(t *testing.T) {
	s, _ := newTestServer(t)
	rec, body := do(t, s.Handler(), http.MethodGet, "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
	if body["file"] == "" || body["file"] == nil {
		t.Fatalf("missing file in response: %v", body)
	}
}

func TestListEmpty(t *testing.T) {
	s, _ := newTestServer(t)
	rec, body := do(t, s.Handler(), http.MethodGet, "/api/tasks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	tasks, ok := body["tasks"].([]any)
	if !ok {
		t.Fatalf("tasks not an array: %v", body)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected empty list, got %d", len(tasks))
	}
}

func TestCreateListPatchToggleDelete(t *testing.T) {
	s, file := newTestServer(t)
	h := s.Handler()

	// CREATE
	rec, body := do(t, h, http.MethodPost, "/api/tasks", map[string]any{
		"title":    "write tests",
		"priority": "high",
		"tags":     []string{"dev", "tsk"},
		"due":      "tomorrow",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	id, ok := body["id"].(float64)
	if !ok || id < 1 {
		t.Fatalf("bad id in create response: %v", body)
	}
	if body["due"] != "2026-06-25" {
		t.Fatalf("due = %v, want 2026-06-25", body["due"])
	}
	if tags, _ := body["tags"].([]any); len(tags) != 2 {
		t.Fatalf("tags = %v, want [dev tsk]", body["tags"])
	}

	// Confirm written to disk in the markdown format.
	st, err := store.Load(file)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(st.Tasks) != 1 || st.Tasks[0].Title != "write tests" {
		t.Fatalf("disk store wrong: %+v", st.Tasks)
	}

	// LIST
	rec, body = do(t, h, http.MethodGet, "/api/tasks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	tasks := body["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("list len = %d, want 1", len(tasks))
	}

	// PATCH title + priority + due-clear
	emptyDue := ""
	rec, body = do(t, h, http.MethodPatch, "/api/tasks/1", map[string]any{
		"title":    "write more tests",
		"priority": "urgent",
		"due":      emptyDue,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", rec.Code, rec.Body.String())
	}
	if body["title"] != "write more tests" {
		t.Fatalf("title = %v", body["title"])
	}
	if body["priority"] != "urgent" {
		t.Fatalf("priority = %v", body["priority"])
	}
	if body["due"] != nil && body["due"] != "" {
		t.Fatalf("due = %v, expected cleared", body["due"])
	}

	// TOGGLE -> done
	rec, body = do(t, h, http.MethodPost, "/api/tasks/1/toggle", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d", rec.Code)
	}
	if body["done"] != true {
		t.Fatalf("done = %v, want true", body["done"])
	}
	if body["completed"] == nil || body["completed"] == "" {
		t.Fatalf("completed timestamp missing after toggle: %v", body)
	}

	// TOGGLE -> undone clears completed
	rec, body = do(t, h, http.MethodPost, "/api/tasks/1/toggle", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle back status = %d", rec.Code)
	}
	if body["done"] != false {
		t.Fatalf("done = %v, want false", body["done"])
	}

	// DELETE
	rec, body = do(t, h, http.MethodDelete, "/api/tasks/1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d", rec.Code)
	}
	if body["ok"] != true {
		t.Fatalf("delete body: %v", body)
	}

	// Confirm gone.
	st, _ = store.Load(file)
	if len(st.Tasks) != 0 {
		t.Fatalf("expected 0 tasks after delete, got %d", len(st.Tasks))
	}
}

func TestCreateRejectsEmptyTitle(t *testing.T) {
	s, _ := newTestServer(t)
	rec, body := do(t, s.Handler(), http.MethodPost, "/api/tasks", map[string]any{"title": "  "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if body["error"] == nil {
		t.Fatalf("expected error field: %v", body)
	}
}

func TestCreateRejectsBadDue(t *testing.T) {
	s, _ := newTestServer(t)
	rec, _ := do(t, s.Handler(), http.MethodPost, "/api/tasks", map[string]any{
		"title": "x", "due": "next groundhog day",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPatchUnknownIDReturns404(t *testing.T) {
	s, _ := newTestServer(t)
	rec, _ := do(t, s.Handler(), http.MethodPatch, "/api/tasks/999", map[string]any{"title": "x"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	rec, _ := do(t, s.Handler(), http.MethodDelete, "/api/tasks", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Allow"), http.MethodGet) {
		t.Fatalf("Allow header missing GET: %q", rec.Header().Get("Allow"))
	}
}

func TestStatsSchema(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	// Seed a couple of tasks.
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "a", "priority": "high", "tags": []string{"x"}})
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "b", "tags": []string{"x", "y"}})
	do(t, h, http.MethodPost, "/api/tasks/1/toggle", nil)

	rec, body := do(t, h, http.MethodGet, "/api/stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body["total"].(float64) != 2 {
		t.Fatalf("total = %v, want 2", body["total"])
	}
	if body["done"].(float64) != 1 {
		t.Fatalf("done = %v, want 1", body["done"])
	}
	if _, ok := body["top_tags"].([]any); !ok {
		t.Fatalf("top_tags should be array, got %T", body["top_tags"])
	}
}

func TestPlaceholderHTMLServedWhenNoSPA(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tsk serve") {
		t.Fatalf("body missing 'tsk serve': %q", rec.Body.String())
	}
}

func TestInvalidJSONReturns400(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestInvalidIDReturns400(t *testing.T) {
	s, _ := newTestServer(t)
	rec, _ := do(t, s.Handler(), http.MethodGet, "/api/tasks/abc", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestParseDateValid(t *testing.T) {
	s, _ := newTestServer(t) // now anchored at 2026-06-24 (a Wednesday) UTC
	h := s.Handler()

	cases := []struct {
		q        string
		date     string
		relative string
	}{
		{"today", "2026-06-24", "today"},
		{"tomorrow", "2026-06-25", "tomorrow"},
		{"2026-07-04", "2026-07-04", "in 10d"},
		{"in 3d", "2026-06-27", "in 3d"},
	}
	for _, c := range cases {
		rec, body := do(t, h, http.MethodGet, "/api/parse-date?q="+url.QueryEscape(c.q), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("q=%q status = %d body=%s", c.q, rec.Code, rec.Body.String())
		}
		if body["ok"] != true {
			t.Fatalf("q=%q ok = %v, want true (body=%v)", c.q, body["ok"], body)
		}
		if body["date"] != c.date {
			t.Fatalf("q=%q date = %v, want %s", c.q, body["date"], c.date)
		}
		if body["relative"] != c.relative {
			t.Fatalf("q=%q relative = %v, want %s", c.q, body["relative"], c.relative)
		}
		if body["pretty"] == "" || body["pretty"] == nil {
			t.Fatalf("q=%q missing pretty label: %v", c.q, body)
		}
	}
}

func TestParseDateInvalidIsSoft200(t *testing.T) {
	s, _ := newTestServer(t)
	// An unparseable string is a soft failure (200 + ok:false), not a 400, so
	// the live picker doesn't spam errors while you're still typing.
	rec, body := do(t, s.Handler(), http.MethodGet, "/api/parse-date?q=next+groundhog+day", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (soft fail)", rec.Code)
	}
	if body["ok"] != false {
		t.Fatalf("ok = %v, want false", body["ok"])
	}
	if body["error"] == nil || body["error"] == "" {
		t.Fatalf("expected error field on soft fail: %v", body)
	}
}

func TestParseDateMissingQueryReturns400(t *testing.T) {
	s, _ := newTestServer(t)
	rec, _ := do(t, s.Handler(), http.MethodGet, "/api/parse-date", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestParseDateMethodNotAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	rec, _ := do(t, s.Handler(), http.MethodPost, "/api/parse-date?q=today", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestMoveReordersAndPersists(t *testing.T) {
	s, file := newTestServer(t)
	h := s.Handler()
	// Seed three tasks: ids 1,2,3 in creation order.
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "alpha"})
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "bravo"})
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "charlie"})

	// Drag #3 (charlie) to sit before #1 (alpha) -> charlie, alpha, bravo.
	rec, body := do(t, h, http.MethodPost, "/api/tasks/3/move", map[string]any{"before": 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("move status = %d body=%s", rec.Code, rec.Body.String())
	}
	tasks, ok := body["tasks"].([]any)
	if !ok || len(tasks) != 3 {
		t.Fatalf("move response tasks bad: %v", body)
	}
	first := tasks[0].(map[string]any)
	if first["title"] != "charlie" {
		t.Fatalf("after move, first title = %v, want charlie", first["title"])
	}

	// Confirm the new order is on disk in file order.
	st, err := store.Load(file)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(st.Tasks) != 3 || st.Tasks[0].Title != "charlie" || st.Tasks[1].Title != "alpha" {
		t.Fatalf("disk order wrong: %+v", st.Tasks)
	}
}

func TestMoveToEndWithZeroBefore(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "a"})
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "b"})
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "c"})

	// before:0 means move #1 (a) to the very end -> b, c, a.
	rec, body := do(t, h, http.MethodPost, "/api/tasks/1/move", map[string]any{"before": 0})
	if rec.Code != http.StatusOK {
		t.Fatalf("move status = %d", rec.Code)
	}
	tasks := body["tasks"].([]any)
	last := tasks[len(tasks)-1].(map[string]any)
	if last["title"] != "a" {
		t.Fatalf("after move-to-end, last title = %v, want a", last["title"])
	}
}

func TestMoveUnknownIDReturns404(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "only"})
	rec, _ := do(t, h, http.MethodPost, "/api/tasks/99/move", map[string]any{"before": 1})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestMoveMethodNotAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	rec, _ := do(t, s.Handler(), http.MethodGet, "/api/tasks/1/move", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
