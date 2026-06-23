package tui

import (
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestCyclePriorityDownDecrements: 'P' steps the priority back
// one notch. medium -> low (one press).
func TestCyclePriorityDownDecrements(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	id := s.Add(model.Task{Title: "stepping down", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('P'))
	got := app.store.ByID(id).Priority
	if got != model.PriorityLow {
		t.Errorf("expected PriorityLow after P from medium, got %v", got)
	}
}

// TestCyclePriorityDownWrapsLowToUrgent: low -> urgent (the
// wrap edge that proves the modular math is right).
func TestCyclePriorityDownWrapsLowToUrgent(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	id := s.Add(model.Task{Title: "wrap", Priority: model.PriorityLow})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('P'))
	got := app.store.ByID(id).Priority
	if got != model.PriorityUrgent {
		t.Errorf("expected PriorityUrgent after P from low (wrap), got %v", got)
	}
}

// TestCyclePriorityDownInverseOfUp: four presses of P returns
// to the starting priority (full loop). Pairs with the four-
// presses-of-p loop in the existing cyclePriority behavior.
func TestCyclePriorityDownFourPressesFullLoop(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	id := s.Add(model.Task{Title: "full-loop", Priority: model.PriorityHigh})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	for i := 0; i < 4; i++ {
		feed(app, keyRune('P'))
	}
	got := app.store.ByID(id).Priority
	if got != model.PriorityHigh {
		t.Errorf("expected PriorityHigh after 4x P (full loop), got %v", got)
	}
}

// TestCyclePriorityPandPDownAreInverses: p followed by P (or
// the other way) is a no-op. The two directions are exact
// inverses — defensive regression guard against future
// modulo-math drift.
func TestCyclePriorityUpAndDownAreInverses(t *testing.T) {
	startPriorities := []model.Priority{
		model.PriorityLow,
		model.PriorityMedium,
		model.PriorityHigh,
		model.PriorityUrgent,
	}
	for _, start := range startPriorities {
		// Fresh store per iteration so visibleTasks ordering is
		// deterministic (one task only).
		dir := t.TempDir()
		s, err := store.Load(dir + "/.tsk.md")
		if err != nil {
			t.Fatal(err)
		}
		id := s.Add(model.Task{Title: "inverse-" + start.String(), Priority: start})
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}
		app := New(s)
		feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
		feed(app, keyRune('p'))
		feed(app, keyRune('P'))
		got := app.store.ByID(id).Priority
		if got != start {
			t.Errorf("start=%v: expected unchanged after p+P, got %v", start, got)
		}
	}
}

// TestCyclePriorityDownEmptyStoreSafe: 'P' on empty store is a
// no-op (doesn't crash).
func TestCyclePriorityDownEmptyStoreSafe(t *testing.T) {
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
	feed(app, keyRune('P'))
	// Nothing crashed; nothing in store changed. Pass.
}

// TestCyclePriorityDownNotConsumedDuringForm: 'P' as part of
// add-form title text should be buffered, not trigger the
// cycle. Defensive coverage.
func TestCyclePriorityDownNotConsumedDuringForm(t *testing.T) {
	app := newSeededApp(t, 3)
	origPrio := app.store.Tasks[0].Priority
	feed(app,
		keyRune('a'),
		keyRune('P'),
		keyRune('a'),
		keyRune('s'),
		keyRune('s'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	got := app.store.Tasks[len(app.store.Tasks)-1].Title
	if !strings.HasPrefix(got, "Pass") {
		t.Errorf("expected title to start with 'Pass' (chars buffered), got %q", got)
	}
	// First task's priority unchanged.
	if app.store.Tasks[0].Priority != origPrio {
		t.Errorf("expected first task priority unchanged, got %v -> %v", origPrio, app.store.Tasks[0].Priority)
	}
}

// TestCyclePriorityDownHelpAndFooterMention: p/P pair shows up
// in the help view AND the footer hint.
func TestCyclePriorityDownHelpAndFooterMention(t *testing.T) {
	app := newSeededApp(t, 1)
	view := app.View()
	if !strings.Contains(view, "p/P prio") {
		t.Errorf("expected 'p/P prio' in footer hint, got fragment:\n%s", view)
	}
	help := app.helpView()
	if !strings.Contains(help, "cycle priority down") {
		t.Errorf("expected 'cycle priority down' in help view, got:\n%s", help)
	}
	if !strings.Contains(help, "cycle priority up") {
		t.Errorf("expected 'cycle priority up' (updated label) in help view, got:\n%s", help)
	}
}
