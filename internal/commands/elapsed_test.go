package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestElapsedSingleReportsHumanizedAndSeconds: `tsk elapsed <id>` on
// a started task shows the humanized duration AND the raw seconds
// inside the human report (so scripts grepping for either work).
func TestElapsedSingleReportsHumanizedAndSeconds(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "the work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	stdout, _, err := runCmd(t, dir, "elapsed", "1")
	if err != nil {
		t.Fatalf("elapsed: %v", err)
	}
	for _, want := range []string{"#1", "the work", "started:", "elapsed:"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in plain output, got:\n%s", want, stdout)
		}
	}
}

// TestElapsedSingleRejectsNotInProgress: starting precondition is enforced.
func TestElapsedSingleRejectsNotInProgress(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "elapsed", "1")
	if err == nil {
		t.Fatal("expected error on a never-started task")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected ExitCode 2 (usage), got %v", err)
	}
}

// TestElapsedSingleRejectsMissingID: a non-existent id returns a
// not-found error (not a usage error), matching `show`/`start`.
func TestElapsedSingleRejectsMissingID(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "elapsed", "99")
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

// TestElapsedAllSortedByStaleness: no-arg form returns every
// in-progress task, OLDEST start first (staleness view).
func TestElapsedAllSortedByStaleness(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"first", "second", "third"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// Start in order with sleeps so 'first' is the stalest.
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start 1: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, _, err := runCmd(t, dir, "start", "3"); err != nil {
		t.Fatalf("start 3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "elapsed")
	if err != nil {
		t.Fatalf("elapsed: %v", err)
	}
	if !strings.Contains(stdout, "first") || !strings.Contains(stdout, "third") {
		t.Fatalf("expected both started tasks, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "second") {
		t.Fatalf("non-started 'second' should NOT appear:\n%s", stdout)
	}
	// Stalest first: 'first' appears BEFORE 'third' (opposite of in-progress).
	if strings.Index(stdout, "first") > strings.Index(stdout, "third") {
		t.Fatalf("expected stalest first (first before third), got:\n%s", stdout)
	}
}

// TestElapsedAllEmpty: friendly message when nothing's in-progress.
func TestElapsedAllEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "elapsed")
	if err != nil {
		t.Fatalf("elapsed: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress") {
		t.Fatalf("expected 'no in-progress', got:\n%s", stdout)
	}
}

// TestElapsedAllJSONSchemaStable: --json emits the documented fields
// and an [] for empty.
func TestElapsedAllJSONSchemaStable(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "elapsed", "--json")
	if err != nil {
		t.Fatalf("elapsed --json empty: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected '[]' for empty case, got %q", stdout)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	stdout, _, err = runCmd(t, dir, "elapsed", "--json")
	if err != nil {
		t.Fatalf("elapsed --json: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(arr))
	}
	for _, k := range []string{"id", "title", "started_at", "elapsed_seconds", "elapsed"} {
		if _, ok := arr[0][k]; !ok {
			t.Fatalf("expected key %q in JSON, got %v", k, arr[0])
		}
	}
	secs, ok := arr[0]["elapsed_seconds"].(float64)
	if !ok || secs < 1 {
		t.Fatalf("elapsed_seconds should be >= 1, got %v (%T)", arr[0]["elapsed_seconds"], arr[0]["elapsed_seconds"])
	}
}

// TestElapsedSingleJSONShape: single-id JSON is an OBJECT (not array).
// Lets consumers `tsk elapsed 3 --json | jq -r .elapsed_seconds`
// without indexing.
func TestElapsedSingleJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "elapsed", "1", "--json")
	if err != nil {
		t.Fatalf("elapsed 1 --json: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatalf("single-id JSON should be object: %v\n%s", err, stdout)
	}
	if obj["id"].(float64) != 1 {
		t.Fatalf("expected id 1, got %v", obj["id"])
	}
}

// TestElapsedClockSkewProtectionDoesNotPanic: a started: timestamp in
// the future (clock skew, hand-edit) should NOT crash — elapsed
// should clamp to 0 instead of returning negative.
func TestElapsedClockSkewProtection(t *testing.T) {
	dir := t.TempDir()
	// Hand-edit a started: timestamp 1h in the future.
	future := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
	writeRawTasks(t, dir,
		"- [ ] skewed <!-- id:1 prio:medium started:"+future+" -->",
	)
	stdout, _, err := runCmd(t, dir, "elapsed", "1", "--json")
	if err != nil {
		t.Fatalf("elapsed: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if got := obj["elapsed_seconds"].(float64); got < 0 {
		t.Fatalf("elapsed_seconds should clamp >= 0, got %v", got)
	}
}
