package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// lintAutofixDocJSON mirrors lintAutofixDoc for test decoding. If
// the production schema changes, this struct must change in lockstep.
type lintAutofixDocJSON struct {
	Path          string `json:"path"`
	FindingsCount int    `json:"findings_count"`
	Findings      []struct {
		Line   int    `json:"line,omitempty"`
		Check  string `json:"check"`
		Detail string `json:"detail"`
		TaskID int    `json:"task_id,omitempty"`
	} `json:"findings"`
	RepairsApplied int    `json:"repairs_applied"`
	BackupDir      string `json:"backup_dir,omitempty"`
}

// TestLintAutofixAllJSONBasic: a file with mixed findings (non-
// canonical bullet AND missing created stamp) emits both the
// findings list AND a positive repairs_applied count in one JSON
// document.
func TestLintAutofixAllJSONBasic(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir,
		"# tasks\n\n"+
			"* [ ] starred <!-- id:1 prio:medium created:2026-01-01T00:00:00Z -->\n"+
			"- [ ] no-created <!-- id:2 prio:medium -->\n",
	)
	stdout, _, err := runCmd(t, dir, "lint", "--autofix-all", "--json")
	if err != nil {
		t.Fatalf("autofix-all --json: %v", err)
	}
	var doc lintAutofixDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("unmarshal: %v\nout:\n%s", err, stdout)
	}
	if doc.FindingsCount < 2 {
		t.Errorf("findings_count: want >=2, got %d", doc.FindingsCount)
	}
	if len(doc.Findings) != doc.FindingsCount {
		t.Errorf("findings array length %d != findings_count %d", len(doc.Findings), doc.FindingsCount)
	}
	if doc.RepairsApplied < 1 {
		t.Errorf("repairs_applied: want >=1, got %d", doc.RepairsApplied)
	}
	// path should point at the scratch .tsk.md.
	if !strings.HasSuffix(doc.Path, ".tsk.md") {
		t.Errorf("path: want suffix .tsk.md, got %q", doc.Path)
	}
	// File should have been written: bullet should be canonical now.
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(content, "* [ ]") {
		t.Errorf("starred bullet should be canonicalized after JSON path, got:\n%s", content)
	}
}

// TestLintAutofixAllJSONEmptyFindings: a clean file passed through
// --autofix-all --json emits findings: [] (empty array, not null)
// and repairs_applied: 0.
func TestLintAutofixAllJSONEmptyFindings(t *testing.T) {
	dir := t.TempDir()
	// Use the normal add path so the file is canonical.
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "lint", "--autofix-all", "--json")
	if err != nil {
		t.Fatalf("autofix-all --json on clean: %v", err)
	}
	// Literal text check — findings must be [] (not null).
	if !strings.Contains(stdout, `"findings": []`) {
		t.Errorf("expected findings: [] for clean file, got:\n%s", stdout)
	}
	var doc lintAutofixDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.FindingsCount != 0 {
		t.Errorf("findings_count: want 0, got %d", doc.FindingsCount)
	}
	if doc.RepairsApplied != 0 {
		t.Errorf("repairs_applied: want 0 (no findings), got %d", doc.RepairsApplied)
	}
}

// TestLintAutofixAllJSONWithBackup: --json + --backup includes
// backup_dir in the envelope so pre-commit hooks can locate the
// rollback snapshot without re-parsing argv.
func TestLintAutofixAllJSONWithBackup(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	writeRawFile(t, dir, "# tasks\n\n* [ ] x <!-- id:1 prio:medium -->\n")
	stdout, _, err := runCmd(t, dir, "lint", "--autofix-all", "--json", "--backup", backupDir)
	if err != nil {
		t.Fatalf("autofix-all --json --backup: %v", err)
	}
	var doc lintAutofixDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("unmarshal: %v\nout:\n%s", err, stdout)
	}
	if doc.BackupDir != backupDir {
		t.Errorf("backup_dir: want %q, got %q", backupDir, doc.BackupDir)
	}
}

// TestLintAutofixAllJSONWithoutBackupOmitsField: --json without
// --backup should NOT include backup_dir in the JSON document
// (omitempty keeps the envelope minimal for the common no-backup
// path).
func TestLintAutofixAllJSONWithoutBackupOmitsField(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n* [ ] x <!-- id:1 prio:medium -->\n")
	stdout, _, err := runCmd(t, dir, "lint", "--autofix-all", "--json")
	if err != nil {
		t.Fatalf("autofix-all --json: %v", err)
	}
	if strings.Contains(stdout, `"backup_dir"`) {
		t.Errorf("backup_dir should be omitted when --backup not set, got:\n%s", stdout)
	}
}

// TestLintAutofixAllJSONNoMixedOutput: the --json envelope is a
// SINGLE coherent document — there should NOT be a stray
// "autofixed: ..." line interleaved with the JSON (which would
// break jq pipelines).
func TestLintAutofixAllJSONNoMixedOutput(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n* [ ] x <!-- id:1 prio:medium -->\n")
	stdout, _, err := runCmd(t, dir, "lint", "--autofix-all", "--json")
	if err != nil {
		t.Fatalf("autofix-all --json: %v", err)
	}
	if strings.Contains(stdout, "autofixed:") {
		t.Errorf("--json mode should NOT also print 'autofixed:' line, got:\n%s", stdout)
	}
	// Should be valid JSON end-to-end.
	var doc lintAutofixDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Errorf("not valid JSON: %v\n%s", err, stdout)
	}
}

// TestLintJSONReadOnlyPathUnchanged: --json WITHOUT --autofix-all
// keeps its existing schema (just findings list). Regression
// guard so consumers of the read-only --json shape don't break
// after the autofix --json extension.
func TestLintJSONReadOnlyPathUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n* [ ] x <!-- id:1 prio:medium -->\n")
	stdout, _, err := runCmd(t, dir, "lint", "--json")
	// --json without --fix returns silentExit{code:1} when there are findings.
	if err == nil {
		t.Fatal("expected silent exit 1 for findings without --fix")
	}
	// The OUTPUT should still be the legacy schema: an object
	// with path + findings, NOT a lintAutofixDoc. Verify by
	// checking that repairs_applied is absent.
	if strings.Contains(stdout, `"repairs_applied"`) {
		t.Errorf("read-only --json should not include repairs_applied, got:\n%s", stdout)
	}
	// findings should still be present in the old shape.
	if !strings.Contains(stdout, `"findings"`) {
		t.Errorf("read-only --json should include findings list, got:\n%s", stdout)
	}
}

// TestLintAutofixAllJSONFindingsArePreFix: the findings array in
// the JSON envelope reflects the PRE-fix state — what the scan
// saw before the repair pass ran. That's the useful CI signal
// ("these were the problems"), not the post-fix empty list.
func TestLintAutofixAllJSONFindingsArePreFix(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir,
		"# tasks\n\n"+
			"* [ ] starred <!-- id:1 prio:medium created:2026-01-01T00:00:00Z -->\n",
	)
	stdout, _, err := runCmd(t, dir, "lint", "--autofix-all", "--json")
	if err != nil {
		t.Fatalf("autofix-all --json: %v", err)
	}
	var doc lintAutofixDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Should include the non_canonical_task_line finding from the
	// pre-fix scan.
	foundNoncanonical := false
	for _, f := range doc.Findings {
		if f.Check == "non_canonical_task_line" {
			foundNoncanonical = true
		}
	}
	if !foundNoncanonical {
		t.Errorf("expected non_canonical_task_line in pre-fix findings, got:\n%+v", doc.Findings)
	}
	// And repairs_applied should reflect that the round-trip ran.
	if doc.RepairsApplied < 1 {
		t.Errorf("repairs_applied should be >=1 after fixing the bullet, got %d", doc.RepairsApplied)
	}
}

// TestLintAutofixAllJSONWritesFile: even when --json suppresses
// the text summary, the actual repair still happens — the file
// is written to disk with the canonical form.
func TestLintAutofixAllJSONWritesFile(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n* [ ] starred <!-- id:1 prio:medium created:2026-01-01T00:00:00Z -->\n")
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all", "--json"); err != nil {
		t.Fatalf("autofix-all --json: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(content, "* [ ]") {
		t.Errorf("file should be canonicalized after --json autofix, still saw '*':\n%s", content)
	}
	if !strings.Contains(content, "- [ ] starred") {
		t.Errorf("expected canonical '- [ ] starred', got:\n%s", content)
	}
}
