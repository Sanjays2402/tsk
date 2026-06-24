package serve

import (
	"net/http"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// seedTwo creates two tasks (ids 1 and 2) on a fresh server and returns the
// server handler + file path so dependency/pin tests can build on a known base.
func seedTwo(t *testing.T) (http.Handler, string) {
	t.Helper()
	s, file := newTestServer(t)
	h := s.Handler()
	for _, title := range []string{"alpha", "bravo"} {
		rec, _ := do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": title})
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed %q: status %d", title, rec.Code)
		}
	}
	return h, file
}

func TestPatchDependsOnSetsAndPersists(t *testing.T) {
	h, file := seedTwo(t)

	// Task 2 depends on task 1.
	rec, body := do(t, h, http.MethodPatch, "/api/tasks/2", map[string]any{
		"depends_on": []int{1},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch depends_on status = %d body=%s", rec.Code, rec.Body.String())
	}
	deps, ok := body["depends_on"].([]any)
	if !ok || len(deps) != 1 || deps[0].(float64) != 1 {
		t.Fatalf("depends_on = %v, want [1]", body["depends_on"])
	}

	// Persisted to disk in the depends: meta key.
	st, err := store.Load(file)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	tk := st.ByID(2)
	if tk == nil || len(tk.DependsOn) != 1 || tk.DependsOn[0] != 1 {
		t.Fatalf("disk DependsOn = %v, want [1]", tk)
	}

	// LIST surfaces it too.
	_, list := do(t, h, http.MethodGet, "/api/tasks", nil)
	tasks := list["tasks"].([]any)
	var found bool
	for _, raw := range tasks {
		m := raw.(map[string]any)
		if m["id"].(float64) == 2 {
			d := m["depends_on"].([]any)
			if len(d) != 1 || d[0].(float64) != 1 {
				t.Fatalf("list depends_on = %v", m["depends_on"])
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("task 2 not in list")
	}
}

func TestPatchDependsOnRejectsSelf(t *testing.T) {
	h, _ := seedTwo(t)
	rec, body := do(t, h, http.MethodPatch, "/api/tasks/1", map[string]any{
		"depends_on": []int{1},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self-dep status = %d, want 400", rec.Code)
	}
	if body["error"] == nil {
		t.Fatalf("expected error field for self-dep: %v", body)
	}
}

func TestPatchDependsOnRejectsUnknownID(t *testing.T) {
	h, _ := seedTwo(t)
	rec, body := do(t, h, http.MethodPatch, "/api/tasks/1", map[string]any{
		"depends_on": []int{999},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown-dep status = %d, want 400", rec.Code)
	}
	if body["error"] == nil {
		t.Fatalf("expected error field for unknown dep: %v", body)
	}
}

func TestPatchDependsOnDedupesPreservingOrder(t *testing.T) {
	h, file := seedTwo(t)
	// Add a third task so we have 1,2,3.
	do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "charlie"})

	rec, body := do(t, h, http.MethodPatch, "/api/tasks/3", map[string]any{
		"depends_on": []int{2, 1, 2, 1},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	deps := body["depends_on"].([]any)
	if len(deps) != 2 || deps[0].(float64) != 2 || deps[1].(float64) != 1 {
		t.Fatalf("depends_on = %v, want [2 1] (deduped, order preserved)", body["depends_on"])
	}
	st, _ := store.Load(file)
	if got := st.ByID(3).DependsOn; len(got) != 2 || got[0] != 2 || got[1] != 1 {
		t.Fatalf("disk DependsOn = %v, want [2 1]", got)
	}
}

func TestPatchDependsOnEmptyClears(t *testing.T) {
	h, file := seedTwo(t)
	// Set then clear.
	do(t, h, http.MethodPatch, "/api/tasks/2", map[string]any{"depends_on": []int{1}})
	rec, body := do(t, h, http.MethodPatch, "/api/tasks/2", map[string]any{"depends_on": []int{}})
	if rec.Code != http.StatusOK {
		t.Fatalf("clear status = %d", rec.Code)
	}
	if body["depends_on"] != nil {
		t.Fatalf("depends_on = %v, want omitted after clear", body["depends_on"])
	}
	st, _ := store.Load(file)
	if len(st.ByID(2).DependsOn) != 0 {
		t.Fatalf("disk DependsOn not cleared: %v", st.ByID(2).DependsOn)
	}
}

func TestPatchDependsOnLeftUntouchedWhenOmitted(t *testing.T) {
	h, file := seedTwo(t)
	do(t, h, http.MethodPatch, "/api/tasks/2", map[string]any{"depends_on": []int{1}})
	// A title-only patch must not wipe the deps.
	rec, _ := do(t, h, http.MethodPatch, "/api/tasks/2", map[string]any{"title": "bravo renamed"})
	if rec.Code != http.StatusOK {
		t.Fatalf("title patch status = %d", rec.Code)
	}
	st, _ := store.Load(file)
	if got := st.ByID(2).DependsOn; len(got) != 1 || got[0] != 1 {
		t.Fatalf("deps wiped by unrelated patch: %v", got)
	}
}
