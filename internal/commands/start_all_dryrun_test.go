package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestStartAllDryRunDoesNotMutate: --dry-run must NOT stamp started:
// on any task. The .tsk.md content stays byte-for-byte the same.
// Critical invariant: no Save() call in the dry-run code path.
func TestStartAllDryRunDoesNotMutate(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title, "-t", "work"); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	path := filepath.Join(dir, ".tsk.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--dry-run")
	if err != nil {
		t.Fatalf("start --all --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "[dry-run]") {
		t.Fatalf("expected [dry-run] prefix in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "would start 3 task(s)") {
		t.Fatalf("expected 'would start 3 task(s)', got:\n%s", stdout)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf(".tsk.md mutated by dry-run\nBEFORE:\n%s\nAFTER:\n%s", before, after)
	}
	s, _ := store.Load(path)
	for _, t2 := range s.Tasks {
		if t2.Started != nil {
			t.Fatalf("task #%d was started despite dry-run", t2.ID)
		}
	}
}

// TestStartAllDryRunListsEveryMatchingID: every filter-matched task
// (that would actually be started) appears in the preview list with
// its id and title.
func TestStartAllDryRunListsEveryMatchingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-t", "work"); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta", "-t", "work"); err != nil {
		t.Fatalf("add beta: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "gamma", "-t", "home"); err != nil {
		t.Fatalf("add gamma: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--dry-run")
	if err != nil {
		t.Fatalf("start --all --tag work --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "#1") || !strings.Contains(stdout, "alpha") {
		t.Fatalf("expected #1 alpha in preview, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#2") || !strings.Contains(stdout, "beta") {
		t.Fatalf("expected #2 beta in preview, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "gamma") {
		t.Fatalf("gamma (home tag) must NOT appear in --tag work preview, got:\n%s", stdout)
	}
}

// TestStartAllDryRunFilterSummaryInOutput: the filter summary
// ("tag=work") appears in the dry-run header line so the user
// remembers WHICH filter generated the preview.
func TestStartAllDryRunFilterSummaryInOutput(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "ops", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "ops", "--priority", "high", "--dry-run")
	if err != nil {
		t.Fatalf("start dry-run: %v", err)
	}
	if !strings.Contains(stdout, "tag=ops") {
		t.Fatalf("expected tag=ops in summary, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "priority=high") {
		t.Fatalf("expected priority=high in summary, got:\n%s", stdout)
	}
}

// TestStartAllDryRunEmptyMatchUsesNonDryWording: an empty filter
// result on dry-run uses the same "no open tasks match" wording the
// non-dry path uses — keeps the two answers consistent.
func TestStartAllDryRunEmptyMatchUsesNonDryWording(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "ghost", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run empty match: %v", err)
	}
	if !strings.Contains(stdout, "no open tasks match") {
		t.Fatalf("expected 'no open tasks match', got:\n%s", stdout)
	}
}

// TestStartAllDryRunAlreadyStartedExcluded: tasks that are already
// in-progress shouldn't appear in the would-start preview (matches
// the idempotent-skip semantics of the actual start path). The
// preview reads truthfully.
func TestStartAllDryRunAlreadyStartedExcluded(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start 1: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(stdout, "would start 1 task(s)") {
		t.Fatalf("expected 'would start 1 task(s)' (b only; a already started), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#1") {
		t.Fatalf("#1 (already started) must NOT appear in dry-run preview, got:\n%s", stdout)
	}
}

// TestStartAllDryRunAllAlreadyStartedClearMessage: when every matched
// task is already in-progress, the preview says so explicitly (not
// just an empty list — that would look broken).
func TestStartAllDryRunAllAlreadyStartedClearMessage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run all started: %v", err)
	}
	if !strings.Contains(stdout, "no tasks would be started") {
		t.Fatalf("expected 'no tasks would be started' message, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "already in-progress") {
		t.Fatalf("expected 'already in-progress' explanation, got:\n%s", stdout)
	}
}

// TestStartAllDryRunWithResetIncludesAlreadyStarted: with --reset,
// already-started tasks WOULD be bumped, so they should appear in
// the dry-run preview. Mirrors the non-dry semantics.
func TestStartAllDryRunWithResetIncludesAlreadyStarted(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--reset", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run --reset: %v", err)
	}
	if !strings.Contains(stdout, "would start 1 task(s)") {
		t.Fatalf("expected 'would start 1 task(s)' under --reset, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#1") {
		t.Fatalf("expected #1 in preview under --reset, got:\n%s", stdout)
	}
}

// TestStartAllDryRunRejectsBareDryRun: --dry-run without --all is
// a usage error (single-id start is already explicit).
func TestStartAllDryRunRejectsBareDryRun(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "1", "--dry-run")
	if err == nil {
		t.Fatal("expected error for --dry-run without --all")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestStartAllNonDryStillWorks: the non-dry --all path keeps working
// unchanged after the dry-run addition. Regression guard.
func TestStartAllNonDryStillWorks(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work")
	if err != nil {
		t.Fatalf("non-dry --all: %v", err)
	}
	if !strings.Contains(stdout, "started 1 task(s)") {
		t.Fatalf("non-dry regression, got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(1).Started == nil {
		t.Fatal("#1 should be started after non-dry --all")
	}
}
