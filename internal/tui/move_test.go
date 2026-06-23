package tui

import (
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// TestMoveCurrentDownSwapsWithNextStoreNeighbor: pressing '>' on
// the selected task swaps it with the next task in store order
// (NOT the visible/grouped order). The .tsk.md file reflects the
// new position; the in-memory store agrees.
func TestMoveCurrentDownSwapsWithNextStoreNeighbor(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	idA := s.Add(model.Task{Title: "alpha", Priority: model.PriorityHigh})
	idB := s.Add(model.Task{Title: "beta", Priority: model.PriorityMedium})
	idC := s.Add(model.Task{Title: "gamma", Priority: model.PriorityLow})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	app.selection = 0 // alpha
	feed(app, keyRune('>'))
	// Reload from disk: alpha and beta should have swapped positions.
	reloaded, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reloaded.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(reloaded.Tasks))
	}
	if reloaded.Tasks[0].ID != idB {
		t.Errorf("expected position 0 = beta (#%d), got #%d (%q)", idB, reloaded.Tasks[0].ID, reloaded.Tasks[0].Title)
	}
	if reloaded.Tasks[1].ID != idA {
		t.Errorf("expected position 1 = alpha (#%d), got #%d (%q)", idA, reloaded.Tasks[1].ID, reloaded.Tasks[1].Title)
	}
	if reloaded.Tasks[2].ID != idC {
		t.Errorf("expected position 2 = gamma (#%d) unchanged, got #%d", idC, reloaded.Tasks[2].ID)
	}
	if !strings.Contains(app.status, "moved #") {
		t.Errorf("expected 'moved #' in status, got %q", app.status)
	}
}

// TestMoveCurrentUpSwapsWithPrevStoreNeighbor: pressing '<' on a
// task that's NOT at the top of the store swaps it with the
// preceding task in store order.
func TestMoveCurrentUpSwapsWithPrevStoreNeighbor(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	idA := s.Add(model.Task{Title: "alpha", Priority: model.PriorityLow})
	idB := s.Add(model.Task{Title: "beta", Priority: model.PriorityMedium})
	idC := s.Add(model.Task{Title: "gamma", Priority: model.PriorityHigh})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Position the cursor on gamma (store index 2). Use visibleTasks
	// length math to find it — sorting/grouping may have shuffled
	// the visible row.
	vt := app.visibleTasks()
	for i, vt := range vt {
		if vt.ID == idC {
			app.selection = i
			break
		}
	}
	_ = vt
	feed(app, keyRune('<'))
	reloaded, _ := store.Load(dir + "/.tsk.md")
	// gamma (was store index 2) should now be at store index 1;
	// beta (was 1) at 2.
	if reloaded.Tasks[0].ID != idA {
		t.Errorf("position 0 should remain alpha (#%d), got #%d", idA, reloaded.Tasks[0].ID)
	}
	if reloaded.Tasks[1].ID != idC {
		t.Errorf("position 1 should be gamma (#%d) after move-up, got #%d", idC, reloaded.Tasks[1].ID)
	}
	if reloaded.Tasks[2].ID != idB {
		t.Errorf("position 2 should be beta (#%d) after gamma moved up, got #%d", idB, reloaded.Tasks[2].ID)
	}
}

// TestMoveCurrentUpAtTopReportsEdge: pressing '<' when the task
// is already at store position 0 leaves the file untouched and
// surfaces "already at start" in the status footer.
func TestMoveCurrentUpAtTopReportsEdge(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	idA := s.Add(model.Task{Title: "alpha", Priority: model.PriorityHigh})
	s.Add(model.Task{Title: "beta", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	app.selection = 0
	feed(app, keyRune('<'))
	if !strings.Contains(app.status, "already at start") {
		t.Errorf("expected 'already at start' in status, got %q", app.status)
	}
	reloaded, _ := store.Load(dir + "/.tsk.md")
	if reloaded.Tasks[0].ID != idA {
		t.Errorf("file should be unchanged after edge-move, alpha (#%d) should still be at 0, got #%d", idA, reloaded.Tasks[0].ID)
	}
}

// TestMoveCurrentDownAtBottomReportsEdge: pressing '>' on the
// last task in store order is a no-op with a "already at end"
// hint.
func TestMoveCurrentDownAtBottomReportsEdge(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "alpha", Priority: model.PriorityHigh})
	idB := s.Add(model.Task{Title: "beta", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Position cursor on beta (store last).
	vt := app.visibleTasks()
	for i, t := range vt {
		if t.ID == idB {
			app.selection = i
			break
		}
	}
	feed(app, keyRune('>'))
	if !strings.Contains(app.status, "already at end") {
		t.Errorf("expected 'already at end' in status, got %q", app.status)
	}
	reloaded, _ := store.Load(dir + "/.tsk.md")
	if reloaded.Tasks[len(reloaded.Tasks)-1].ID != idB {
		t.Errorf("beta should still be last after edge-move, got #%d at end", reloaded.Tasks[len(reloaded.Tasks)-1].ID)
	}
}

// TestMoveCurrentSelectionFollowsTask: after a move, the cursor
// stays on the SAME TASK (not the same visual row index). The
// user's mental anchor is the task they were holding.
func TestMoveCurrentSelectionFollowsTask(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	idA := s.Add(model.Task{Title: "alpha", Priority: model.PriorityHigh})
	s.Add(model.Task{Title: "beta", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Find alpha's visible index before the move.
	vt := app.visibleTasks()
	for i, t := range vt {
		if t.ID == idA {
			app.selection = i
			break
		}
	}
	feed(app, keyRune('>'))
	// After the move, alpha is at store index 1; visible index
	// may differ due to grouping. Verify selection still points at
	// alpha.
	vtAfter := app.visibleTasks()
	if app.selection < 0 || app.selection >= len(vtAfter) {
		t.Fatalf("selection out of range after move: %d (len %d)", app.selection, len(vtAfter))
	}
	if vtAfter[app.selection].ID != idA {
		t.Errorf("selection should still point at alpha (#%d) after move, got #%d", idA, vtAfter[app.selection].ID)
	}
}

// TestMoveCurrentRoundTripIsIdentity: a move-down immediately
// followed by a move-up (or vice-versa) returns the file to its
// original order. Catches drift in the swap math.
func TestMoveCurrentRoundTripIsIdentity(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	idA := s.Add(model.Task{Title: "alpha", Priority: model.PriorityHigh})
	idB := s.Add(model.Task{Title: "beta", Priority: model.PriorityMedium})
	idC := s.Add(model.Task{Title: "gamma", Priority: model.PriorityLow})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	originalOrder := []int{idA, idB, idC}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	app.selection = 0
	feed(app, keyRune('>'))
	feed(app, keyRune('<'))
	reloaded, _ := store.Load(dir + "/.tsk.md")
	for i, want := range originalOrder {
		if reloaded.Tasks[i].ID != want {
			t.Errorf("position %d after round-trip: want #%d, got #%d", i, want, reloaded.Tasks[i].ID)
		}
	}
}

// TestMoveCurrentEmptyStoreNoOp: pressing '<' or '>' on an empty
// store surfaces "no task selected" without crashing.
func TestMoveCurrentEmptyStoreNoOp(t *testing.T) {
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
	feed(app, keyRune('<'))
	if !strings.Contains(app.status, "no task selected") {
		t.Errorf("expected 'no task selected' on empty store '<', got %q", app.status)
	}
	app.status = ""
	feed(app, keyRune('>'))
	if !strings.Contains(app.status, "no task selected") {
		t.Errorf("expected 'no task selected' on empty store '>', got %q", app.status)
	}
}

// TestMoveCurrentNotShadowedByFormInput: with a form open ('a' add
// or 'e' edit), '<' and '>' get consumed as literal text input —
// the move should NOT fire. Regression guard for the same input-
// shadowing bug archive/clone/togglePin guard against.
func TestMoveCurrentNotShadowedByFormInput(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	idA := s.Add(model.Task{Title: "alpha", Priority: model.PriorityHigh})
	idB := s.Add(model.Task{Title: "beta", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	originalOrder := []int{idA, idB}
	app := New(s)
	feed(app, tea.WindowSizeMsg{Width: 80, Height: 24})
	app.selection = 0
	// Open the add form.
	feed(app, keyRune('a'))
	// While the form is open, '>' should be captured as form input
	// (rune), NOT trigger a move.
	feed(app, keyRune('>'))
	reloaded, _ := store.Load(dir + "/.tsk.md")
	for i, want := range originalOrder {
		if reloaded.Tasks[i].ID != want {
			t.Errorf("file should be unchanged while form is open: position %d want #%d got #%d", i, want, reloaded.Tasks[i].ID)
		}
	}
	if app.inputCur.value != ">" {
		t.Errorf("expected '>' to land in form input, got %q", app.inputCur.value)
	}
}

// TestMoveCurrentHelpFooterMention: the footer hint and help
// overlay both advertise '<' and '>' so the user discovers them.
func TestMoveCurrentHelpFooterMention(t *testing.T) {
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
	if !strings.Contains(view, "</>") {
		t.Errorf("footer should mention '</>' reorder hint, got:\n%s", view)
	}
	help := app.helpView()
	if !strings.Contains(help, "<") || !strings.Contains(help, ">") {
		t.Errorf("help overlay should mention '<' and '>', got:\n%s", help)
	}
	if !strings.Contains(help, "move task up") {
		t.Errorf("help should describe move-up, got:\n%s", help)
	}
	if !strings.Contains(help, "move task down") {
		t.Errorf("help should describe move-down, got:\n%s", help)
	}
}
