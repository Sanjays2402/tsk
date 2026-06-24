package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestWipTagFilter: --tag narrows the wip list to in-progress tasks
// carrying the named tag. Case-insensitive (mirrors `tsk ls --tag`
// and `tsk depend --pending --tag`).
func TestWipTagFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work task", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home task", "--tag", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "no tag task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2", "3"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--tag", "work")
	if err != nil {
		t.Fatalf("wip --tag work: %v", err)
	}
	if !strings.Contains(stdout, "work task") {
		t.Errorf("expected 'work task' in --tag work output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "home task") {
		t.Errorf("'home task' should be filtered, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "no tag task") {
		t.Errorf("untagged task should be filtered, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "filter:") || !strings.Contains(stdout, "tag=work") {
		t.Errorf("expected filter header to mention tag=work, got:\n%s", stdout)
	}
}

// TestWipTagCaseInsensitive: --tag is case-insensitive (delegated
// to Task.HasTag, which uses lowercase comparison).
func TestWipTagCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work task", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Filter with uppercase: same task should surface.
	stdout, _, err := runCmd(t, dir, "wip", "--tag", "WORK")
	if err != nil {
		t.Fatalf("wip --tag WORK: %v", err)
	}
	if !strings.Contains(stdout, "work task") {
		t.Errorf("--tag should be case-insensitive, got:\n%s", stdout)
	}
}

// TestWipTagJSON: --tag composes with --json. The output is a
// filtered []Task array (no envelope shape change — same surface
// the existing --json path returns).
func TestWipTagJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work task", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home task", "--tag", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--tag", "work", "--json")
	if err != nil {
		t.Fatalf("wip --tag --json: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("unmarshal: %v\ngot:\n%s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "work task" {
		t.Errorf("expected 'work task', got %q", tasks[0].Title)
	}
}

// TestWipTagEmptyMatch: --tag with a tag no in-progress task
// carries yields a clean empty-state message naming the tag.
func TestWipTagEmptyMatch(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work task", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--tag", "missing")
	if err != nil {
		t.Fatalf("wip --tag missing: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks with tag missing") {
		t.Fatalf("expected tag-specific empty message, got:\n%s", stdout)
	}
}

// TestWipTagComposesWithStale: --tag and --stale combine as AND
// (only stale tasks tagged correctly surface).
func TestWipTagComposesWithStale(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "old work", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "old home", "--tag", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "fresh work", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2", "3"); err != nil {
		t.Fatalf("start: %v", err)
	}
	old := time.Now().Add(-48 * time.Hour)
	mutateWipStartedAt(t, dir, 1, old)
	mutateWipStartedAt(t, dir, 2, old)

	stdout, _, err := runCmd(t, dir, "wip", "--stale", "24h", "--tag", "work")
	if err != nil {
		t.Fatalf("wip --stale --tag: %v", err)
	}
	if !strings.Contains(stdout, "old work") {
		t.Errorf("expected 'old work' (matches both filters), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "old home") {
		t.Errorf("'old home' should be filtered by --tag, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "fresh work") {
		t.Errorf("'fresh work' should be filtered by --stale, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "stale>") || !strings.Contains(stdout, "tag=work") {
		t.Errorf("expected combined filter summary, got:\n%s", stdout)
	}
}

// TestWipTagEmptyValue: empty --tag (defensive against unset shell
// vars) means no tag filter; every in-progress task is listed.
func TestWipTagEmptyValue(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work task", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home task", "--tag", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--tag", "")
	if err != nil {
		t.Fatalf("wip --tag '': %v", err)
	}
	if !strings.Contains(stdout, "work task") || !strings.Contains(stdout, "home task") {
		t.Errorf("expected both tasks with empty --tag, got:\n%s", stdout)
	}
}

// TestWipTagHelpMentionsFlag: --help text mentions the new
// --tag flag for discoverability.
func TestWipTagHelpMentionsFlag(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "wip", "--help")
	if err != nil {
		t.Fatalf("wip --help: %v", err)
	}
	if !strings.Contains(stdout, "--tag") {
		t.Errorf("--help should mention --tag, got:\n%s", stdout)
	}
}
