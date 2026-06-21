package commands

import (
	"strings"
	"testing"
)

// TestTopoSinceTrimsPrereqs: chain 3 -> 2 -> 1, `topo --since 2`
// drops #1 from the head (it's a prereq before the checkpoint) and
// emits the trailing slice [2, 3].
func TestTopoSinceTrimsPrereqs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "middle", "top"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3->2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--since", "2")
	if err != nil {
		t.Fatalf("topo --since 2: %v", err)
	}
	if strings.Contains(stdout, "prereq") {
		t.Fatalf("#1 (prereq) should be dropped, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "middle") || !strings.Contains(stdout, "top") {
		t.Fatalf("expected #2 (middle) and #3 (top) to remain, got:\n%s", stdout)
	}
	i2 := strings.Index(stdout, "middle")
	i3 := strings.Index(stdout, "top")
	if !(i2 >= 0 && i3 > i2) {
		t.Fatalf("expected middle before top, got positions (%d, %d):\n%s", i2, i3, stdout)
	}
}

// TestTopoSinceMissingID errors out with an exit-2 usage error
// rather than emitting silent empty output. The user typoed.
func TestTopoSinceMissingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "topo", "--since", "999")
	if err == nil {
		t.Fatal("expected error for missing id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
}

// TestTopoSinceDoneIDRequiresAll: if #1 is done, `topo --since 1`
// without --all errors out (done tasks are excluded from the
// emitted pool, so the id won't appear). The error message should
// mention --all so the user knows the workaround.
func TestTopoSinceDoneIDRequiresAll(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"done-one", "open-two"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "topo", "--since", "1")
	if err == nil {
		t.Fatal("expected usage error since #1 isn't in default topo output")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Fatalf("error should mention --all workaround, got: %v", err)
	}
	// With --all: the slice should work; head is #1.
	stdout, _, err := runCmd(t, dir, "topo", "--since", "1", "--all")
	if err != nil {
		t.Fatalf("topo --since 1 --all: %v", err)
	}
	if !strings.Contains(stdout, "done-one") || !strings.Contains(stdout, "open-two") {
		t.Fatalf("expected both rows, got:\n%s", stdout)
	}
}

// TestTopoSinceComposesWithIDs: --since trims the comma-separated
// id output too. Useful for piped commands.
func TestTopoSinceComposesWithIDs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Chain: 4 -> 3 -> 2 -> 1
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "3")
	if err != nil {
		t.Fatalf("topo --ids --since 3: %v", err)
	}
	if strings.TrimSpace(stdout) != "3,4" {
		t.Fatalf("expected '3,4', got %q", stdout)
	}
}

// TestTopoSinceFirstIDIsNoop: --since on the natural head returns
// the full sequence unchanged.
func TestTopoSinceFirstIDIsNoop(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"head", "tail"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "1")
	if err != nil {
		t.Fatalf("topo --since 1: %v", err)
	}
	if strings.TrimSpace(stdout) != "1,2" {
		t.Fatalf("expected '1,2' (no trim), got %q", stdout)
	}
}

// TestSliceTopoSinceIsolated is a direct unit test on the helper to
// guard the edge case where the id is in the cycle tail (a
// hand-edited corruption is still a useful anchor).
func TestSliceTopoSinceIsolated(t *testing.T) {
	mk := func(id int, cycle bool) topoTask {
		return topoTask{InCycle: cycle}
	}
	_ = mk // satisfy unused warning
	// Synthesize a slice by hand so we don't need a store.
	a := topoTask{InCycle: false}
	b := topoTask{InCycle: false}
	c := topoTask{InCycle: true}
	// Set ids via the embedded Task field.
	a.Task.ID = 1
	b.Task.ID = 2
	c.Task.ID = 3
	in := []topoTask{a, b, c}
	got := sliceTopoSince(in, 3) // cycle row, but a valid anchor
	if len(got) != 1 || got[0].Task.ID != 3 {
		t.Fatalf("expected one row (#3), got %d rows", len(got))
	}
	got = sliceTopoSince(in, 99) // missing id
	if got != nil {
		t.Fatalf("expected nil for missing id, got %d rows", len(got))
	}
	got = sliceTopoSince(in, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows from #2 onward, got %d", len(got))
	}
	if got[0].Task.ID != 2 || got[1].Task.ID != 3 {
		t.Fatalf("expected ids [2, 3], got %d, %d", got[0].Task.ID, got[1].Task.ID)
	}
}
