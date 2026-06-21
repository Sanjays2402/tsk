package commands

import (
	"strings"
	"testing"
)

// TestLintAutofixAllBackfillsMissingCreated: a task with no
// created: meta has the stamp filled in after --autofix-all,
// and the subsequent `tsk lint` reports zero findings.
func TestLintAutofixAllBackfillsMissingCreated(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] no-created <!-- id:1 prio:medium -->\n")
	// Confirm the precondition: lint reports missing_created_timestamp.
	stdout, _, _ := runCmd(t, dir, "lint")
	if !strings.Contains(stdout, "missing_created_timestamp") {
		t.Fatalf("precondition failed — expected lint to flag missing created:\n%s", stdout)
	}
	// Apply autofix-all.
	out, _, err := runCmd(t, dir, "lint", "--autofix-all")
	if err != nil {
		t.Fatalf("autofix-all: %v", err)
	}
	if !strings.Contains(out, "autofixed:") {
		t.Fatalf("expected 'autofixed:' summary, got:\n%s", out)
	}
	// File now has a created: stamp.
	content := readFile(t, dir+"/.tsk.md")
	if !strings.Contains(content, "created:") {
		t.Fatalf("expected created: stamp after autofix, got:\n%s", content)
	}
	// Re-lint: should report zero findings now.
	clean, _, err := runCmd(t, dir, "lint")
	if err != nil {
		t.Fatalf("post-autofix lint: %v", err)
	}
	if !strings.Contains(clean, "all checks passed") {
		t.Fatalf("expected clean state after autofix, got:\n%s", clean)
	}
}

// TestLintAutofixAllAlsoFixesRoundTrippable: a file with BOTH a
// non-canonical bullet ('*' instead of '-') AND a missing created:
// has BOTH fixed in one --autofix-all pass.
func TestLintAutofixAllAlsoFixesRoundTrippable(t *testing.T) {
	dir := t.TempDir()
	// Mix of issues: id:1 has '*' bullet, id:2 has missing created.
	writeRawFile(t, dir,
		"# tasks\n\n"+
			"* [ ] starred <!-- id:1 prio:medium created:2026-01-01T00:00:00Z -->\n"+
			"- [ ] no-created <!-- id:2 prio:medium -->\n",
	)
	preReport, _, _ := runCmd(t, dir, "lint")
	if !strings.Contains(preReport, "non_canonical_task_line") {
		t.Fatalf("precondition: missing non_canonical finding:\n%s", preReport)
	}
	if !strings.Contains(preReport, "missing_created_timestamp") {
		t.Fatalf("precondition: missing missing_created finding:\n%s", preReport)
	}
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all"); err != nil {
		t.Fatalf("autofix-all: %v", err)
	}
	// File should now be canonical ('-' bullet) and #2 has a created stamp.
	content := readFile(t, dir+"/.tsk.md")
	if strings.Contains(content, "* [ ] starred") {
		t.Fatalf("expected canonical '-' bullet, still saw '*':\n%s", content)
	}
	if !strings.Contains(content, "- [ ] starred") {
		t.Fatalf("expected '- [ ] starred' canonical form, got:\n%s", content)
	}
	// Both tasks should have a created stamp now.
	if strings.Count(content, "created:") != 2 {
		t.Fatalf("expected 2 created: stamps after autofix, got:\n%s", content)
	}
	// Re-lint: clean.
	clean, _, err := runCmd(t, dir, "lint")
	if err != nil {
		t.Fatalf("post-autofix lint: %v", err)
	}
	if !strings.Contains(clean, "all checks passed") {
		t.Fatalf("expected clean state, got:\n%s", clean)
	}
}

// TestLintAutofixAllReportsRepairCount: the summary message
// includes a sensible repair count — one per backfill, plus one
// total for the round-trippable bucket.
func TestLintAutofixAllReportsRepairCount(t *testing.T) {
	dir := t.TempDir()
	// Three tasks all missing created:
	writeRawFile(t, dir,
		"# tasks\n\n"+
			"- [ ] a <!-- id:1 prio:medium -->\n"+
			"- [ ] b <!-- id:2 prio:medium -->\n"+
			"- [ ] c <!-- id:3 prio:medium -->\n",
	)
	out, _, err := runCmd(t, dir, "lint", "--autofix-all")
	if err != nil {
		t.Fatalf("autofix-all: %v", err)
	}
	// 3 backfills, no round-trippable issues here → expect "3 repair(s)".
	if !strings.Contains(out, "3 repair(s)") {
		t.Fatalf("expected '3 repair(s)' summary (3 backfills), got:\n%s", out)
	}
}

// TestLintAutofixAllNoFindingsExitsZero: a clean file with no
// findings doesn't trigger any autofix and exits 0 with the
// normal "all checks passed" message.
func TestLintAutofixAllNoFindingsExitsZero(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "clean"); err != nil {
		t.Fatalf("add: %v", err)
	}
	out, _, err := runCmd(t, dir, "lint", "--autofix-all")
	if err != nil {
		t.Fatalf("autofix-all on clean: %v", err)
	}
	if !strings.Contains(out, "all checks passed") {
		t.Fatalf("expected clean message, got:\n%s", out)
	}
	if strings.Contains(out, "autofixed:") {
		t.Fatalf("clean file should NOT emit autofixed summary, got:\n%s", out)
	}
}

// TestLintAutofixAllCreatesBakSnapshot: the autofix path goes
// through s.Save, which writes a .tsk.md.bak alongside. This
// guards the "tsk undo-last after autofix-all" workflow.
func TestLintAutofixAllCreatesBakSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] no-created <!-- id:1 prio:medium -->\n")
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all"); err != nil {
		t.Fatalf("autofix-all: %v", err)
	}
	bakContent := readFile(t, dir+"/.tsk.md.bak")
	if !strings.Contains(bakContent, "no-created") {
		t.Fatalf(".bak should contain pre-autofix content, got:\n%s", bakContent)
	}
	// Original raw content had NO created: stamp; the .bak must
	// reflect that pre-state.
	if strings.Contains(bakContent, "created:") {
		t.Fatalf(".bak should be the PRE-fix state (no created:), got:\n%s", bakContent)
	}
}

// TestLintAutofixAllIdempotent: running it twice on the same file
// produces no further changes the second time (and reports the
// clean state).
func TestLintAutofixAllIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] x <!-- id:1 prio:medium -->\n")
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all"); err != nil {
		t.Fatalf("first autofix: %v", err)
	}
	after1 := readFile(t, dir+"/.tsk.md")
	// Second run: nothing to do.
	out, _, err := runCmd(t, dir, "lint", "--autofix-all")
	if err != nil {
		t.Fatalf("second autofix: %v", err)
	}
	if !strings.Contains(out, "all checks passed") {
		t.Fatalf("second run should be clean, got:\n%s", out)
	}
	after2 := readFile(t, dir+"/.tsk.md")
	if after1 != after2 {
		t.Fatalf("file changed on second autofix (should be idempotent):\nFIRST:\n%s\nSECOND:\n%s", after1, after2)
	}
}

// TestLintAutofixAllJSONReportShape: --autofix-all + --json
// produces the JSON report THEN applies fixes — the JSON output
// must come first (so consumers can capture pre-fix state).
func TestLintAutofixAllJSONReportShape(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] x <!-- id:1 prio:medium -->\n")
	stdout, _, err := runCmd(t, dir, "lint", "--autofix-all", "--json")
	if err != nil {
		t.Fatalf("autofix-all --json: %v", err)
	}
	// First non-empty content must be the JSON object (it always
	// starts with '{' for LintReport). The "autofixed:" message
	// follows.
	trimmed := strings.TrimLeft(stdout, " \n")
	if !strings.HasPrefix(trimmed, "{") {
		t.Fatalf("expected JSON to come first in --json mode, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "autofixed:") {
		t.Fatalf("autofix summary should still print after JSON, got:\n%s", stdout)
	}
}
