package commands

import (
	"strings"
	"testing"
)

// TestLsFormatTable verifies --format=table emits aligned columns with a
// header row and per-task rows. Columns: ID, DONE, P, DUE, TITLE, TAGS.
func TestLsFormatTable(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "write more tests", "-p", "high", "-t", "dev"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "buy milk", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls", "--format", "table")
	if err != nil {
		t.Fatalf("ls --format table: %v", err)
	}
	// Header present
	for _, h := range []string{"ID", "DONE", "P", "DUE", "TITLE", "TAGS"} {
		if !strings.Contains(stdout, h) {
			t.Errorf("table output missing header %q:\n%s", h, stdout)
		}
	}
	// Task rows present with checkbox markers
	if !strings.Contains(stdout, "[ ]") {
		t.Errorf("table output missing checkbox marker:\n%s", stdout)
	}
	if !strings.Contains(stdout, "write more tests") {
		t.Errorf("table missing task title:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#dev") {
		t.Errorf("table missing tag column:\n%s", stdout)
	}
	// Each row should be at least as long as the header row.
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected header + 2 rows, got %d lines:\n%s", len(lines), stdout)
	}
	headerLen := len(lines[0])
	for i, line := range lines[1:] {
		if len(line) < headerLen-10 { // allow some slop for short tag column
			t.Errorf("row %d shorter than header (%d vs %d): %q", i+1, len(line), headerLen, line)
		}
	}
}

// TestLsFormatJSONStillWorks verifies the legacy --json flag is preserved.
func TestLsFormatJSONStillWorks(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "json task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls", "--json")
	if err != nil {
		t.Fatalf("ls --json: %v", err)
	}
	if !strings.Contains(stdout, `"Title": "json task"`) {
		t.Errorf("--json output missing expected JSON:\n%s", stdout)
	}
}

// TestLsFormatJSONViaFormatFlag verifies --format=json is equivalent to --json.
func TestLsFormatJSONViaFormatFlag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "json task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls", "--format", "json")
	if err != nil {
		t.Fatalf("ls --format json: %v", err)
	}
	if !strings.Contains(stdout, `"Title": "json task"`) {
		t.Errorf("--format=json output missing expected JSON:\n%s", stdout)
	}
}

// TestLsFormatRejectsBoth verifies --format and --json are mutually exclusive.
func TestLsFormatRejectsBoth(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "ls", "--format", "table", "--json")
	if err == nil {
		t.Fatal("expected error for --format + --json combo")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error message missing 'mutually exclusive': %v", err)
	}
}

// TestLsFormatRejectsUnknown verifies bogus --format values are rejected.
func TestLsFormatRejectsUnknown(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "ls", "--format", "xml")
	if err == nil {
		t.Fatal("expected error for unknown --format")
	}
	if !strings.Contains(err.Error(), "unknown --format") {
		t.Errorf("error message missing 'unknown --format': %v", err)
	}
}

// TestLsFormatTableTruncatesLongTitle verifies title cap of 40 runes works
// and a truncated row ends with the ellipsis character.
func TestLsFormatTableTruncatesLongTitle(t *testing.T) {
	dir := t.TempDir()
	longTitle := strings.Repeat("abcdefghij", 6) // 60 chars
	if _, _, err := runCmd(t, dir, "add", longTitle); err != nil {
		t.Fatalf("add long: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls", "--format", "table")
	if err != nil {
		t.Fatalf("ls table: %v", err)
	}
	if !strings.Contains(stdout, "…") {
		t.Errorf("expected ellipsis in truncated title row:\n%s", stdout)
	}
	if strings.Contains(stdout, longTitle) {
		t.Errorf("table should not contain full 60-char title:\n%s", stdout)
	}
}

// TestLsFormatTableEmpty verifies the empty-list message still appears.
func TestLsFormatTableEmpty(t *testing.T) {
	dir := t.TempDir()
	// Add then delete to exercise the empty-table path on a real .tsk.md
	if _, _, err := runCmd(t, dir, "add", "soon to be gone"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "rm", "1"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls", "--format", "table")
	if err != nil {
		t.Fatalf("ls empty: %v", err)
	}
	if !strings.Contains(stdout, "no tasks") {
		t.Errorf("empty table should say 'no tasks':\n%s", stdout)
	}
}
