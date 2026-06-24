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

// TestLogPriorityFiltersByPriority: --priority urgent narrows the
// recent-completions feed to tasks at exactly urgent priority.
func TestLogPriorityFiltersByPriority(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"urgent-1", "low-2", "high-3"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "2", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "3", "high"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
	}

	stdout, _, err := runCmd(t, dir, "log", "--priority", "urgent", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 {
		t.Errorf("expected only #1 (urgent); got %+v", tasks)
	}
}

// TestLogPriorityShortFormAccepted: short forms (u/h/m/l) work
// just like wip and depend --pending — model.ParsePriority handles
// the alias resolution.
func TestLogPriorityShortFormAccepted(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "log", "--priority", "u", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 {
		t.Errorf("expected #1 to match short-form 'u'; got %+v", tasks)
	}
}

// TestLogPriorityInvalidRejected: a bogus priority string produces
// a clean usage error naming the offending value.
func TestLogPriorityInvalidRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "log", "--priority", "ultraurgent")
	if err == nil {
		t.Fatal("expected error for invalid priority")
	}
	if !strings.Contains(err.Error(), "invalid --priority") {
		t.Errorf("expected 'invalid --priority' error; got: %v", err)
	}
}

// TestLogPriorityEmptyValueNoFilter: an empty --priority is a
// no-op (defensive against unset shell vars; matches wip's
// stance).
func TestLogPriorityEmptyValueNoFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "1", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "log", "--priority", "", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task (empty filter is no-op); got %d", len(tasks))
	}
}

// TestLogPriorityComposesWithTagAsAND: --priority and --tag
// compose as AND. The task must match both filters to survive.
func TestLogPriorityComposesWithTagAsAND(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"urgent-work", "urgent-home", "low-work"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "2", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "3", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "tag", "1", "+work"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "tag", "2", "+home"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "tag", "3", "+work"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done: %v", err)
		}
	}

	stdout, _, err := runCmd(t, dir, "log", "--priority", "urgent", "--tag", "work", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 {
		t.Errorf("expected only #1 (urgent + work); got %+v", tasks)
	}
}

// TestLogPriorityComposesWithSince: --since and --priority compose
// as AND — recency window AND priority filter both apply.
func TestLogPriorityComposesWithSince(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"recent-urgent", "old-urgent"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "2", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	for _, id := range []string{"1", "2"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	// Push #2's completion 30 days ago.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	old := time.Now().Add(-30 * 24 * time.Hour)
	t2 := s.ByID(2)
	t2.Completed = &old
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "log", "--priority", "urgent", "--since", "7d", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 {
		t.Errorf("expected only #1 (recent + urgent); got %+v", tasks)
	}
}

// TestLogPriorityEmptyResultMessage: when no completions match
// the priority filter, the plain-text path reports "no completed
// tasks" (the existing empty-result branch).
func TestLogPriorityEmptyResultMessage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "1", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "log", "--priority", "urgent")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(stdout, "no completed tasks") {
		t.Errorf("expected 'no completed tasks' for unmatched filter; got: %q", stdout)
	}
}

// TestLogPriorityJSONEmptyArrayNotNull: under --json the empty
// result is an empty array, not null — jq pipelines that iterate
// don't crash.
func TestLogPriorityJSONEmptyArrayNotNull(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "log", "--priority", "urgent", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(stdout, "[]") {
		t.Errorf("expected '[]' for empty result; got: %q", stdout)
	}
}

// TestLogPriorityHelpMention: --help surfaces the new flag.
func TestLogPriorityHelpMention(t *testing.T) {
	dir := t.TempDir()
	_, combined, err := runCmd(t, dir, "log", "--help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(combined, "--priority") {
		t.Errorf("expected --priority in help text; got:\n%s", combined)
	}
}
