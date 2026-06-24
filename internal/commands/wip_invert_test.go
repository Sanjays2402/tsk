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

// startInProgressTask is a small helper: adds a task and starts it,
// so the wip tests don't all repeat the boilerplate.
func startInProgressTask(t *testing.T, dir, title string) {
	t.Helper()
	if _, _, err := runCmd(t, dir, "add", title); err != nil {
		t.Fatalf("add %q: %v", title, err)
	}
}

// TestWipInvertPriorityFlipsPredicate: --priority urgent --invert
// surfaces every in-progress task NOT at urgent priority. Without
// --invert, only the urgent task would surface; with --invert, the
// urgent task is the one excluded.
func TestWipInvertPriorityFlipsPredicate(t *testing.T) {
	dir := t.TempDir()
	startInProgressTask(t, dir, "urgent-task")
	startInProgressTask(t, dir, "low-task")
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "2", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "wip", "--priority", "urgent", "--invert", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 NOT-urgent task; got %d\nstdout: %s", len(tasks), stdout)
	}
	if tasks[0].ID != 2 {
		t.Errorf("expected #2 (low-task) to survive --invert; got #%d", tasks[0].ID)
	}
}

// TestWipInvertTagFlipsPredicate: --tag work --invert surfaces
// every in-progress task NOT carrying tag 'work'.
func TestWipInvertTagFlipsPredicate(t *testing.T) {
	dir := t.TempDir()
	startInProgressTask(t, dir, "work-task")
	startInProgressTask(t, dir, "home-task")
	if _, _, err := runCmd(t, dir, "tag", "1", "+work"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "tag", "2", "+home"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "wip", "--tag", "work", "--invert", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 NOT-work-tagged task; got %d", len(tasks))
	}
	if tasks[0].ID != 2 {
		t.Errorf("expected #2 (home-task) to survive --invert; got #%d", tasks[0].ID)
	}
}

// TestWipInvertStrictAndTagFlipsPredicate: --strict-and-tag a,b
// --invert surfaces every WIP NOT carrying both a AND b
// simultaneously (so tasks carrying neither, or one but not both,
// survive).
func TestWipInvertStrictAndTagFlipsPredicate(t *testing.T) {
	dir := t.TempDir()
	startInProgressTask(t, dir, "both-tags")
	startInProgressTask(t, dir, "only-work")
	startInProgressTask(t, dir, "no-tags")
	if _, _, err := runCmd(t, dir, "tag", "1", "+work", "+p0"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "tag", "2", "+work"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2", "3"); err != nil {
		t.Fatalf("start: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "wip", "--strict-and-tag", "work,p0", "--invert", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	kept := make(map[int]bool)
	for _, ts := range tasks {
		kept[ts.ID] = true
	}
	if kept[1] {
		t.Errorf("expected #1 (both tags) to be excluded under --invert; got %+v", tasks)
	}
	if !kept[2] || !kept[3] {
		t.Errorf("expected #2 (only work) and #3 (no tags) to survive --invert; got %+v", tasks)
	}
}

// TestWipInvertComposesWithStaleAsAND: --stale and the inverted
// selectors compose as AND. --stale is NOT itself inverted by
// --invert; the inversion applies only to selector axes.
func TestWipInvertComposesWithStaleAsAND(t *testing.T) {
	dir := t.TempDir()
	startInProgressTask(t, dir, "old-urgent")
	startInProgressTask(t, dir, "old-low")
	startInProgressTask(t, dir, "new-low")
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "2", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "3", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2", "3"); err != nil {
		t.Fatalf("start: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	// Push #1 and #2 to 30 days ago (stale); leave #3 fresh.
	old := time.Now().Add(-30 * 24 * time.Hour)
	t1 := s.ByID(1)
	t1.Started = &old
	t2 := s.ByID(2)
	t2.Started = &old
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// --stale 24h --priority urgent --invert: only stale AND
	// NOT-urgent — exactly #2 (low + stale).
	stdout, _, err := runCmd(t, dir, "wip", "--stale", "24h", "--priority", "urgent", "--invert", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (#2: stale+low); got %d\nbody: %s", len(tasks), stdout)
	}
	if tasks[0].ID != 2 {
		t.Errorf("expected #2 to survive; got #%d", tasks[0].ID)
	}
}

// TestWipInvertWithoutSelectorRejected: --invert with no selector
// is a no-op and a usage error.
func TestWipInvertWithoutSelectorRejected(t *testing.T) {
	dir := t.TempDir()
	startInProgressTask(t, dir, "x")
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, _, err := runCmd(t, dir, "wip", "--invert")
	if err == nil {
		t.Fatal("expected error for --invert without selector")
	}
	if !strings.Contains(err.Error(), "--invert requires at least one selector") {
		t.Errorf("expected 'requires at least one selector' error; got: %v", err)
	}
}

// TestWipInvertStaleAloneNotEnoughForInvert: --invert with ONLY
// --stale active is rejected — stale isn't a selector and there's
// no way to invert it via this flag.
func TestWipInvertStaleAloneNotEnoughForInvert(t *testing.T) {
	dir := t.TempDir()
	startInProgressTask(t, dir, "x")
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, _, err := runCmd(t, dir, "wip", "--stale", "24h", "--invert")
	if err == nil {
		t.Fatal("expected error for --invert with only --stale active")
	}
	if !strings.Contains(err.Error(), "--invert requires at least one selector") {
		t.Errorf("expected 'requires at least one selector' error; got: %v", err)
	}
}

// TestWipInvertEmptyResultMessage: when no tasks survive the
// inverted filter, the empty-result message surfaces the
// inversion explicitly (so the user sees that NOT-X was the
// filter that matched nothing, not X).
func TestWipInvertEmptyResultMessage(t *testing.T) {
	dir := t.TempDir()
	startInProgressTask(t, dir, "x")
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Only one task, urgent. --priority urgent --invert filters it out.
	stdout, _, err := runCmd(t, dir, "wip", "--priority", "urgent", "--invert")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	if !strings.Contains(stdout, "NOT at priority urgent") {
		t.Errorf("expected empty-result message to surface inversion; got: %q", stdout)
	}
}

// TestWipInvertFilterSummaryShowsBang: when results survive, the
// header line surfaces the inversion via the '!' prefix on each
// inverted axis (e.g. '!priority=urgent').
func TestWipInvertFilterSummaryShowsBang(t *testing.T) {
	dir := t.TempDir()
	startInProgressTask(t, dir, "urgent-task")
	startInProgressTask(t, dir, "low-task")
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "2", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "wip", "--priority", "urgent", "--invert")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	if !strings.Contains(stdout, "!priority=urgent") {
		t.Errorf("expected '!priority=urgent' in header summary; got: %q", stdout)
	}
}

// TestWipInvertJSONComposes: --invert composes cleanly with --json
// (which it must — it just narrows the in-memory slice that gets
// encoded). Verifies the JSON shape doesn't change under --invert.
func TestWipInvertJSONComposes(t *testing.T) {
	dir := t.TempDir()
	startInProgressTask(t, dir, "x")
	if _, _, err := runCmd(t, dir, "tag", "1", "+work"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--tag", "work", "--invert", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 0 {
		t.Errorf("expected empty array (only WIP is tagged work, inverted excludes it); got %+v", tasks)
	}
	// Empty array, not null — jq pipelines that iterate should
	// not crash.
	if !strings.Contains(stdout, "[]") {
		t.Errorf("expected '[]' for empty result; got: %q", stdout)
	}
}

// TestWipInvertHelpMention: --help surfaces the new flag so it's
// discoverable.
func TestWipInvertHelpMention(t *testing.T) {
	dir := t.TempDir()
	_, combined, err := runCmd(t, dir, "wip", "--help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(combined, "--invert") {
		t.Errorf("expected --invert in help text; got:\n%s", combined)
	}
}
