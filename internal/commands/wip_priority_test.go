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

// TestWipPriorityFilter: --priority narrows the wip list to tasks
// at exactly the named priority. Tasks at OTHER priorities are
// filtered out. Mirrors `tsk depend --pending --priority` semantics
// (exact-match, not minimum-threshold).
func TestWipPriorityFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "low task", "--priority", "low"); err != nil {
		t.Fatalf("add low: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "high task", "--priority", "high"); err != nil {
		t.Fatalf("add high: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "urgent task", "--priority", "urgent"); err != nil {
		t.Fatalf("add urgent: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2", "3"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--priority", "urgent")
	if err != nil {
		t.Fatalf("wip --priority urgent: %v", err)
	}
	if !strings.Contains(stdout, "urgent task") {
		t.Errorf("expected urgent task in --priority urgent output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "low task") || strings.Contains(stdout, "high task") {
		t.Errorf("non-urgent tasks should be filtered, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "filter:") || !strings.Contains(stdout, "priority=urgent") {
		t.Errorf("expected filter header to mention priority=urgent, got:\n%s", stdout)
	}
}

// TestWipPriorityFilterJSON: --priority composes with --json.
// The JSON array shape stays a plain []Task — the filter is applied
// before encoding.
func TestWipPriorityFilterJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "low task", "--priority", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "high task", "--priority", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--priority", "high", "--json")
	if err != nil {
		t.Fatalf("wip --priority --json: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("unmarshal: %v\ngot:\n%s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "high task" {
		t.Errorf("expected 'high task', got %q", tasks[0].Title)
	}
	if tasks[0].Priority != model.PriorityHigh {
		t.Errorf("expected High priority, got %v", tasks[0].Priority)
	}
}

// TestWipPriorityShortForm: --priority accepts the short form
// (model.ParsePriority handles "u" → urgent, "h" → high, etc.) so
// `tsk wip --priority u` works the same as `tsk wip --priority urgent`.
func TestWipPriorityShortForm(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "urgent task", "--priority", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--priority", "u")
	if err != nil {
		t.Fatalf("wip --priority u: %v", err)
	}
	if !strings.Contains(stdout, "urgent task") {
		t.Errorf("expected urgent task with short-form, got:\n%s", stdout)
	}
}

// TestWipPriorityInvalidRejected: a non-priority value produces a
// usage error with a useful message.
func TestWipPriorityInvalidRejected(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "wip", "--priority", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid --priority")
	}
	if !strings.Contains(err.Error(), "invalid --priority") {
		t.Errorf("expected useful error, got: %v", err)
	}
}

// TestWipPriorityEmptyValue: an empty --priority (defensive against
// unset shell vars) means no filter; every in-progress task is
// listed.
func TestWipPriorityEmptyValue(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "low task", "--priority", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "high task", "--priority", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--priority", "")
	if err != nil {
		t.Fatalf("wip --priority '': %v", err)
	}
	if !strings.Contains(stdout, "low task") || !strings.Contains(stdout, "high task") {
		t.Errorf("expected both tasks with empty --priority, got:\n%s", stdout)
	}
}

// TestWipPriorityEmptyMatch: --priority urgent with no urgent
// in-progress tasks yields a clean empty-state message naming the
// priority.
func TestWipPriorityEmptyMatch(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "low task", "--priority", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--priority", "urgent")
	if err != nil {
		t.Fatalf("wip --priority: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks at priority urgent") {
		t.Fatalf("expected priority-specific empty message, got:\n%s", stdout)
	}
}

// TestWipPriorityComposesWithStale: --priority and --stale combine
// as AND (only stale tasks at the named priority surface).
func TestWipPriorityComposesWithStale(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "old urgent", "--priority", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "old low", "--priority", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "fresh urgent", "--priority", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2", "3"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Push #1 and #2 back 48h (old); leave #3 fresh.
	old := time.Now().Add(-48 * time.Hour)
	mutateWipStartedAt(t, dir, 1, old)
	mutateWipStartedAt(t, dir, 2, old)

	stdout, _, err := runCmd(t, dir, "wip", "--stale", "24h", "--priority", "urgent")
	if err != nil {
		t.Fatalf("wip --stale --priority: %v", err)
	}
	if !strings.Contains(stdout, "old urgent") {
		t.Errorf("expected 'old urgent' (matches both filters), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "old low") {
		t.Errorf("'old low' should be filtered by --priority, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "fresh urgent") {
		t.Errorf("'fresh urgent' should be filtered by --stale, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "filter:") || !strings.Contains(stdout, "stale>") || !strings.Contains(stdout, "priority=urgent") {
		t.Errorf("expected combined filter summary in header, got:\n%s", stdout)
	}
}

// TestWipPriorityHelpMentionsFlag: --help text mentions the new
// --priority flag for discoverability.
func TestWipPriorityHelpMentionsFlag(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "wip", "--help")
	if err != nil {
		t.Fatalf("wip --help: %v", err)
	}
	if !strings.Contains(stdout, "--priority") {
		t.Errorf("--help should mention --priority, got:\n%s", stdout)
	}
}

// mutateWipStartedAt hand-edits a task's Started timestamp directly
// in the store. Used by --stale/--priority composition tests that
// need to simulate "started N hours ago" without actually waiting.
// Shared across the wip_*_test.go suite to avoid duplicate boilerplate.
func mutateWipStartedAt(t *testing.T, dir string, id int, when time.Time) {
	t.Helper()
	s, err := store.Load(filepath.Join(dir, ".tsk.md"))
	if err != nil {
		t.Fatalf("mutateWipStartedAt load: %v", err)
	}
	task := s.ByID(id)
	if task == nil {
		t.Fatalf("mutateWipStartedAt: no task with id %d", id)
	}
	w := when
	task.Started = &w
	if err := s.Save(); err != nil {
		t.Fatalf("mutateWipStartedAt save: %v", err)
	}
}
