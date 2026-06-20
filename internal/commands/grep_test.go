package commands

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestGrepMatchesTitleByDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "fix deploy script"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "write docs"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "deploy")
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "fix deploy script") {
		t.Fatalf("expected title match, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "write docs") {
		t.Fatalf("non-matching task should be excluded, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "matched in: title") {
		t.Fatalf("expected field annotation, got:\n%s", stdout)
	}
}

func TestGrepDefaultsToCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "Deploy The Thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "deploy")
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "Deploy The Thing") {
		t.Fatalf("expected case-insensitive match, got:\n%s", stdout)
	}
}

func TestGrepCaseSensitive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "Deploy The Thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "-i=false", "deploy")
	if err != nil {
		t.Fatalf("grep -i=false: %v", err)
	}
	if strings.Contains(stdout, "Deploy The Thing") {
		t.Fatalf("case-sensitive 'deploy' should NOT match 'Deploy', got:\n%s", stdout)
	}
}

func TestGrepRealRegex(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "TODO(api): retry logic"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "ship it"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", `TODO\([^)]+\)`)
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "TODO(api)") {
		t.Fatalf("expected regex match, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "ship it") {
		t.Fatalf("non-matching task leaked, got:\n%s", stdout)
	}
}

func TestGrepMatchesNotes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-n", "see PR #123 for details"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "PR #123")
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "thing") {
		t.Fatalf("expected notes match, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "matched in: notes") {
		t.Fatalf("expected notes annotation, got:\n%s", stdout)
	}
}

func TestGrepMatchesTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-t", "deploy"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "deploy")
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "matched in: tag") {
		t.Fatalf("expected tag annotation, got:\n%s", stdout)
	}
}

func TestGrepTitleOnlySkipsNotes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-n", "deploy is mentioned here"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "deploy", "--title-only")
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "no matches") {
		t.Fatalf("--title-only should skip notes, got:\n%s", stdout)
	}
}

func TestGrepNotesOnlySkipsTitle(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "deploy script"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "deploy", "--notes-only")
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "no matches") {
		t.Fatalf("--notes-only should skip title, got:\n%s", stdout)
	}
}

func TestGrepInvertReturnsNonMatching(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "alpha", "--invert")
	if err != nil {
		t.Fatalf("grep --invert: %v", err)
	}
	if !strings.Contains(stdout, "beta") {
		t.Fatalf("expected beta (non-matching), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "alpha") {
		t.Fatalf("alpha should be inverted out, got:\n%s", stdout)
	}
}

func TestGrepFilesOnlyEmitsIDs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "alphabet"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "alpha", "--files-only")
	if err != nil {
		t.Fatalf("grep --files-only: %v", err)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 ID lines, got %d:\n%s", len(lines), stdout)
	}
	for _, l := range lines {
		if _, err := strconv.Atoi(strings.TrimSpace(l)); err != nil {
			t.Fatalf("--files-only should emit integers, got %q", l)
		}
	}
}

func TestGrepCountEmitsNumber(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "alpha two"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "alpha", "--count")
	if err != nil {
		t.Fatalf("grep --count: %v", err)
	}
	if strings.TrimSpace(stdout) != "2" {
		t.Fatalf("expected count of 2, got %q", stdout)
	}
}

func TestGrepJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "matched thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "matched", "--json")
	if err != nil {
		t.Fatalf("grep --json: %v", err)
	}
	var tasks []map[string]any
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0]["Title"] != "matched thing" {
		t.Fatalf("expected 1 task, got %v", tasks)
	}
}

func TestGrepLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if _, _, err := runCmd(t, dir, "add", "matchy thing"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "grep", "matchy", "--limit", "2")
	if err != nil {
		t.Fatalf("grep --limit: %v", err)
	}
	if n := strings.Count(stdout, "\n"); n != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", n, stdout)
	}
}

func TestGrepDoneScope(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "closed thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Default: undone only.
	stdout, _, err := runCmd(t, dir, "grep", "thing")
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "open thing") || strings.Contains(stdout, "closed thing") {
		t.Fatalf("default scope wrong, got:\n%s", stdout)
	}
	// --done: only the closed one.
	stdout, _, err = runCmd(t, dir, "grep", "thing", "--done")
	if err != nil {
		t.Fatalf("grep --done: %v", err)
	}
	if !strings.Contains(stdout, "closed thing") || strings.Contains(stdout, "open thing") {
		t.Fatalf("--done scope wrong, got:\n%s", stdout)
	}
	// --all: both.
	stdout, _, err = runCmd(t, dir, "grep", "thing", "--all")
	if err != nil {
		t.Fatalf("grep --all: %v", err)
	}
	if !strings.Contains(stdout, "open thing") || !strings.Contains(stdout, "closed thing") {
		t.Fatalf("--all scope wrong, got:\n%s", stdout)
	}
}

func TestGrepBadRegex(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "grep", "[unclosed")
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestGrepEmptyPatternRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "grep", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only pattern")
	}
}

func TestGrepNoMatches(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "zzz")
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "no matches") {
		t.Fatalf("expected 'no matches', got:\n%s", stdout)
	}
}

func TestGrepMutexFlags(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	cases := [][]string{
		{"grep", "x", "--files-only", "--count"},
		{"grep", "x", "--files-only", "--json"},
		{"grep", "x", "--count", "--json"},
		{"grep", "x", "--title-only", "--notes-only"},
		{"grep", "x", "--done", "--all"},
	}
	for _, args := range cases {
		_, _, err := runCmd(t, dir, args...)
		if err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}

func TestGrepRejectsNegativeLimit(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "grep", "x", "--limit", "-1")
	if err == nil {
		t.Fatal("expected error for negative --limit")
	}
}

func TestGrepCountWithZeroMatches(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "zzz", "--count")
	if err != nil {
		t.Fatalf("grep --count: %v", err)
	}
	if strings.TrimSpace(stdout) != "0" {
		t.Fatalf("expected 0 for no matches, got %q", stdout)
	}
}

func TestGrepTitleFieldPrecedenceOverTag(t *testing.T) {
	dir := t.TempDir()
	// A task whose title contains "x" AND has tag "x"; the annotation
	// should report the first matched field — title — for stability.
	if _, _, err := runCmd(t, dir, "add", "x in title", "-t", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "grep", "x")
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(stdout, "matched in: title") {
		t.Fatalf("expected title precedence, got:\n%s", stdout)
	}
}
