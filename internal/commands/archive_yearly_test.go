package commands

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestArchiveStrategyYearlyGroupsByYear: tasks completed in three
// distinct calendar years land in three distinct "## YYYY"
// sections, oldest first.
func TestArchiveStrategyYearlyGroupsByYear(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"y2024-task", "y2025-task", "y2026-task"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	path := filepath.Join(dir, ".tsk.md")
	stampCompleted(t, path, 1, time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC))
	stampCompleted(t, path, 2, time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))
	stampCompleted(t, path, 3, time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC))

	stdout, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "yearly")
	if err != nil {
		t.Fatalf("archive yearly: %v", err)
	}
	if !strings.Contains(stdout, "strategy=yearly") {
		t.Fatalf("expected 'strategy=yearly' in output, got:\n%s", stdout)
	}
	arch := readArchive(t, dir)
	// Section headers look like "## YYYY" (4 digits, nothing else
	// after — distinguishing from the other bucket strategies).
	rx := regexp.MustCompile(`(?m)^## \d{4}$`)
	hits := rx.FindAllString(arch, -1)
	if len(hits) != 3 {
		t.Fatalf("expected exactly 3 yearly sections, got %d in:\n%s", len(hits), arch)
	}
	for _, want := range []string{"## 2024", "## 2025", "## 2026"} {
		if !strings.Contains(arch, want) {
			t.Fatalf("missing %q in archive:\n%s", want, arch)
		}
	}
	// Chronological ordering.
	i2024 := strings.Index(arch, "## 2024")
	i2025 := strings.Index(arch, "## 2025")
	i2026 := strings.Index(arch, "## 2026")
	if !(i2024 < i2025 && i2025 < i2026) {
		t.Fatalf("years out of order: 2024@%d 2025@%d 2026@%d in:\n%s", i2024, i2025, i2026, arch)
	}
}

// TestArchiveStrategyYearlyUndatedBucket: tasks without a Completed
// timestamp fall into "## undated" at the tail.
func TestArchiveStrategyYearlyUndatedBucket(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ghost"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stripCompletedTimestamps(t, filepath.Join(dir, ".tsk.md"))
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "yearly"); err != nil {
		t.Fatalf("archive yearly: %v", err)
	}
	arch := readArchive(t, dir)
	if !strings.Contains(arch, "## undated") {
		t.Fatalf("expected '## undated' section, got:\n%s", arch)
	}
	if !strings.Contains(arch, "- [x] ghost") {
		t.Fatalf("ghost task missing from archive, got:\n%s", arch)
	}
}

// TestArchiveStrategyYearlySameYearOneSection: when ALL tasks fall
// in the same calendar year, the layout emits a single "## YYYY"
// section (sanity that the bucketing collapses correctly).
func TestArchiveStrategyYearlySameYearOneSection(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"jan", "jul", "dec"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	path := filepath.Join(dir, ".tsk.md")
	stampCompleted(t, path, 1, time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC))
	stampCompleted(t, path, 2, time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC))
	stampCompleted(t, path, 3, time.Date(2026, 12, 15, 12, 0, 0, 0, time.UTC))
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "yearly"); err != nil {
		t.Fatalf("archive yearly: %v", err)
	}
	arch := readArchive(t, dir)
	rx := regexp.MustCompile(`(?m)^## \d{4}$`)
	hits := rx.FindAllString(arch, -1)
	if len(hits) != 1 {
		t.Fatalf("expected exactly 1 yearly section, got %d in:\n%s", len(hits), arch)
	}
	if !strings.Contains(arch, "## 2026") {
		t.Fatalf("expected '## 2026', got:\n%s", arch)
	}
	// All three tasks present under that one section.
	for _, want := range []string{"jan", "jul", "dec"} {
		if !strings.Contains(arch, want) {
			t.Fatalf("missing %q in archive:\n%s", want, arch)
		}
	}
}

// TestBucketByYearBasics: unit-level checks for the year math.
// Year boundaries (Dec 31 vs Jan 1) and the undated case.
func TestBucketByYearBasics(t *testing.T) {
	mk := func(y int, month time.Month, day int) model.Task {
		ts := time.Date(y, month, day, 12, 0, 0, 0, time.UTC)
		return model.Task{Completed: &ts, Done: true}
	}
	cases := []struct {
		y       int
		month   time.Month
		day     int
		wantKey string
		wantSK  int
	}{
		{2024, time.January, 1, "2024", 2024},
		{2025, time.December, 31, "2025", 2025},
		{2026, time.January, 1, "2026", 2026},
		{2099, time.December, 31, "2099", 2099},
	}
	for _, c := range cases {
		key, sk := bucketByYear(mk(c.y, c.month, c.day))
		if key != c.wantKey {
			t.Errorf("%d %s %d: key got %q want %q", c.y, c.month, c.day, key, c.wantKey)
		}
		if sk != c.wantSK {
			t.Errorf("%d %s %d: sortKey got %d want %d", c.y, c.month, c.day, sk, c.wantSK)
		}
	}
	gotKey, gotSK := bucketByYear(model.Task{Done: true})
	if gotKey != "undated" || gotSK != 0 {
		t.Errorf("undated: got (%q, %d), want (\"undated\", 0)", gotKey, gotSK)
	}
}

// TestArchiveStrategyYearlyDifferentFromQuarterly: same input task
// to two fresh stores: yearly produces "## YYYY" and quarterly
// produces "## YYYY-Q#". Same data, distinct layouts — regression
// against the two strategies ever rendering identically.
func TestArchiveStrategyYearlyDifferentFromQuarterly(t *testing.T) {
	mkDir := func() string {
		d := t.TempDir()
		if _, _, err := runCmd(t, d, "add", "thing"); err != nil {
			t.Fatalf("add: %v", err)
		}
		if _, _, err := runCmd(t, d, "done", "1"); err != nil {
			t.Fatalf("done: %v", err)
		}
		stampCompleted(t, filepath.Join(d, ".tsk.md"), 1, time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC))
		return d
	}
	yearlyDir := mkDir()
	quarterlyDir := mkDir()
	if _, _, err := runCmd(t, yearlyDir, "archive", "--all", "--strategy", "yearly"); err != nil {
		t.Fatalf("yearly archive: %v", err)
	}
	if _, _, err := runCmd(t, quarterlyDir, "archive", "--all", "--strategy", "quarterly"); err != nil {
		t.Fatalf("quarterly archive: %v", err)
	}
	yearly := readArchive(t, yearlyDir)
	quarterly := readArchive(t, quarterlyDir)
	// Yearly: no Q marker.
	if strings.Contains(yearly, "-Q") {
		t.Fatalf("yearly archive should not contain quarterly markers:\n%s", yearly)
	}
	// Quarterly: must have Q marker.
	if !strings.Contains(quarterly, "-Q") {
		t.Fatalf("quarterly archive should contain Q marker:\n%s", quarterly)
	}
}

// TestArchiveStrategyYearlyErrorMessageMentionsYearly: typo of
// "yearli" lands the user at the discovery anchor; "yearly" must
// appear in the message.
func TestArchiveStrategyYearlyErrorMessageMentionsYearly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--strategy", "yearli")
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
	if !strings.Contains(err.Error(), "yearly") {
		t.Fatalf("error should mention 'yearly' so users discover it, got: %v", err)
	}
}
