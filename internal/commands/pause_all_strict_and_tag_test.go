package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPauseAllStrictAndTagIntersection: --strict-and-tag work,p0
// only pauses in-progress tasks that carry BOTH tags. Tasks with
// only one of the listed tags are left running. Sister of
// `tsk depend --pending --strict-and-tag` (shipped tick #26) — the
// same logical operator on the tag axis for the bulk-pause surface.
func TestPauseAllStrictAndTagIntersection(t *testing.T) {
	dir := t.TempDir()
	// Three tasks: A has BOTH work+p0, B has only work, C has only p0.
	if _, _, err := runCmd(t, dir, "add", "both", "-t", "work,p0"); err != nil {
		t.Fatalf("add both: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "work-only", "-t", "work"); err != nil {
		t.Fatalf("add work-only: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "p0-only", "-t", "p0"); err != nil {
		t.Fatalf("add p0-only: %v", err)
	}
	// Start all three so they're in-progress.
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	// Pause with intersection filter — only #1 should be paused.
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--strict-and-tag", "work,p0")
	if err != nil {
		t.Fatalf("pause --all --strict-and-tag: %v", err)
	}
	if !strings.Contains(stdout, "stopped 1 task") {
		t.Errorf("expected 'stopped 1 task' (only the both-tag task), got: %s", stdout)
	}
	// Verify by re-checking wip: should have #2 and #3 still in-progress.
	wip, _, err := runCmd(t, dir, "wip")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	if !strings.Contains(wip, "work-only") {
		t.Errorf("#2 (work-only) should still be in-progress, wip:\n%s", wip)
	}
	if !strings.Contains(wip, "p0-only") {
		t.Errorf("#3 (p0-only) should still be in-progress, wip:\n%s", wip)
	}
	if strings.Contains(wip, "both") {
		t.Errorf("#1 (both) should be paused, wip:\n%s", wip)
	}
}

// TestPauseAllStrictAndTagMutexWithTag: --tag and --strict-and-tag
// are mutually exclusive (each is a different tag-selector axis;
// combining would muddle which logical operator applies). Mirrors
// the same mutex `tsk depend --pending` enforces.
func TestPauseAllStrictAndTagMutexWithTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, _, err := runCmd(t, dir, "pause", "--all", "--tag", "work", "--strict-and-tag", "p0,urgent")
	if err == nil {
		t.Fatal("expected error for --tag + --strict-and-tag combination")
	}
	if !strings.Contains(err.Error(), "--tag") || !strings.Contains(err.Error(), "--strict-and-tag") {
		t.Fatalf("expected error to mention both flag names, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %v", err)
	}
}

// TestPauseAllStrictAndTagRequiresAllTags: a task missing any one
// of the listed tags is excluded from the bulk-pause, even if it
// carries most of them.
func TestPauseAllStrictAndTagRequiresAllTags(t *testing.T) {
	dir := t.TempDir()
	// Task #1 carries 2 of 3 listed tags — should NOT be paused.
	if _, _, err := runCmd(t, dir, "add", "partial", "-t", "a,b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--strict-and-tag", "a,b,c")
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks match") {
		t.Errorf("expected 'no in-progress tasks match' (task lacks tag c), got: %s", stdout)
	}
	// Confirm #1 is still in-progress.
	wip, _, err := runCmd(t, dir, "wip")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	if !strings.Contains(wip, "partial") {
		t.Errorf("#1 should still be in-progress, wip:\n%s", wip)
	}
}

// TestPauseAllStrictAndTagComposesWithPriority: --strict-and-tag
// + --priority compose as AND — the task must carry ALL listed
// tags AND match the priority.
func TestPauseAllStrictAndTagComposesWithPriority(t *testing.T) {
	dir := t.TempDir()
	// Two tasks both carry work+p0; only #1 is urgent.
	if _, _, err := runCmd(t, dir, "add", "urgent-pair", "-t", "work,p0", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "low-pair", "-t", "work,p0", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, id := range []string{"1", "2"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--strict-and-tag", "work,p0", "--priority", "urgent")
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !strings.Contains(stdout, "stopped 1 task") {
		t.Errorf("expected 'stopped 1 task' (only urgent+work+p0), got: %s", stdout)
	}
	// Verify #2 (low) still in-progress.
	wip, _, err := runCmd(t, dir, "wip")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	if !strings.Contains(wip, "low-pair") {
		t.Errorf("#2 (low priority) should still be in-progress, wip:\n%s", wip)
	}
}

// TestPauseAllStrictAndTagDryRunPreview: --dry-run --strict-and-tag
// previews the intersection without actually pausing. The filter
// shows up in the human summary as "tag=a&b" (the &-separated
// disambiguation marker that distinguishes intersection from
// union in the on-screen output).
func TestPauseAllStrictAndTagDryRunPreview(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "both", "-t", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--strict-and-tag", "work,p0", "--dry-run")
	if err != nil {
		t.Fatalf("pause --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "[dry-run] would pause 1 task") {
		t.Errorf("expected dry-run preview header, got: %s", stdout)
	}
	if !strings.Contains(stdout, "tag=work&p0") {
		t.Errorf("expected filter summary 'tag=work&p0' (& marker for intersection), got: %s", stdout)
	}
	// Confirm task is still in-progress (dry-run wrote nothing).
	wip, _, err := runCmd(t, dir, "wip")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	if !strings.Contains(wip, "both") {
		t.Errorf("#1 should still be in-progress after dry-run, wip:\n%s", wip)
	}
}

// TestPauseAllStrictAndTagJSON: --dry-run --json with
// --strict-and-tag surfaces the filter in the JSON envelope under
// the "strict_and_tag" key AND in the human-readable "filter"
// summary (rendered as "tag=a&b"). Scripted pipelines have both
// axes available.
func TestPauseAllStrictAndTagJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "both", "-t", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--strict-and-tag", "work,p0", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("pause --json: %v", err)
	}
	var doc pauseAllDryRunDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.StrictAndTag != "work,p0" {
		t.Errorf("strict_and_tag: want 'work,p0', got %q", doc.StrictAndTag)
	}
	if doc.Filter != "tag=work&p0" {
		t.Errorf("filter: want 'tag=work&p0', got %q", doc.Filter)
	}
	if doc.TotalCount != 1 {
		t.Errorf("total_count: want 1, got %d", doc.TotalCount)
	}
	if len(doc.WouldPause) != 1 {
		t.Fatalf("would_pause: want 1 row, got %d", len(doc.WouldPause))
	}
	if doc.WouldPause[0].ID != 1 {
		t.Errorf("would_pause[0].id: want 1, got %d", doc.WouldPause[0].ID)
	}
}

// TestPauseAllStrictAndTagEmptyResult: filter matches no tasks —
// the empty-result message includes the filter summary so the
// user understands WHY no tasks matched (typo visibility).
func TestPauseAllStrictAndTagEmptyResult(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work-only", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--strict-and-tag", "nonexistent,alsogone")
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks match") {
		t.Errorf("expected 'no in-progress tasks match' header, got: %s", stdout)
	}
	if !strings.Contains(stdout, "tag=nonexistent&alsogone") {
		t.Errorf("expected filter summary in empty message, got: %s", stdout)
	}
}

// TestPauseAllStrictAndTagSingleElementCSVIdentity: a single-element
// CSV like --strict-and-tag work behaves equivalently to --tag work
// (no union/intersection distinction on a one-tag set). Important
// edge case so scripts can use --strict-and-tag uniformly.
func TestPauseAllStrictAndTagSingleElementCSVIdentity(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work-task", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "other", "-t", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, id := range []string{"1", "2"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--strict-and-tag", "work")
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !strings.Contains(stdout, "stopped 1 task") {
		t.Errorf("expected 'stopped 1 task' (only work-tagged), got: %s", stdout)
	}
}

// TestPauseAllStrictAndTagOnlyAppliesWithAll: passing
// --strict-and-tag on a positional-id pause is a usage error (the
// filter only makes sense for the --all path; on a single id the
// caller already named the task).
func TestPauseAllStrictAndTagOnlyAppliesWithAll(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, _, err := runCmd(t, dir, "pause", "1", "--strict-and-tag", "work,p0")
	if err == nil {
		t.Fatal("expected error for --strict-and-tag without --all")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Fatalf("expected error to mention --all, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestPauseAllStrictAndTagTolerantCSV: empty/whitespace tokens in
// the CSV (e.g. `work,,p0` or ` work , p0 `) are tolerated — the
// splitTagCSV helper trims and drops empties. Defensive against
// shell-quote-mishap inputs.
func TestPauseAllStrictAndTagTolerantCSV(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "both", "-t", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--strict-and-tag", " work , , p0 ", "--dry-run")
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !strings.Contains(stdout, "would pause 1 task") {
		t.Errorf("expected 'would pause 1 task' (CSV tolerated), got: %s", stdout)
	}
}

// TestPauseAllStrictAndTagHelpsMentionFlag: `pause --help` lists
// the new flag. Sanity check the surface so a user discovering the
// command can find it.
func TestPauseAllStrictAndTagHelpsMentionFlag(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "pause", "--help")
	if err != nil {
		t.Fatalf("pause --help: %v", err)
	}
	if !strings.Contains(stdout, "--strict-and-tag") {
		t.Errorf("expected --strict-and-tag to appear in pause --help, got:\n%s", stdout)
	}
}
