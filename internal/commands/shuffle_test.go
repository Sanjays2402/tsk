package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestShuffleDefaultPicksOne: no-arg form picks exactly one task from
// the undone pool.
func TestShuffleDefaultPicksOne(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "shuffle", "--seed", "1")
	if err != nil {
		t.Fatalf("shuffle: %v", err)
	}
	// Exactly one task row (one '[ ] #N' marker).
	count := strings.Count(stdout, "[ ] #")
	if count != 1 {
		t.Fatalf("expected exactly 1 task row, got %d:\n%s", count, stdout)
	}
}

// TestShuffleSeedDeterministic: same seed yields the same pick.
// Guards the documented --seed contract (tests and scripts).
func TestShuffleSeedDeterministic(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d", "e"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	out1, _, err := runCmd(t, dir, "shuffle", "3", "--seed", "42")
	if err != nil {
		t.Fatalf("shuffle 1: %v", err)
	}
	out2, _, err := runCmd(t, dir, "shuffle", "3", "--seed", "42")
	if err != nil {
		t.Fatalf("shuffle 2: %v", err)
	}
	if out1 != out2 {
		t.Fatalf("same seed should produce same output:\n%s\n---\n%s", out1, out2)
	}
}

// TestShuffleWithoutReplacement: 5 distinct tasks, ask for 5, get 5
// distinct ids back — never a duplicate.
func TestShuffleWithoutReplacement(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d", "e"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "shuffle", "5", "--seed", "1", "--json")
	if err != nil {
		t.Fatalf("shuffle: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 5 {
		t.Fatalf("expected 5 tasks, got %d", len(tasks))
	}
	seen := map[int]bool{}
	for _, x := range tasks {
		if seen[x.ID] {
			t.Fatalf("duplicate id %d in pick — sampling broke without-replacement", x.ID)
		}
		seen[x.ID] = true
	}
}

// TestShuffleExcludesDoneAndWaiting: default pool excludes done and
// waiting tasks (matches top/next).
func TestShuffleExcludesDoneAndWaiting(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alive", "marked", "frozen"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "freeze", "3"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	// Pick a huge N — we expect only #1 to survive.
	stdout, _, err := runCmd(t, dir, "shuffle", "10", "--seed", "1", "--json")
	if err != nil {
		t.Fatalf("shuffle: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 {
		t.Fatalf("expected only #1 (alive) in pick, got %+v", tasks)
	}
}

// TestShuffleAllIncludesEverything: --all relaxes the exclusion.
func TestShuffleAllIncludesEverything(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alive", "marked"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "shuffle", "5", "--all", "--seed", "1", "--json")
	if err != nil {
		t.Fatalf("shuffle --all: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected both tasks with --all, got %d", len(tasks))
	}
}

// TestShuffleCappedHeadsUp: asking for more than the pool gets the
// pool-size note in the plain output.
func TestShuffleCappedHeadsUp(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "only one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "shuffle", "10", "--seed", "1")
	if err != nil {
		t.Fatalf("shuffle: %v", err)
	}
	if !strings.Contains(stdout, "only 1 task") {
		t.Fatalf("expected pool-size heads-up, got:\n%s", stdout)
	}
}

// TestShuffleEmptyPool: nothing to pick → friendly empty message.
func TestShuffleEmptyPool(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "done one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "shuffle", "--seed", "1")
	if err != nil {
		t.Fatalf("shuffle: %v", err)
	}
	if !strings.Contains(stdout, "no tasks to pick from") {
		t.Fatalf("expected empty-pool note, got:\n%s", stdout)
	}
}

// TestShuffleEmptyJSONIsArray: empty JSON output is [] not null.
func TestShuffleEmptyJSONIsArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "done one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "shuffle", "--seed", "1", "--json")
	if err != nil {
		t.Fatalf("shuffle: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected '[]' for empty case, got %q", stdout)
	}
}

// TestShuffleTagFilter: --tag narrows the candidate pool.
func TestShuffleTagFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "dev one", "-t", "dev"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "no tag"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "shuffle", "5", "--tag", "dev", "--seed", "1", "--json")
	if err != nil {
		t.Fatalf("shuffle: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 {
		t.Fatalf("expected only the dev task, got %+v", tasks)
	}
}

// TestShufflePriorityFilter: --priority narrows by priority.
func TestShufflePriorityFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "low one", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "high one", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "shuffle", "5", "--priority", "high", "--seed", "1", "--json")
	if err != nil {
		t.Fatalf("shuffle: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 2 {
		t.Fatalf("expected only the high task, got %+v", tasks)
	}
}

// TestShufflePriorityRejectsBogus: bad --priority is a usage error.
func TestShufflePriorityRejectsBogus(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "shuffle", "--priority", "banana")
	if err == nil {
		t.Fatal("expected error for bad --priority")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected ExitCode 2, got %v", err)
	}
}

// TestShuffleZeroRejected: N=0 is a usage error (no useful semantics).
func TestShuffleZeroRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "shuffle", "0")
	if err == nil {
		t.Fatal("expected error for N=0")
	}
}
