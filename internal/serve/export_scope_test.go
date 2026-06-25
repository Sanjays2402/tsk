package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
)

// F75: ?ids=... narrows the export to exactly that subset ("export what you
// see"), in store order, skipping unknown ids. Absent ids => whole store.

// exportJSONTitles GETs /api/export?<query> and returns the task titles in the
// JSON array body, in order.
func exportJSONTitles(t *testing.T, h http.Handler, query string) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/export?format=json&"+query, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var arr []model.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &arr); err != nil {
		t.Fatalf("export json not an array: %v\n%s", err, rec.Body.String())
	}
	titles := make([]string, len(arr))
	for i, tk := range arr {
		titles[i] = tk.Title
	}
	return titles
}

func seedThree(t *testing.T, h http.Handler) {
	t.Helper()
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "alpha"})
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "bravo"})
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "charlie"})
}

func TestExportIDsSubset(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	seedThree(t, h)
	// Export only #1 and #3 -> alpha, charlie (in store order).
	got := exportJSONTitles(t, h, "ids=1,3")
	if len(got) != 2 || got[0] != "alpha" || got[1] != "charlie" {
		t.Fatalf("ids=1,3 export = %v, want [alpha charlie]", got)
	}
}

func TestExportIDsPreservesStoreOrderNotQueryOrder(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	seedThree(t, h)
	// Query lists ids out of order; the export must still be in STORE order.
	got := exportJSONTitles(t, h, "ids=3,1")
	if len(got) != 2 || got[0] != "alpha" || got[1] != "charlie" {
		t.Fatalf("ids=3,1 export = %v, want store order [alpha charlie]", got)
	}
}

func TestExportIDsSkipsUnknownAndMalformed(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	seedThree(t, h)
	// 999 doesn't exist, "x" is malformed; only #2 resolves.
	got := exportJSONTitles(t, h, "ids=2,999,x")
	if len(got) != 1 || got[0] != "bravo" {
		t.Fatalf("ids=2,999,x export = %v, want [bravo]", got)
	}
}

func TestExportNoIDsIsWholeStore(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	seedThree(t, h)
	got := exportJSONTitles(t, h, "")
	if len(got) != 3 {
		t.Fatalf("no-ids export = %v, want all 3", got)
	}
}

func TestExportEmptyIDsYieldsEmpty(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	seedThree(t, h)
	// ids present but empty -> you asked for a subset that resolved to nothing.
	req := httptest.NewRequest(http.MethodGet, "/api/export?format=json&ids=", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var arr []model.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &arr); err != nil {
		t.Fatalf("not an array: %v\n%s", err, rec.Body.String())
	}
	if len(arr) != 0 {
		t.Fatalf("empty ids export = %v, want []", arr)
	}
}

func TestExportIDsScopesCSVToo(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	seedThree(t, h)
	req := httptest.NewRequest(http.MethodGet, "/api/export?format=csv&ids=2", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "bravo") {
		t.Fatalf("csv subset missing bravo: %q", body)
	}
	if strings.Contains(body, "alpha") || strings.Contains(body, "charlie") {
		t.Fatalf("csv subset leaked other rows: %q", body)
	}
}

func TestFilterTasksByIDsUnit(t *testing.T) {
	tasks := []model.Task{
		{ID: 1, Title: "a"},
		{ID: 2, Title: "b"},
		{ID: 3, Title: "c"},
	}
	// in store order regardless of query order, skipping unknown/blank/malformed
	got := filterTasksByIDs(tasks, " 3 , 1 , 9 , , x ")
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Fatalf("filterTasksByIDs = %+v, want ids [1 3] in order", got)
	}
	if filterTasksByIDs(tasks, "") != nil {
		t.Fatalf("blank list should be nil")
	}
	if filterTasksByIDs(tasks, " , ,") != nil {
		t.Fatalf("all-blank list should be nil")
	}
}
