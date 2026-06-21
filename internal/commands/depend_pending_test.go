package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPendingListsFreshlyUnblockedTasks: a task whose every dep was
// recently completed should show up. Setup: #2 depends on #1; mark
// #1 done; --pending should list #2 with a "(unblocked by #1 ...)"
// annotation.
func TestPendingListsFreshlyUnblockedTasks(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocked"} {
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
	stdout, _, err := runCmd(t, dir, "depend", "--pending")
	if err != nil {
		t.Fatalf("depend --pending: %v", err)
	}
	if !strings.Contains(stdout, "freshly unblocked") {
		t.Fatalf("expected 'freshly unblocked' header, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#2  blocked") {
		t.Fatalf("expected #2 in pending list, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "unblocked by #1") {
		t.Fatalf("expected '(unblocked by #1 …)' annotation, got:\n%s", stdout)
	}
}

// TestPendingExcludesStillBlockedTasks: a task with an open prereq
// must NOT be on the pending list — it's not actionable.
func TestPendingExcludesStillBlockedTasks(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq-open", "blocked"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// #1 stays open; #2 is still blocked.
	stdout, _, err := runCmd(t, dir, "depend", "--pending")
	if err != nil {
		t.Fatalf("depend --pending: %v", err)
	}
	if strings.Contains(stdout, "#2") {
		t.Fatalf("still-blocked #2 must not appear in pending, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "no tasks freshly unblocked") {
		t.Fatalf("expected empty-state message, got:\n%s", stdout)
	}
}

// TestPendingExcludesTasksWithNoDeps: a task that was always
// actionable (no DependsOn) is not "freshly unblocked" — exclude.
func TestPendingExcludesTasksWithNoDeps(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "free-task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending")
	if err != nil {
		t.Fatalf("depend --pending: %v", err)
	}
	if strings.Contains(stdout, "#1") {
		t.Fatalf("no-dep task must not appear in pending, got:\n%s", stdout)
	}
}

// TestPendingExcludesDoneTasks: a task that's itself done isn't
// "newly actionable" — it's finished. Exclude.
func TestPendingExcludesDoneTasks(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "dependent"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending")
	if err != nil {
		t.Fatalf("depend --pending: %v", err)
	}
	if strings.Contains(stdout, "#2") {
		t.Fatalf("done #2 must not appear in pending, got:\n%s", stdout)
	}
}

// TestPendingHonorsSinceWindow: a task unblocked LONG AGO (outside
// the --since window) must be excluded — that's the whole point of
// "pending" vs "actionable". Default window is 24h.
func TestPendingHonorsSinceWindow(t *testing.T) {
	dir := t.TempDir()
	// Two prereq+dependent pairs. We'll backdate one prereq's
	// completion 5 days into the past by hand-editing the file.
	for _, title := range []string{"old-prereq", "old-dependent", "new-prereq", "new-dependent"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Backdate #1's Completed timestamp 5 days into the past by
	// rewriting the .tsk.md file directly. The store reads ISO
	// timestamps in the `completed:` meta key.
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	oldStamp := time.Now().AddDate(0, 0, -5).UTC().Format(time.RFC3339)
	lines := strings.Split(string(body), "\n")
	rewritten := false
	for i, line := range lines {
		// Find #1's task line and replace whatever completed:STAMP it has
		// with a 5d-old stamp.
		if !strings.Contains(line, "id:1 ") {
			continue
		}
		start := strings.Index(line, "completed:")
		if start < 0 {
			continue
		}
		// completed:RFC3339 — RFC3339 has no spaces, so the next
		// space terminates the value (the next meta key starts
		// after it, or " -->" closes the meta block). Looking for
		// `-` would match INSIDE the date (2026-06-21) — broken.
		valStart := start + len("completed:")
		rest := line[valStart:]
		end := strings.Index(rest, " ")
		if end < 0 {
			continue
		}
		lines[i] = line[:valStart] + oldStamp + line[valStart+end:]
		rewritten = true
		break
	}
	if !rewritten {
		t.Fatal("could not find #1's completed: stamp to backdate")
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending")
	if err != nil {
		t.Fatalf("depend --pending: %v", err)
	}
	// #4 is freshly unblocked (just now). #2 was unblocked 5d ago.
	if !strings.Contains(stdout, "#4") {
		t.Fatalf("expected #4 (new-dependent) in pending output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#2") {
		t.Fatalf("#2 was unblocked 5d ago — must be outside default 24h window, got:\n%s", stdout)
	}
	// Now ask for a 7d window — #2 should reappear.
	stdout, _, err = runCmd(t, dir, "depend", "--pending", "--since", "7d")
	if err != nil {
		t.Fatalf("depend --pending --since 7d: %v", err)
	}
	if !strings.Contains(stdout, "#2") {
		t.Fatalf("--since 7d should include the 5d-old unblock, got:\n%s", stdout)
	}
}

// TestPendingJSONShape: --pending --json emits an array of objects
// with the documented schema, empty case = `[]` (not null).
func TestPendingJSONShape(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--json")
	if err != nil {
		t.Fatalf("depend --pending --json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 pending row, got %d:\n%s", len(rows), stdout)
	}
	r := rows[0]
	if id, _ := r["id"].(float64); int(id) != 2 {
		t.Fatalf("expected id=2, got %v", r["id"])
	}
	if trig, _ := r["trigger_id"].(float64); int(trig) != 1 {
		t.Fatalf("expected trigger_id=1, got %v", r["trigger_id"])
	}
	if _, ok := r["trigger_completed"].(string); !ok {
		t.Fatalf("trigger_completed should be string (RFC3339), got %T", r["trigger_completed"])
	}
}

// TestPendingJSONEmptyArray: zero pending tasks must emit `[]`, not
// `null`.
func TestPendingJSONEmptyArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--json")
	if err != nil {
		t.Fatalf("depend --pending --json: %v", err)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed != "[]" {
		t.Fatalf("expected '[]', got %q", trimmed)
	}
}

// TestPendingRejectsBadSince: invalid --since must error out (exit 2)
// before any store work happens, so the user sees the typo cleanly.
func TestPendingRejectsBadSince(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "--pending", "--since", "banana")
	if err == nil {
		t.Fatal("expected error for bogus --since")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %v", err)
	}
}

// TestPendingMutexFlags: --pending can't combine with --list or per-id
// modes. Each is a different global/per-id view — combining is
// nonsensical.
func TestPendingMutexFlags(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, conflict := range [][]string{
		{"depend", "--pending", "--list"},
		{"depend", "1", "--pending", "--tree"},
		{"depend", "1", "--pending"}, // positional id with --pending is illegal
	} {
		_, _, err := runCmd(t, dir, conflict...)
		if err == nil {
			t.Fatalf("expected error for %v", conflict)
		}
	}
}
