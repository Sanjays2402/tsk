package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRawFile is a helper for hand-crafted .tsk.md content used in lint
// tests. Unlike writeRawTasks (in commands_test.go), this writes the
// EXACT bytes provided — no header, no auto-newlines — so we can test
// non-canonical inputs the writer would normalize away.
func writeRawFile(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, ".tsk.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestLintCleanFileReportsNoFindings(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "clean task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "lint")
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	if !strings.Contains(stdout, "all checks passed") {
		t.Fatalf("expected clean report, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "0 finding(s)") {
		t.Fatalf("expected 0 findings count, got:\n%s", stdout)
	}
}

func TestLintFlagsNonCanonicalBullets(t *testing.T) {
	dir := t.TempDir()
	// '*' bullet, 'X' uppercase, leading-space task — all tolerated by
	// the parser but non-canonical.
	writeRawFile(t, dir, "# tasks\n\n* [ ] star bullet <!-- id:1 prio:medium -->\n  - [X] uppercase X <!-- id:2 prio:medium -->\n")
	_, _, err := runCmd(t, dir, "lint")
	if err == nil {
		t.Fatal("expected non-zero exit for non-canonical findings")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %v", err)
	}
}

func TestLintDetectsUnknownMetaKey(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] thing <!-- id:1 prio:medium foo:bar -->\n")
	stdout, _, _ := runCmd(t, dir, "lint")
	if !strings.Contains(stdout, "unknown_meta_key") {
		t.Fatalf("expected unknown_meta_key, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "\"foo\"") {
		t.Fatalf("expected key name surfaced, got:\n%s", stdout)
	}
}

func TestLintKnownAliasMetaKeysOK(t *testing.T) {
	dir := t.TempDir()
	// "priority" (alias for "prio") and "pinned" (alias for "pin")
	// should NOT trip the unknown_meta_key check.
	writeRawFile(t, dir, "# tasks\n\n- [ ] thing <!-- id:1 priority:high pinned:true -->\n")
	stdout, _, _ := runCmd(t, dir, "lint")
	if strings.Contains(stdout, "unknown_meta_key") {
		t.Fatalf("aliases priority/pinned should not be flagged, got:\n%s", stdout)
	}
}

func TestLintDetectsMissingCreatedTimestamp(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] no-created <!-- id:1 prio:medium -->\n")
	stdout, _, _ := runCmd(t, dir, "lint")
	if !strings.Contains(stdout, "missing_created_timestamp") {
		t.Fatalf("expected missing_created_timestamp, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#1") {
		t.Fatalf("expected task id reference, got:\n%s", stdout)
	}
}

func TestLintDetectsStrayNotesBeforeTask(t *testing.T) {
	dir := t.TempDir()
	// 6+ space indent BEFORE any task. The parser would dump this into
	// s.Header and lose it on re-save.
	writeRawFile(t, dir, "# tasks\n      this is stray notes-shaped content\n\n- [ ] real task <!-- id:1 prio:medium -->\n")
	stdout, _, _ := runCmd(t, dir, "lint")
	if !strings.Contains(stdout, "stray_notes_before_task") {
		t.Fatalf("expected stray_notes_before_task, got:\n%s", stdout)
	}
}

func TestLintFixCanonicalizes(t *testing.T) {
	dir := t.TempDir()
	// Include created: so the only finding is the non-canonical bullet,
	// and we can verify --fix produces a fully clean file.
	writeRawFile(t, dir, "# tasks\n\n* [ ] star <!-- id:1 prio:medium created:2026-01-01T12:00:00-08:00 -->\n")
	// First lint exits 1 (findings present).
	if _, _, err := runCmd(t, dir, "lint"); err == nil {
		t.Fatal("expected non-zero exit before fix")
	}
	// --fix should round-trip; exit 0; canonicalize on disk.
	stdout, _, err := runCmd(t, dir, "lint", "--fix")
	if err != nil {
		t.Fatalf("lint --fix: %v", err)
	}
	if !strings.Contains(stdout, "re-rendered") {
		t.Fatalf("expected --fix confirmation, got:\n%s", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] star") {
		t.Fatalf("expected canonical bullet after fix, got:\n%s", content)
	}
	if strings.Contains(content, "* [ ] star") {
		t.Fatalf("'*' bullet should be gone after fix, got:\n%s", content)
	}
	// Subsequent lint is clean.
	stdout2, _, err := runCmd(t, dir, "lint")
	if err != nil {
		t.Fatalf("lint after fix: %v", err)
	}
	if !strings.Contains(stdout2, "all checks passed") {
		t.Fatalf("expected clean after fix, got:\n%s", stdout2)
	}
}

func TestLintFixDropsUnknownMetaKey(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] thing <!-- id:1 prio:medium foo:bar -->\n")
	if _, _, err := runCmd(t, dir, "lint", "--fix"); err != nil {
		t.Fatalf("lint --fix: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(content, "foo:bar") {
		t.Fatalf("--fix should have dropped the unknown meta, got:\n%s", content)
	}
}

func TestLintJSONShape(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n* [ ] star <!-- id:1 prio:medium -->\n")
	stdout, _, _ := runCmd(t, dir, "lint", "--json")
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if _, ok := doc["path"].(string); !ok {
		t.Fatalf("path should be string, got %T", doc["path"])
	}
	findings, ok := doc["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("expected non-empty findings array, got %v", doc["findings"])
	}
}

func TestLintJSONEmptyFindings(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "lint", "--json")
	if err != nil {
		t.Fatalf("lint --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	// findings must be [] (not null) on a clean file.
	findings, ok := doc["findings"].([]any)
	if !ok {
		t.Fatalf("findings should be array even when empty, got %T", doc["findings"])
	}
	if len(findings) != 0 {
		t.Fatalf("expected empty findings array, got %d", len(findings))
	}
}

func TestLintFindingsAreSortedByLine(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] first <!-- id:1 prio:medium foo:bar -->\n* [ ] second <!-- id:2 prio:medium -->\n+ [ ] third <!-- id:3 prio:medium baz:qux -->\n")
	stdout, _, _ := runCmd(t, dir, "lint")
	// Pull the line:N numbers in order and assert ascending.
	var ords []int
	for _, line := range strings.Split(stdout, "\n") {
		idx := strings.Index(line, "line ")
		if idx < 0 {
			continue
		}
		var n int
		_, err := fmtSscanLine(line[idx:], &n)
		if err == nil {
			ords = append(ords, n)
		}
	}
	if len(ords) < 2 {
		t.Fatalf("expected multiple line-numbered findings, got:\n%s", stdout)
	}
	for i := 1; i < len(ords); i++ {
		if ords[i] < ords[i-1] {
			t.Fatalf("findings not sorted ascending: %v", ords)
		}
	}
}

// fmtSscanLine is a tiny helper: parses "line N  ..." into N. Local so
// the test file doesn't acquire a real "fmt" import just for Sscanf.
func fmtSscanLine(s string, out *int) (int, error) {
	// strip "line " prefix
	const prefix = "line "
	if !strings.HasPrefix(s, prefix) {
		return 0, errInvalid
	}
	rest := s[len(prefix):]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, errInvalid
	}
	n := 0
	for _, r := range rest[:end] {
		n = n*10 + int(r-'0')
	}
	*out = n
	return n, nil
}

var errInvalid = &lintTestErr{"invalid"}

type lintTestErr struct{ m string }

func (e *lintTestErr) Error() string { return e.m }
