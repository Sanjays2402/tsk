package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveJSONAppendCreatesFreshFile: the first --append call
// to a non-existent .jsonl file creates it with the JSONL convention
// (single compact record, one trailing newline). Same UX as shell
// `>>` to a missing file — no pre-create needed. Sister of
// `tsk graph --json --output --append` first-call behavior.
func TestArchiveJSONAppendCreatesFreshFile(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	outPath := filepath.Join(dir, "history.jsonl")
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append")
	if err != nil {
		t.Fatalf("archive --json --output --append: %v", err)
	}
	if !strings.Contains(stdout, "appended ") || !strings.Contains(stdout, "history.jsonl") || !strings.Contains(stdout, "format=jsonl") {
		t.Fatalf("expected appended-to-file confirmation, got: %s", stdout)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Exactly one line, terminated by trailing newline from json.Encoder.
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line in fresh JSONL file, got %d:\n%s", len(lines), body)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(lines[0]), &doc); err != nil {
		t.Fatalf("parse first line: %v\nbody: %s", err, body)
	}
	if doc.TotalCount != 1 {
		t.Errorf("expected total_count=1, got %d", doc.TotalCount)
	}
}

// TestArchiveJSONAppendBuildsHistoryAcrossCalls: three sequential
// --append calls produce three records in the file, in call order.
// This is the snapshot-history use case — over time the file
// builds up a chronological record of archive runs (per-day
// completions, bucket-distribution drift, anomaly spotting).
func TestArchiveJSONAppendBuildsHistoryAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "runs.jsonl")
	// Three consecutive archive runs, each archiving one task.
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
		if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append"); err != nil {
			t.Fatalf("append after done %s: %v", id, err)
		}
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines after 3 appends, got %d:\n%s", len(lines), body)
	}
	// Parse each line; confirm each is a single-line JSON record
	// with TotalCount=1 (one task archived per call).
	for i, line := range lines {
		var doc archiveDoc
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("parse line %d: %v\nline: %s", i, err, line)
		}
		if doc.TotalCount != 1 {
			t.Errorf("line %d: expected total_count=1, got %d", i, doc.TotalCount)
		}
	}
}

// TestArchiveJSONAppendUsesCompactRecords: even without an explicit
// compact-JSON flag, --append produces single-line records (no
// indentation). The implicit upgrade keeps the on-disk shape
// valid JSONL — indented JSON across records would corrupt every
// consumer that splits on \n.
func TestArchiveJSONAppendUsesCompactRecords(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	outPath := filepath.Join(dir, "snap.jsonl")
	if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	trimmed := strings.TrimRight(string(body), "\n")
	if strings.Contains(trimmed, "\n") {
		t.Fatalf("append-mode output must be a single line, got:\n%s", body)
	}
	// Compact JSON has no leading indentation on field names.
	if strings.Contains(trimmed, "  \"") {
		t.Fatalf("append-mode output must be compact (no indentation), got:\n%s", body)
	}
}

// TestArchiveJSONAppendAcceptsJSONLAndJSON: both .jsonl (canonical)
// and .json are accepted with --append. The validator matrix
// matches `tsk graph --json --output --append`'s extension matrix
// so the two append surfaces read symmetrically.
func TestArchiveJSONAppendAcceptsJSONLAndJSON(t *testing.T) {
	for _, ext := range []string{"jsonl", "json"} {
		t.Run("ext_"+ext, func(t *testing.T) {
			dir := t.TempDir()
			if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
				t.Fatalf("add: %v", err)
			}
			if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
				t.Fatalf("done: %v", err)
			}
			outPath := filepath.Join(dir, "history."+ext)
			if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append"); err != nil {
				t.Fatalf("archive --append .%s: %v", ext, err)
			}
			if _, err := os.Stat(outPath); err != nil {
				t.Errorf("expected file at %s, got: %v", outPath, err)
			}
		})
	}
}

// TestArchiveJSONAppendRejectsBadExtension: in append mode, only
// .json and .jsonl are accepted. Other extensions surface a clear
// usage error. Catches the silent-footgun case where a user types
// `--append --output history.svg` and the file lands with JSON
// bytes under a misleading name.
func TestArchiveJSONAppendRejectsBadExtension(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	for _, bad := range []string{"history.svg", "history.txt", "history"} {
		outPath := filepath.Join(dir, bad)
		_, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append")
		if err == nil {
			t.Fatalf("expected error for --append --output %q, got nil", bad)
		}
		if !strings.Contains(err.Error(), ".json or .jsonl") {
			t.Fatalf("for %q: expected error to mention .json or .jsonl, got %v", bad, err)
		}
		var ec ExitCoder
		if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
			t.Fatalf("for %q: expected exit code 2, got %v", bad, err)
		}
	}
}

// TestArchiveJSONAppendRequiresOutput: --append without --output is
// a usage error (where would we append to?). Matches the
// `tsk graph --json --output --append` contract.
func TestArchiveJSONAppendRequiresOutput(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--json", "--append")
	if err == nil {
		t.Fatal("expected error for --append without --output")
	}
	if !strings.Contains(err.Error(), "--output") {
		t.Fatalf("expected error to mention --output, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestArchiveJSONAppendRequiresJSON: --append without --json is a
// usage error (the append mode is exclusively a JSON-envelope
// path). Matches the surface contract every other --append flag
// in tsk uses.
func TestArchiveJSONAppendRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	outPath := filepath.Join(dir, "history.jsonl")
	_, _, err := runCmd(t, dir, "archive", "--all", "--output", outPath, "--append")
	if err == nil {
		t.Fatal("expected error for --append without --json")
	}
	if !strings.Contains(err.Error(), "--json") {
		t.Fatalf("expected error to mention --json, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestArchiveJSONAppendDryRunWritesRecord: --dry-run --json
// --output --append writes a single compact preview record to the
// file without touching the active store or the archive. Useful for
// CI rehearsals where the snapshot history is built ahead of the
// real run.
func TestArchiveJSONAppendDryRunWritesRecord(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	for _, id := range []string{"1", "2"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	outPath := filepath.Join(dir, "preview.jsonl")
	if _, _, err := runCmd(t, dir, "archive", "--all", "--dry-run", "--json", "--output", outPath, "--append"); err != nil {
		t.Fatalf("dry-run append: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(body))), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, body)
	}
	if !doc.DryRun {
		t.Error("dry_run should be true on dry-run path")
	}
	if doc.TotalCount != 2 {
		t.Errorf("total_count: want 2, got %d", doc.TotalCount)
	}
	// Active store should still have both tasks (dry-run wrote
	// nothing to the actual archive file).
	ls, _, err := runCmd(t, dir, "ls", "--done")
	if err != nil {
		t.Fatalf("ls --done: %v", err)
	}
	if !strings.Contains(ls, "a") || !strings.Contains(ls, "b") {
		t.Errorf("dry-run shouldn't have removed tasks, ls says:\n%s", ls)
	}
}

// TestArchiveJSONAppendNoTasksWritesEmptyRecord: when nothing
// qualifies, --append still emits a one-line record with
// archived: [] and total_count: 0. Same semantic the non-append
// path uses — pipelines that always read the latest line don't
// crash on a "nothing to archive" run.
func TestArchiveJSONAppendNoTasksWritesEmptyRecord(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "still open"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "history.jsonl")
	if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append"); err != nil {
		t.Fatalf("archive append: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d:\n%s", len(lines), body)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(lines[0]), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, body)
	}
	if doc.TotalCount != 0 {
		t.Errorf("total_count: want 0, got %d", doc.TotalCount)
	}
	if doc.Archived == nil || len(doc.Archived) != 0 {
		t.Errorf("archived: want empty array, got %v", doc.Archived)
	}
}

// TestArchiveJSONAppendPreservesPriorLines: appending to an
// existing JSONL file adds a record at the END; existing lines
// are untouched. The whole point of append-mode — anything that
// truncates would defeat the snapshot-history use case.
func TestArchiveJSONAppendPreservesPriorLines(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "history.jsonl")
	// Pre-seed the file with a hand-crafted record.
	seed := `{"archive_path":"/tmp/seed.md","strategy":"flat","dry_run":false,"total_count":99,"active_count":0,"archived":[]}` + "\n"
	if err := os.WriteFile(outPath, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Now run a real archive --append.
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath, "--append"); err != nil {
		t.Fatalf("archive --append: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.HasPrefix(string(body), seed) {
		t.Errorf("expected file to START with the seed line, got:\n%s", body)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (seed + new), got %d:\n%s", len(lines), body)
	}
	// Second line must be the real archive run record.
	var doc archiveDoc
	if err := json.Unmarshal([]byte(lines[1]), &doc); err != nil {
		t.Fatalf("parse line 2: %v\nline: %s", err, lines[1])
	}
	if doc.TotalCount != 1 {
		t.Errorf("line 2 total_count: want 1 (new archive), got %d", doc.TotalCount)
	}
}
