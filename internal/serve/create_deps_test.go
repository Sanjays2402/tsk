package serve

import (
	"net/http"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// F38: the web composer's `depends:#N` token sets blockers at creation time via
// the create endpoint's optional depends_on field. These tests cover the happy
// path, validation (unknown id), and that omitting the field is a no-op.

func TestCreateWithDependsOnSetsBlockers(t *testing.T) {
	h, file := seedTwo(t) // ids 1, 2

	rec, body := do(t, h, http.MethodPost, "/api/tasks", map[string]any{
		"title":      "charlie",
		"depends_on": []int{1, 2},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	deps, ok := body["depends_on"].([]any)
	if !ok || len(deps) != 2 || deps[0].(float64) != 1 || deps[1].(float64) != 2 {
		t.Fatalf("depends_on = %v, want [1 2]", body["depends_on"])
	}

	// Persisted to disk in the depends: meta key.
	st, err := store.Load(file)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	tk := st.ByID(3)
	if tk == nil || len(tk.DependsOn) != 2 || tk.DependsOn[0] != 1 || tk.DependsOn[1] != 2 {
		t.Fatalf("disk DependsOn = %v, want [1 2]", tk)
	}
}

func TestCreateWithDependsOnDedupesPreservingOrder(t *testing.T) {
	h, _ := seedTwo(t) // ids 1, 2
	_, body := do(t, h, http.MethodPost, "/api/tasks", map[string]any{
		"title":      "charlie",
		"depends_on": []int{2, 1, 2, 1},
	})
	deps := body["depends_on"].([]any)
	if len(deps) != 2 || deps[0].(float64) != 2 || deps[1].(float64) != 1 {
		t.Fatalf("depends_on = %v, want [2 1] (deduped, order preserved)", body["depends_on"])
	}
}

func TestCreateWithUnknownDependsOnRejectsAndRollsBack(t *testing.T) {
	h, file := seedTwo(t) // ids 1, 2

	rec, respBody := do(t, h, http.MethodPost, "/api/tasks", map[string]any{
		"title":      "charlie",
		"depends_on": []int{999},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown-dep create status = %d, want 400", rec.Code)
	}
	if respBody["error"] == nil {
		t.Fatalf("expected error field for unknown dep: %v", respBody)
	}

	// Roll-back: the partially-added task must NOT linger in the store.
	st, err := store.Load(file)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(st.Tasks) != 2 {
		t.Fatalf("store has %d tasks after failed create, want 2 (rolled back)", len(st.Tasks))
	}
	if st.ByID(3) != nil {
		t.Fatalf("task 3 should not exist after a rejected create")
	}
}

func TestCreateWithoutDependsOnHasNone(t *testing.T) {
	h, _ := seedTwo(t)
	_, body := do(t, h, http.MethodPost, "/api/tasks", map[string]any{"title": "charlie"})
	if body["depends_on"] != nil {
		t.Fatalf("depends_on = %v, want omitted when not requested", body["depends_on"])
	}
}
