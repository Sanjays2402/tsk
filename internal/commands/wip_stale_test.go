package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// TestWipStaleFiltersFreshTasks: a recently-started task (started
// well within the threshold) is filtered out by --stale. The
// alert mode only surfaces tasks running LONGER than the threshold.
func TestWipStaleFiltersFreshTasks(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "fresh"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Threshold of 24h vs a just-started task: should filter.
	stdout, _, err := runCmd(t, dir, "wip", "--stale", "24h")
	if err != nil {
		t.Fatalf("wip --stale: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks running longer than") {
		t.Fatalf("expected stale-empty message, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "fresh") {
		t.Fatalf("fresh task should NOT appear in --stale 24h list, got:\n%s", stdout)
	}
}

// TestWipStaleSurfacesOldTasks: an old task (started before the
// threshold) is surfaced. Hand-edits started: to simulate
// "started 25 hours ago".
func TestWipStaleSurfacesOldTasks(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "old work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "new work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Push #1's start back 25 hours, save.
	s, err := store.Load(filepath.Join(dir, ".tsk.md"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	old := time.Now().Add(-25 * time.Hour)
	t1 := s.ByID(1)
	if t1 == nil {
		t.Fatalf("expected task 1")
	}
	t1.Started = &old
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "wip", "--stale", "24h")
	if err != nil {
		t.Fatalf("wip --stale: %v", err)
	}
	if !strings.Contains(stdout, "old work") {
		t.Errorf("expected 'old work' in --stale 24h list, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "new work") {
		t.Errorf("'new work' (fresh) should NOT appear, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "in-progress (filter: stale>") {
		t.Errorf("expected stale-header line, got:\n%s", stdout)
	}
}

// TestWipStaleJSONOutput: --stale composes with --json so scripted
// alerts can flag stale WIP without parsing humanized strings.
func TestWipStaleJSONOutput(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "stale one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "fresh one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	old := time.Now().Add(-3 * time.Hour)
	t1 := s.ByID(1)
	if t1 == nil {
		t.Fatalf("expected task 1")
	}
	t1.Started = &old
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "wip", "--stale", "1h", "--json")
	if err != nil {
		t.Fatalf("wip --stale --json: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse JSON: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 stale task in JSON, got %d:\n%s", len(tasks), stdout)
	}
	if tasks[0].Title != "stale one" {
		t.Errorf("expected stale task to be 'stale one', got %q", tasks[0].Title)
	}
}

// TestWipStaleJSONEmptyArray: stale filter with no matches returns
// `[]` (not null) for jq pipeline safety.
func TestWipStaleJSONEmptyArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--stale", "24h", "--json")
	if err != nil {
		t.Fatalf("wip --stale --json: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected '[]' for empty stale set, got %q", stdout)
	}
}

// TestWipStaleRejectsInvalidDuration: a non-parseable --stale
// value surfaces as a usage error.
func TestWipStaleRejectsInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "wip", "--stale", "garbage")
	if err == nil {
		t.Fatal("expected error for invalid --stale duration")
	}
	if !strings.Contains(err.Error(), "invalid --stale") {
		t.Fatalf("expected invalid-stale error, got: %v", err)
	}
}

// TestWipStaleRejectsZero: zero is not a valid threshold (every
// elapsed value would be > 0, defeating the filter's purpose).
func TestWipStaleRejectsZero(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "wip", "--stale", "0s")
	if err == nil {
		t.Fatal("expected error for --stale 0s")
	}
	if !strings.Contains(err.Error(), "must be a positive duration") {
		t.Fatalf("expected positive-duration error, got: %v", err)
	}
}

// TestWipStaleRejectsNegative: explicit negative durations are
// rejected the same way zero is.
func TestWipStaleRejectsNegative(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "wip", "--stale", "-1h")
	if err == nil {
		t.Fatal("expected error for negative --stale")
	}
}

// TestWipStaleEmptyValueNoFilter: an empty --stale string (e.g.
// from an unset shell variable) means "no filter" — every
// in-progress task surfaces. Defensive against shell vars like
// `tsk wip --stale "$X"` where $X is unset.
func TestWipStaleEmptyValueNoFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "any"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--stale", "")
	if err != nil {
		t.Fatalf("wip --stale '': %v", err)
	}
	if !strings.Contains(stdout, "any") {
		t.Fatalf("expected task in output with empty stale value, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "in-progress (filter:") {
		t.Errorf("did not expect filter header when filter is inactive, got:\n%s", stdout)
	}
}

// TestWipStaleSortPreserved: stale-filtered output retains the
// most-recent-start-first ordering.
func TestWipStaleSortPreserved(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"older-stale", "newer-stale"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	older := time.Now().Add(-48 * time.Hour)
	newer := time.Now().Add(-25 * time.Hour)
	t1 := s.ByID(1)
	t2 := s.ByID(2)
	t1.Started = &older
	t2.Started = &newer
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "wip", "--stale", "24h")
	if err != nil {
		t.Fatalf("wip --stale: %v", err)
	}
	if !strings.Contains(stdout, "older-stale") || !strings.Contains(stdout, "newer-stale") {
		t.Fatalf("both stale tasks should appear, got:\n%s", stdout)
	}
	// newer-stale's start is more recent → it appears FIRST.
	newerIdx := strings.Index(stdout, "newer-stale")
	olderIdx := strings.Index(stdout, "older-stale")
	if newerIdx > olderIdx {
		t.Errorf("expected newer-stale before older-stale (most-recent-start first), got:\n%s", stdout)
	}
}

// TestWipStaleAtBoundaryExcluded: a task with elapsed near 0 (just
// started) is excluded from --stale 1h. The filter requires
// strictly greater than threshold.
func TestWipStaleAtBoundaryExcluded(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "boundary"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t1 := s.ByID(1)
	// Set Started to "now" — elapsed will be ~0.
	now := time.Now()
	t1.Started = &now
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--stale", "1h")
	if err != nil {
		t.Fatalf("wip --stale: %v", err)
	}
	if strings.Contains(stdout, "boundary") {
		t.Fatalf("just-started task should not appear in --stale 1h, got:\n%s", stdout)
	}
}

// TestWipStaleHelpMentionsFlag: --help text mentions the new flag
// (discoverability sanity check).
func TestWipStaleHelpMentionsFlag(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "wip", "--help")
	if err != nil {
		t.Fatalf("wip --help: %v", err)
	}
	if !strings.Contains(stdout, "--stale") {
		t.Errorf("expected --stale in help, got:\n%s", stdout)
	}
}
