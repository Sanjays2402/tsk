package serve

import (
	"net/http"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

func TestPinToggleFlipsAndPersists(t *testing.T) {
	h, file := seedTwo(t)

	// Pin task 1.
	rec, body := do(t, h, http.MethodPost, "/api/tasks/1/pin", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("pin status = %d body=%s", rec.Code, rec.Body.String())
	}
	if body["pinned"] != true {
		t.Fatalf("pinned = %v, want true", body["pinned"])
	}

	st, err := store.Load(file)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !st.ByID(1).Pinned {
		t.Fatalf("disk task 1 not pinned")
	}

	// Toggle again -> unpinned (field omitted when false).
	rec, body = do(t, h, http.MethodPost, "/api/tasks/1/pin", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unpin status = %d", rec.Code)
	}
	if body["pinned"] != nil {
		t.Fatalf("pinned = %v, want omitted (false) after second toggle", body["pinned"])
	}
	st, _ = store.Load(file)
	if st.ByID(1).Pinned {
		t.Fatalf("disk task 1 still pinned after second toggle")
	}
}

func TestPinUnknownIDReturns404(t *testing.T) {
	h, _ := seedTwo(t)
	rec, body := do(t, h, http.MethodPost, "/api/tasks/999/pin", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if body["error"] == nil {
		t.Fatalf("expected error field: %v", body)
	}
}

func TestPinRejectsNonPost(t *testing.T) {
	h, _ := seedTwo(t)
	rec, _ := do(t, h, http.MethodGet, "/api/tasks/1/pin", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestPatchPinnedField(t *testing.T) {
	h, file := seedTwo(t)

	// PATCH can also set the pin (idempotent set vs the toggle endpoint).
	rec, body := do(t, h, http.MethodPatch, "/api/tasks/2", map[string]any{"pinned": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch pinned status = %d", rec.Code)
	}
	if body["pinned"] != true {
		t.Fatalf("pinned = %v, want true", body["pinned"])
	}
	st, _ := store.Load(file)
	if !st.ByID(2).Pinned {
		t.Fatalf("disk task 2 not pinned after PATCH")
	}

	// PATCH pinned:false clears it.
	rec, body = do(t, h, http.MethodPatch, "/api/tasks/2", map[string]any{"pinned": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch unpin status = %d", rec.Code)
	}
	if body["pinned"] != nil {
		t.Fatalf("pinned = %v, want omitted after clear", body["pinned"])
	}
}

func TestPinnedLeftUntouchedWhenOmitted(t *testing.T) {
	h, file := seedTwo(t)
	do(t, h, http.MethodPost, "/api/tasks/1/pin", nil) // pin it
	// A notes-only patch must not clear the pin.
	rec, _ := do(t, h, http.MethodPatch, "/api/tasks/1", map[string]any{"notes": "hello"})
	if rec.Code != http.StatusOK {
		t.Fatalf("notes patch status = %d", rec.Code)
	}
	st, _ := store.Load(file)
	if !st.ByID(1).Pinned {
		t.Fatalf("pin wiped by unrelated patch")
	}
}
