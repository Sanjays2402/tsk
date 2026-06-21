package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestJustifyDoneTaskClearMessage: marking the root done should
// short-circuit with a clear "not blocked" line — the user
// shouldn't see a confusing chain when they're asking about
// already-completed work.
func TestJustifyDoneTaskClearMessage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "finished"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--justify")
	if err != nil {
		t.Fatalf("justify: %v", err)
	}
	if !strings.Contains(stdout, "done, not blocked") {
		t.Fatalf("expected 'done, not blocked' for done root, got:\n%s", stdout)
	}
}

// TestJustifyNoDeps: a task with no prereqs at all is actionable —
// the message should reflect that (distinct from "blocked by no
// open prereqs" which means deps exist but are satisfied).
func TestJustifyNoDeps(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "free"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--justify")
	if err != nil {
		t.Fatalf("justify: %v", err)
	}
	if !strings.Contains(stdout, "no dependencies") {
		t.Fatalf("expected 'no dependencies' message, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "actionable") {
		t.Fatalf("expected 'actionable' framing, got:\n%s", stdout)
	}
}

// TestJustifyAllDepsSatisfied: task with deps but all already
// done — should report "open with no unmet prereqs", distinct
// from the no-deps message.
func TestJustifyAllDepsSatisfied(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "dependent"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "2", "--justify")
	if err != nil {
		t.Fatalf("justify: %v", err)
	}
	if !strings.Contains(stdout, "no unmet prereqs") {
		t.Fatalf("expected 'no unmet prereqs' message, got:\n%s", stdout)
	}
}

// TestJustifyDeepChain: 4 → 3 → 2 → 1 chain. Asking justify on #4
// should walk all the way to #1 and mark it "START HERE".
func TestJustifyDeepChain(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deepest", "mid", "next", "top"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "4", "--justify")
	if err != nil {
		t.Fatalf("justify: %v", err)
	}
	// All four ids appear in chain order.
	for _, want := range []string{"#4", "#3", "#2", "#1"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in chain, got:\n%s", want, stdout)
		}
	}
	if !strings.Contains(stdout, "START HERE") {
		t.Fatalf("expected 'START HERE' marker on leaf, got:\n%s", stdout)
	}
	// Order check: #4 before #3 before #2 before #1.
	i4 := strings.Index(stdout, "#4")
	i3 := strings.Index(stdout, "#3")
	i2 := strings.Index(stdout, "#2")
	i1 := strings.Index(stdout, "#1")
	if !(i4 < i3 && i3 < i2 && i2 < i1) {
		t.Fatalf("expected chain order, got positions (4=%d, 3=%d, 2=%d, 1=%d):\n%s",
			i4, i3, i2, i1, stdout)
	}
}

// TestJustifyTieBreakLowestID: when a task has multiple open
// prereqs, justify follows the LOWEST id at each step (determ-
// inistic). Setup: #3 depends on both #1 and #2; both are open;
// justify should pick #1.
func TestJustifyTieBreakLowestID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "3", "--justify")
	if err != nil {
		t.Fatalf("justify: %v", err)
	}
	// #1 should appear (lowest id chosen).
	if !strings.Contains(stdout, "#1") {
		t.Fatalf("expected #1 (lowest id pick), got:\n%s", stdout)
	}
	// #2 should NOT appear — justify follows ONE chain.
	if strings.Contains(stdout, "#2") {
		t.Fatalf("justify should follow one chain only, but #2 also appears:\n%s", stdout)
	}
}

// TestJustifyJSONShape: --justify --json emits an array of step
// objects with stable schema. Each step has id and status; blocked
// rows also have blocked_by.
func TestJustifyJSONShape(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "2", "--justify", "--json")
	if err != nil {
		t.Fatalf("justify --json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 steps, got %d:\n%s", len(rows), stdout)
	}
	if int(rows[0]["id"].(float64)) != 2 || rows[0]["status"].(string) != "blocked" {
		t.Fatalf("expected root step id=2 status=blocked, got %v", rows[0])
	}
	if int(rows[0]["blocked_by"].(float64)) != 1 {
		t.Fatalf("expected blocked_by=1, got %v", rows[0]["blocked_by"])
	}
	if rows[1]["status"].(string) != "open-leaf" {
		t.Fatalf("expected leaf status=open-leaf, got %v", rows[1])
	}
	// Leaf has no blocked_by.
	if _, has := rows[1]["blocked_by"]; has {
		t.Fatalf("leaf should not have blocked_by, got %v", rows[1])
	}
}

// TestJustifyCycleSafe: a hand-edited cycle must not loop the
// walker. The chain should terminate with "(cycle)" sentinel.
func TestJustifyCycleSafe(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Splice 1 → 2 into the file via hand-edit so a cycle exists.
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(string(body), "\n")
	for i, l := range lines {
		if strings.Contains(l, "id:1 ") && !strings.Contains(l, "depends:") {
			lines[i] = strings.Replace(l, "-->", "depends:2 -->", 1)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "2", "--justify")
	if err != nil {
		t.Fatalf("justify cycle: %v", err)
	}
	if !strings.Contains(stdout, "(cycle") {
		t.Fatalf("expected '(cycle' marker, got:\n%s", stdout)
	}
}

// TestJustifyRequiresID: --justify without a positional id is a
// usage error (exit 2).
func TestJustifyRequiresID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "--justify")
	if err == nil {
		t.Fatal("expected error for --justify without id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestJustifyRejectsMutationFlags: --justify is read-only; can't
// combine with --on/--add/etc.
func TestJustifyRejectsMutationFlags(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	_, _, err := runCmd(t, dir, "depend", "1", "--justify", "--on", "2")
	if err == nil {
		t.Fatal("expected error for --justify + --on combo")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestJustifyMutuallyExclusiveWithTree: --tree and --justify both
// walk the chain but render differently — combining them is a
// usage error.
func TestJustifyMutuallyExclusiveWithTree(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	_, _, err := runCmd(t, dir, "depend", "1", "--justify", "--tree")
	if err == nil {
		t.Fatal("expected error for --justify + --tree combo")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestJustifyIndentsByDepth: each hop should be indented two spaces
// more than its parent so the chain shape is visually obvious.
func TestJustifyIndentsByDepth(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "3", "--justify")
	if err != nil {
		t.Fatalf("justify: %v", err)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), stdout)
	}
	// Root flush-left, second indented 2, third indented 4.
	if strings.HasPrefix(lines[0], " ") {
		t.Fatalf("root should be flush-left, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  ") || strings.HasPrefix(lines[1], "    ") {
		t.Fatalf("second line should start with exactly 2 spaces, got %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "    ") {
		t.Fatalf("third line should start with 4 spaces, got %q", lines[2])
	}
}
