package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestArchiveStrategyDailyGroupsByDate: tasks completed on
// distinct calendar days land in distinct "## YYYY-MM-DD" sections,
// oldest first.
func TestArchiveStrategyDailyGroupsByDate(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"day-a", "day-b", "day-c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Backdate each task to a different calendar day. Anchor at noon
	// in UTC to dodge midnight-boundary flakes in the local zone.
	path := filepath.Join(dir, ".tsk.md")
	base := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	stampCompleted(t, path, 1, base)
	stampCompleted(t, path, 2, base.AddDate(0, 0, 1))
	stampCompleted(t, path, 3, base.AddDate(0, 0, 2))

	stdout, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "daily")
	if err != nil {
		t.Fatalf("archive daily: %v", err)
	}
	if !strings.Contains(stdout, "strategy=daily") {
		t.Fatalf("expected 'strategy=daily' in output, got:\n%s", stdout)
	}
	arch := readArchive(t, dir)
	rx := regexp.MustCompile(`(?m)^## \d{4}-\d{2}-\d{2}$`)
	hits := rx.FindAllString(arch, -1)
	if len(hits) != 3 {
		t.Fatalf("expected 3 daily sections (3 distinct dates), got %d in:\n%s", len(hits), arch)
	}
	for _, want := range []string{"day-a", "day-b", "day-c"} {
		if !strings.Contains(arch, want) {
			t.Fatalf("missing %q in archive:\n%s", want, arch)
		}
	}
}

// TestArchiveStrategyDailyOldestFirst: when two days bucket out,
// the OLDER day appears earlier in the file than the newer.
func TestArchiveStrategyDailyOldestFirst(t *testing.T) {
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
	stampCompleted(t, path, 1, time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC))
	stampCompleted(t, path, 2, time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC))
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "daily"); err != nil {
		t.Fatalf("archive daily: %v", err)
	}
	arch := readArchive(t, dir)
	iOlder := strings.Index(arch, "older")
	iNewer := strings.Index(arch, "newer")
	if iOlder < 0 || iNewer < 0 {
		t.Fatalf("missing tasks:\n%s", arch)
	}
	if !(iOlder < iNewer) {
		t.Fatalf("expected 'older' before 'newer' (chronological asc), got %d vs %d:\n%s", iOlder, iNewer, arch)
	}
}

// TestArchiveStrategyDailyUndatedBucket: hand-edited done tasks
// without a Completed stamp land in "## undated" at the tail —
// same policy as weekly/monthly.
func TestArchiveStrategyDailyUndatedBucket(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ghost"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
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
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "daily"); err != nil {
		t.Fatalf("archive daily: %v", err)
	}
	arch := readArchive(t, dir)
	if !strings.Contains(arch, "## undated") {
		t.Fatalf("expected '## undated' section, got:\n%s", arch)
	}
	if !strings.Contains(arch, "- [x] ghost") {
		t.Fatalf("ghost missing from archive:\n%s", arch)
	}
}

// TestArchiveStrategyDailyDoesNotRebucketExisting: existing
// (flat or otherwise) content must be preserved verbatim when a
// new --strategy daily call lands.
func TestArchiveStrategyDailyDoesNotRebucketExisting(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "first-batch"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all"); err != nil {
		t.Fatalf("archive flat: %v", err)
	}
	archBefore := readArchive(t, dir)
	if _, _, err := runCmd(t, dir, "add", "second-batch"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "daily"); err != nil {
		t.Fatalf("archive daily: %v", err)
	}
	archAfter := readArchive(t, dir)
	if !strings.Contains(archAfter, "first-batch") {
		t.Fatalf("first-batch must survive:\n%s", archAfter)
	}
	if !strings.Contains(archAfter, "second-batch") {
		t.Fatalf("second-batch must appear:\n%s", archAfter)
	}
	idxBefore := strings.Index(archBefore, "first-batch")
	idxAfter := strings.Index(archAfter, "first-batch")
	if idxBefore != idxAfter {
		t.Fatalf("first-batch moved (re-bucketed). before=%d after=%d", idxBefore, idxAfter)
	}
	rx := regexp.MustCompile(`(?m)^## \d{4}-\d{2}-\d{2}$`)
	if !rx.MatchString(archAfter) {
		t.Fatalf("expected a YYYY-MM-DD header in daily archive:\n%s", archAfter)
	}
}

// TestArchiveStrategyDailyVsMonthlyVsWeekly: same input, three
// fresh stores, three different layouts. Daily produces a
// YYYY-MM-DD header, weekly produces -W, monthly produces -## with
// no day component.
func TestArchiveStrategyDailyDifferentFromOthers(t *testing.T) {
	mkDir := func() string {
		d := t.TempDir()
		if _, _, err := runCmd(t, d, "add", "thing"); err != nil {
			t.Fatalf("add: %v", err)
		}
		if _, _, err := runCmd(t, d, "done", "1"); err != nil {
			t.Fatalf("done: %v", err)
		}
		stampCompleted(t, filepath.Join(d, ".tsk.md"), 1, time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC))
		return d
	}
	dailyDir := mkDir()
	weeklyDir := mkDir()
	monthlyDir := mkDir()
	if _, _, err := runCmd(t, dailyDir, "archive", "--all", "--strategy", "daily"); err != nil {
		t.Fatalf("daily: %v", err)
	}
	if _, _, err := runCmd(t, weeklyDir, "archive", "--all", "--strategy", "weekly"); err != nil {
		t.Fatalf("weekly: %v", err)
	}
	if _, _, err := runCmd(t, monthlyDir, "archive", "--all", "--strategy", "monthly"); err != nil {
		t.Fatalf("monthly: %v", err)
	}
	daily := readArchive(t, dailyDir)
	weekly := readArchive(t, weeklyDir)
	monthly := readArchive(t, monthlyDir)

	dayRx := regexp.MustCompile(`(?m)^## \d{4}-\d{2}-\d{2}$`)
	if !dayRx.MatchString(daily) {
		t.Fatalf("daily should contain YYYY-MM-DD header:\n%s", daily)
	}
	if !strings.Contains(weekly, "-W") {
		t.Fatalf("weekly should contain -W marker:\n%s", weekly)
	}
	// Monthly: has YYYY-MM but no day suffix. Strict check: the
	// YYYY-MM-DD regex must NOT match the monthly file.
	if dayRx.MatchString(monthly) {
		t.Fatalf("monthly should not contain YYYY-MM-DD headers:\n%s", monthly)
	}
}

// TestArchiveStrategyBogusValueErrorMentionsDaily: error message
// must list "daily" so users discover the option from a typo.
func TestArchiveStrategyBogusValueErrorMentionsDaily(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--strategy", "hourly")
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
	if !strings.Contains(err.Error(), "daily") {
		t.Fatalf("error should mention 'daily', got: %v", err)
	}
}

// TestBucketByDayKey: direct unit-test on the bucketFn — verifies
// the YYYY-MM-DD format, and that the sortKey orders dates
// correctly across month + year boundaries.
func TestBucketByDayKey(t *testing.T) {
	mk := func(y int, m time.Month, d int) (string, int) {
		ts := time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
		task := model.Task{Completed: &ts}
		return bucketByDay(task)
	}
	cases := []struct {
		y    int
		m    time.Month
		d    int
		key  string
		sort int
	}{
		{2026, 3, 15, "2026-03-15", 20260315},
		{2026, 1, 1, "2026-01-01", 20260101},
		{2025, 12, 31, "2025-12-31", 20251231},
		{2026, 12, 31, "2026-12-31", 20261231},
	}
	for _, c := range cases {
		k, s := mk(c.y, c.m, c.d)
		if k != c.key || s != c.sort {
			t.Errorf("bucketByDay(%d-%02d-%02d) = (%q, %d) want (%q, %d)",
				c.y, c.m, c.d, k, s, c.key, c.sort)
		}
	}
	// Year-boundary safety: Dec 31 2025 < Jan 1 2026 by sortKey.
	_, prev := mk(2025, 12, 31)
	_, next := mk(2026, 1, 1)
	if !(prev < next) {
		t.Fatalf("sortKey must be year-boundary safe: %d should be < %d", prev, next)
	}
	// Nil Completed → "undated" bucket with sortKey 0.
	k, s := bucketByDay(model.Task{})
	if k != "undated" || s != 0 {
		t.Fatalf("bucketByDay(nil Completed) = (%q, %d) want (\"undated\", 0)", k, s)
	}
}
