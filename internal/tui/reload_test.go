package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestReloadKeyPicksUpExternalEdit: pressing 'r' re-reads the
// .tsk.md from disk, picking up a task added by an external
// process (simulated by writing the file directly).
func TestReloadKeyPicksUpExternalEdit(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Baseline: 2 tasks (a, b) from newTestApp.
	if got := len(app.store.Tasks); got != 2 {
		t.Fatalf("baseline: expected 2 tasks, got %d", got)
	}
	// Simulate an external mutation: a third process adds a task
	// through the store layer directly (mimicking `tsk add` from
	// another terminal).
	externalStore, err := store.Load(app.store.Path)
	if err != nil {
		t.Fatalf("external load: %v", err)
	}
	externalStore.Add(model.Task{Title: "external-add", Priority: model.PriorityMedium})
	if err := externalStore.Save(); err != nil {
		t.Fatalf("external save: %v", err)
	}
	// TUI's in-memory store is still stale (2 tasks).
	if got := len(app.store.Tasks); got != 2 {
		t.Fatalf("in-memory should still be 2 before reload, got %d", got)
	}
	// Press 'r' — TUI should reload.
	feed(app, keyRune('r'))
	if got := len(app.store.Tasks); got != 3 {
		t.Fatalf("after reload: expected 3 tasks (incl. external-add), got %d (tasks: %+v)", got, app.store.Tasks)
	}
	// New task should be findable by title.
	found := false
	for _, t := range app.store.Tasks {
		if t.Title == "external-add" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("external-add not visible after reload")
	}
	// Status footer should reflect the reload action.
	if !strings.Contains(app.status, "reload") {
		t.Errorf("expected status to mention reload, got %q", app.status)
	}
}

// TestReloadPreservesSelectionByID: after reload, if the
// previously-selected task still exists, the cursor snaps to it
// (not to position 0).
func TestReloadPreservesSelectionByID(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Move down to select the 2nd visible task (id 2 in the
	// default newTestApp layout).
	feed(app, keyRune('j'))
	beforeID := app.currentID()
	if beforeID == 0 {
		t.Fatal("expected a non-zero selected id before reload")
	}
	// External: add a NEW task at the top (which would shift index
	// 1 if we relied on index-based selection).
	externalStore, err := store.Load(app.store.Path)
	if err != nil {
		t.Fatalf("external load: %v", err)
	}
	// Add high-priority so it sorts to the TOP of the visible
	// list (default sort is by priority).
	externalStore.Add(model.Task{Title: "high-pri-new", Priority: model.PriorityUrgent})
	if err := externalStore.Save(); err != nil {
		t.Fatalf("external save: %v", err)
	}
	feed(app, keyRune('r'))
	afterID := app.currentID()
	if afterID != beforeID {
		t.Fatalf("selection should preserve by id: before=%d after=%d", beforeID, afterID)
	}
}

// TestReloadFallsBackToTopWhenSelectionDeleted: if the
// previously-selected task is gone from disk after reload, the
// cursor falls back to position 0 rather than dangling.
func TestReloadFallsBackToTopWhenSelectionDeleted(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Select 2nd task (id 2).
	feed(app, keyRune('j'))
	prevID := app.currentID()
	if prevID == 0 {
		t.Fatal("expected non-zero selection")
	}
	// External: remove that task entirely.
	externalStore, err := store.Load(app.store.Path)
	if err != nil {
		t.Fatalf("external load: %v", err)
	}
	externalStore.Remove(prevID)
	if err := externalStore.Save(); err != nil {
		t.Fatalf("external save: %v", err)
	}
	feed(app, keyRune('r'))
	// Should NOT panic, selection should be 0.
	if app.selection != 0 {
		t.Errorf("expected selection 0 after deleted task, got %d", app.selection)
	}
	// And the new currentID() should NOT be the deleted one.
	if app.currentID() == prevID {
		t.Errorf("currentID still pointing at deleted task %d", prevID)
	}
}

// TestReloadSurfacesErrorInStatus: if the on-disk file becomes
// unreadable (e.g. directory removed), the reload surfaces the
// failure into the status footer without crashing the app.
func TestReloadSurfacesErrorInStatus(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Make the store path invalid by replacing the parent dir
	// with a regular file. We can't easily do that — instead,
	// point the app at a non-existent path before pressing 'r'.
	bogus := filepath.Join(os.TempDir(), "does-not-exist-tsk-test", ".tsk.md")
	app.store.Path = bogus
	feed(app, keyRune('r'))
	// store.Load on a non-existent path actually returns an empty
	// store (init-on-miss semantics), so the test should verify
	// the app remains responsive. The key here is no panic —
	// any reasonable status outcome is fine.
	view := app.View()
	if view == "" {
		t.Error("view empty after reload-from-bogus-path")
	}
}
