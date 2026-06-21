package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestFindMatchesTitleOnly: a regex that would match in notes via
// `grep` must NOT match via `find` — the whole point of the verb is
// the title-only contract.
func TestFindMatchesTitleOnly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "deploy the app"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "ship something"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "note", "2", "deploy steps go here"); err != nil {
		t.Fatalf("note: %v", err)
	}
	// `grep deploy` would match both (#1 title, #2 notes).
	// `find deploy` should match ONLY #1.
	stdout, _, err := runCmd(t, dir, "find", "deploy")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !strings.Contains(stdout, "#1") {
		t.Fatalf("expected #1 in find output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#2") {
		t.Fatalf("find should NOT match notes; got #2 in:\n%s", stdout)
	}
}

// TestFindCaseInsensitiveDefault: default behavior matches POSIX
// grep -i and mirrors `tsk grep`.
func TestFindCaseInsensitiveDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "Refactor Parser"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "find", "refactor")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !strings.Contains(stdout, "#1") {
		t.Fatalf("expected case-insensitive match, got:\n%s", stdout)
	}
}

// TestFindCaseSensitiveFlag: -i=false respects case.
func TestFindCaseSensitiveFlag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "Refactor Parser"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "find", "-i=false", "refactor")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if strings.Contains(stdout, "#1") {
		t.Fatalf("expected no match with -i=false, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "no matches") {
		t.Fatalf("expected 'no matches' empty message, got:\n%s", stdout)
	}
}

// TestFindUndoneOnlyByDefault: done tasks excluded; --done flips.
func TestFindUndoneOnlyByDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "still open"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "already shipped"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "find", "shipped")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if strings.Contains(stdout, "#2") {
		t.Fatalf("done task should be excluded by default:\n%s", stdout)
	}
	// --done flips
	stdout, _, err = runCmd(t, dir, "find", "shipped", "--done")
	if err != nil {
		t.Fatalf("find --done: %v", err)
	}
	if !strings.Contains(stdout, "#2") {
		t.Fatalf("--done should surface the done task:\n%s", stdout)
	}
}

// TestFindInvert: --invert returns the complement.
func TestFindInvert(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "deploy a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "other thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "find", "deploy", "--invert")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !strings.Contains(stdout, "#2") || strings.Contains(stdout, "#1") {
		t.Fatalf("--invert should return only the non-matching task, got:\n%s", stdout)
	}
}

// TestFindFilesOnlyAndCount: --files-only emits ID-only lines;
// --count emits just the number.
func TestFindFilesOnlyAndCount(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deploy a", "deploy b", "other"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// --files-only
	stdout, _, err := runCmd(t, dir, "find", "deploy", "--files-only")
	if err != nil {
		t.Fatalf("find -l: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 id lines, got %d:\n%s", len(lines), stdout)
	}
	for _, line := range lines {
		if line != "1" && line != "2" {
			t.Fatalf("expected pure id, got %q", line)
		}
	}
	// --count
	stdout, _, err = runCmd(t, dir, "find", "deploy", "--count")
	if err != nil {
		t.Fatalf("find --count: %v", err)
	}
	if strings.TrimSpace(stdout) != "2" {
		t.Fatalf("expected count 2, got %q", stdout)
	}
}

// TestFindJSONShape: --json emits a stable array; empty case is [].
func TestFindJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "deploy a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "find", "deploy", "--json")
	if err != nil {
		t.Fatalf("find --json: %v", err)
	}
	var tasks []model.Task
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0].Title != "deploy a" {
		t.Fatalf("expected one task 'deploy a', got %+v", tasks)
	}
	// Empty case
	stdout, _, err = runCmd(t, dir, "find", "nope", "--json")
	if err != nil {
		t.Fatalf("find --json empty: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected [] for empty case, got %q", stdout)
	}
}

// TestFindLimit: caps the result count.
func TestFindLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if _, _, err := runCmd(t, dir, "add", "match me"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "find", "match", "--limit", "2", "--files-only")
	if err != nil {
		t.Fatalf("find --limit: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected limit=2 lines, got %d:\n%s", len(lines), stdout)
	}
}

// TestFindInvalidRegex: bogus regex is a usage error.
func TestFindInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "find", "(unclosed")
	if err == nil {
		t.Fatal("expected error for bad regex")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected ExitCode 2, got %v", err)
	}
}

// TestFindMutexFlags: any two of {--files-only, --count, --json} is
// a usage error.
func TestFindMutexFlags(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "find", "x", "--files-only", "--json")
	if err == nil {
		t.Fatal("expected error for combined --files-only + --json")
	}
}
