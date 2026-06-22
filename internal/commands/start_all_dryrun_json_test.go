package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestStartAllDryRunJSONShape: --dry-run --json emits a stable schema.
// Decoding the output must succeed and would_start must contain the
// expected entries in id-ascending order.
func TestStartAllDryRunJSONShape(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run json: %v", err)
	}
	var doc struct {
		WouldStart []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		} `json:"would_start"`
		TotalCount int    `json:"total_count"`
		Filter     string `json:"filter"`
		Tag        string `json:"tag"`
		Reset      bool   `json:"reset"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if doc.TotalCount != 2 {
		t.Fatalf("expected total_count=2, got %d", doc.TotalCount)
	}
	if len(doc.WouldStart) != 2 {
		t.Fatalf("expected 2 would_start entries, got %d", len(doc.WouldStart))
	}
	if doc.WouldStart[0].ID != 1 || doc.WouldStart[0].Title != "alpha" {
		t.Fatalf("expected #1 alpha first, got %+v", doc.WouldStart[0])
	}
	if doc.WouldStart[1].ID != 2 || doc.WouldStart[1].Title != "beta" {
		t.Fatalf("expected #2 beta second, got %+v", doc.WouldStart[1])
	}
	if doc.Tag != "work" {
		t.Fatalf("expected tag=work, got %q", doc.Tag)
	}
	if doc.Reset {
		t.Fatal("expected reset=false (default)")
	}
}

// TestStartAllDryRunJSONEmptyArray: a no-match preview emits
// would_start: [] (not null) so jq pipelines iterating the array
// don't crash. Verified by literal substring check.
func TestStartAllDryRunJSONEmptyArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "ghost", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run json empty: %v", err)
	}
	if !strings.Contains(stdout, `"would_start": []`) {
		t.Fatalf("expected literal `would_start: []`, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"total_count": 0`) {
		t.Fatalf("expected total_count=0, got:\n%s", stdout)
	}
}

// TestStartAllDryRunJSONRejectsBareJSON: --json without --dry-run is
// rejected — start has no non-dry JSON output mode (the actual
// mutation has a different shape, "started N task(s)"), so --json
// only makes sense in preview mode.
func TestStartAllDryRunJSONRejectsBareJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--json")
	if err == nil {
		t.Fatal("expected error for --json without --dry-run")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestStartAllDryRunJSONRejectsBareJSONOnSingleID: --json without
// --all is also rejected (no JSON output mode for per-id start).
func TestStartAllDryRunJSONRejectsBareJSONOnSingleID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "1", "--json")
	if err == nil {
		t.Fatal("expected error for --json on per-id start")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestStartAllDryRunJSONResetField: when --reset is set, the reset
// field is true AND already-started tasks appear in would_start
// (matches the human-mode --reset behavior).
func TestStartAllDryRunJSONResetField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Without --reset, already-started tasks are excluded.
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run json (no reset): %v", err)
	}
	if !strings.Contains(stdout, `"would_start": []`) {
		t.Fatalf("expected empty would_start without --reset, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"reset": false`) {
		t.Fatalf("expected reset=false, got:\n%s", stdout)
	}
	// With --reset, already-started tasks DO appear.
	stdout, _, err = runCmd(t, dir, "start", "--all", "--tag", "work", "--reset", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run json (reset): %v", err)
	}
	if !strings.Contains(stdout, `"reset": true`) {
		t.Fatalf("expected reset=true, got:\n%s", stdout)
	}
	var doc struct {
		WouldStart []struct {
			ID int `json:"id"`
		} `json:"would_start"`
		TotalCount int `json:"total_count"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if doc.TotalCount != 1 || len(doc.WouldStart) != 1 {
		t.Fatalf("expected 1 entry under --reset, got total=%d len=%d", doc.TotalCount, len(doc.WouldStart))
	}
	if doc.WouldStart[0].ID != 1 {
		t.Fatalf("expected #1 in reset preview, got %d", doc.WouldStart[0].ID)
	}
}

// TestStartAllDryRunJSONPriorityField: --priority shows up as a
// dedicated field (lowercased, trimmed) so a script can branch on
// the explicit filter without parsing the summary string.
func TestStartAllDryRunJSONPriorityField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "u1", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--priority", "urgent", "--dry-run", "--json")
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
		t.Fatalf("expected filter to contain priority=urgent, got %q", doc.Filter)
	}
}

// TestStartAllDryRunJSONNoMutate: critical invariant inherited from
// the non-JSON dry-run — the JSON path must also write nothing to
// disk. Defensive regression guard against future refactors that
// might accidentally Save() before emitting JSON.
func TestStartAllDryRunJSONNoMutate(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--dry-run", "--json"); err != nil {
		t.Fatalf("dry-run json: %v", err)
	}
	// Subsequent non-dry call must succeed (i.e. nothing was started
	// by the JSON path; the task is still in the open set).
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work")
	if err != nil {
		t.Fatalf("subsequent non-dry: %v", err)
	}
	if !strings.Contains(stdout, "started 1 task(s)") {
		t.Fatalf("expected 1 started post-dry, got:\n%s", stdout)
	}
}
