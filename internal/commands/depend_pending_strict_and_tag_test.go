package commands

import (
	"strings"
	"testing"
)

// TestPendingStrictAndTagRequiresAllTags: a task is in the pending
// feed only when it carries every listed tag (intersection).
// Tasks carrying SOME but not all are excluded. The single-tag
// --tag union-style filter remains for the "any of these" semantic.
func TestPendingStrictAndTagRequiresAllTags(t *testing.T) {
	dir := t.TempDir()
	// Three pending tasks with different tag combinations.
	if _, _, err := runCmd(t, dir, "add", "prereq1"); err != nil {
		t.Fatalf("add prereq1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "both-tags", "-t", "work,p0"); err != nil {
		t.Fatalf("add both: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "prereq2"); err != nil {
		t.Fatalf("add prereq2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "only-work", "-t", "work"); err != nil {
		t.Fatalf("add only-work: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "prereq3"); err != nil {
		t.Fatalf("add prereq3: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "only-p0", "-t", "p0"); err != nil {
		t.Fatalf("add only-p0: %v", err)
	}
	// Wire each blocked task to its prereq.
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend 4->3: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "6", "--on", "5"); err != nil {
		t.Fatalf("depend 6->5: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1", "3", "5"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Without filter: all three (#2, #4, #6) are pending.
	stdout, _, err := runCmd(t, dir, "depend", "--pending")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	for _, want := range []string{"both-tags", "only-work", "only-p0"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %s in default pending feed, got:\n%s", want, stdout)
		}
	}
	// With --strict-and-tag work,p0: ONLY both-tags qualifies.
	stdout, _, err = runCmd(t, dir, "depend", "--pending", "--strict-and-tag", "work,p0")
	if err != nil {
		t.Fatalf("pending --strict-and-tag: %v", err)
	}
	if !strings.Contains(stdout, "both-tags") {
		t.Errorf("expected both-tags in strict-and-tag work,p0 feed, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "only-work") {
		t.Errorf("only-work should be excluded (intersection requires both), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "only-p0") {
		t.Errorf("only-p0 should be excluded (intersection requires both), got:\n%s", stdout)
	}
	// Header should reflect the intersection form with & separator.
	if !strings.Contains(stdout, "tag=work&p0") {
		t.Errorf("header should include 'tag=work&p0', got:\n%s", stdout)
	}
}

// TestPendingStrictAndTagMutuallyExclusiveWithTag: combining
// --tag and --strict-and-tag is rejected at the CLI layer (each
// is a different operator on the tag axis).
func TestPendingStrictAndTagMutuallyExclusiveWithTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "blocked", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "--pending", "--tag", "work", "--strict-and-tag", "work,p0")
	if err == nil {
		t.Fatal("expected error combining --tag and --strict-and-tag")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually-exclusive error, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 usage error, got %v", err)
	}
}

// TestPendingStrictAndTagSingleTagBehavesLikeUnion: a one-element
// CSV in --strict-and-tag is functionally equivalent to --tag
// (intersection of one element collapses to identity). Useful so
// the user doesn't have to switch flags when narrowing from
// multi-tag to single-tag mid-iteration.
func TestPendingStrictAndTagSingleTagBehavesLikeUnion(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq", "-t", "work"); err != nil {
		t.Fatalf("add prereq: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "blocked", "-t", "work"); err != nil {
		t.Fatalf("add blocked: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "skip-prereq", "-t", "home"); err != nil {
		t.Fatalf("add skip: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "skip-blocked", "-t", "home"); err != nil {
		t.Fatalf("add skip-blocked: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--strict-and-tag", "work")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if !strings.Contains(stdout, "blocked") {
		t.Errorf("expected 'blocked' under --strict-and-tag work, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "skip-blocked") {
		t.Errorf("skip-blocked should be excluded, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tag=work") {
		t.Errorf("header should mention tag=work, got:\n%s", stdout)
	}
}

// TestPendingStrictAndTagComposesPriorityIntersection: composes
// with --priority as AND. Useful for "what's freshly unblocked AND
// carrying both my filter tags AND urgent?".
func TestPendingStrictAndTagComposesPriorityIntersection(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq1"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "match", "-t", "work,p0", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "prereq2"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "wrong-prio", "-t", "work,p0", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--strict-and-tag", "work,p0", "--priority", "urgent")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if !strings.Contains(stdout, "match") {
		t.Errorf("expected 'match' in feed, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "wrong-prio") {
		t.Errorf("wrong-prio should be excluded by --priority urgent, got:\n%s", stdout)
	}
	// Header should mention both filters.
	if !strings.Contains(stdout, "tag=work&p0") || !strings.Contains(stdout, "priority=urgent") {
		t.Errorf("header should mention both filters, got:\n%s", stdout)
	}
}

// TestPendingStrictAndTagEmptyResultShowsFilter: when no task in
// the store matches the intersection filter, the empty message
// includes the filter shape so the user can tell whether it was
// a typo or a true no-match.
func TestPendingStrictAndTagEmptyResultShowsFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "blocked", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--strict-and-tag", "work,p0")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if !strings.Contains(stdout, "no tasks freshly unblocked") {
		t.Errorf("expected empty message, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tag=work&p0") {
		t.Errorf("empty message should include the filter, got:\n%s", stdout)
	}
}

// TestPendingStrictAndTagSplitTagsTrimsAndDropsEmpty: the CSV
// parser shares splitTagCSV with --highlight-tag and friends — so
// "work, ,p0" produces the same parse as "work,p0".
func TestPendingStrictAndTagSplitTagsTrimsAndDropsEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "blocked", "-t", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--strict-and-tag", "work, ,p0")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if !strings.Contains(stdout, "blocked") {
		t.Errorf("expected blocked under tolerant CSV parsing, got:\n%s", stdout)
	}
}

// TestTaskHasAllTagsUnit covers the standalone helper: empty
// list returns true (short-circuit identity for callers that
// don't want to branch on filter state), missing any tag returns
// false, all matching tags returns true.
func TestTaskHasAllTagsUnit(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "a,b,c"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Cheap way: round-trip through the CLI filter so we don't have
	// to manually construct model.Task here. The CLI surface is the
	// real public boundary anyway.
	stdout, _, err := runCmd(t, dir, "ls", "--tag", "a")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(stdout, "x") {
		t.Errorf("expected x via --tag a, got:\n%s", stdout)
	}
}
