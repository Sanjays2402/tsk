package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPendingTagFiltersByTag: tasks carrying the requested tag are
// kept; tasks with different tags are excluded.
func TestPendingTagFiltersByTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work-prereq", "-t", "work"); err != nil {
		t.Fatalf("add work-prereq: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "work-blocked", "-t", "work"); err != nil {
		t.Fatalf("add work-blocked: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home-prereq", "-t", "home"); err != nil {
		t.Fatalf("add home-prereq: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home-blocked", "-t", "home"); err != nil {
		t.Fatalf("add home-blocked: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend 4->3: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Default (no tag): both #2 and #4 are pending.
	stdout, _, err := runCmd(t, dir, "depend", "--pending")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if !strings.Contains(stdout, "work-blocked") || !strings.Contains(stdout, "home-blocked") {
		t.Fatalf("expected both pending tasks without tag filter, got:\n%s", stdout)
	}
	// With --tag work: only #2.
	stdout, _, err = runCmd(t, dir, "depend", "--pending", "--tag", "work")
	if err != nil {
		t.Fatalf("pending --tag work: %v", err)
	}
	if !strings.Contains(stdout, "work-blocked") {
		t.Fatalf("expected work-blocked, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "home-blocked") {
		t.Fatalf("home-blocked must be excluded under --tag work, got:\n%s", stdout)
	}
	// Header should mention the tag for transparency.
	if !strings.Contains(stdout, "tag=work") {
		t.Fatalf("header should include 'tag=work', got:\n%s", stdout)
	}
}

// TestPendingTagCaseInsensitive: --tag matches case-insensitively
// (matches HasTag's contract).
func TestPendingTagCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq", "-t", "Work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "blocked", "-t", "WORK"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--tag", "WORK")
	if err != nil {
		t.Fatalf("pending --tag: %v", err)
	}
	if !strings.Contains(stdout, "blocked") {
		t.Fatalf("expected blocked under --tag WORK (case-insensitive), got:\n%s", stdout)
	}
}

// TestPendingTagEmptyResult: --tag for a tag with no matching
// pending tasks gives an explicit "no tasks" message including
// the tag, not silent output.
func TestPendingTagEmptyResult(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--tag", "personal")
	if err != nil {
		t.Fatalf("pending --tag personal: %v", err)
	}
	if !strings.Contains(stdout, "no tasks") {
		t.Fatalf("expected explicit empty-state message, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tag=personal") {
		t.Fatalf("empty message should mention 'tag=personal', got:\n%s", stdout)
	}
}

// TestPendingTagComposesWithJSON: --tag + --json produces the same
// schema, just narrowed.
func TestPendingTagComposesWithJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "wp", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "wb", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "hp", "-t", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "hb", "-t", "home"); err != nil {
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
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--tag", "home", "--json")
	if err != nil {
		t.Fatalf("pending --tag home --json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d:\n%s", len(rows), stdout)
	}
	if id, _ := rows[0]["id"].(float64); int(id) != 4 {
		t.Fatalf("expected id=4 (hb), got %v", rows[0]["id"])
	}
}

// TestPendingTagEmptyValueIsNoFilter: --tag "" should behave like
// no tag flag at all (defensive: a misconfigured shell var setting
// the value to empty shouldn't silently filter to nothing).
func TestPendingTagEmptyValueIsNoFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "blocked"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--tag", "")
	if err != nil {
		t.Fatalf("pending --tag '': %v", err)
	}
	if !strings.Contains(stdout, "blocked") {
		t.Fatalf("--tag '' should NOT filter; expected 'blocked', got:\n%s", stdout)
	}
	if strings.Contains(stdout, "tag=") {
		t.Fatalf("--tag '' should NOT annotate header with tag=, got:\n%s", stdout)
	}
}
