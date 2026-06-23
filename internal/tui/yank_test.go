package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestYankCurrentEmitsOSC52: pressing 'y' on a task emits the OSC52
// terminal escape sequence to yankWriter and captures the payload
// in lastYank for status confirmation.
func TestYankCurrentEmitsOSC52(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	id := s.Add(model.Task{Title: "buy milk", Priority: model.PriorityHigh})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('y'))
	// OSC52 sequence: ESC ] 52 ; c ; <base64> ESC \
	out := buf.String()
	if !strings.HasPrefix(out, "\x1b]52;") {
		t.Errorf("expected OSC52 prefix '\\x1b]52;', got %q", out)
	}
	if !strings.HasSuffix(out, "\x07") && !strings.HasSuffix(out, "\x1b\\") {
		t.Errorf("expected OSC52 suffix (BEL or ST), got tail %q", out[len(out)-4:])
	}
	// lastYank should contain the rendered text payload (NOT the
	// escape-wrapped form).
	if app.lastYank == "" {
		t.Error("expected lastYank to be populated after yank")
	}
	want := "#" // header should start with #<id>
	if !strings.HasPrefix(app.lastYank, want) {
		t.Errorf("lastYank should start with #<id>, got %q", app.lastYank)
	}
	if !strings.Contains(app.lastYank, "buy milk") {
		t.Errorf("lastYank should contain title, got %q", app.lastYank)
	}
	if !strings.Contains(app.status, "yanked #") {
		t.Errorf("expected 'yanked #' in status, got %q", app.status)
	}
	_ = id
}

// TestFormatTaskYankMinimal: a task with only id+title (no
// non-default priority, no due, no tags, no notes) renders as
// just the one header line.
func TestFormatTaskYankMinimal(t *testing.T) {
	got := formatTaskYank(model.Task{ID: 7, Title: "minimal task"})
	want := "#7 minimal task\n"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

// TestFormatTaskYankOmitsLowPriority: low is the implicit default;
// surfacing it on every yank would clutter the paste.
func TestFormatTaskYankOmitsLowPriority(t *testing.T) {
	got := formatTaskYank(model.Task{ID: 1, Title: "x", Priority: model.PriorityLow})
	if strings.Contains(got, "priority:") {
		t.Errorf("low priority should be omitted, got %q", got)
	}
}

// TestFormatTaskYankIncludesNonLowPriority: medium/high/urgent
// each render as "priority: <name>" on their own line.
func TestFormatTaskYankIncludesNonLowPriority(t *testing.T) {
	for _, p := range []model.Priority{model.PriorityMedium, model.PriorityHigh, model.PriorityUrgent} {
		got := formatTaskYank(model.Task{ID: 1, Title: "x", Priority: p})
		if !strings.Contains(got, "priority: "+p.String()) {
			t.Errorf("expected 'priority: %s' for %v, got %q", p.String(), p, got)
		}
	}
}

// TestFormatTaskYankIncludesDue: due date renders as "due:
// YYYY-MM-DD" on its own line when set.
func TestFormatTaskYankIncludesDue(t *testing.T) {
	d := time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)
	got := formatTaskYank(model.Task{ID: 1, Title: "x", Due: &d})
	if !strings.Contains(got, "due: 2026-12-25") {
		t.Errorf("expected 'due: 2026-12-25' in yank, got %q", got)
	}
}

// TestFormatTaskYankIncludesTagsSorted: tags render as "tags:
// #a #b #c" alphabetized so two yanks of the same task produce
// the same bytes (stability for diff/snapshot use cases).
func TestFormatTaskYankIncludesTagsSorted(t *testing.T) {
	got := formatTaskYank(model.Task{ID: 1, Title: "x", Tags: []string{"work", "p0", "release"}})
	if !strings.Contains(got, "tags: #p0 #release #work") {
		t.Errorf("expected alphabetized tags 'tags: #p0 #release #work', got %q", got)
	}
}

// TestFormatTaskYankIncludesNotesIndented: notes render as a
// "notes:" header followed by each line indented by 2 spaces.
func TestFormatTaskYankIncludesNotesIndented(t *testing.T) {
	got := formatTaskYank(model.Task{
		ID:    1,
		Title: "x",
		Notes: "line one\nline two\nline three",
	})
	want := "#1 x\nnotes:\n  line one\n  line two\n  line three\n"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

// TestFormatTaskYankOmitsBlankNotes: whitespace-only notes don't
// trigger the "notes:" header.
func TestFormatTaskYankOmitsBlankNotes(t *testing.T) {
	got := formatTaskYank(model.Task{ID: 1, Title: "x", Notes: "   \n  \n"})
	if strings.Contains(got, "notes:") {
		t.Errorf("blank notes should be omitted, got %q", got)
	}
}

// TestFormatTaskYankFullShape: every field set — the format is
// stable and self-contained.
func TestFormatTaskYankFullShape(t *testing.T) {
	d := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	got := formatTaskYank(model.Task{
		ID:       42,
		Title:    "ship the thing",
		Priority: model.PriorityUrgent,
		Due:      &d,
		Tags:     []string{"release"},
		Notes:    "blocker: design review",
	})
	want := "#42 ship the thing\npriority: urgent\ndue: 2026-06-23\ntags: #release\nnotes:\n  blocker: design review\n"
	if got != want {
		t.Errorf("want:\n%q\ngot:\n%q", want, got)
	}
}

// TestYankCurrentEmptyStoreNoOp: pressing 'y' with no task selected
// surfaces 'no task selected' and writes nothing to the clipboard.
func TestYankCurrentEmptyStoreNoOp(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('y'))
	if !strings.Contains(app.status, "no task selected") {
		t.Errorf("expected 'no task selected' on empty-store yank, got %q", app.status)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no OSC52 emission on empty-store yank, got %d bytes", buf.Len())
	}
}

// TestYankCurrentNotShadowedByFormInput: with a form open ('a' add
// or 'e' edit), 'y' is captured as literal text — the yank should
// NOT fire. Regression guard for the same input-shadowing bug
// every TUI single-action verb has to guard against.
func TestYankCurrentNotShadowedByFormInput(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "alpha"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Open the add form.
	feed(app, keyRune('a'))
	// While the form is open, 'y' should land in the input value,
	// NOT trigger a yank.
	feed(app, keyRune('y'))
	if buf.Len() != 0 {
		t.Errorf("expected no OSC52 emission while form is open, got %d bytes", buf.Len())
	}
	if app.inputCur.value != "y" {
		t.Errorf("expected 'y' to land in form input, got %q", app.inputCur.value)
	}
}

// TestYankCurrentDuringDeletePromptConfirmsDelete: 'y' is also
// bound to Confirm during a delete prompt. The dispatch order
// (confirm before nav) ensures the delete-confirm path takes
// precedence — verifying both behaviors coexist cleanly.
func TestYankCurrentDuringDeletePromptConfirmsDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	id := s.Add(model.Task{Title: "to delete"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Press 'd' to open the delete-confirm prompt for the task.
	feed(app, keyRune('d'))
	if app.confirm != id {
		t.Fatalf("expected confirm=%d after 'd', got %d", id, app.confirm)
	}
	// Now 'y' should CONFIRM the delete, not yank.
	feed(app, keyRune('y'))
	if app.confirm != 0 {
		t.Errorf("expected confirm cleared after 'y' on delete prompt, got %d", app.confirm)
	}
	// And the task should be deleted.
	if s.ByID(id) != nil {
		t.Errorf("expected task #%d removed by delete-confirm, still present", id)
	}
	// No OSC52 should have been emitted (the 'y' was eaten by confirm).
	if buf.Len() != 0 {
		t.Errorf("expected no OSC52 emission during delete-confirm 'y', got %d bytes", buf.Len())
	}
}

// TestYankCurrentHelpFooterMention: footer hint and help overlay
// both advertise the 'y' yank shortcut.
func TestYankCurrentHelpFooterMention(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "a"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := app.View()
	if !strings.Contains(view, "y yank") {
		t.Errorf("footer should mention 'y yank', got:\n%s", view)
	}
	help := app.helpView()
	if !strings.Contains(help, "yank task as text") {
		t.Errorf("help overlay should describe 'yank task as text', got:\n%s", help)
	}
}

// TestYankCurrentStatusTruncatesLongTitle: a very long title is
// truncated to a sensible preview length in the status footer
// (the full payload is still on the clipboard).
func TestYankCurrentStatusTruncatesLongTitle(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("x", 200)
	s.Add(model.Task{Title: long})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('y'))
	if len(app.status) > 80 {
		t.Errorf("status should be truncated; got length %d:\n%s", len(app.status), app.status)
	}
	if !strings.Contains(app.status, "…") {
		t.Errorf("expected truncation marker '…' in status for long title, got %q", app.status)
	}
	// Full payload is still on lastYank (clipboard).
	if !strings.Contains(app.lastYank, long) {
		t.Errorf("lastYank should contain full title, got %q", app.lastYank)
	}
}
