package commands

import (
	"encoding/csv"
	"strings"
	"testing"
)

// TestExportCSVHeaderAndRow exercises the previously-untested CSV export path:
// header row presence, column order, and that a task's fields land in the
// right columns.
func TestExportCSVHeaderAndRow(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship release", "-p", "high", "-t", "dev"); err != nil {
		t.Fatalf("add: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "export", "--csv")
	if err != nil {
		t.Fatalf("export --csv: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(stdout)).ReadAll()
	if err != nil {
		t.Fatalf("export --csv produced invalid CSV: %v\n%s", err, stdout)
	}
	if len(records) < 2 {
		t.Fatalf("expected header + at least one row, got %d records:\n%s", len(records), stdout)
	}

	wantHeader := []string{"id", "done", "priority", "title", "due", "tags", "created", "completed", "notes"}
	if len(records[0]) != len(wantHeader) {
		t.Fatalf("header column count: want %d, got %d (%v)", len(wantHeader), len(records[0]), records[0])
	}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Fatalf("header[%d]: want %q, got %q", i, h, records[0][i])
		}
	}

	row := records[1]
	if row[0] != "1" {
		t.Fatalf("id column: want 1, got %q", row[0])
	}
	if row[1] != "false" {
		t.Fatalf("done column: want false, got %q", row[1])
	}
	if row[2] != "high" {
		t.Fatalf("priority column: want high, got %q", row[2])
	}
	if row[3] != "ship release" {
		t.Fatalf("title column: want %q, got %q", "ship release", row[3])
	}
	if row[5] != "dev" {
		t.Fatalf("tags column: want dev, got %q", row[5])
	}
}

// TestExportCSVDoneAndUndoneRows verifies the done flag is rendered per-row and
// the completed column is populated only for done tasks.
func TestExportCSVDoneAndUndoneRows(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"first", "second"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "export", "-f", "csv")
	if err != nil {
		t.Fatalf("export -f csv: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(stdout)).ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v\n%s", err, stdout)
	}
	if len(records) != 3 {
		t.Fatalf("want header + 2 rows, got %d:\n%s", len(records), stdout)
	}

	// records[1] is id 1 (done), records[2] is id 2 (undone).
	done, undone := records[1], records[2]
	if done[1] != "true" {
		t.Fatalf("task 1 done column: want true, got %q", done[1])
	}
	if done[7] == "" {
		t.Fatalf("task 1 completed column should be populated, got empty")
	}
	if undone[1] != "false" {
		t.Fatalf("task 2 done column: want false, got %q", undone[1])
	}
	if undone[7] != "" {
		t.Fatalf("task 2 completed column should be empty, got %q", undone[7])
	}
}
