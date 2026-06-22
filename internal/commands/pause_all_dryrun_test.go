package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestPauseAllDryRunDoesNotMutate: --dry-run must NOT clear started:
// on any task. The .tsk.md content stays byte-for-byte the same.
// Critical invariant: no Save() call in the dry-run code path. Sister
// of TestStartAllDryRunDoesNotMutate.
func TestPauseAllDryRunDoesNotMutate(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	path := filepath.Join(dir, ".tsk.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--dry-run")
	if err != nil {
		t.Fatalf("pause --all --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "[dry-run]") {
		t.Fatalf("expected [dry-run] prefix, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "would pause 3 task(s)") {
		t.Fatalf("expected 'would pause 3 task(s)', got:\n%s", stdout)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf(".tsk.md mutated by dry-run\nBEFORE:\n%s\nAFTER:\n%s", before, after)
	}
	s, _ := store.Load(path)
	for _, task := range s.Tasks {
		if task.Started == nil {
			t.Fatalf("task #%d should still be in-progress after dry-run", task.ID)
		}
	}
}

// TestPauseAllDryRunListsEveryMatchingID: every filter-matched in-
// progress task appears in the preview list with its id and title.
func TestPauseAllDryRunListsEveryMatchingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "gamma", "-t", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--tag", "work", "--dry-run")
	if err != nil {
		t.Fatalf("pause dry-run: %v", err)
	}
	if !strings.Contains(stdout, "#1") || !strings.Contains(stdout, "alpha") {
		t.Fatalf("expected #1 alpha, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#2") || !strings.Contains(stdout, "beta") {
		t.Fatalf("expected #2 beta, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "gamma") {
		t.Fatalf("gamma (home tag) must NOT appear in --tag work preview, got:\n%s", stdout)
	}
}

// TestPauseAllDryRunFilterSummaryInOutput: the filter summary appears
// in the dry-run header line so the user remembers WHICH filter
// generated the preview.
func TestPauseAllDryRunFilterSummaryInOutput(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "ops", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--tag", "ops", "--priority", "high", "--dry-run")
	if err != nil {
		t.Fatalf("pause dry-run: %v", err)
	}
	if !strings.Contains(stdout, "tag=ops") {
		t.Fatalf("expected tag=ops summary, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "priority=high") {
		t.Fatalf("expected priority=high summary, got:\n%s", stdout)
	}
}

// TestPauseAllDryRunNoFilter: --dry-run with no filter previews every
// in-progress task (no filter parens in the header line).
func TestPauseAllDryRunNoFilter(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	for _, id := range []string{"1", "2"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--dry-run")
	if err != nil {
		t.Fatalf("pause dry-run no filter: %v", err)
	}
	if !strings.Contains(stdout, "would pause 2 task(s):") {
		t.Fatalf("expected 'would pause 2 task(s):' (no filter parens), got:\n%s", stdout)
	}
}

// TestPauseAllDryRunEmptyWipNoFilter: when nothing is in-progress at
// all, --dry-run reports the same "no in-progress tasks" wording the
// non-dry path uses — same wording across the two paths.
func TestPauseAllDryRunEmptyWipNoFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run empty: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks") {
		t.Fatalf("expected 'no in-progress tasks', got:\n%s", stdout)
	}
}

// TestPauseAllDryRunFilteredEmptyMatchUsesFilterWording: WIP tasks
// exist but none match the filter — the empty message includes the
// filter summary so a typo is visible (mirrors the non-dry wording).
func TestPauseAllDryRunFilteredEmptyMatchUsesFilterWording(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--tag", "ghost", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run filtered empty: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks match") {
		t.Fatalf("expected 'no in-progress tasks match', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tag=ghost") {
		t.Fatalf("expected filter summary in empty message, got:\n%s", stdout)
	}
}

// TestPauseAllDryRunRejectsBareDryRun: --dry-run without --all is a
// usage error (single-id pause is already explicit).
func TestPauseAllDryRunRejectsBareDryRun(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pause", "1", "--dry-run")
	if err == nil {
		t.Fatal("expected error for --dry-run without --all")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestPauseAllNonDryStillWorks: the non-dry --all path keeps working
// after the dry-run addition. Regression guard.
func TestPauseAllNonDryStillWorks(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all")
	if err != nil {
		t.Fatalf("non-dry pause --all: %v", err)
	}
	if !strings.Contains(stdout, "stopped 1 task(s)") {
		t.Fatalf("non-dry regression, got:\n%s", stdout)
	}
}

// TestPauseAllDryRunJSONShape: --dry-run --json emits a stable schema.
// Decoding the output must succeed and the would_pause array must
// contain the expected entries.
func TestPauseAllDryRunJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "gamma", "-t", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--tag", "work", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run json: %v", err)
	}
	var doc struct {
		WouldPause []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		} `json:"would_pause"`
		TotalCount int    `json:"total_count"`
		Filter     string `json:"filter"`
		Tag        string `json:"tag"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if doc.TotalCount != 2 {
		t.Fatalf("expected total_count=2, got %d", doc.TotalCount)
	}
	if len(doc.WouldPause) != 2 {
		t.Fatalf("expected 2 would_pause entries, got %d", len(doc.WouldPause))
	}
	if doc.WouldPause[0].ID != 1 || doc.WouldPause[0].Title != "alpha" {
		t.Fatalf("expected #1 alpha first, got %+v", doc.WouldPause[0])
	}
	if doc.WouldPause[1].ID != 2 || doc.WouldPause[1].Title != "beta" {
		t.Fatalf("expected #2 beta second, got %+v", doc.WouldPause[1])
	}
	if doc.Tag != "work" {
		t.Fatalf("expected tag=work, got %q", doc.Tag)
	}
	if !strings.Contains(doc.Filter, "tag=work") {
		t.Fatalf("expected filter=tag=work, got %q", doc.Filter)
	}
}

// TestPauseAllDryRunJSONEmptyArray: a no-match preview emits
// would_pause: [] (not null) so jq pipelines iterating the array
// don't crash. Verified by decoding into a non-nullable slice.
func TestPauseAllDryRunJSONEmptyArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run json empty: %v", err)
	}
	if !strings.Contains(stdout, `"would_pause": []`) {
		t.Fatalf("expected literal `would_pause: []`, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"total_count": 0`) {
		t.Fatalf("expected total_count=0, got:\n%s", stdout)
	}
}

// TestPauseAllDryRunJSONRejectsBareJSON: --json without --dry-run is
// rejected — pause has no non-dry JSON output mode (the actual
// mutation has a different shape, "stopped N task(s)"), so --json
// only makes sense in preview mode.
func TestPauseAllDryRunJSONRejectsBareJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, _, err := runCmd(t, dir, "pause", "--all", "--json")
	if err == nil {
		t.Fatal("expected error for --json without --dry-run")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestPauseAllDryRunJSONPriorityField: when --priority is set, the
// priority field appears in the JSON output. Sister test to the
// start --all --json (when shipped) — same field shape.
func TestPauseAllDryRunJSONPriorityField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "u1", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--priority", "urgent", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run json: %v", err)
	}
	var doc struct {
		Priority string `json:"priority"`
		Filter   string `json:"filter"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if doc.Priority != "urgent" {
		t.Fatalf("expected priority=urgent, got %q", doc.Priority)
	}
	if !strings.Contains(doc.Filter, "priority=urgent") {
		t.Fatalf("expected filter=priority=urgent, got %q", doc.Filter)
	}
}
