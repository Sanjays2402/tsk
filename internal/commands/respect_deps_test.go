package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Setup helper: creates a graph where:
//   - id 1: LOW prereq, open
//   - id 2: URGENT blocked task (depends on 1)
//   - id 3: MEDIUM unblocked task
//
// Default ordering puts id=2 first (urgent priority wins). With
// --respect-deps, id=2 is filtered out and id=3 (medium) beats id=1
// (low), so id=3 wins. That makes both branches of the test
// observable from the public CLI surface.
func setupRespectDepsScenario(t *testing.T, dir string) {
	t.Helper()
	if _, _, err := runCmd(t, dir, "add", "prereq", "-p", "low"); err != nil {
		t.Fatalf("add prereq: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "blocked-thing", "-p", "urgent"); err != nil {
		t.Fatalf("add blocked: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "free-thing", "-p", "medium"); err != nil {
		t.Fatalf("add free: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
}

// TestNextRespectDepsSkipsBlocked: with --respect-deps, the highest-
// priority blocked task is skipped in favor of a lower-priority
// unblocked one.
func TestNextRespectDepsSkipsBlocked(t *testing.T) {
	dir := t.TempDir()
	setupRespectDepsScenario(t, dir)
	// Legacy behavior: blocked-thing wins on priority alone.
	legacy, _, err := runCmd(t, dir, "next")
	if err != nil {
		t.Fatalf("next legacy: %v", err)
	}
	if !strings.Contains(legacy, "blocked-thing") {
		t.Fatalf("expected blocked-thing in legacy next, got:\n%s", legacy)
	}
	// New behavior: free-thing wins because blocked-thing is filtered
	// and free-thing (medium) beats prereq (low).
	stdout, _, err := runCmd(t, dir, "next", "--respect-deps")
	if err != nil {
		t.Fatalf("next --respect-deps: %v", err)
	}
	if !strings.Contains(stdout, "free-thing") {
		t.Fatalf("expected free-thing with --respect-deps, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "blocked-thing") {
		t.Fatalf("blocked-thing should be filtered out, got:\n%s", stdout)
	}
}

// TestNextRespectDepsAllBlockedFallback: when every undone task is
// blocked (e.g. a hand-edited deeper cycle the writer doesn't detect),
// --respect-deps falls back to the best-blocked task with a
// "(blocked by ...)" annotation rather than printing "all caught up"
// (which would be a lie).
//
// The only way to get a fully-blocked open pool is a 3+ node cycle —
// the writer rejects self-deps and direct 2-cycles, but bigger cycles
// are intentionally tolerated (documented in tick #9). We construct
// one via direct file mutation to test the fallback path.
func TestNextRespectDepsAllBlockedFallback(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 1->2, 2->3 via the CLI (legal); then hand-splice 3->1 to close
	// the cycle the writer would reject.
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend 1->2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "3"); err != nil {
		t.Fatalf("depend 2->3: %v", err)
	}
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Splice depends:1 into task 3's meta block.
	lines := strings.Split(string(body), "\n")
	for i, l := range lines {
		if strings.Contains(l, "id:3 ") && !strings.Contains(l, "depends:") {
			lines[i] = strings.Replace(l, "-->", "depends:1 -->", 1)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "next", "--respect-deps")
	if err != nil {
		t.Fatalf("next --respect-deps all-blocked: %v", err)
	}
	if !strings.Contains(stdout, "(blocked by") {
		t.Fatalf("expected '(blocked by ...)' annotation in fallback, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "all caught up") {
		t.Fatalf("must NOT say 'all caught up' when blocked tasks exist, got:\n%s", stdout)
	}
}

// TestNextNoTasks: empty store yields "all caught up" with or without
// --respect-deps (no fallback path triggers because there's no
// blocked candidate either).
func TestNextNoTasksRespectDeps(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "first"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "next", "--respect-deps")
	if err != nil {
		t.Fatalf("next --respect-deps: %v", err)
	}
	if !strings.Contains(stdout, "all caught up") {
		t.Fatalf("expected 'all caught up', got:\n%s", stdout)
	}
}

// TestTopRespectDepsFiltersList: --respect-deps drops every blocked
// task from the top-N output.
func TestTopRespectDepsFiltersList(t *testing.T) {
	dir := t.TempDir()
	setupRespectDepsScenario(t, dir)
	stdout, _, err := runCmd(t, dir, "top", "--respect-deps")
	if err != nil {
		t.Fatalf("top --respect-deps: %v", err)
	}
	if strings.Contains(stdout, "blocked-thing") {
		t.Fatalf("blocked-thing should be filtered out of top, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "free-thing") {
		t.Fatalf("expected free-thing in top, got:\n%s", stdout)
	}
	// prereq itself is open and has no deps, so it should appear.
	if !strings.Contains(stdout, "prereq") {
		t.Fatalf("expected prereq in top (it's unblocked), got:\n%s", stdout)
	}
}

// TestLsRespectDepsFilters: ls --respect-deps drops blocked tasks
// across the full listing.
func TestLsRespectDepsFilters(t *testing.T) {
	dir := t.TempDir()
	setupRespectDepsScenario(t, dir)
	// Without --respect-deps: all three tasks visible.
	stdout, _, err := runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(stdout, "blocked-thing") {
		t.Fatalf("ls without --respect-deps should include blocked-thing, got:\n%s", stdout)
	}
	// With --respect-deps: blocked-thing is gone.
	stdout, _, err = runCmd(t, dir, "ls", "--respect-deps")
	if err != nil {
		t.Fatalf("ls --respect-deps: %v", err)
	}
	if strings.Contains(stdout, "blocked-thing") {
		t.Fatalf("ls --respect-deps should drop blocked-thing, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "free-thing") {
		t.Fatalf("free-thing should still be present, got:\n%s", stdout)
	}
}

// TestRespectDepsLegacyDefaultUnchanged: without the flag, behavior
// matches the legacy "pure priority" ordering — important for not
// breaking existing scripts that pipe `tsk next`.
func TestRespectDepsLegacyDefaultUnchanged(t *testing.T) {
	dir := t.TempDir()
	setupRespectDepsScenario(t, dir)
	stdout, _, err := runCmd(t, dir, "next")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	// blocked-thing (urgent) beats free-thing (medium) and prereq (low)
	// on priority alone, even though it's blocked.
	if !strings.Contains(stdout, "blocked-thing") {
		t.Fatalf("legacy next should return blocked-thing on priority, got:\n%s", stdout)
	}
}

// TestRespectDepsSatisfiedDepNotBlocking: when the only dep is DONE,
// the task is not "blocked" — should appear in --respect-deps view.
func TestRespectDepsSatisfiedDepNotBlocking(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "top", "--respect-deps")
	if err != nil {
		t.Fatalf("top --respect-deps: %v", err)
	}
	// Task 2 (dependent) should appear since its prereq is satisfied.
	if !strings.Contains(stdout, "dependent") {
		t.Fatalf("dependent task should appear once prereq is done, got:\n%s", stdout)
	}
}
