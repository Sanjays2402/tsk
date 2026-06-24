package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestWipStrictAndTagIntersection: --strict-and-tag CSV narrows
// the wip list to tasks carrying ALL listed tags. Sister of
// --tag's union-style single-tag filter; mirrors the same flag on
// `tsk pause --all`, `tsk start --all`, and `tsk depend --pending`.
func TestWipStrictAndTagIntersection(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work-only", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "p0-only", "--tag", "p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "both", "--tag", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2", "3"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--strict-and-tag", "work,p0")
	if err != nil {
		t.Fatalf("wip --strict-and-tag: %v", err)
	}
	if !strings.Contains(stdout, "both") {
		t.Errorf("expected 'both' (carries both tags), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "work-only") || strings.Contains(stdout, "p0-only") {
		t.Errorf("single-tag tasks should be filtered, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "filter:") || !strings.Contains(stdout, "tag=work&p0") {
		t.Errorf("expected filter header tag=work&p0, got:\n%s", stdout)
	}
}

// TestWipStrictAndTagSingleTagEquivalence: --strict-and-tag with a
// single CSV value is functionally equivalent to --tag. Same
// intersection-of-one semantic as the other strict-and-tag verbs.
func TestWipStrictAndTagSingleTagEquivalence(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work task", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home task", "--tag", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--strict-and-tag", "work")
	if err != nil {
		t.Fatalf("wip --strict-and-tag work: %v", err)
	}
	if !strings.Contains(stdout, "work task") {
		t.Errorf("expected 'work task' under --strict-and-tag work, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "home task") {
		t.Errorf("'home task' should be filtered, got:\n%s", stdout)
	}
}

// TestWipStrictAndTagMutexWithTag: --strict-and-tag and --tag are
// mutually exclusive (each is a different selector axis). Matches
// the rejection contract used by every other strict-and-tag verb.
func TestWipStrictAndTagMutexWithTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "task", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, _, err := runCmd(t, dir, "wip", "--tag", "work", "--strict-and-tag", "work,p0")
	if err == nil {
		t.Fatal("expected error combining --tag and --strict-and-tag")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutual-exclusion message, got: %v", err)
	}
}

// TestWipStrictAndTagJSON: --strict-and-tag composes with --json,
// emitting the intersection-filtered task array.
func TestWipStrictAndTagJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work-only", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "both", "--tag", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--strict-and-tag", "work,p0", "--json")
	if err != nil {
		t.Fatalf("wip --strict-and-tag --json: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("unmarshal: %v\ngot:\n%s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "both" {
		t.Errorf("expected 'both', got %q", tasks[0].Title)
	}
}

// TestWipStrictAndTagWhitespaceTolerant: CSV tokenization mirrors
// the other strict-and-tag verbs — whitespace and empty entries
// are dropped.
func TestWipStrictAndTagWhitespaceTolerant(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "both", "--tag", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// "work, ,p0" should parse identically to "work,p0".
	stdout, _, err := runCmd(t, dir, "wip", "--strict-and-tag", "work, ,p0")
	if err != nil {
		t.Fatalf("wip --strict-and-tag whitespace: %v", err)
	}
	if !strings.Contains(stdout, "both") {
		t.Errorf("expected 'both' under whitespace-CSV filter, got:\n%s", stdout)
	}
}

// TestWipStrictAndTagComposesWithPriority: --strict-and-tag and
// --priority combine as AND (only tasks carrying ALL tags AND at
// the named priority surface).
func TestWipStrictAndTagComposesWithPriority(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "match", "--tag", "work,p0", "--priority", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "wrong-prio", "--tag", "work,p0", "--priority", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "wrong-tag", "--tag", "work", "--priority", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2", "3"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--strict-and-tag", "work,p0", "--priority", "urgent")
	if err != nil {
		t.Fatalf("wip composed: %v", err)
	}
	if !strings.Contains(stdout, "match") {
		t.Errorf("expected 'match' (both filters pass), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "wrong-prio") || strings.Contains(stdout, "wrong-tag") {
		t.Errorf("non-matching tasks should be filtered, got:\n%s", stdout)
	}
}

// TestWipStrictAndTagEmptyMatch: a CSV that matches nothing yields
// a clean empty-state message naming the intersection.
func TestWipStrictAndTagEmptyMatch(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work-only", "--tag", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--strict-and-tag", "work,p0")
	if err != nil {
		t.Fatalf("wip empty match: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks with tags work&p0") {
		t.Fatalf("expected intersection-empty message naming the tags, got:\n%s", stdout)
	}
}

// TestWipStrictAndTagHelpMentionsFlag: --help text mentions the
// new --strict-and-tag flag for discoverability.
func TestWipStrictAndTagHelpMentionsFlag(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "wip", "--help")
	if err != nil {
		t.Fatalf("wip --help: %v", err)
	}
	if !strings.Contains(stdout, "--strict-and-tag") {
		t.Errorf("--help should mention --strict-and-tag, got:\n%s", stdout)
	}
}
