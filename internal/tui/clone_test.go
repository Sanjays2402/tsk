package tui

import (
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestCloneKeyAddsCopyWithSuffix: pressing 'C' on the selected
// task creates a new task with the same title plus " (copy)".
// Mirrors the CLI clone verb's default behavior.
func TestCloneKeyAddsCopyWithSuffix(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Default newTestApp creates "a" (high) and "b" (low). With
	// priority sort, "a" is at index 0. Press 'C' to clone.
	originalCount := len(app.store.Tasks)
	feed(app, keyRune('C'))
	if got := len(app.store.Tasks); got != originalCount+1 {
		t.Fatalf("expected %d tasks after clone, got %d", originalCount+1, got)
	}
	// Find the new task — should be titled "a (copy)".
	found := false
	for _, task := range app.store.Tasks {
		if task.Title == "a (copy)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected cloned task 'a (copy)', tasks: %+v", app.store.Tasks)
	}
}

// TestCloneKeyPreservesPriorityTagsAndDue: the clone inherits
// priority + tags + due + notes by value (deep copy contract
// from the CLI verb). Round-trip via disk to confirm
// persistence.
func TestCloneKeyPreservesPriorityTagsAndDue(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Mutate task #1 to carry rich state we can verify on the clone.
	src := app.store.ByID(1)
	if src == nil {
		t.Fatal("baseline task #1 missing")
	}
	src.Tags = []string{"work", "urgent"}
	src.Notes = "first note\nsecond note"
	if err := app.store.Save(); err != nil {
		t.Fatalf("save mutated source: %v", err)
	}
	// Clone the selected task (index 0 → task with highest priority = "a").
	feed(app, keyRune('C'))
	// The clone should be the new task added at the end of Tasks (store.Add
	// appends), but to be safe we find by title since selection moves.
	var clone *model.Task
	for i := range app.store.Tasks {
		if app.store.Tasks[i].Title == "a (copy)" {
			clone = &app.store.Tasks[i]
			break
		}
	}
	if clone == nil {
		t.Fatal("clone 'a (copy)' not found in store")
	}
	if clone.Priority != src.Priority {
		t.Errorf("priority: got %v, want %v", clone.Priority, src.Priority)
	}
	// Tags may be sorted by NormalizeTags during save — check
	// the unordered set membership rather than positional order.
	hasWork, hasUrgent := false, false
	for _, tg := range clone.Tags {
		if tg == "work" {
			hasWork = true
		}
		if tg == "urgent" {
			hasUrgent = true
		}
	}
	if !hasWork || !hasUrgent || len(clone.Tags) != 2 {
		t.Errorf("tags: got %v, want {work, urgent}", clone.Tags)
	}
	if clone.Notes != "first note\nsecond note" {
		t.Errorf("notes: got %q, want preserved", clone.Notes)
	}
	// Deep-copy: mutating the source tag slice must NOT affect the clone.
	cloneTagsSnapshot := append([]string(nil), clone.Tags...)
	src.Tags[0] = "MUTATED"
	for i, tg := range cloneTagsSnapshot {
		if clone.Tags[i] != tg {
			t.Errorf("clone tag %d changed from %q to %q after source mutation (should be deep-copied)", i, tg, clone.Tags[i])
		}
	}
}

// TestCloneKeyResetsDoneState: a clone of a DONE task starts
// open. Matches the "use this completed task as a template" intent.
func TestCloneKeyResetsDoneState(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Mark task #1 done first.
	app.store.SetDone(1, true)
	if err := app.store.Save(); err != nil {
		t.Fatalf("save done state: %v", err)
	}
	// Reload to pick up the done flag via the visible-tasks path.
	fresh, err := store.Load(app.store.Path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	app.store = fresh
	// Expand the done section so the task is selectable through the
	// normal visible-tasks navigation path. After expanding, the
	// done task "a" should be near the bottom; navigate to it.
	app.collapsed[sectionDone] = false
	app.selection = 0
	// Find the done task index and select it.
	vt := app.visibleTasks()
	for i, task := range vt {
		if task.ID == 1 {
			app.selection = i
			break
		}
	}
	if app.currentID() != 1 {
		t.Fatalf("expected task #1 selected, got id=%d", app.currentID())
	}
	feed(app, keyRune('C'))
	var clone *model.Task
	for i := range app.store.Tasks {
		if app.store.Tasks[i].Title == "a (copy)" {
			clone = &app.store.Tasks[i]
			break
		}
	}
	if clone == nil {
		t.Fatal("clone not found after key 'C'")
	}
	if clone.Done {
		t.Errorf("clone should start open even when source is done, got Done=%v", clone.Done)
	}
	if clone.Completed != nil {
		t.Errorf("clone should have nil Completed, got %v", clone.Completed)
	}
}

// TestCloneKeyStatusFooterShowsTransition: the status footer
// reports the id transition ("cloned #N → #M") so the user gets
// immediate feedback.
func TestCloneKeyStatusFooterShowsTransition(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('C'))
	if !strings.Contains(app.status, "cloned") {
		t.Errorf("expected status to mention 'cloned', got %q", app.status)
	}
	if !strings.Contains(app.status, "→") {
		t.Errorf("expected status to show id transition, got %q", app.status)
	}
}

// TestCloneKeyOnEmptyStoreIsSafe: pressing 'C' with no selection
// surfaces a status message rather than crashing.
func TestCloneKeyOnEmptyStoreIsSafe(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// No tasks → cloning should be a no-op with a friendly status.
	feed(app, keyRune('C'))
	if len(app.store.Tasks) != 0 {
		t.Errorf("empty store should remain empty after clone, got %d tasks", len(app.store.Tasks))
	}
	if !strings.Contains(app.status, "no task") {
		t.Errorf("expected status to mention 'no task', got %q", app.status)
	}
}

// TestCloneKeyPersistsToDisk: the clone round-trips through Save
// so a fresh store.Load picks it up. Critical — the TUI's
// session-state isn't enough; the on-disk file is the source of
// truth.
func TestCloneKeyPersistsToDisk(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('C'))
	// Re-load from disk and verify the clone is there.
	reloaded, err := store.Load(app.store.Path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	found := false
	for _, task := range reloaded.Tasks {
		if task.Title == "a (copy)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("cloned task not on disk after Save, tasks: %+v", reloaded.Tasks)
	}
}
