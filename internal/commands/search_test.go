package commands

import (
	"strings"
	"testing"
)

// TestSearchMatchesTitle verifies a plain fuzzy match on the title.
func TestSearchMatchesTitle(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "buy milk")
	mustAdd(t, dir, "ship feature")
	mustAdd(t, dir, "fix login bug")

	stdout, _, err := runCmd(t, dir, "search", "milk")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(stdout, "buy milk") {
		t.Errorf("expected match on 'buy milk', got:\n%s", stdout)
	}
	if strings.Contains(stdout, "ship feature") || strings.Contains(stdout, "fix login bug") {
		t.Errorf("unexpected non-matching task in output:\n%s", stdout)
	}
}

// TestSearchMatchesTag verifies fuzzy search hits tags by default.
func TestSearchMatchesTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "task one", "-t", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "task two", "-t", "later"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "search", "urgent")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(stdout, "task one") {
		t.Errorf("expected match via #urgent tag, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "task two") {
		t.Errorf("unexpected match for task two:\n%s", stdout)
	}
}

// TestSearchTitleOnly verifies --title-only skips tag matching.
func TestSearchTitleOnly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "task one", "-t", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "search", "urgent", "--title-only")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(stdout, "no matches") {
		t.Errorf("expected 'no matches' (tag should be ignored), got:\n%s", stdout)
	}
}

// TestSearchExcludesDoneByDefault verifies done tasks are hidden.
func TestSearchExcludesDoneByDefault(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "ship feature")
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "search", "ship")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(stdout, "no matches") {
		t.Errorf("expected 'no matches' for done task by default, got:\n%s", stdout)
	}
}

// TestSearchAllIncludesDone verifies --all includes done tasks.
func TestSearchAllIncludesDone(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "ship feature")
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "search", "ship", "--all")
	if err != nil {
		t.Fatalf("search --all: %v", err)
	}
	if !strings.Contains(stdout, "ship feature") {
		t.Errorf("expected ship feature with --all, got:\n%s", stdout)
	}
}

// TestSearchLimit verifies --limit caps results.
func TestSearchLimit(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "milk one")
	mustAdd(t, dir, "milk two")
	mustAdd(t, dir, "milk three")
	stdout, _, err := runCmd(t, dir, "search", "milk", "--limit", "2")
	if err != nil {
		t.Fatalf("search --limit: %v", err)
	}
	count := strings.Count(stdout, "milk")
	if count != 2 {
		t.Errorf("expected 2 hits, got %d:\n%s", count, stdout)
	}
}

// TestSearchJSON verifies JSON output is emitted.
func TestSearchJSON(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "ship feature")
	stdout, _, err := runCmd(t, dir, "search", "ship", "--json")
	if err != nil {
		t.Fatalf("search --json: %v", err)
	}
	if !strings.Contains(stdout, `"Title": "ship feature"`) {
		t.Errorf("expected JSON output with title field:\n%s", stdout)
	}
}

// TestSearchJSONEmpty verifies empty JSON output is `[]`, not the text msg.
func TestSearchJSONEmpty(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "ship feature")
	stdout, _, err := runCmd(t, dir, "search", "nonexistent", "--json")
	if err != nil {
		t.Fatalf("search --json: %v", err)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed != "[]" {
		t.Errorf("expected empty JSON array, got: %q", trimmed)
	}
}

// TestSearchEmptyQuery verifies empty query is rejected.
func TestSearchEmptyQuery(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "x")
	_, _, err := runCmd(t, dir, "search", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only query")
	}
}

// TestSearchTableFormat verifies the table format works for search results.
func TestSearchTableFormat(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "ship feature")
	stdout, _, err := runCmd(t, dir, "search", "ship", "--format", "table")
	if err != nil {
		t.Fatalf("search --format table: %v", err)
	}
	if !strings.Contains(stdout, "ID") || !strings.Contains(stdout, "TITLE") {
		t.Errorf("expected table headers in output:\n%s", stdout)
	}
}

// mustAdd is a test helper for adding a task or failing the test.
func mustAdd(t *testing.T, dir, title string, extraArgs ...string) {
	t.Helper()
	args := append([]string{"add", title}, extraArgs...)
	if _, _, err := runCmd(t, dir, args...); err != nil {
		t.Fatalf("add %q: %v", title, err)
	}
}
