package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestArchiveStrategyMonthlyGroupsByMonth: tasks completed in two
// distinct calendar months land in two distinct "## YYYY-MM"
// sections, oldest first.
func TestArchiveStrategyMonthlyGroupsByMonth(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"jan-task", "march-task", "now-task"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Backdate #1 ~120 days ago and #2 ~60 days ago (likely two
	// different months, and both distinct from "now"). #3 keeps its
	// just-now completion.
	path := filepath.Join(dir, ".tsk.md")
	stampCompleted(t, path, 1, time.Now().AddDate(0, 0, -120))
	stampCompleted(t, path, 2, time.Now().AddDate(0, 0, -60))

	stdout, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "monthly")
	if err != nil {
		t.Fatalf("archive monthly: %v", err)
	}
	if !strings.Contains(stdout, "strategy=monthly") {
		t.Fatalf("expected 'strategy=monthly' in output, got:\n%s", stdout)
	}
	arch := readArchive(t, dir)
	// Section headers look like "## YYYY-MM" (no W). Count distinct
	// matches — at least 2 month buckets given the three timestamps
	// span well over a month.
	rx := regexp.MustCompile(`(?m)^## \d{4}-\d{2}$`)
	hits := rx.FindAllString(arch, -1)
	if len(hits) < 2 {
		t.Fatalf("expected at least 2 monthly sections, got %d in:\n%s", len(hits), arch)
	}
	// All three tasks must be present.
	for _, want := range []string{"jan-task", "march-task", "now-task"} {
		if !strings.Contains(arch, want) {
			t.Fatalf("missing %q in archive:\n%s", want, arch)
		}
	}
}

// TestArchiveStrategyMonthlyOldestFirst: when two months bucket
// out, the OLDER month appears earlier in the file than the newer.
func TestArchiveStrategyMonthlyOldestFirst(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"older", "newer"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	path := filepath.Join(dir, ".tsk.md")
	// #1 → 120 days ago, #2 → 60 days ago. The 60-day one is newer.
	stampCompleted(t, path, 1, time.Now().AddDate(0, 0, -120))
	stampCompleted(t, path, 2, time.Now().AddDate(0, 0, -60))
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "monthly"); err != nil {
		t.Fatalf("archive monthly: %v", err)
	}
	arch := readArchive(t, dir)
	iOlder := strings.Index(arch, "older")
	iNewer := strings.Index(arch, "newer")
	if iOlder < 0 || iNewer < 0 {
		t.Fatalf("missing tasks in archive:\n%s", arch)
	}
	if !(iOlder < iNewer) {
		t.Fatalf("expected 'older' task before 'newer' (chronological asc), got positions %d vs %d:\n%s", iOlder, iNewer, arch)
	}
}

// TestArchiveStrategyMonthlyUndatedBucket: done tasks without a
// Completed timestamp land in "## undated" at the tail of the
// monthly layout — same policy as weekly.
func TestArchiveStrategyMonthlyUndatedBucket(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ghost"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Strip the completed: stamp to simulate hand-edited done state.
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
		valStart := idx + len("completed:")
		end := strings.Index(s[valStart:], " ")
		if end < 0 {
			s = s[:idx]
			break
		}
		removeStart := idx
		if idx > 0 && s[idx-1] == ' ' {
			removeStart = idx - 1
		}
		s = s[:removeStart] + s[valStart+end:]
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "monthly"); err != nil {
		t.Fatalf("archive monthly: %v", err)
	}
	arch := readArchive(t, dir)
	if !strings.Contains(arch, "## undated") {
		t.Fatalf("expected '## undated' section, got:\n%s", arch)
	}
	if !strings.Contains(arch, "- [x] ghost") {
		t.Fatalf("ghost task missing from archive, got:\n%s", arch)
	}
}

// TestArchiveStrategyMonthlyDoesNotRebucketExisting: existing
// content (whether previously flat or weekly) must be preserved
// verbatim when a new --strategy monthly call lands.
func TestArchiveStrategyMonthlyDoesNotRebucketExisting(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "first"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	// First archive: flat — leaves no section headers.
	if _, _, err := runCmd(t, dir, "archive", "--all"); err != nil {
		t.Fatalf("archive 1: %v", err)
	}
	archBefore := readArchive(t, dir)
	// Second batch: add + done + archive monthly.
	if _, _, err := runCmd(t, dir, "add", "second"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	// "second" got id 1 in the active store (id space densified
	// after archive). Done it and archive monthly.
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "monthly"); err != nil {
		t.Fatalf("archive monthly: %v", err)
	}
	archAfter := readArchive(t, dir)
	if !strings.Contains(archAfter, "first") {
		t.Fatalf("first must survive in archive:\n%s", archAfter)
	}
	if !strings.Contains(archAfter, "second") {
		t.Fatalf("second must appear in archive:\n%s", archAfter)
	}
	// The first-batch row's POSITION must be unchanged (no
	// re-bucketing): the substring "first" must appear at the
	// SAME byte offset in archBefore and archAfter.
	idxBefore := strings.Index(archBefore, "first")
	idxAfter := strings.Index(archAfter, "first")
	if idxBefore != idxAfter {
		t.Fatalf("first-batch task moved (re-bucketed). before=%d after=%d\nBEFORE:\n%s\nAFTER:\n%s", idxBefore, idxAfter, archBefore, archAfter)
	}
	// And the new batch lands under a YYYY-MM header.
	rx := regexp.MustCompile(`(?m)^## \d{4}-\d{2}$`)
	if !rx.MatchString(archAfter) {
		t.Fatalf("expected a YYYY-MM section header after monthly archive:\n%s", archAfter)
	}
}

// TestArchiveStrategyMonthlyDifferentFromWeekly: same input
// task to two fresh stores: weekly produces "## YYYY-W##" and
// monthly produces "## YYYY-MM". Same data, different layout.
func TestArchiveStrategyMonthlyDifferentFromWeekly(t *testing.T) {
	mkDir := func() string {
		d := t.TempDir()
		if _, _, err := runCmd(t, d, "add", "thing"); err != nil {
			t.Fatalf("add: %v", err)
		}
		if _, _, err := runCmd(t, d, "done", "1"); err != nil {
			t.Fatalf("done: %v", err)
		}
		stampCompleted(t, filepath.Join(d, ".tsk.md"), 1, time.Now().AddDate(0, 0, -60))
		return d
	}
	weeklyDir := mkDir()
	monthlyDir := mkDir()
	if _, _, err := runCmd(t, weeklyDir, "archive", "--all", "--strategy", "weekly"); err != nil {
		t.Fatalf("weekly archive: %v", err)
	}
	if _, _, err := runCmd(t, monthlyDir, "archive", "--all", "--strategy", "monthly"); err != nil {
		t.Fatalf("monthly archive: %v", err)
	}
	weekly := readArchive(t, weeklyDir)
	monthly := readArchive(t, monthlyDir)
	if !strings.Contains(weekly, "-W") {
		t.Fatalf("weekly archive should contain a -W marker:\n%s", weekly)
	}
	monthRx := regexp.MustCompile(`(?m)^## \d{4}-\d{2}$`)
	if !monthRx.MatchString(monthly) {
		t.Fatalf("monthly archive should contain YYYY-MM header:\n%s", monthly)
	}
	// Monthly must NOT contain -W (sanity: the strategies are distinct).
	if strings.Contains(monthly, "-W") {
		t.Fatalf("monthly archive should not contain weekly markers:\n%s", monthly)
	}
}

// TestArchiveStrategyBogusValueErrorMentionsMonthly: the error
// message must list "monthly" so users discover the option from
// a typo.
func TestArchiveStrategyBogusValueErrorMentionsMonthly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--strategy", "nonsense")
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
	if !strings.Contains(err.Error(), "monthly") {
		t.Fatalf("error should mention 'monthly' so users discover it, got: %v", err)
	}
}
