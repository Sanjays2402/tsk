package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveJSONOutputWritesEnvelope: --json --output writes the
// archive run's JSON envelope to the named .json file instead of
// stdout. The on-disk content is byte-identical to what stdout
// would have produced (same encoder settings, same indent), just
// landed at a stable filename — the whole point of --output.
//
// Stdout in the file path carries ONLY the "wrote N bytes" line
// (so a script chaining `tsk archive --json --output run.json |
// jq` doesn't see JSON bytes on stdout that would compete with the
// file contents).
func TestArchiveJSONOutputWritesEnvelope(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
	}
	outPath := filepath.Join(dir, "run.json")
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath)
	if err != nil {
		t.Fatalf("archive --json --output: %v", err)
	}
	if !strings.Contains(stdout, "wrote ") || !strings.Contains(stdout, "run.json") || !strings.Contains(stdout, "format=json") {
		t.Fatalf("expected wrote-to-file confirmation, got: %s", stdout)
	}
	// Confirm stdout is ONLY the confirmation line — no JSON bytes.
	if strings.Contains(stdout, "\"archived\"") || strings.Contains(stdout, "\"archive_path\"") {
		t.Fatalf("stdout should NOT contain the JSON envelope when --output is used, got:\n%s", stdout)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse JSON: %v\nbody:\n%s", err, body)
	}
	if doc.TotalCount != 3 {
		t.Errorf("total_count: want 3, got %d", doc.TotalCount)
	}
	if len(doc.Archived) != 3 {
		t.Fatalf("archived: want 3 rows, got %d", len(doc.Archived))
	}
	if doc.Strategy != "flat" {
		t.Errorf("strategy: want flat, got %q", doc.Strategy)
	}
	if doc.DryRun {
		t.Error("dry_run should be false on real archive run")
	}
}

// TestArchiveJSONOutputDryRunWritesEnvelope: --dry-run --json
// --output writes the SIMULATED envelope to the named file without
// touching the active store or the archive file. The file contents
// match what stdout would have shown for the equivalent
// --dry-run --json call.
func TestArchiveJSONOutputDryRunWritesEnvelope(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	for _, id := range []string{"1", "2"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
	}
	outPath := filepath.Join(dir, "preview.json")
	if _, _, err := runCmd(t, dir, "archive", "--all", "--dry-run", "--json", "--output", outPath); err != nil {
		t.Fatalf("archive --dry-run --json --output: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse JSON: %v\nbody:\n%s", err, body)
	}
	if !doc.DryRun {
		t.Error("dry_run should be true on dry-run path")
	}
	if doc.TotalCount != 2 {
		t.Errorf("total_count: want 2, got %d", doc.TotalCount)
	}
	// Confirm the active store still has both tasks (dry-run
	// wrote nothing).
	ls, _, err := runCmd(t, dir, "ls", "--done")
	if err != nil {
		t.Fatalf("ls --done: %v", err)
	}
	if !strings.Contains(ls, "a") || !strings.Contains(ls, "b") {
		t.Errorf("dry-run shouldn't have removed tasks, ls says:\n%s", ls)
	}
	// Archive file should not exist.
	archPath := filepath.Join(dir, ".tsk.archive.md")
	if _, err := os.Stat(archPath); err == nil {
		t.Errorf("dry-run shouldn't have created the archive file at %s", archPath)
	}
}

// TestArchiveJSONOutputNoTasksWritesEmptyEnvelope: when nothing
// qualifies for archiving, --json --output still writes an envelope
// with archived: [] and total_count: 0. Important so a CI pipeline
// that always reads the file doesn't crash on a "nothing to archive"
// run.
func TestArchiveJSONOutputNoTasksWritesEmptyEnvelope(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "still open"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "empty.json")
	if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath); err != nil {
		t.Fatalf("archive --json --output: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse JSON: %v\nbody:\n%s", err, body)
	}
	if doc.TotalCount != 0 {
		t.Errorf("total_count: want 0, got %d", doc.TotalCount)
	}
	if doc.Archived == nil {
		t.Error("archived should be empty array (not null) on no-tasks path")
	}
	if len(doc.Archived) != 0 {
		t.Errorf("archived: want empty, got %d rows", len(doc.Archived))
	}
}

// TestArchiveJSONOutputRequiresJSON: --output without --json is a
// usage error (exit 2). The flag is exclusively a modifier for the
// JSON envelope path; combining it with the plain-text path would
// be ambiguous (write the human summary to a file? probably a typo).
func TestArchiveJSONOutputRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	outPath := filepath.Join(dir, "run.json")
	_, _, err := runCmd(t, dir, "archive", "--all", "--output", outPath)
	if err == nil {
		t.Fatal("expected error for --output without --json")
	}
	if !strings.Contains(err.Error(), "--output") || !strings.Contains(err.Error(), "--json") {
		t.Fatalf("expected error to mention --output requires --json, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %v", err)
	}
}

// TestArchiveJSONOutputRejectsBadExtension: --json --output with a
// non-.json extension surfaces a clear usage error. Catches the
// silent-footgun case where a user types `--json --output run.svg`
// — without this check, the file would land with JSON bytes under
// a name that breaks every downstream tool inspecting it by
// extension.
func TestArchiveJSONOutputRejectsBadExtension(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	for _, bad := range []string{"run.svg", "run.txt", "run.md", "run"} {
		outPath := filepath.Join(dir, bad)
		_, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath)
		if err == nil {
			t.Fatalf("expected error for --output %q, got nil", bad)
		}
		if !strings.Contains(err.Error(), ".json") {
			t.Fatalf("for %q: expected error to mention .json, got %v", bad, err)
		}
		var ec ExitCoder
		if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
			t.Fatalf("for %q: expected exit code 2, got %v", bad, err)
		}
	}
}

// TestArchiveJSONOutputAcceptsUppercaseExtension: case-insensitive
// extension match so `.JSON` passes (matches the contract every
// other tsk extension-validator uses — capitalization shouldn't
// reject a sensible filename).
func TestArchiveJSONOutputAcceptsUppercaseExtension(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	outPath := filepath.Join(dir, "RUN.JSON")
	if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath); err != nil {
		t.Fatalf("archive --json --output RUN.JSON: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected output file at %s, got: %v", outPath, err)
	}
}

// TestArchiveJSONOutputOverwritesExisting: a second --output run to
// the same path replaces the prior content (truncating write, not
// append). The append semantic is reserved for a future --append
// flag (sister of `tsk graph --json --output --append`); the
// default --output is overwrite, matching the contract `tsk graph
// --json --output` uses.
func TestArchiveJSONOutputOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	outPath := filepath.Join(dir, "run.json")
	// First call: 1 task archived.
	if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath); err != nil {
		t.Fatalf("first archive: %v", err)
	}
	first, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	var firstDoc archiveDoc
	if err := json.Unmarshal(first, &firstDoc); err != nil {
		t.Fatalf("parse first: %v", err)
	}
	if firstDoc.TotalCount != 1 {
		t.Errorf("first run total_count: want 1, got %d", firstDoc.TotalCount)
	}
	// Mark #2 done and archive again. The file should now have a
	// run reflecting THIS call (1 task), NOT a concatenation with
	// the previous (which would be invalid JSON anyway).
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--json", "--output", outPath); err != nil {
		t.Fatalf("second archive: %v", err)
	}
	second, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	var secondDoc archiveDoc
	if err := json.Unmarshal(second, &secondDoc); err != nil {
		t.Fatalf("parse second (should be single JSON doc, not concatenation): %v\nbody:\n%s", err, second)
	}
	if secondDoc.TotalCount != 1 {
		t.Errorf("second run total_count: want 1 (this call's archive), got %d", secondDoc.TotalCount)
	}
}

// TestArchiveJSONOutputContentMatchesStdout: the bytes on disk
// (--json --output) are identical to the bytes stdout would have
// produced (--json alone). Regression test guarding against a
// future change accidentally introducing an extra newline,
// different indent, or modified field order on the file path.
func TestArchiveJSONOutputContentMatchesStdout(t *testing.T) {
	// Use TWO separate dirs so the archive ID space is identical
	// in both runs (a fresh sibling .tsk.archive.md in each).
	dirStdout := t.TempDir()
	dirFile := t.TempDir()
	for _, dir := range []string{dirStdout, dirFile} {
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
	}
	stdout, _, err := runCmd(t, dirStdout, "archive", "--all", "--json")
	if err != nil {
		t.Fatalf("archive --json stdout: %v", err)
	}
	outPath := filepath.Join(dirFile, "run.json")
	if _, _, err := runCmd(t, dirFile, "archive", "--all", "--json", "--output", outPath); err != nil {
		t.Fatalf("archive --json --output: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// The archive_path differs (different tempdir per run), so
	// parse both and compare structure.
	var stdoutDoc, fileDoc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &stdoutDoc); err != nil {
		t.Fatalf("parse stdout: %v", err)
	}
	if err := json.Unmarshal(body, &fileDoc); err != nil {
		t.Fatalf("parse file: %v", err)
	}
	if stdoutDoc.TotalCount != fileDoc.TotalCount {
		t.Errorf("total_count mismatch: stdout=%d, file=%d", stdoutDoc.TotalCount, fileDoc.TotalCount)
	}
	if len(stdoutDoc.Archived) != len(fileDoc.Archived) {
		t.Fatalf("archived rows mismatch: stdout=%d, file=%d", len(stdoutDoc.Archived), len(fileDoc.Archived))
	}
	for i := range stdoutDoc.Archived {
		if stdoutDoc.Archived[i].ActiveID != fileDoc.Archived[i].ActiveID {
			t.Errorf("row %d active_id mismatch: stdout=%d, file=%d", i, stdoutDoc.Archived[i].ActiveID, fileDoc.Archived[i].ActiveID)
		}
		if stdoutDoc.Archived[i].ArchiveID != fileDoc.Archived[i].ArchiveID {
			t.Errorf("row %d archive_id mismatch: stdout=%d, file=%d", i, stdoutDoc.Archived[i].ArchiveID, fileDoc.Archived[i].ArchiveID)
		}
	}
}

// TestValidateArchiveOutputJSONFlagsTable: direct table test of the
// validator helper so future format gains don't break the matrix.
// Sister of the validateGraphOutputJSONExtension table.
func TestValidateArchiveOutputJSONFlagsTable(t *testing.T) {
	cases := []struct {
		path       string
		appendMode bool
		wantErr    bool
		errFrag    string
	}{
		{"", false, false, ""},
		{"", true, false, ""},
		{"run.json", false, false, ""},
		{"RUN.JSON", false, false, ""},
		{"deep/nested/run.json", false, false, ""},
		{"run.jsonl", false, true, ".json"},
		{"run.jsonl", true, false, ""},
		{"run.json", true, false, ""},
		{"run.svg", false, true, ".json"},
		{"run.svg", true, true, ".json or .jsonl"},
		{"run.txt", false, true, ".json"},
		{"run", false, true, ".json"},
	}
	for _, c := range cases {
		err := validateArchiveOutputJSONFlags(c.path, c.appendMode)
		if c.wantErr {
			if err == nil {
				t.Errorf("path=%q append=%v: expected error, got nil", c.path, c.appendMode)
				continue
			}
			if c.errFrag != "" && !strings.Contains(err.Error(), c.errFrag) {
				t.Errorf("path=%q append=%v: expected error containing %q, got %v", c.path, c.appendMode, c.errFrag, err)
			}
			var ec ExitCoder
			if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
				t.Errorf("path=%q append=%v: expected exit code 2, got %v", c.path, c.appendMode, err)
			}
		} else {
			if err != nil {
				t.Errorf("path=%q append=%v: unexpected error: %v", c.path, c.appendMode, err)
			}
		}
	}
}
