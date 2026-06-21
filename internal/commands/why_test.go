package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestWhyPlainListsCreatedAtMinimum: every task added via `tsk add`
// has a Created timestamp, so `tsk why` should always have at least
// one event row.
func TestWhyPlainListsCreatedAtMinimum(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "first thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "why", "1")
	if err != nil {
		t.Fatalf("why: %v", err)
	}
	if !strings.Contains(stdout, "#1") || !strings.Contains(stdout, "first thing") {
		t.Fatalf("expected task header, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "created") {
		t.Fatalf("expected 'created' event, got:\n%s", stdout)
	}
}

// TestWhyOrdersEventsChronologically: created → started → completed
// when all three exist. (created and started can land in the same
// second on a fast machine — assert the BOTH-present case with a
// short sleep between start and done so completed is strictly later.)
func TestWhyOrdersEventsChronologically(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "the work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	// 'done' clears Started, so 'started' won't be in the timeline.
	// Use 'why' on an in-progress task to assert created-before-started.
	stdout, _, err := runCmd(t, dir, "why", "1")
	if err != nil {
		t.Fatalf("why: %v", err)
	}
	idxCreated := strings.Index(stdout, "created")
	idxStarted := strings.Index(stdout, "started")
	if idxCreated < 0 || idxStarted < 0 {
		t.Fatalf("expected both created and started, got:\n%s", stdout)
	}
	if idxCreated > idxStarted {
		t.Fatalf("created should appear BEFORE started:\n%s", stdout)
	}
}

// TestWhyCompletedAppearsAfterCreated: a done task surfaces created
// THEN completed; started should be absent (done clears it).
func TestWhyCompletedAppearsAfterCreated(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "to ship"); err != nil {
		t.Fatalf("add: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "why", "1")
	if err != nil {
		t.Fatalf("why: %v", err)
	}
	idxCreated := strings.Index(stdout, "created")
	idxCompleted := strings.Index(stdout, "completed")
	if idxCreated < 0 || idxCompleted < 0 {
		t.Fatalf("expected created and completed, got:\n%s", stdout)
	}
	if idxCreated > idxCompleted {
		t.Fatalf("created should be before completed:\n%s", stdout)
	}
}

// TestWhyDueAnnotation: due dates show a relative annotation. Uses an
// explicit future date so the test isn't sensitive to the documented
// UTC-midnight persistence boundary (see STATE.md tick #5 footgun
// note); 'upcoming' is the stable annotation here.
func TestWhyDueAnnotation(t *testing.T) {
	dir := t.TempDir()
	future := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	if _, _, err := runCmd(t, dir, "add", "deadline", "-d", future); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "why", "1")
	if err != nil {
		t.Fatalf("why: %v", err)
	}
	if !strings.Contains(stdout, "due") {
		t.Fatalf("expected 'due' event, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "upcoming") {
		t.Fatalf("expected '(upcoming)' annotation, got:\n%s", stdout)
	}
}

// TestWhyWaitAnnotation: a future wait surfaces "hidden until <date>";
// a past wait surfaces "expired".
func TestWhyWaitAnnotation(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "later", "-d", "tomorrow"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "1", "2099-01-01"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "why", "1")
	if err != nil {
		t.Fatalf("why: %v", err)
	}
	if !strings.Contains(stdout, "hidden until") {
		t.Fatalf("expected 'hidden until' annotation for future wait, got:\n%s", stdout)
	}
}

// TestWhyJSONSchemaStable: --json emits {task, events: [{at, kind}]}.
func TestWhyJSONSchemaStable(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "why", "1", "--json")
	if err != nil {
		t.Fatalf("why --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if _, ok := doc["task"]; !ok {
		t.Fatalf("missing 'task', got: %v", doc)
	}
	events, ok := doc["events"].([]any)
	if !ok {
		t.Fatalf("'events' should be array, got %T", doc["events"])
	}
	if len(events) < 2 {
		t.Fatalf("expected at least created+started events, got %d", len(events))
	}
	for i, raw := range events {
		obj, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("event[%d] should be object, got %T", i, raw)
		}
		for _, k := range []string{"at", "kind"} {
			if _, ok := obj[k]; !ok {
				t.Fatalf("event[%d] missing key %q: %v", i, k, obj)
			}
		}
	}
}

// TestWhyRejectsMissingID: not-found returns an error.
func TestWhyRejectsMissingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "why", "99")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

// TestWhyHandlesHandEditedNoTimestamps: a task with no Created field
// (hand-edited) should not crash; the output explains the gap.
func TestWhyHandlesHandEditedNoTimestamps(t *testing.T) {
	dir := t.TempDir()
	writeRawTasks(t, dir, "- [ ] bare <!-- id:1 prio:medium -->")
	stdout, _, err := runCmd(t, dir, "why", "1")
	if err != nil {
		t.Fatalf("why on bare task: %v", err)
	}
	if !strings.Contains(stdout, "no timestamped events") {
		t.Fatalf("expected empty-events explainer, got:\n%s", stdout)
	}
}
