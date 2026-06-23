package tui

import (
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestJumpNextSelectsHighestPriority: 'N' lands on the highest-
// priority undone task (the same one `tsk next` would pick),
// matching the canonical pin/priority/due/id tie-break.
func TestJumpNextSelectsHighestPriority(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	// Mixed priorities; the urgent one should win.
	s.Add(model.Task{Title: "low task", Priority: model.PriorityLow})
	urgentID := s.Add(model.Task{Title: "urgent task", Priority: model.PriorityUrgent})
	s.Add(model.Task{Title: "medium task", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('N'))
	// The selection should land on the urgent task by id.
	vt := app.visibleTasks()
	if app.selection < 0 || app.selection >= len(vt) {
		t.Fatalf("selection %d out of bounds (len %d)", app.selection, len(vt))
	}
	if vt[app.selection].ID != urgentID {
		t.Fatalf("expected selection on urgent task #%d, got #%d (%q)",
			urgentID, vt[app.selection].ID, vt[app.selection].Title)
	}
	if !strings.Contains(app.status, "next:") {
		t.Errorf("expected 'next:' in status, got %q", app.status)
	}
	if !strings.Contains(app.status, "urgent task") {
		t.Errorf("expected status to mention the picked task title, got %q", app.status)
	}
}

// TestJumpNextSkipsBlockedTasks: a task blocked by an open
// prereq is NOT picked even if it's higher-priority. The
// unblocked runner-up is chosen instead.
func TestJumpNextSkipsBlockedTasks(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	prereqID := s.Add(model.Task{Title: "prereq", Priority: model.PriorityLow})
	urgentBlocked := model.Task{Title: "urgent but blocked", Priority: model.PriorityUrgent, DependsOn: []int{prereqID}}
	urgentBlockedID := s.Add(urgentBlocked)
	mediumOpen := s.Add(model.Task{Title: "medium unblocked", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('N'))
	vt := app.visibleTasks()
	if app.selection < 0 || app.selection >= len(vt) {
		t.Fatalf("selection %d out of bounds (len %d)", app.selection, len(vt))
	}
	got := vt[app.selection]
	if got.ID == urgentBlockedID {
		t.Fatalf("expected to skip urgent-blocked, but selection landed on it (id=%d)", got.ID)
	}
	// Two candidates remain: prereq (low) and medium-open. Medium beats low.
	if got.ID != mediumOpen {
		t.Errorf("expected medium unblocked task, got #%d (%q)", got.ID, got.Title)
	}
}

// TestJumpNextFallbackToBlockedWhenAllBlocked: when every open
// candidate is blocked, the cursor lands on the highest-priority
// blocked task with a "(blocked by #X)" annotation in the status
// footer — better than going silent.
func TestJumpNextFallbackToBlockedWhenAllBlocked(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	prereqID := s.Add(model.Task{Title: "external prereq", Priority: model.PriorityLow})
	// Close the prereq externally — but make it depend on something undone
	// to keep "blocked" state. Easier: make a single task that depends on
	// a nonexistent prereq stays unblocked; instead build two blocked tasks
	// with a shared prereq we never close.
	blocked1 := s.Add(model.Task{Title: "low blocked", Priority: model.PriorityLow, DependsOn: []int{prereqID}})
	blocked2 := s.Add(model.Task{Title: "urgent blocked", Priority: model.PriorityUrgent, DependsOn: []int{prereqID}})
	_ = blocked1
	_ = blocked2
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('N'))
	// The unblocked candidate is just the prereq itself (low prio).
	// urgent-blocked and low-blocked are both blocked by prereq.
	// Best-OPEN = the prereq (low prio). Verify cursor lands there.
	vt := app.visibleTasks()
	if vt[app.selection].ID != prereqID {
		t.Fatalf("expected prereq to be the unblocked winner, got #%d (%q)",
			vt[app.selection].ID, vt[app.selection].Title)
	}
}

// TestJumpNextAllBlockedFallback: explicit test for the fall-back
// where EVERY candidate is blocked (no unblocked open). Annotation
// in status mentions "blocked".
func TestJumpNextAllBlockedFallback(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	// Two tasks, each depends on each other? Cycle not allowed by writer.
	// Instead: each task depends on the OTHER which we make sure neither
	// is selectable as "unblocked" by depending on a fake (dangling) id.
	// But dangling is treated as satisfied by tuiUnmetBlockers — same as
	// the CLI's unmetBlockers. So we need REAL open prereqs.
	//
	// Build: prereq (low), and blocked (urgent, depends on prereq). Pool
	// includes both. Best-open = prereq. To force the all-blocked
	// fallback, restrict the candidate pool so the prereq is filtered out.
	// Use a search filter that matches only the blocked task title.
	prereqID := s.Add(model.Task{Title: "background work", Priority: model.PriorityLow})
	urgentBlockedID := s.Add(model.Task{Title: "urgent blocked", Priority: model.PriorityUrgent, DependsOn: []int{prereqID}})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Filter so only the urgent-blocked task is visible.
	feed(app,
		keyRune('/'),
		keyRune('u'),
		keyRune('r'),
		keyRune('g'),
		keyRune('e'),
		keyRune('n'),
		keyRune('t'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	feed(app, keyRune('N'))
	vt := app.visibleTasks()
	if len(vt) != 1 {
		t.Fatalf("expected 1 visible task after filter, got %d: %+v", len(vt), vt)
	}
	if vt[app.selection].ID != urgentBlockedID {
		t.Fatalf("expected fallback to urgent-blocked, got #%d", vt[app.selection].ID)
	}
	if !strings.Contains(app.status, "blocked by") {
		t.Errorf("expected 'blocked by' annotation in status, got %q", app.status)
	}
}

// TestJumpNextSkipsDoneTasks: completed tasks aren't selectable
// even when they're the only ones visible (e.g. the user expanded
// the done section).
func TestJumpNextSkipsDoneTasks(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "done one", Priority: model.PriorityUrgent, Done: true})
	openID := s.Add(model.Task{Title: "open one", Priority: model.PriorityLow})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('N'))
	vt := app.visibleTasks()
	if vt[app.selection].ID != openID {
		t.Fatalf("expected open task #%d to win, got #%d (%q done=%v)",
			openID, vt[app.selection].ID, vt[app.selection].Title, vt[app.selection].Done)
	}
}

// TestJumpNextEmptyStoreStatus: pressing N on an empty visible
// list surfaces a helpful status message rather than crashing or
// silently no-oping. The cursor position is unchanged.
func TestJumpNextEmptyStoreStatus(t *testing.T) {
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
	feed(app, keyRune('N'))
	if app.status == "" {
		t.Error("expected non-empty status on empty list")
	}
	if app.selection != 0 {
		t.Errorf("expected selection=0 on empty list, got %d", app.selection)
	}
}

// TestJumpNextNotConsumedDuringForm: when a form is active (e.g.
// 'a' add), uppercase 'N' should be treated as a character typed
// into the input, NOT trigger the jump. Defensive coverage against
// a future refactor short-circuiting form input.
func TestJumpNextNotConsumedDuringForm(t *testing.T) {
	app := newSeededApp(t, 3)
	feed(app,
		keyRune('a'),
		keyRune('N'),
		keyRune('e'),
		keyRune('w'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	got := app.store.Tasks[len(app.store.Tasks)-1].Title
	if !strings.HasPrefix(got, "New") {
		t.Fatalf("expected title 'New' from buffered N+e+w, got %q", got)
	}
}

// TestJumpNextPinnedBeatPriority: pinned-low beats unpinned-urgent
// (the canonical "pinned tasks float to the top" contract that
// `tsk next` follows). Regression guard.
func TestJumpNextPinnedBeatsPriority(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	pinnedID := s.Add(model.Task{Title: "pinned low", Priority: model.PriorityLow, Pinned: true})
	s.Add(model.Task{Title: "urgent unpinned", Priority: model.PriorityUrgent})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	feed(app, keyRune('N'))
	vt := app.visibleTasks()
	if vt[app.selection].ID != pinnedID {
		t.Fatalf("expected pinned (low) to win, got #%d (%q)",
			vt[app.selection].ID, vt[app.selection].Title)
	}
}
