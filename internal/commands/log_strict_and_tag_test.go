package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
)

// addAndDoneWithTags is a small test helper: adds N tasks with the
// given titles + tag sets, then marks each done. Keeps the body of
// the strict-and-tag tests focused on the assertion logic.
func addAndDoneWithTags(t *testing.T, dir string, items []struct {
	Title string
	Tags  []string
}) {
	t.Helper()
	for _, it := range items {
		if _, _, err := runCmd(t, dir, "add", it.Title); err != nil {
			t.Fatalf("add %q: %v", it.Title, err)
		}
	}
	for i, it := range items {
		id := i + 1
		for _, tag := range it.Tags {
			if _, _, err := runCmd(t, dir, "tag", itoaTest(id), "+"+tag); err != nil {
				t.Fatalf("tag %d +%s: %v", id, tag, err)
			}
		}
		if _, _, err := runCmd(t, dir, "done", itoaTest(id)); err != nil {
			t.Fatalf("done %d: %v", id, err)
		}
	}
}

func itoaTest(n int) string {
	// Tiny test helper to keep import surface small — strconv.Itoa
	// would work but we already need strings/etc, and a one-off
	// avoids the import.
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+(n/10))) + string(rune('0'+(n%10)))
}

// TestLogStrictAndTagIntersectionFilter: --strict-and-tag a,b keeps
// only completions carrying BOTH 'a' AND 'b' tags. Tasks with one
// of the two tags but not both are excluded.
func TestLogStrictAndTagIntersectionFilter(t *testing.T) {
	dir := t.TempDir()
	addAndDoneWithTags(t, dir, []struct {
		Title string
		Tags  []string
	}{
		{"both-tags", []string{"work", "p0"}},
		{"only-work", []string{"work"}},
		{"only-p0", []string{"p0"}},
		{"no-tags", []string{}},
	})

	stdout, _, err := runCmd(t, dir, "log", "--strict-and-tag", "work,p0", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 {
		t.Errorf("expected only #1 (both tags); got %+v", tasks)
	}
}

// TestLogStrictAndTagSingleTagEquivalentToTag: --strict-and-tag X
// (single tag CSV) behaves identically to --tag X.
func TestLogStrictAndTagSingleTagEquivalentToTag(t *testing.T) {
	dir := t.TempDir()
	addAndDoneWithTags(t, dir, []struct {
		Title string
		Tags  []string
	}{
		{"has-work", []string{"work"}},
		{"no-work", []string{}},
	})

	stdout, _, err := runCmd(t, dir, "log", "--strict-and-tag", "work", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 {
		t.Errorf("expected only #1 (has-work); got %+v", tasks)
	}
}

// TestLogStrictAndTagMutexWithTag: --tag and --strict-and-tag set
// together is a usage error (each is a different tag-selector
// axis; combining is ambiguous).
func TestLogStrictAndTagMutexWithTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "log", "--tag", "work", "--strict-and-tag", "release,p0")
	if err == nil {
		t.Fatal("expected error for --tag with --strict-and-tag")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' error; got: %v", err)
	}
}

// TestLogStrictAndTagWhitespaceTolerant: CSV tokens tolerate
// surrounding whitespace and drop empties so " work , , p0 " and
// "work,p0" parse identically.
func TestLogStrictAndTagWhitespaceTolerant(t *testing.T) {
	dir := t.TempDir()
	addAndDoneWithTags(t, dir, []struct {
		Title string
		Tags  []string
	}{
		{"both", []string{"work", "p0"}},
	})

	stdout, _, err := runCmd(t, dir, "log", "--strict-and-tag", " work , , p0 ", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task (whitespace tolerant); got %d", len(tasks))
	}
}

// TestLogStrictAndTagComposesWithPriorityAsAND: --strict-and-tag
// composes with --priority as AND. The task must satisfy both.
func TestLogStrictAndTagComposesWithPriorityAsAND(t *testing.T) {
	dir := t.TempDir()
	addAndDoneWithTags(t, dir, []struct {
		Title string
		Tags  []string
	}{
		{"both-and-urgent", []string{"work", "p0"}},
		{"both-but-low", []string{"work", "p0"}},
	})
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "2", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "log", "--strict-and-tag", "work,p0", "--priority", "urgent", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 {
		t.Errorf("expected only #1 (both tags AND urgent); got %+v", tasks)
	}
}

// TestLogStrictAndTagEmptyValueNoFilter: an empty CSV is a no-op
// (defensive against unset shell vars; matches the rest of the
// verb-surface family).
func TestLogStrictAndTagEmptyValueNoFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "log", "--strict-and-tag", "", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task (empty CSV is no-op); got %d", len(tasks))
	}
}

// TestLogStrictAndTagEmptyResult: when no completion matches the
// intersection, the empty-result plain-text path fires.
func TestLogStrictAndTagEmptyResult(t *testing.T) {
	dir := t.TempDir()
	addAndDoneWithTags(t, dir, []struct {
		Title string
		Tags  []string
	}{
		{"only-work", []string{"work"}},
	})

	stdout, _, err := runCmd(t, dir, "log", "--strict-and-tag", "work,p0")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(stdout, "no completed tasks") {
		t.Errorf("expected 'no completed tasks'; got: %q", stdout)
	}
}

// TestLogStrictAndTagHelpMention: --help surfaces the new flag.
func TestLogStrictAndTagHelpMention(t *testing.T) {
	dir := t.TempDir()
	_, combined, err := runCmd(t, dir, "log", "--help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(combined, "--strict-and-tag") {
		t.Errorf("expected --strict-and-tag in help text; got:\n%s", combined)
	}
}
