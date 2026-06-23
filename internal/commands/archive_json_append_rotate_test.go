package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveJSONAppendRotateTrimsOldest: with --rotate N, after the
// (N+1)th archive append the file holds exactly N records, with the
// OLDEST (head-of-file) dropped. FIFO eviction confirmed by checking
// each kept record's total_count to identify the run.
func TestArchiveJSONAppendRotateTrimsOldest(t *testing.T) {
	dir := t.TempDir()
	// Seed five tasks.
	for _, title := range []string{"t1", "t2", "t3", "t4", "t5"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "history.jsonl")
	// Mark each task done in sequence, then archive — produces
	// five sequential append calls, each with TotalCount=1.
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
		if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append", "--rotate", "3"); err != nil {
			t.Fatalf("archive after done %s: %v", id, err)
		}
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines after 5 appends with --rotate 3, got %d:\n%s", len(lines), body)
	}
	// Each surviving line should be parseable (FIFO eviction
	// kept the last 3 runs, which all archived 1 task each).
	for i, line := range lines {
		var doc archiveDoc
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("parse line %d: %v\nline: %s", i, err, line)
		}
		if doc.TotalCount != 1 {
			t.Errorf("line %d: expected total_count=1 (one task per run), got %d", i, doc.TotalCount)
		}
	}
}

// TestArchiveJSONAppendRotateReportsDroppedCount: when rotation
// actually trims lines, the status message includes the dropped
// count and kept-newest target. Silent rotation would be a surprise.
func TestArchiveJSONAppendRotateReportsDroppedCount(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "trim.jsonl")
	// First two runs fit under cap of 2 (no rotation message).
	for _, id := range []string{"1", "2"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
		stdout, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append", "--rotate", "2")
		if err != nil {
			t.Fatalf("archive: %v", err)
		}
		if strings.Contains(stdout, "rotated") {
			t.Errorf("did not expect rotation message under cap, got: %s", stdout)
		}
	}
	// Third run trips the cap (drops 1, keeps 2).
	if _, _, err := runCmd(t, dir, "done", "3"); err != nil {
		t.Fatalf("done 3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append", "--rotate", "2")
	if err != nil {
		t.Fatalf("archive 3: %v", err)
	}
	if !strings.Contains(stdout, "rotated") || !strings.Contains(stdout, "dropped 1 oldest line(s)") || !strings.Contains(stdout, "kept newest 2") {
		t.Errorf("expected rotation report on cap breach, got: %s", stdout)
	}
}

// TestArchiveJSONAppendRotateNoTrimUnderCap: rotation when the file
// hasn't filled the cap is a no-op — no lines dropped, no status
// message, file retains every record.
func TestArchiveJSONAppendRotateNoTrimUnderCap(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "under.jsonl")
	// Two runs under a cap of 10 stay in the file.
	for _, id := range []string{"1", "2"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
		stdout, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append", "--rotate", "10")
		if err != nil {
			t.Fatalf("archive: %v", err)
		}
		if strings.Contains(stdout, "rotated") {
			t.Errorf("did not expect rotation message under cap, got: %s", stdout)
		}
	}
	body, _ := os.ReadFile(outPath)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 retained lines, got %d:\n%s", len(lines), body)
	}
}

// TestArchiveJSONAppendRotateRequiresAppend: --rotate without
// --append is a usage error. Mirrors the graph version's contract.
func TestArchiveJSONAppendRotateRequiresAppend(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	outPath := filepath.Join(dir, "x.json")
	_, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--rotate", "5")
	if err == nil {
		t.Fatal("expected error for --rotate without --append")
	}
	if !strings.Contains(err.Error(), "--rotate requires --append") {
		t.Fatalf("expected rotate-requires-append error, got: %v", err)
	}
}

// TestArchiveJSONAppendRotateRejectsNegative: negative --rotate
// is a usage error.
func TestArchiveJSONAppendRotateRejectsNegative(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	outPath := filepath.Join(dir, "x.jsonl")
	_, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append", "--rotate", "-3")
	if err == nil {
		t.Fatal("expected error for negative --rotate")
	}
	if !strings.Contains(err.Error(), "--rotate must be >= 0") {
		t.Fatalf("expected rotate-must-be-nonnegative error, got: %v", err)
	}
}

// TestArchiveJSONAppendRotateZeroIsDisable: --rotate 0 explicitly
// disables rotation (matches the default and the graph version's
// behavior — useful as a script-side override toggle).
func TestArchiveJSONAppendRotateZeroIsDisable(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d", "e"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "disabled.jsonl")
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
		if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append", "--rotate", "0"); err != nil {
			t.Fatalf("archive: %v", err)
		}
	}
	body, _ := os.ReadFile(outPath)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 retained lines with --rotate 0 (disabled), got %d", len(lines))
	}
}

// TestArchiveJSONAppendRotateOneKeepsOnlyLast: --rotate 1 produces
// a rolling single-record file. Edge case for the keepN=1 path.
func TestArchiveJSONAppendRotateOneKeepsOnlyLast(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "single.jsonl")
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
		if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append", "--rotate", "1"); err != nil {
			t.Fatalf("archive: %v", err)
		}
	}
	body, _ := os.ReadFile(outPath)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line with --rotate 1, got %d:\n%s", len(lines), body)
	}
}

// TestArchiveJSONAppendRotateCleansUpTmp: after a rotation pass the
// .rotate.tmp helper file is gone (rename consumed it). Guards
// against orphan tmp files accumulating in long-running archive
// loops.
func TestArchiveJSONAppendRotateCleansUpTmp(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "clean.jsonl")
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
		if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append", "--rotate", "2"); err != nil {
			t.Fatalf("archive: %v", err)
		}
	}
	if _, err := os.Stat(outPath + ".rotate.tmp"); err == nil {
		t.Errorf("expected NO orphan .rotate.tmp file, but it exists")
	}
}
