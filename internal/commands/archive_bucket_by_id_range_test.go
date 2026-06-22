package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveBucketByIDRangeBasic: archiving a batch of tasks with
// --bucket-by id-range:50 yields one section per id window, labeled
// "1-50", "51-100", etc.
func TestArchiveBucketByIDRangeBasic(t *testing.T) {
	dir := t.TempDir()
	// Create 5 tasks, all done. After archive, they get archive
	// ids 1..5 (fresh sequence in the empty archive).
	for i := 0; i < 5; i++ {
		if _, _, err := runCmd(t, dir, "add", "task-"+itoa(i+1)); err != nil {
			t.Fatalf("add: %v", err)
		}
		if _, _, err := runCmd(t, dir, "done", itoa(i+1)); err != nil {
			t.Fatalf("done %d: %v", i+1, err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "id-range:50"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// All 5 ids (1..5) fall in window 1-50.
	if !strings.Contains(archive, "## 1-50") {
		t.Fatalf("expected '## 1-50' section, got:\n%s", archive)
	}
	// No other window should appear.
	if strings.Contains(archive, "## 51-100") {
		t.Fatalf("unexpected '## 51-100' section for small batch:\n%s", archive)
	}
	// Every task should land in the archive.
	for i := 0; i < 5; i++ {
		want := "task-" + itoa(i+1)
		if !strings.Contains(archive, want) {
			t.Errorf("missing task %q in archive:\n%s", want, archive)
		}
	}
}

// TestArchiveBucketByIDRangeMultipleWindows: when the batch spans
// multiple id windows, each gets its own section labeled by the
// window range, sorted ascending by window start.
func TestArchiveBucketByIDRangeMultipleWindows(t *testing.T) {
	dir := t.TempDir()
	// Create 12 tasks to span window 1-5 and 6-10 and 11-15 with N=5.
	for i := 0; i < 12; i++ {
		if _, _, err := runCmd(t, dir, "add", "t"+itoa(i+1)); err != nil {
			t.Fatalf("add: %v", err)
		}
		if _, _, err := runCmd(t, dir, "done", itoa(i+1)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "id-range:5"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// Expect three sections in order: 1-5, 6-10, 11-15.
	wantOrder := []string{"## 1-5", "## 6-10", "## 11-15"}
	prev := -1
	for _, w := range wantOrder {
		idx := strings.Index(archive, w)
		if idx < 0 {
			t.Fatalf("missing %q section, archive:\n%s", w, archive)
		}
		if idx <= prev {
			t.Errorf("%q at offset %d but expected after %d (ascending order):\n%s", w, idx, prev, archive)
		}
		prev = idx
	}
}

// TestArchiveBucketByIDRangeRejectsZeroN: id-range:0 is rejected
// (window size must be positive) at exit 2.
func TestArchiveBucketByIDRangeRejectsZeroN(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "id-range:0")
	if err == nil {
		t.Fatal("expected error for id-range:0")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
	if !strings.Contains(err.Error(), "id-range:N requires N > 0") {
		t.Fatalf("error should explain N > 0 requirement, got: %v", err)
	}
}

// TestArchiveBucketByIDRangeRejectsBadInteger: a non-integer N
// surfaces a usage error.
func TestArchiveBucketByIDRangeRejectsBadInteger(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "id-range:abc")
	if err == nil {
		t.Fatal("expected error for non-integer N")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
}

// TestArchiveBucketByIDRangeEmptyN: id-range: (no number after colon)
// surfaces a clear usage error pointing at the missing parameter.
func TestArchiveBucketByIDRangeEmptyN(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "id-range:")
	if err == nil {
		t.Fatal("expected error for empty N")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
}

// TestArchiveBucketByIDRangeStrategyLabelInSummary: the success
// message uses the bucket-by label "bucket-by=id-range:N".
func TestArchiveBucketByIDRangeStrategyLabelInSummary(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "id-range:100")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "bucket-by=id-range:100") {
		t.Fatalf("expected 'bucket-by=id-range:100' in summary, got:\n%s", stdout)
	}
}
