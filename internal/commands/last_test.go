package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestMostRecentlyMutatedSort directly exercises the sort algorithm with
// synthetic timestamps so we don't depend on RFC3339 second-precision on
// real CLI invocations.
func TestMostRecentlyMutatedSort(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		{ID: 1, Title: "first", Created: base},
		{ID: 2, Title: "second", Created: base.Add(time.Minute)},
		{ID: 3, Title: "third", Created: base.Add(2 * time.Minute)},
	}
	got := mostRecentlyMutated(tasks, 2)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].Title != "third" || got[1].Title != "second" {
		t.Fatalf("unexpected order: %s, %s", got[0].Title, got[1].Title)
	}
}

// TestMostRecentlyMutatedCompletionWins: completion timestamp beats
// creation when picking the "latest" task.
func TestMostRecentlyMutatedCompletionWins(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	late := base.Add(time.Hour)
	tasks := []model.Task{
		{ID: 1, Title: "old", Done: true, Created: base, Completed: &late},
		{ID: 2, Title: "new", Created: base.Add(10 * time.Minute)},
	}
	got := mostRecentlyMutated(tasks, 1)
	if len(got) != 1 || got[0].Title != "old" {
		t.Fatalf("expected 'old' (completed-late) to win, got %+v", got)
	}
}

// TestMostRecentlyMutatedSkipsNoTimestamp: tasks with neither timestamp
// must be dropped, not bubble to the top with epoch zero.
func TestMostRecentlyMutatedSkipsNoTimestamp(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		{ID: 1, Title: "hand-edited"},
		{ID: 2, Title: "real", Created: base},
	}
	got := mostRecentlyMutated(tasks, 5)
	if len(got) != 1 || got[0].Title != "real" {
		t.Fatalf("expected only 'real', got %+v", got)
	}
}

// TestMostRecentlyMutatedTieByID: same timestamp -> larger ID first.
func TestMostRecentlyMutatedTieByID(t *testing.T) {
	now := time.Now()
	tasks := []model.Task{
		{ID: 1, Title: "alpha", Created: now},
		{ID: 7, Title: "gamma", Created: now},
		{ID: 3, Title: "beta", Created: now},
	}
	got := mostRecentlyMutated(tasks, 3)
	if got[0].ID != 7 || got[1].ID != 3 || got[2].ID != 1 {
		t.Fatalf("tie-break order wrong: %d,%d,%d", got[0].ID, got[1].ID, got[2].ID)
	}
}

// TestLastCLIDefault smoke-tests the actual CLI path: add one task,
// `tsk last` reports it.
func TestLastCLIDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "only-task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "last")
	if err != nil {
		t.Fatalf("last: %v", err)
	}
	if !strings.Contains(stdout, "only-task") {
		t.Fatalf("expected 'only-task' in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "created") {
		t.Fatalf("expected 'created' reason marker:\n%s", stdout)
	}
}

// TestLastCLIJSONEmptyStore: empty store -> [] (not null).
func TestLastCLIJSONEmptyStore(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "last", "--json")
	if err != nil {
		t.Fatalf("last --json: %v", err)
	}
	stdout = strings.TrimSpace(stdout)
	if stdout != "[]" {
		t.Fatalf("expected []  got %q", stdout)
	}
}

// TestLastCLIJSONShape: --json emits a stable array even for the default n=1.
func TestLastCLIJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "last", "--json")
	if err != nil {
		t.Fatalf("last --json: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(arr) != 1 || arr[0]["Title"] != "thing" {
		t.Fatalf("unexpected JSON shape: %s", stdout)
	}
}

// TestLastNFlagOverlap: --n and positional N must agree or error.
func TestLastNFlagOverlap(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "last", "2", "--n", "2"); err != nil {
		t.Fatalf("agreeing values: %v", err)
	}
	_, _, err := runCmd(t, dir, "last", "2", "--n", "1")
	if err == nil {
		t.Fatal("expected error when --n disagrees with positional N")
	}
}
