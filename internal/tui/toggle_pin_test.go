package tui

import (
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestTogglePinAddsPinFlag: pressing '*' on an unpinned task
// makes it pinned, and the change persists to disk.
func TestTogglePinAddsPinFlag(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	id := s.Add(model.Task{Title: "to pin", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('*'))
	reloaded, _ := store.Load(dir + "/.tsk.md")
	if !reloaded.ByID(id).Pinned {
		t.Errorf("expected task #%d pinned after '*', not pinned", id)
	}
	if !strings.Contains(app.status, "pinned #") {
		t.Errorf("expected 'pinned #' in status, got %q", app.status)
	}
}

// TestTogglePinClearsPinFlag: pressing '*' on a pinned task
// unpins it. Round-trip toggle.
func TestTogglePinClearsPinFlag(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	id := s.Add(model.Task{Title: "already pinned", Pinned: true, Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('*'))
	reloaded, _ := store.Load(dir + "/.tsk.md")
	if reloaded.ByID(id).Pinned {
		t.Errorf("expected task #%d unpinned after '*', still pinned", id)
	}
	if !strings.Contains(app.status, "unpinned #") {
		t.Errorf("expected 'unpinned #' in status, got %q", app.status)
	}
}

// TestTogglePinRoundTrip: '*' twice returns the task to its
// original state. Toggle semantics.
func TestTogglePinRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	id := s.Add(model.Task{Title: "double-toggle", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('*')) // on
	feed(app, keyRune('*')) // off
	reloaded, _ := store.Load(dir + "/.tsk.md")
	if reloaded.ByID(id).Pinned {
		t.Errorf("expected task #%d back to unpinned after two '*' presses, still pinned", id)
	}
}

// TestTogglePinBuildsThenFocusesPinnedSet: the workflow the
// feature is for — pin a task with '*', then 'F' to focus on
// the pinned set. End-to-end integration with the existing
// focus-pinned toggle.
func TestTogglePinBuildsThenFocusesPinnedSet(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "regular", Priority: model.PriorityMedium})
	id := s.Add(model.Task{Title: "to be pinned", Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "another regular", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Move to the middle task (index 1 in this all-medium-priority list).
	feed(app, keyRune('j'))
	if app.currentID() != id {
		t.Fatalf("setup: cursor expected on #%d, got #%d", id, app.currentID())
	}
	feed(app, keyRune('*')) // pin the middle task
	feed(app, keyRune('F')) // focus pinned
	vt := app.visibleTasks()
	if len(vt) != 1 || vt[0].ID != id {
		t.Fatalf("expected only the just-pinned task visible, got %+v", vt)
	}
}

// TestTogglePinUnpinShrinksFocusList: unpinning while focus-
// pinned is active makes the task disappear from view; cursor
// snaps to the new last visible (or 0 on empty).
func TestTogglePinUnpinShrinksFocusList(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	a := s.Add(model.Task{Title: "pinned A", Pinned: true, Priority: model.PriorityMedium})
	b := s.Add(model.Task{Title: "pinned B", Pinned: true, Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	_ = a
	_ = b
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('F')) // pinnedOnly=true
	if got := len(app.visibleTasks()); got != 2 {
		t.Fatalf("expected 2 pinned visible, got %d", got)
	}
	// Move to second pinned and unpin it.
	feed(app, keyRune('j'))
	feed(app, keyRune('*'))
	vt := app.visibleTasks()
	if len(vt) != 1 {
		t.Fatalf("expected 1 visible after unpin (focus-pinned still on), got %d", len(vt))
	}
	if app.selection >= len(vt) {
		t.Errorf("expected selection clamped to %d, got %d", len(vt)-1, app.selection)
	}
}

// TestTogglePinEmptyStoreStatus: pressing '*' on an empty
// store surfaces the "no task selected" hint without crashing.
func TestTogglePinEmptyStoreStatus(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('*'))
	if !strings.Contains(app.status, "no task selected") {
		t.Errorf("expected 'no task selected' in status, got %q", app.status)
	}
}

// TestTogglePinNotConsumedDuringForm: when a form is active,
// '*' should be typed into the input, not trigger the toggle.
// Defensive coverage.
func TestTogglePinNotConsumedDuringForm(t *testing.T) {
	app := newSeededApp(t, 2)
	originalPinned := app.store.Tasks[0].Pinned
	feed(app,
		keyRune('a'),
		keyRune('*'),
		keyRune('x'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	got := app.store.Tasks[len(app.store.Tasks)-1].Title
	if !strings.Contains(got, "*") {
		t.Errorf("expected '*' to be buffered in title, got %q", got)
	}
	// First task's pinned should be unchanged.
	if app.store.Tasks[0].Pinned != originalPinned {
		t.Errorf("expected first task pinned unchanged, got %v -> %v", originalPinned, app.store.Tasks[0].Pinned)
	}
}

// TestTogglePinHelpAndFooterMention: '*' shows up in the help
// view and the always-visible footer hint.
func TestTogglePinHelpAndFooterMention(t *testing.T) {
	app := newSeededApp(t, 1)
	view := app.View()
	if !strings.Contains(view, "* pin") {
		t.Errorf("expected '* pin' in footer hint, got fragment:\n%s", view)
	}
	help := app.helpView()
	if !strings.Contains(help, "toggle pin") {
		t.Errorf("expected 'toggle pin' in help view, got:\n%s", help)
	}
}

// TestTogglePinComposesWithJumpNext: pin a low-priority task,
// then press 'N' — the pinned-low should beat unpinned-urgent
// (pinned beats priority in tsk's next selector). Confirms the
// fresh pin participates in the next-pick contract immediately.
func TestTogglePinComposesWithJumpNext(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	lowID := s.Add(model.Task{Title: "low to pin", Priority: model.PriorityLow})
	s.Add(model.Task{Title: "urgent unpinned", Priority: model.PriorityUrgent})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Move to the low-priority task (it's at index 1 with priority sort).
	feed(app, keyRune('j'))
	if app.currentID() != lowID {
		t.Fatalf("setup: cursor expected on #%d, got #%d", lowID, app.currentID())
	}
	feed(app, keyRune('*')) // pin it
	// Move cursor away so we can verify 'N' selects the freshly-pinned one.
	feed(app, keyRune('k'))
	feed(app, keyRune('N'))
	vt := app.visibleTasks()
	if vt[app.selection].ID != lowID {
		t.Errorf("expected N to pick freshly-pinned low (#%d), got #%d", lowID, vt[app.selection].ID)
	}
}
