package tui

import (
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestFocusPinnedHidesUnpinnedTasks: pressing 'F' narrows the
// visible list to ONLY pinned tasks. Mirrors `tsk top --pinned-only`.
func TestFocusPinnedHidesUnpinnedTasks(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "regular work", Priority: model.PriorityMedium})
	pinnedID := s.Add(model.Task{Title: "bookmark task", Priority: model.PriorityLow, Pinned: true})
	s.Add(model.Task{Title: "another task", Priority: model.PriorityHigh})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	if got := len(app.visibleTasks()); got != 3 {
		t.Fatalf("expected 3 visible before F, got %d", got)
	}
	feed(app, keyRune('F'))
	vt := app.visibleTasks()
	if len(vt) != 1 {
		t.Fatalf("expected 1 visible after F (pinned only), got %d: %+v", len(vt), vt)
	}
	if vt[0].ID != pinnedID {
		t.Errorf("expected pinned task #%d visible, got #%d", pinnedID, vt[0].ID)
	}
	if !strings.Contains(app.status, "pinned only") {
		t.Errorf("expected 'pinned only' in status, got %q", app.status)
	}
}

// TestFocusPinnedToggleRoundTrip: 'F' twice returns to the full
// unfiltered view. Toggle semantics — no permanent commitment.
func TestFocusPinnedToggleRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "a", Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "b", Pinned: true})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('F')) // on
	if got := len(app.visibleTasks()); got != 1 {
		t.Fatalf("expected 1 visible after first F, got %d", got)
	}
	feed(app, keyRune('F')) // off
	if got := len(app.visibleTasks()); got != 2 {
		t.Fatalf("expected 2 visible after second F (toggle off), got %d", got)
	}
	if !strings.Contains(app.status, "all tasks") {
		t.Errorf("expected 'all tasks' in status after toggle off, got %q", app.status)
	}
}

// TestFocusPinnedEmptyMessage: when there are NO pinned tasks,
// pressing 'F' surfaces a helpful diagnostic ("no pinned tasks")
// rather than just an empty list. Helps users discover that they
// haven't pinned anything yet.
func TestFocusPinnedEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "nothing pinned", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('F'))
	if got := len(app.visibleTasks()); got != 0 {
		t.Fatalf("expected 0 visible (no pinned), got %d", got)
	}
	if !strings.Contains(app.status, "no pinned tasks") {
		t.Errorf("expected 'no pinned tasks' in status, got %q", app.status)
	}
}

// TestFocusPinnedPreservesSelection: when toggling ON, if the
// currently-selected task is already pinned, the cursor stays
// on it (doesn't snap to position 0). Selection-by-id contract.
func TestFocusPinnedPreservesSelectionWhenPinned(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "first unpinned", Priority: model.PriorityMedium})
	pinnedID := s.Add(model.Task{Title: "pinned one", Pinned: true, Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "third unpinned", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Move to the pinned task before toggling F.
	feed(app, keyRune('j'))
	if app.currentID() != pinnedID {
		t.Fatalf("setup: expected cursor on pinned #%d, got #%d", pinnedID, app.currentID())
	}
	feed(app, keyRune('F'))
	// After toggling on, cursor should still be on the pinned task
	// (now at index 0 of the filtered list).
	vt := app.visibleTasks()
	if len(vt) != 1 || vt[app.selection].ID != pinnedID {
		t.Fatalf("expected cursor still on pinned #%d after F, got selection=%d list=%+v",
			pinnedID, app.selection, vt)
	}
}

// TestFocusPinnedSelectionSnapsToZeroWhenNotPinned: when toggling
// ON and the currently-selected task isn't pinned (so it disappears
// from view), the cursor snaps to position 0 of the new (filtered)
// list. Defensive: cursor can't dangle past the visible bounds.
func TestFocusPinnedSelectionSnapsToZeroWhenSelectedDisappears(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "unpinned cursor here", Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "pinned target", Pinned: true, Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Cursor is on first task (unpinned).
	if app.selection != 0 {
		t.Fatalf("setup: expected selection=0, got %d", app.selection)
	}
	feed(app, keyRune('F'))
	if app.selection != 0 {
		t.Errorf("expected selection=0 after F (cursor's old task is gone), got %d", app.selection)
	}
}

// TestFocusPinnedComposesWithFilter: F + / (text filter) compose
// via intersection — only tasks that are BOTH pinned AND match
// the search string remain visible. The most common workflow is
// "show me my pinned 'release' tasks".
func TestFocusPinnedComposesWithTextFilter(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "release-stub unpinned", Priority: model.PriorityMedium})
	releasePinnedID := s.Add(model.Task{Title: "release-cut pinned", Pinned: true, Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "scaffold pinned", Pinned: true, Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('F'))
	// Now pinned-only — 2 visible (release-cut + scaffold).
	if got := len(app.visibleTasks()); got != 2 {
		t.Fatalf("expected 2 pinned visible, got %d", got)
	}
	// Add text filter "release".
	feed(app,
		keyRune('/'),
		keyRune('r'),
		keyRune('e'),
		keyRune('l'),
		keyRune('e'),
		keyRune('a'),
		keyRune('s'),
		keyRune('e'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	vt := app.visibleTasks()
	if len(vt) != 1 {
		t.Fatalf("expected 1 visible (pinned AND release-match), got %d: %+v", len(vt), vt)
	}
	if vt[0].ID != releasePinnedID {
		t.Errorf("expected release-cut pinned #%d, got #%d", releasePinnedID, vt[0].ID)
	}
}

// TestFocusPinnedFooterAndHelpMention: the always-visible footer
// hint includes "F pin-focus" so users discover the binding
// without opening help. Help view also has a dedicated row.
func TestFocusPinnedFooterAndHelpMention(t *testing.T) {
	app := newSeededApp(t, 1)
	view := app.View()
	if !strings.Contains(view, "F pin-focus") {
		t.Errorf("expected footer to include 'F pin-focus', got fragment:\n%s", view)
	}
	help := app.helpView()
	if !strings.Contains(help, "focus pinned only") {
		t.Errorf("expected help to mention 'focus pinned only', got:\n%s", help)
	}
}

// TestFocusPinnedNotConsumedDuringForm: when a form is active
// (e.g. 'a' add), uppercase 'F' should be a character typed into
// the input, not trigger the toggle. Defensive coverage.
func TestFocusPinnedNotConsumedDuringForm(t *testing.T) {
	app := newSeededApp(t, 3)
	feed(app,
		keyRune('a'),
		keyRune('F'),
		keyRune('o'),
		keyRune('o'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	got := app.store.Tasks[len(app.store.Tasks)-1].Title
	if !strings.HasPrefix(got, "Foo") {
		t.Errorf("expected title to start with 'Foo' (chars buffered), got %q", got)
	}
	if app.pinnedOnly {
		t.Error("did not expect pinnedOnly=true after F-in-form")
	}
}
