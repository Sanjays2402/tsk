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

// TestArchiveStrategyQuarterlyGroupsByQuarter: tasks completed in
// distinct fiscal quarters land in distinct "## YYYY-Q#" sections,
// oldest first.
func TestArchiveStrategyQuarterlyGroupsByQuarter(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"q1-task", "q2-task", "q3-task"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Backdate the three tasks to three different fiscal quarters
	// in the same year. We anchor on 2025-02-15 (Q1), 2025-05-15
	// (Q2), 2025-08-15 (Q3) — picked away from quarter boundaries
	// so DST/UTC midnight jitter can't slide a row into a wrong
	// bucket.
	path := filepath.Join(dir, ".tsk.md")
	stampCompleted(t, path, 1, time.Date(2025, 2, 15, 12, 0, 0, 0, time.UTC))
	stampCompleted(t, path, 2, time.Date(2025, 5, 15, 12, 0, 0, 0, time.UTC))
	stampCompleted(t, path, 3, time.Date(2025, 8, 15, 12, 0, 0, 0, time.UTC))

	stdout, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "quarterly")
	if err != nil {
		t.Fatalf("archive quarterly: %v", err)
	}
	if !strings.Contains(stdout, "strategy=quarterly") {
		t.Fatalf("expected 'strategy=quarterly' in output, got:\n%s", stdout)
	}
	arch := readArchive(t, dir)
	rx := regexp.MustCompile(`(?m)^## \d{4}-Q[1-4]$`)
	hits := rx.FindAllString(arch, -1)
	if len(hits) != 3 {
		t.Fatalf("expected exactly 3 quarterly sections, got %d in:\n%s", len(hits), arch)
	}
	for _, want := range []string{"## 2025-Q1", "## 2025-Q2", "## 2025-Q3"} {
		if !strings.Contains(arch, want) {
			t.Fatalf("missing %q in archive:\n%s", want, arch)
		}
	}
	// Ordering: Q1 < Q2 < Q3 by byte offset.
	iQ1 := strings.Index(arch, "## 2025-Q1")
	iQ2 := strings.Index(arch, "## 2025-Q2")
	iQ3 := strings.Index(arch, "## 2025-Q3")
	if !(iQ1 < iQ2 && iQ2 < iQ3) {
		t.Fatalf("quarters out of order: Q1@%d Q2@%d Q3@%d in:\n%s", iQ1, iQ2, iQ3, arch)
	}
}

// TestArchiveStrategyQuarterlyUndatedBucket: tasks without a Completed
// timestamp fall into "## undated" at the tail (same policy as the
// other bucketed strategies).
func TestArchiveStrategyQuarterlyUndatedBucket(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ghost"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stripCompletedTimestamps(t, filepath.Join(dir, ".tsk.md"))
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "quarterly"); err != nil {
		t.Fatalf("archive quarterly: %v", err)
	}
	arch := readArchive(t, dir)
	if !strings.Contains(arch, "## undated") {
		t.Fatalf("expected '## undated' section, got:\n%s", arch)
	}
	if !strings.Contains(arch, "- [x] ghost") {
		t.Fatalf("ghost task missing from archive, got:\n%s", arch)
	}
}

// TestArchiveStrategyQuarterlyYearBoundary: a Q4 2025 task and a
// Q1 2026 task must land in two distinct sections with Q4 2025
// sorted before Q1 2026 (sortKey ordering across year boundary).
func TestArchiveStrategyQuarterlyYearBoundary(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"dec-task", "jan-task"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	path := filepath.Join(dir, ".tsk.md")
	stampCompleted(t, path, 1, time.Date(2025, 12, 1, 12, 0, 0, 0, time.UTC))
	stampCompleted(t, path, 2, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "quarterly"); err != nil {
		t.Fatalf("archive quarterly: %v", err)
	}
	arch := readArchive(t, dir)
	iQ4 := strings.Index(arch, "## 2025-Q4")
	iQ1 := strings.Index(arch, "## 2026-Q1")
	if iQ4 < 0 || iQ1 < 0 {
		t.Fatalf("missing one of the expected sections:\n%s", arch)
	}
	if !(iQ4 < iQ1) {
		t.Fatalf("expected 2025-Q4 before 2026-Q1, got Q4@%d Q1@%d:\n%s", iQ4, iQ1, arch)
	}
}

// TestBucketByQuarterBoundaries: unit-level coverage for the
// quarter-of-year math. March is Q1, April is Q2, June is Q2,
// July is Q3, September is Q3, October is Q4, December is Q4.
func TestBucketByQuarterBoundaries(t *testing.T) {
	mk := func(month time.Month, day int) model.Task {
		ts := time.Date(2026, month, day, 12, 0, 0, 0, time.UTC)
		return model.Task{Completed: &ts, Done: true}
	}
	cases := []struct {
		month   time.Month
		day     int
		wantKey string
		wantSK  int
	}{
		{time.January, 1, "2026-Q1", 20261},
		{time.March, 31, "2026-Q1", 20261},
		{time.April, 1, "2026-Q2", 20262},
		{time.June, 30, "2026-Q2", 20262},
		{time.July, 1, "2026-Q3", 20263},
		{time.September, 30, "2026-Q3", 20263},
		{time.October, 1, "2026-Q4", 20264},
		{time.December, 31, "2026-Q4", 20264},
	}
	for _, c := range cases {
		key, sk := bucketByQuarter(mk(c.month, c.day))
		if key != c.wantKey {
			t.Errorf("%s %d: key got %q want %q", c.month, c.day, key, c.wantKey)
		}
		if sk != c.wantSK {
			t.Errorf("%s %d: sortKey got %d want %d", c.month, c.day, sk, c.wantSK)
		}
	}
	// Undated bucket case.
	gotKey, gotSK := bucketByQuarter(model.Task{Done: true})
	if gotKey != "undated" || gotSK != 0 {
		t.Errorf("undated: got (%q, %d), want (\"undated\", 0)", gotKey, gotSK)
	}
}

// TestArchiveStrategyQuarterlyDoesNotRebucketExisting: existing flat
// or weekly content stays in place; only the new batch lands under
// quarterly sections.
func TestArchiveStrategyQuarterlyDoesNotRebucketExisting(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "first"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all"); err != nil {
		t.Fatalf("flat archive: %v", err)
	}
	archBefore := readArchive(t, dir)
	if _, _, err := runCmd(t, dir, "add", "second"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "quarterly"); err != nil {
		t.Fatalf("archive quarterly: %v", err)
	}
	archAfter := readArchive(t, dir)
	idxBefore := strings.Index(archBefore, "first")
	idxAfter := strings.Index(archAfter, "first")
	if idxBefore != idxAfter {
		t.Fatalf("first-batch task moved (re-bucketed). before=%d after=%d\nBEFORE:\n%s\nAFTER:\n%s", idxBefore, idxAfter, archBefore, archAfter)
	}
	rx := regexp.MustCompile(`(?m)^## \d{4}-Q[1-4]$`)
	if !rx.MatchString(archAfter) {
		t.Fatalf("expected a YYYY-Q# section after quarterly archive:\n%s", archAfter)
	}
}

// stripCompletedTimestamps mutates the .tsk.md file in place to
// remove every `completed:RFC3339` token from meta blocks. Used
// to simulate a hand-edited done task without a completion stamp,
// matching the "undated" bucket trigger across the bucketed
// strategies. Helper instead of copy-pasting the byte-walk into
// each test.
func stripCompletedTimestamps(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
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
		t.Fatalf("write %s: %v", path, err)
	}
}
