package commands

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDiffNoSnapshotFails(t *testing.T) {
	dir := t.TempDir()
	// Create the file but no .bak — diff has nothing to compare.
	mustAdd(t, dir, "thing", "-p", "medium")
	// Remove the .bak that mustAdd's add created.
	// (add doesn't create a .bak because there was no prior file —
	// .bak is only written when overwriting.) So this is naturally
	// the "first save" state already.
	_, _, err := runCmd(t, dir, "diff")
	if err == nil {
		t.Fatal("expected error when .bak missing")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (no snapshot), got %v", err)
	}
}

func TestDiffShowsAddedTask(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "first")
	// Now a second add writes .bak first, then the new file.
	mustAdd(t, dir, "second")
	stdout, _, err := runCmd(t, dir, "diff")
	if err == nil {
		t.Fatal("expected non-nil err (exit 1 on diff present)")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 1 {
		t.Fatalf("expected exit 1 (changes present), got %v", err)
	}
	if !strings.Contains(stdout, "+- [ ] second") {
		t.Fatalf("expected added 'second' task in diff, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--- ") || !strings.Contains(stdout, "+++ ") {
		t.Fatalf("expected unified diff headers, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "@@ ") {
		t.Fatalf("expected hunk header, got:\n%s", stdout)
	}
}

func TestDiffShowsRemovedTask(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "alpha")
	mustAdd(t, dir, "beta")
	// Rm one — that operation writes .bak then a smaller file.
	if _, _, err := runCmd(t, dir, "rm", "1"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "diff")
	if err == nil {
		t.Fatal("expected exit 1 with changes")
	}
	// The removed task line should appear with a leading '-'.
	if !strings.Contains(stdout, "-- [ ] alpha") {
		t.Fatalf("expected removed alpha line, got:\n%s", stdout)
	}
}

func TestDiffNoChangesQuiet(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "alpha")
	mustAdd(t, dir, "beta")
	// Manually copy live to .bak so they're identical, then diff says "no changes".
	livePath := dir + "/.tsk.md"
	bakPath := livePath + ".bak"
	body, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("read live: %v", err)
	}
	if err := os.WriteFile(bakPath, body, 0o644); err != nil {
		t.Fatalf("write bak: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "diff")
	if err != nil {
		t.Fatalf("diff should exit 0 when identical, got: %v", err)
	}
	if !strings.Contains(stdout, "no changes") {
		t.Fatalf("expected 'no changes', got:\n%s", stdout)
	}
}

func TestDiffStat(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "first")
	mustAdd(t, dir, "second")
	stdout, _, err := runCmd(t, dir, "diff", "--stat")
	if !errors.As(err, new(ExitCoder)) {
		t.Fatalf("expected exit-coder error, got: %v", err)
	}
	if !strings.Contains(stdout, ": +") || !strings.Contains(stdout, "-") {
		t.Fatalf("expected stat line, got:\n%s", stdout)
	}
}

func TestDiffNameOnly(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "first")
	mustAdd(t, dir, "second")
	stdout, _, err := runCmd(t, dir, "diff", "--name-only")
	if err == nil {
		t.Fatal("expected exit 1")
	}
	if !strings.Contains(stdout, ".tsk.md") {
		t.Fatalf("expected path in output, got:\n%s", stdout)
	}
	// No diff body should be present.
	if strings.Contains(stdout, "@@ ") || strings.Contains(stdout, "+- [ ]") {
		t.Fatalf("name-only should be path only, got:\n%s", stdout)
	}
}

func TestDiffNegativeContextRejected(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "x")
	mustAdd(t, dir, "y")
	_, _, err := runCmd(t, dir, "diff", "--context", "-1")
	if err == nil {
		t.Fatal("expected error for --context -1")
	}
}

func TestDiffScriptCorrectness(t *testing.T) {
	// Direct test of the internal LCS to lock down the contract.
	before := []string{"a", "b", "c"}
	after := []string{"a", "B", "c"}
	ops := diffScript(before, after)
	// Expect: eq a, del b, add B, eq c. (Order of del/add for a
	// substitution can vary; verify by counting.)
	added, removed := 0, 0
	for _, op := range ops {
		if op.kind == opAdd {
			added++
		}
		if op.kind == opDel {
			removed++
		}
	}
	if added != 1 || removed != 1 {
		t.Fatalf("expected 1 add + 1 del, got %d add %d del", added, removed)
	}
}

func TestDiffSplitLinesNoTrailingEmpty(t *testing.T) {
	if got := splitLines("a\nb\n"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("splitLines: %v", got)
	}
	if got := splitLines(""); got != nil {
		t.Fatalf("empty must be nil, got %v", got)
	}
	if got := splitLines("solo"); len(got) != 1 || got[0] != "solo" {
		t.Fatalf("no trailing newline: %v", got)
	}
}

// (no extra helpers needed — uses os.ReadFile/os.WriteFile directly.)
