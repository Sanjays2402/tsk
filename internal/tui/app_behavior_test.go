package tui

import (
	"os"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func feed(app *App, msgs ...tea.Msg) {
	var m tea.Model = app
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
}

// TestToggleDonePersists asserts that pressing space marks the task done
// AND that the change round-trips through save.
func TestToggleDonePersists(t *testing.T) {
	app := newTestApp(t)
	// Nav to task "a" is already at selection 0. Toggle.
	feed(app,
		tea.WindowSizeMsg{Width: 80, Height: 24},
		tea.KeyMsg{Type: tea.KeySpace},
	)
	// Reload store from disk and assert task 1 is done.
	reloaded, err := store.Load(app.store.Path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	done := false
	for _, task := range reloaded.Tasks {
		if task.ID == 1 && task.Done {
			done = true
		}
	}
	if !done {
		t.Fatal("expected task 1 to be marked done on disk after space")
	}
}

// TestDeleteFlowRequiresConfirmation asserts that pressing 'd' arms the
// confirm prompt (nothing deleted yet) and then 'y' removes the task.
func TestDeleteFlowRequiresConfirmation(t *testing.T) {
	app := newTestApp(t)
	feed(app,
		tea.WindowSizeMsg{Width: 80, Height: 24},
		keyRune('d'),
	)
	// After 'd', confirm should be armed but task should still exist on disk.
	if app.confirm == 0 {
		t.Fatal("expected confirm to be armed after 'd'")
	}
	reloaded, _ := store.Load(app.store.Path)
	if len(reloaded.Tasks) != 2 {
		t.Fatalf("expected 2 tasks still present, got %d", len(reloaded.Tasks))
	}
	// Confirm with 'y'.
	feed(app, keyRune('y'))
	reloaded, _ = store.Load(app.store.Path)
	if len(reloaded.Tasks) != 1 {
		t.Fatalf("expected 1 task after confirm, got %d", len(reloaded.Tasks))
	}
}

// TestDeleteCancelLeavesTaskIntact — any non-confirm key aborts the prompt.
func TestDeleteCancelLeavesTaskIntact(t *testing.T) {
	app := newTestApp(t)
	feed(app,
		tea.WindowSizeMsg{Width: 80, Height: 24},
		keyRune('d'),
		tea.KeyMsg{Type: tea.KeyEsc},
	)
	if app.confirm != 0 {
		t.Fatal("confirm should be cleared after cancel key")
	}
	reloaded, _ := store.Load(app.store.Path)
	if len(reloaded.Tasks) != 2 {
		t.Fatalf("expected 2 tasks preserved, got %d", len(reloaded.Tasks))
	}
}

// TestPriorityCycleRotates — 'p' rotates through the four priorities.
func TestPriorityCycleRotates(t *testing.T) {
	app := newTestApp(t)
	feed(app,
		tea.WindowSizeMsg{Width: 80, Height: 24},
	)
	// Task "a" starts as PriorityHigh (3). Cycling once should land on
	// PriorityUrgent (0 mod 4 after increment). Just assert it changed and
	// landed within the legal 0..3 range.
	origPriority := app.store.Tasks[0].Priority
	feed(app, keyRune('p'))
	newPriority := app.store.Tasks[0].Priority
	if newPriority == origPriority {
		t.Fatal("priority should have changed after 'p'")
	}
	if int(newPriority) < 0 || int(newPriority) > 3 {
		t.Fatalf("priority out of range: %d", newPriority)
	}
}

// TestSaveErrorSurfacesInStatus — if the underlying file becomes
// unwriteable, the TUI surfaces it rather than silently eating the error.
func TestSaveErrorSurfacesInStatus(t *testing.T) {
	app := newTestApp(t)
	// AtomicWriteFile does MkdirAll, so a non-existent parent dir is not
	// enough. Make MkdirAll itself fail by pointing at a path whose parent
	// is an existing *file* — mkdir on that returns ENOTDIR.
	blocker := t.TempDir() + "/blocker"
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	app.store.Path = blocker + "/child/.tsk.md"

	feed(app,
		tea.WindowSizeMsg{Width: 80, Height: 24},
		tea.KeyMsg{Type: tea.KeySpace}, // toggle → triggers save → should fail
	)

	if app.status == "" {
		t.Fatal("expected status message after failed save")
	}
	want := "save failed:"
	if len(app.status) < len(want) || app.status[:len(want)] != want {
		t.Fatalf("expected status to start with %q, got %q", want, app.status)
	}
}

// TestQuitKeyRequestsQuit — 'q' returns a tea.Quit command.
func TestQuitKeyRequestsQuit(t *testing.T) {
	app := newTestApp(t)
	_, cmd := app.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("expected tea.Quit command from 'q'")
	}
}

// TestHelpToggles — '?' toggles help state on and off.
func TestHelpToggles(t *testing.T) {
	app := newTestApp(t)
	if app.showHelp {
		t.Fatal("showHelp should default to false")
	}
	feed(app, keyRune('?'))
	if !app.showHelp {
		t.Fatal("expected showHelp true after ?")
	}
	feed(app, keyRune('?'))
	if app.showHelp {
		t.Fatal("expected showHelp false after second ?")
	}
}

// TestAddFormFlow — 'a' opens add form, typed chars buffer, enter saves.
func TestAddFormFlow(t *testing.T) {
	app := newTestApp(t)
	before := len(app.store.Tasks)
	feed(app,
		tea.WindowSizeMsg{Width: 80, Height: 24},
		keyRune('a'),
		keyRune('x'),
		keyRune('y'),
		keyRune('z'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	if len(app.store.Tasks) != before+1 {
		t.Fatalf("expected 1 task added, got %d → %d", before, len(app.store.Tasks))
	}
	newTask := app.store.Tasks[len(app.store.Tasks)-1]
	if newTask.Title != "xyz" {
		t.Fatalf("expected title 'xyz', got %q", newTask.Title)
	}
	// Verify it persisted.
	if _, err := os.Stat(app.store.Path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}

func TestMoveSelectionWraps(t *testing.T) {
	app := newTestApp(t)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Two tasks → selection 0. Press j twice, selection should wrap to 0.
	start := app.selection
	feed(app, keyRune('j'), keyRune('j'))
	// With 2 tasks, j j should wrap back to the starting index.
	if app.selection != start {
		t.Fatalf("expected selection to wrap back to %d, got %d", start, app.selection)
	}
}

func TestModelListContainsBothTasks(t *testing.T) {
	app := newTestApp(t)
	if len(app.store.Tasks) != 2 {
		t.Fatalf("seed expected 2, got %d", len(app.store.Tasks))
	}
	if app.store.Tasks[0].Title != "a" || app.store.Tasks[1].Title != "b" {
		t.Fatalf("seed order wrong: %+v", app.store.Tasks)
	}
	// Sanity: model types match.
	_ = model.PriorityHigh
}
