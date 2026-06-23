package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestYankAllVisibleEmitsOSC52: pressing 'Y' with multiple
// visible tasks emits a single OSC52 sequence carrying every
// visible task's formatTaskYank block, separated by blank lines.
func TestYankAllVisibleEmitsOSC52(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "first task", Priority: model.PriorityHigh})
	s.Add(model.Task{Title: "second task"})
	s.Add(model.Task{Title: "third task"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 40})
	feed(app, keyRune('Y'))
	out := buf.String()
	if !strings.HasPrefix(out, "\x1b]52;") {
		t.Errorf("expected OSC52 prefix, got %q", out)
	}
	if app.lastYank == "" {
		t.Fatal("expected lastYank populated")
	}
	// Should contain all three titles.
	for _, want := range []string{"first task", "second task", "third task"} {
		if !strings.Contains(app.lastYank, want) {
			t.Errorf("expected %q in payload, got:\n%s", want, app.lastYank)
		}
	}
	// Should have N-1 blank-line separators between N tasks.
	// "<task>\n\n<task>\n\n<task>\n" — count occurrences of "\n\n#"
	separators := strings.Count(app.lastYank, "\n\n#")
	if separators != 2 {
		t.Errorf("expected 2 blank-line separators for 3 tasks, got %d in:\n%s", separators, app.lastYank)
	}
}

// TestYankAllVisibleStatusReportsCountAndSize: the status footer
// shows the task count and a human-readable size.
func TestYankAllVisibleStatusReportsCountAndSize(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "task a"})
	s.Add(model.Task{Title: "task b"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('Y'))
	if !strings.Contains(app.status, "yanked 2 tasks") {
		t.Errorf("expected 'yanked 2 tasks' in status, got %q", app.status)
	}
	if !strings.Contains(app.status, "B)") && !strings.Contains(app.status, "KB)") {
		t.Errorf("expected size in status (Nb or NKB), got %q", app.status)
	}
}

// TestYankAllVisibleEmptyStoreNoop: pressing 'Y' with no visible
// tasks surfaces a clear status and does NOT write to the clipboard.
func TestYankAllVisibleEmptyStoreNoop(t *testing.T) {
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
	feed(app, keyRune('Y'))
	if buf.Len() != 0 {
		t.Errorf("expected no OSC52 write on empty store, got %d bytes: %q", buf.Len(), buf.String())
	}
	if !strings.Contains(app.status, "yank-all: no visible tasks") {
		t.Errorf("expected empty-set status, got %q", app.status)
	}
}

// TestYankAllVisibleRespectsFilter: 'Y' after a search filter
// yanks only the filter-matched tasks (not the entire store).
func TestYankAllVisibleRespectsFilter(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "alpha task"})
	s.Add(model.Task{Title: "beta task"})
	s.Add(model.Task{Title: "alphabetical task"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 40})
	// Apply filter "alpha" — matches "alpha task" and "alphabetical task"
	app.filter = "alpha"
	feed(app, keyRune('Y'))
	if !strings.Contains(app.lastYank, "alpha task") {
		t.Errorf("expected 'alpha task' in payload, got:\n%s", app.lastYank)
	}
	if !strings.Contains(app.lastYank, "alphabetical task") {
		t.Errorf("expected 'alphabetical task' in payload, got:\n%s", app.lastYank)
	}
	if strings.Contains(app.lastYank, "beta task") {
		t.Errorf("'beta task' should be EXCLUDED by filter, got:\n%s", app.lastYank)
	}
	if !strings.Contains(app.status, "yanked 2 tasks") {
		t.Errorf("expected 'yanked 2 tasks' (filter respected), got %q", app.status)
	}
}

// TestYankAllVisibleFormInputShadowing: while the add form is
// open, uppercase 'Y' is consumed as form input and does NOT
// trigger a yank-all.
func TestYankAllVisibleFormInputShadowing(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "existing"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Open the add form
	feed(app, keyRune('a'))
	// Send 'Y' — should land in the input value, NOT trigger yank-all
	feed(app, keyRune('Y'))
	if buf.Len() != 0 {
		t.Errorf("expected no OSC52 write while form open, got %d bytes", buf.Len())
	}
	if app.inputCur.value != "Y" {
		t.Errorf("expected 'Y' to land in form input, got %q", app.inputCur.value)
	}
}

// TestYankAllVisibleSizeFormatsScaleProperly: payload of varying
// sizes should report in B / KB units (no plain "0" or unscaled).
func TestYankAllVisibleSizeFormatsScaleProperly(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	// One short task → < 1KB → "B" suffix
	s.Add(model.Task{Title: "tiny"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('Y'))
	if !strings.Contains(app.status, "B)") {
		t.Errorf("expected B-suffix for small payload, got %q", app.status)
	}
}

// TestYankAllVisibleBlankLineSeparator: a 2-task payload has
// exactly ONE blank line between the two header lines. Stability
// check on the separator format consumers can rely on.
func TestYankAllVisibleBlankLineSeparator(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "first"})
	s.Add(model.Task{Title: "second"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('Y'))
	// Payload should be:  "#1 first\n\n#2 second\n"
	want := "#1 first\n\n#2 second\n"
	if app.lastYank != want {
		t.Errorf("payload shape wrong\nwant: %q\ngot:  %q", want, app.lastYank)
	}
}

// TestYankAllVisibleHelpAndFooterMention: the help overlay and
// the footer hint both mention 'Y'.
func TestYankAllVisibleHelpAndFooterMention(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "x"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := app.View()
	if !strings.Contains(view, "Y yank-all") {
		t.Errorf("expected 'Y yank-all' in footer hint, got fragment:\n%s", view)
	}
	help := app.helpView()
	if !strings.Contains(help, "Y") || !strings.Contains(help, "yank ALL visible") {
		t.Errorf("expected 'Y' + 'yank ALL visible' in help view, got:\n%s", help)
	}
}

// TestYankAllVisiblePreservesOrder: tasks appear in the payload
// in the same order the TUI renders them (visibleTasks() order),
// so the paste reads like a top-to-bottom transcript of the view.
func TestYankAllVisiblePreservesOrder(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "task-A"})
	s.Add(model.Task{Title: "task-B"})
	s.Add(model.Task{Title: "task-C"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('Y'))
	idxA := strings.Index(app.lastYank, "task-A")
	idxB := strings.Index(app.lastYank, "task-B")
	idxC := strings.Index(app.lastYank, "task-C")
	if !(idxA < idxB && idxB < idxC) {
		t.Errorf("order broken: A=%d B=%d C=%d in payload:\n%s",
			idxA, idxB, idxC, app.lastYank)
	}
}

// TestYankAllVisibleLowercaseYStillWorksForSingle: regression
// guard — adding 'Y' must not accidentally rebind 'y'. Lowercase
// 'y' still yanks just the current task.
func TestYankAllVisibleLowercaseYStillWorksForSingle(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "first"})
	s.Add(model.Task{Title: "second"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	var buf bytes.Buffer
	app.yankWriter = &buf
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('y'))
	// Single-task yank: status shape is "yanked #N title" — NOT
	// "yanked 2 tasks".
	if !strings.Contains(app.status, "yanked #") {
		t.Errorf("expected single-task status from lowercase 'y', got %q", app.status)
	}
	if strings.Contains(app.status, "yanked 2 tasks") {
		t.Errorf("lowercase 'y' should NOT trigger yank-all, got %q", app.status)
	}
}
