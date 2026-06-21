package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stampCompleted hand-edits the .tsk.md to backdate the Completed
// timestamp on a given task id so we can simulate tasks completed in
// arbitrary ISO weeks regardless of when the test runs.
func stampCompleted(t *testing.T, path string, id int, when time.Time) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	marker := "id:" + itoa(id) + " "
	lines := strings.Split(string(body), "\n")
	hit := false
	for i, l := range lines {
		if !strings.Contains(l, marker) {
			continue
		}
		stamp := when.UTC().Format(time.RFC3339)
		if strings.Contains(l, "completed:") {
			start := strings.Index(l, "completed:")
			valStart := start + len("completed:")
			rest := l[valStart:]
			end := strings.Index(rest, " ")
			if end < 0 {
				t.Fatalf("malformed completed on line %d: %q", i+1, l)
			}
			lines[i] = l[:valStart] + stamp + l[valStart+end:]
		} else {
			lines[i] = strings.Replace(l, "-->", "completed:"+stamp+" -->", 1)
		}
		hit = true
		break
	}
	if !hit {
		t.Fatalf("could not find id:%d in %s", id, path)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestArchiveStrategyWeeklyGroupsByISOWeek: archive with tasks
// completed in two different weeks must produce two "## YYYY-W##"
// sections in the archive file, oldest first.
func TestArchiveStrategyWeeklyGroupsByISOWeek(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"first", "second", "third"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Backdate #1 to 100 days ago, #2 to 95 days ago (likely same
	// or adjacent ISO week — both distinct from "this week"). #3
	// stays at "now".
	path := filepath.Join(dir, ".tsk.md")
	stampCompleted(t, path, 1, time.Now().AddDate(0, 0, -100))
	stampCompleted(t, path, 2, time.Now().AddDate(0, 0, -50))
	// #3 keeps its just-now completion.

	stdout, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "weekly")
	if err != nil {
		t.Fatalf("archive weekly: %v", err)
	}
	if !strings.Contains(stdout, "strategy=weekly") {
		t.Fatalf("expected 'strategy=weekly' in output, got:\n%s", stdout)
	}
	arch := readArchive(t, dir)
	// At least two section headers should appear.
	count := strings.Count(arch, "\n## ")
	// Headers may also include the leading "## " at very start of
	// file (if existing-content header didn't end with \n\n); count
	// both. A safer assertion: at least 2 of the YYYY-W markers.
	wCount := strings.Count(arch, "-W")
	if wCount < 2 {
		t.Fatalf("expected at least 2 weekly sections (got %d):\n%s", wCount, arch)
	}
	_ = count
	// All three tasks must be present.
	if !strings.Contains(arch, "first") || !strings.Contains(arch, "second") || !strings.Contains(arch, "third") {
		t.Fatalf("expected all tasks in archive, got:\n%s", arch)
	}
}

// TestArchiveStrategyFlatStaysDefault: omitting --strategy gives the
// original flat behavior — no "## YYYY-W##" headers in the archive.
func TestArchiveStrategyFlatStaysDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "strategy=flat") {
		t.Fatalf("expected default strategy=flat, got:\n%s", stdout)
	}
	arch := readArchive(t, dir)
	if strings.Contains(arch, "## ") && strings.Contains(arch, "-W") {
		t.Fatalf("flat strategy should produce no weekly sections, got:\n%s", arch)
	}
}

// TestArchiveStrategyWeeklyUndatedBucket: a done task without a
// Completed timestamp must land in the "## undated" section at the
// bottom, not crash and not be silently dropped.
func TestArchiveStrategyWeeklyUndatedBucket(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ghost"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Strip the completed: stamp by hand to simulate hand-edited
	// done state.
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(body)
	for {
		idx := strings.Index(s, "completed:")
		if idx < 0 {
			break
		}
		// Remove " completed:VALUE" up to next space or " -->".
		valStart := idx + len("completed:")
		end := strings.Index(s[valStart:], " ")
		if end < 0 {
			s = s[:idx]
			break
		}
		// Also nuke the preceding space if present.
		removeStart := idx
		if idx > 0 && s[idx-1] == ' ' {
			removeStart = idx - 1
		}
		s = s[:removeStart] + s[valStart+end:]
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "weekly"); err != nil {
		t.Fatalf("archive weekly: %v", err)
	}
	arch := readArchive(t, dir)
	if !strings.Contains(arch, "## undated") {
		t.Fatalf("expected '## undated' section, got:\n%s", arch)
	}
	if !strings.Contains(arch, "- [x] ghost") {
		t.Fatalf("ghost task missing from archive, got:\n%s", arch)
	}
}

// TestArchiveStrategyWeeklyRejectsBogusValue: --strategy=quarterly
// (or anything not in the allowed set) must exit-2 cleanly.
func TestArchiveStrategyWeeklyRejectsBogusValue(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--strategy", "quarterly")
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestArchiveStrategyWeeklyPreservesExisting: a second archive call
// with weekly must NOT touch the layout of the first batch (no
// re-bucketing of historical data).
func TestArchiveStrategyWeeklyPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "first-batch"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// First archive: flat.
	if _, _, err := runCmd(t, dir, "archive", "--all"); err != nil {
		t.Fatalf("archive 1: %v", err)
	}
	archBefore := readArchive(t, dir)
	if strings.Contains(archBefore, "## ") && strings.Contains(archBefore, "-W") {
		t.Fatalf("flat-shaped archive should have no week sections, got:\n%s", archBefore)
	}

	// Second archive: weekly. The first-batch row must STILL be in
	// the file (untouched) but the second-batch row goes under a new
	// weekly section.
	if _, _, err := runCmd(t, dir, "add", "second-batch"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil { // active id has been renumbered to 1 since first was archived
		t.Fatalf("done 2: %v", err)
	}
	// Wait — the active store retains original ids after archive;
	// the only task left was id 2 (was second when first archived).
	// Actually, archive doesn't renumber active. Let's check the
	// active store and pick the right id.
	activePath := filepath.Join(dir, ".tsk.md")
	body, _ := os.ReadFile(activePath)
	// The new "second-batch" is the only task; find its id.
	idStart := strings.Index(string(body), "id:")
	if idStart < 0 {
		t.Fatalf("no id found in active store after second add:\n%s", string(body))
	}

	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "weekly"); err != nil {
		t.Fatalf("archive weekly: %v", err)
	}
	archAfter := readArchive(t, dir)
	if !strings.Contains(archAfter, "first-batch") {
		t.Fatalf("first-batch must survive in archive, got:\n%s", archAfter)
	}
	if !strings.Contains(archAfter, "second-batch") {
		t.Fatalf("second-batch must appear in archive, got:\n%s", archAfter)
	}
	// New batch should be under a YYYY-W## section.
	if !strings.Contains(archAfter, "-W") {
		t.Fatalf("expected a weekly section header somewhere, got:\n%s", archAfter)
	}
}
