package tui

import (
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestReloadClearKeyClearsFilter: pressing 'R' (capital) clears
// any active search filter as part of the reload. Sister of
// lowercase 'r' which preserves the filter.
func TestReloadClearKeyClearsFilter(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Set an active filter directly (skips having to type through
	// the search form in the test).
	app.filter = "nonexistent-substring"
	if app.filter == "" {
		t.Fatal("baseline: filter should be set")
	}
	// Press 'R' — should clear the filter.
	feed(app, keyRune('R'))
	if app.filter != "" {
		t.Errorf("expected filter cleared after 'R', got %q", app.filter)
	}
}

// TestReloadKeyPreservesFilter: lowercase 'r' must NOT clear the
// filter — that's the whole point of having two keys. This is
// the regression guard.
func TestReloadKeyPreservesFilter(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	app.filter = "active-filter"
	feed(app, keyRune('r'))
	if app.filter != "active-filter" {
		t.Errorf("lowercase 'r' should preserve filter, got %q", app.filter)
	}
}

// TestReloadClearKeyStatusFooterDistinguishesAction: when the
// uppercase 'R' clears a filter, the footer says so — so the
// user can tell they hit uppercase by mistake (capslock, etc).
func TestReloadClearKeyStatusFooterDistinguishesAction(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	app.filter = "x"
	feed(app, keyRune('R'))
	if !strings.Contains(app.status, "filter cleared") {
		t.Errorf("expected status to mention 'filter cleared', got %q", app.status)
	}
}

// TestReloadClearKeyPicksUpExternalEdit: 'R' should still pull
// in external file mutations (it IS a reload after all, not
// just a filter reset).
func TestReloadClearKeyPicksUpExternalEdit(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	baseline := len(app.store.Tasks)
	// External add.
	externalStore, err := store.Load(app.store.Path)
	if err != nil {
		t.Fatalf("external load: %v", err)
	}
	externalStore.Add(model.Task{Title: "external-via-R", Priority: model.PriorityMedium})
	if err := externalStore.Save(); err != nil {
		t.Fatalf("external save: %v", err)
	}
	feed(app, keyRune('R'))
	if got := len(app.store.Tasks); got != baseline+1 {
		t.Errorf("expected %d tasks after 'R' reload, got %d", baseline+1, got)
	}
}

// TestReloadClearKeyNoFilterStillReloads: pressing 'R' when no
// filter is active is equivalent to lowercase 'r' — same reload
// semantics, no surprise change. Verified by no panic + status
// reflects a reload action.
func TestReloadClearKeyNoFilterStillReloads(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	if app.filter != "" {
		t.Fatal("baseline: filter should be empty")
	}
	feed(app, keyRune('R'))
	// Should mention "reload" in some form (either "reloaded" or
	// "reloaded (filter cleared)" — both are acceptable since
	// uppercase R DOES technically clear, even if no-op).
	if !strings.Contains(app.status, "reload") {
		t.Errorf("expected status to mention 'reload', got %q", app.status)
	}
}

// TestReloadClearKeyPreservesSelectionByID: same selection-by-id
// preservation as lowercase 'r' — the cursor follows the task,
// not the row index, across the reload.
func TestReloadClearKeyPreservesSelectionByID(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('j')) // move to 2nd task
	beforeID := app.currentID()
	if beforeID == 0 {
		t.Fatal("expected non-zero selection")
	}
	// External: add a new high-priority task (would shift indices
	// if we relied on index-based selection).
	externalStore, err := store.Load(app.store.Path)
	if err != nil {
		t.Fatalf("external load: %v", err)
	}
	externalStore.Add(model.Task{Title: "top-priority", Priority: model.PriorityUrgent})
	if err := externalStore.Save(); err != nil {
		t.Fatalf("external save: %v", err)
	}
	feed(app, keyRune('R'))
	if got := app.currentID(); got != beforeID {
		t.Errorf("selection should preserve by id: before=%d after=%d", beforeID, got)
	}
}
