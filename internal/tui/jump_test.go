package tui

import (
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// newSeededApp builds a TUI app with N tasks, all undated (so they
// all land in the same visible section regardless of `now`). Returns
// the app after a sizing message so the visible-task computation is
// stable.
func newSeededApp(t *testing.T, n int) *App {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		s.Add(model.Task{Title: "task " + string(rune('a'+i)), Priority: model.PriorityMedium})
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	return app
}

// TestJumpTopSnapsSelectionToZero: from any selection, pressing 'g'
// snaps to the first visible task. Vim-style top-of-list jump.
func TestJumpTopSnapsSelectionToZero(t *testing.T) {
	app := newSeededApp(t, 5)
	// Move down a few times so selection isn't already 0.
	feed(app, keyRune('j'), keyRune('j'), keyRune('j'))
	if app.selection == 0 {
		t.Fatal("seed should have moved selection off 0")
	}
	feed(app, keyRune('g'))
	if app.selection != 0 {
		t.Fatalf("expected selection=0 after 'g', got %d", app.selection)
	}
}

// TestJumpBottomSnapsSelectionToLastVisible: from any selection,
// pressing 'G' snaps to the last visible task. Vim-style end-of-list
// jump. Operates on visibleTasks(), so "last" respects collapse +
// filter state.
func TestJumpBottomSnapsSelectionToLastVisible(t *testing.T) {
	app := newSeededApp(t, 5)
	// Selection starts at 0; G should jump to len(visibleTasks)-1.
	feed(app, keyRune('G'))
	want := len(app.visibleTasks()) - 1
	if want < 1 {
		t.Fatalf("seed should produce >1 visible task, got %d", len(app.visibleTasks()))
	}
	if app.selection != want {
		t.Fatalf("expected selection=%d after 'G', got %d", want, app.selection)
	}
}

// TestJumpTopAfterBottomRoundTrip: G then g returns to the top. Sanity
// check that the two helpers are pure (no state side effects beyond
// selection).
func TestJumpTopAfterBottomRoundTrip(t *testing.T) {
	app := newSeededApp(t, 4)
	feed(app, keyRune('G'), keyRune('g'))
	if app.selection != 0 {
		t.Fatalf("expected selection=0 after G then g, got %d", app.selection)
	}
}

// TestJumpBottomOnEmptyListStaysAtZero: G on an empty list (no tasks)
// must not panic and must keep selection at 0. Defensive — visibleTasks
// can be empty if every section is collapsed AND there are no overdue
// items, etc.
func TestJumpBottomOnEmptyListStaysAtZero(t *testing.T) {
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
	feed(app, keyRune('G'))
	if app.selection != 0 {
		t.Fatalf("expected selection=0 on empty list after G, got %d", app.selection)
	}
}

// TestJumpTopRespectsCollapsedSections: when the Done section is
// collapsed (default), 'G' jumps to the last NON-DONE visible task.
// Done tasks aren't visible so they're not the "bottom".
func TestJumpTopRespectsCollapsedSections(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	// Two open + two done.
	s.Add(model.Task{Title: "open-a", Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "open-b", Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "done-a", Priority: model.PriorityMedium, Done: true})
	s.Add(model.Task{Title: "done-b", Priority: model.PriorityMedium, Done: true})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Done is collapsed by default; visibleTasks should contain only
	// the two open tasks.
	if got := len(app.visibleTasks()); got != 2 {
		t.Fatalf("expected 2 visible tasks (done collapsed), got %d", got)
	}
	feed(app, keyRune('G'))
	if app.selection != 1 {
		t.Fatalf("expected selection=1 (last visible open task), got %d", app.selection)
	}
}

// TestJumpKeysNotConsumedDuringForm: when a form is active (e.g. add),
// 'g' and 'G' should NOT trigger top/bottom jumps — they're text
// characters being typed into the input. Defensive coverage so a
// future refactor doesn't accidentally short-circuit form input.
func TestJumpKeysNotConsumedDuringForm(t *testing.T) {
	app := newSeededApp(t, 3)
	// Open the add form, then type 'g' and 'G' as part of the title.
	feed(app,
		keyRune('a'),
		keyRune('g'),
		keyRune('G'),
		keyRune('o'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	// Last task should be "gGo" — characters were buffered into the
	// form, not interpreted as nav keys.
	got := app.store.Tasks[len(app.store.Tasks)-1].Title
	if !strings.HasPrefix(got, "gGo") && !strings.HasPrefix(got, "gG") {
		t.Fatalf("expected new task title to start with 'gG' (chars buffered), got %q", got)
	}
}

// TestHelpViewIncludesGGRow: the help table mentions g/G so users
// discover the binding via '?'. Regression guard against the help
// table drifting from the actual keymap.
func TestHelpViewIncludesGGRow(t *testing.T) {
	app := newSeededApp(t, 1)
	help := app.helpView()
	if !strings.Contains(help, "g/G") {
		t.Fatalf("expected 'g/G' row in help view, got:\n%s", help)
	}
	if !strings.Contains(help, "jump top / bottom") {
		t.Fatalf("expected 'jump top / bottom' label in help, got:\n%s", help)
	}
}

// TestFooterIncludesGGHint: the always-visible footer line mentions
// g/G so the binding is discoverable without opening help.
func TestFooterIncludesGGHint(t *testing.T) {
	app := newSeededApp(t, 1)
	view := app.View()
	if !strings.Contains(view, "g/G top/bottom") {
		t.Fatalf("expected footer to include 'g/G top/bottom', got footer fragment from view:\n%s", view)
	}
}

// TestJumpBottomThenSearchPreservesSelectionWithinBounds: after G
// lands at the end, opening search shouldn't crash even if the
// filter then drops the visible list. moveSelection/jumpBottom must
// stay within bounds.
func TestJumpBottomThenSearchPreservesSelectionWithinBounds(t *testing.T) {
	app := newSeededApp(t, 5)
	feed(app, keyRune('G'))
	// Open search, type a filter that matches nothing, close.
	feed(app,
		keyRune('/'),
		keyRune('z'),
		keyRune('z'),
		keyRune('z'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	// Now visibleTasks could be empty. Pressing G again must not
	// panic and must reset selection to 0 (the safe-empty branch).
	feed(app, keyRune('G'))
	if app.selection != 0 {
		t.Fatalf("expected selection=0 after G on empty-filter list, got %d", app.selection)
	}
}
