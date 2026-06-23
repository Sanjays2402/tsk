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

// TestArchiveCurrentMovesDoneTaskToSiblingFile: pressing 'X' on a
// done task moves it from .tsk.md into .tsk.archive.md (flat
// strategy). The active store loses the task; the sibling archive
// gains it with a fresh archive id.
func TestArchiveCurrentMovesDoneTaskToSiblingFile(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	id := s.Add(model.Task{Title: "completed work", Done: true, Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Expand done section so the done task is visible & selectable.
	app.collapsed[sectionDone] = false
	app.selection = 0
	feed(app, keyRune('X'))
	// Active store should no longer contain the task.
	reloaded, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatalf("reload active: %v", err)
	}
	if reloaded.ByID(id) != nil {
		t.Errorf("expected #%d removed from active store, still present", id)
	}
	// Archive file should exist with one task.
	archivePath := dir + "/.tsk.archive.md"
	if _, statErr := os.Stat(archivePath); statErr != nil {
		t.Fatalf("expected archive file at %s, got %v", archivePath, statErr)
	}
	arch, err := store.Load(archivePath)
	if err != nil {
		t.Fatalf("load archive: %v", err)
	}
	if len(arch.Tasks) != 1 {
		t.Fatalf("expected 1 task in archive, got %d", len(arch.Tasks))
	}
	if arch.Tasks[0].Title != "completed work" {
		t.Errorf("expected archived title preserved, got %q", arch.Tasks[0].Title)
	}
	if arch.Tasks[0].ID != 1 {
		t.Errorf("expected archive id=1 (fresh archive id space), got %d", arch.Tasks[0].ID)
	}
	if !strings.Contains(app.status, "archived #") {
		t.Errorf("expected 'archived #' in status, got %q", app.status)
	}
}

// TestArchiveCurrentRefusesOpenTask: an open (not-done) task can't
// be archived — surfaced as a status hint rather than a silent
// no-op.
func TestArchiveCurrentRefusesOpenTask(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "open work", Priority: model.PriorityHigh})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('X'))
	if !strings.Contains(app.status, "not done") {
		t.Errorf("expected 'not done' in status, got %q", app.status)
	}
	// Active store still contains the task.
	reloaded, _ := store.Load(dir + "/.tsk.md")
	if len(reloaded.Tasks) != 1 {
		t.Errorf("expected 1 task still in active, got %d", len(reloaded.Tasks))
	}
	// Archive file should NOT exist.
	if _, statErr := os.Stat(dir + "/.tsk.archive.md"); statErr == nil {
		t.Error("expected no archive file (refused open task)")
	}
}

// TestArchiveCurrentRefusesIfBlockingOpenDependents: archiving a
// done task that another OPEN task lists in DependsOn would leave
// a dangling ref in the active store. Refuse and tell the user
// which tasks block the archive.
func TestArchiveCurrentRefusesIfBlockingOpenDependents(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	prereqID := s.Add(model.Task{Title: "done prereq", Done: true, Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "open dependent", Priority: model.PriorityMedium, DependsOn: []int{prereqID}})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	app.collapsed[sectionDone] = false
	// Find the done task in the visible list and select it.
	vt := app.visibleTasks()
	for i, t := range vt {
		if t.ID == prereqID {
			app.selection = i
			break
		}
	}
	feed(app, keyRune('X'))
	if !strings.Contains(app.status, "prereq for") {
		t.Errorf("expected 'prereq for' in status, got %q", app.status)
	}
	// Active store still contains both tasks.
	reloaded, _ := store.Load(dir + "/.tsk.md")
	if len(reloaded.Tasks) != 2 {
		t.Errorf("expected 2 tasks still in active, got %d", len(reloaded.Tasks))
	}
}

// TestArchiveCurrentAllowsArchiveOfPrereqWhenDependentAlsoDone:
// done-task dependents are tolerated (their dep ref becomes
// dangling but they're already satisfied, so it's a no-op
// semantically). Only OPEN dependents block the archive.
func TestArchiveCurrentAllowsArchiveOfPrereqWhenDependentAlsoDone(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	prereqID := s.Add(model.Task{Title: "done prereq", Done: true, Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "done dependent", Done: true, Priority: model.PriorityMedium, DependsOn: []int{prereqID}})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	app.collapsed[sectionDone] = false
	vt := app.visibleTasks()
	for i, t := range vt {
		if t.ID == prereqID {
			app.selection = i
			break
		}
	}
	feed(app, keyRune('X'))
	if !strings.Contains(app.status, "archived #") {
		t.Errorf("expected 'archived #' in status (done dependent should be tolerated), got %q", app.status)
	}
}

// TestArchiveCurrentContinuesArchiveIDSpace: a second archive
// gets the next archive id (max+1), not the active store's id
// (which is independent). Mirrors the CLI archive's id contract.
func TestArchiveCurrentContinuesArchiveIDSpace(t *testing.T) {
	dir := t.TempDir()
	// Pre-seed archive with an existing task at id=5.
	archivePath := dir + "/.tsk.archive.md"
	preseed, _ := store.Load(archivePath)
	preseed.Header = "# tsk archive\n"
	preseed.Add(model.Task{ID: 5, Title: "old archived", Done: true, Priority: model.PriorityMedium})
	if err := preseed.Save(); err != nil {
		t.Fatalf("preseed archive: %v", err)
	}
	// Now load active store and archive a done task from it.
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "new completion", Done: true, Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	app.collapsed[sectionDone] = false
	app.selection = 0
	feed(app, keyRune('X'))
	arch, err := store.Load(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(arch.Tasks) != 2 {
		t.Fatalf("expected 2 tasks in archive, got %d", len(arch.Tasks))
	}
	// New task should have ID=6 (max=5 + 1).
	found := false
	for _, task := range arch.Tasks {
		if task.Title == "new completion" {
			if task.ID != 6 {
				t.Errorf("expected new archive id=6 (continues max+1), got %d", task.ID)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected 'new completion' in archive")
	}
}

// TestArchiveCurrentEmptyStoreNoSelection: pressing X on an empty
// store surfaces the "no task selected" hint rather than crashing.
func TestArchiveCurrentEmptyStoreNoSelection(t *testing.T) {
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
	feed(app, keyRune('X'))
	if !strings.Contains(app.status, "no task selected") {
		t.Errorf("expected 'no task selected' in status, got %q", app.status)
	}
}

// TestArchiveCurrentNotConsumedDuringForm: when a form is active,
// uppercase 'X' should be a character typed into the input, not
// trigger the archive. Defensive coverage.
func TestArchiveCurrentNotConsumedDuringForm(t *testing.T) {
	app := newSeededApp(t, 3)
	feed(app,
		keyRune('a'),
		keyRune('X'),
		keyRune('e'),
		keyRune('n'),
		keyRune('o'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	got := app.store.Tasks[len(app.store.Tasks)-1].Title
	if !strings.HasPrefix(got, "Xeno") {
		t.Errorf("expected title to start with 'Xeno' (chars buffered), got %q", got)
	}
}

// TestArchiveCurrentHelpAndFooterMention: X is documented in the
// help view AND the always-visible footer hint so users can
// discover it.
func TestArchiveCurrentHelpAndFooterMention(t *testing.T) {
	app := newSeededApp(t, 1)
	view := app.View()
	if !strings.Contains(view, "X archive") {
		t.Errorf("expected 'X archive' in footer hint, got fragment:\n%s", view)
	}
	help := app.helpView()
	if !strings.Contains(help, "archive current") {
		t.Errorf("expected 'archive current' in help view, got:\n%s", help)
	}
}

// TestTuiArchivePathDefaultSibling: pure-function check that the
// archive path resolver picks the sibling .tsk.archive.md next to
// the active file, regardless of how the active path is shaped.
func TestTuiArchivePathDefaultSibling(t *testing.T) {
	cases := []struct {
		active string
		want   string
	}{
		{"/Users/x/proj/.tsk.md", "/Users/x/proj/.tsk.archive.md"},
		{"./.tsk.md", filepath.Join(".", ".tsk.archive.md")},
		{"/tmp/sub/.tsk.md", "/tmp/sub/.tsk.archive.md"},
	}
	for _, c := range cases {
		got := tuiArchivePath(c.active)
		if got != c.want {
			t.Errorf("active=%q: got %q, want %q", c.active, got, c.want)
		}
	}
}
