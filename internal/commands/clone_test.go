package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneDuplicatesWithSuffix(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "clone", "1")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if !strings.Contains(stdout, "cloned #1 -> #2") {
		t.Fatalf("expected 'cloned #1 -> #2', got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	// Both originals and clone with " (copy)" suffix should appear.
	if !strings.Contains(content, "- [ ] thing <") {
		t.Fatalf("expected original task present, got:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] thing (copy) <") {
		t.Fatalf("expected '(copy)' suffix on clone, got:\n%s", content)
	}
}

func TestCloneInheritsAllFields(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "with everything",
		"-p", "urgent",
		"-t", "dev",
		"-t", "ship",
		"-d", "2099-12-31",
		"-n", "important context",
	); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "clone", "1", "--no-suffix"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	// Use show --json to inspect the clone's fields exactly.
	stdout, _, err := runCmd(t, dir, "show", "2", "--json")
	if err != nil {
		t.Fatalf("show 2: %v", err)
	}
	for _, want := range []string{
		`"Title": "with everything"`,
		`"Done": false`,
		`"Priority": 3`, // PriorityUrgent
		`"dev"`,
		`"ship"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("clone missing %q in show output:\n%s", want, stdout)
		}
	}
	// Notes preserved
	if !strings.Contains(stdout, "important context") {
		t.Fatalf("clone should preserve notes, got:\n%s", stdout)
	}
	// Due date preserved
	if !strings.Contains(stdout, "2099-12-31") {
		t.Fatalf("clone should preserve due date, got:\n%s", stdout)
	}
}

func TestCloneStartsOpenEvenWhenSourceDone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "clone", "1"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	// Source is done, clone is open.
	if !strings.Contains(content, "- [x] thing <") {
		t.Fatalf("expected source marked done, got:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] thing (copy) <") {
		t.Fatalf("expected clone marked open, got:\n%s", content)
	}
	// Clone should not carry a completed: timestamp.
	if strings.Contains(content, "thing (copy)") &&
		strings.Contains(strings.Split(content, "thing (copy)")[1], "completed:") {
		// Tolerate `completed:` later in the file (the source has one),
		// but the (copy) task's own line should not have it. The crude
		// check above can over-trigger if "completed:" appears anywhere
		// after the clone line — narrow it to that single line.
		lines := strings.Split(content, "\n")
		for _, ln := range lines {
			if strings.Contains(ln, "thing (copy)") && strings.Contains(ln, "completed:") {
				t.Fatalf("clone line carries completed:, should not: %s", ln)
			}
		}
	}
}

func TestCloneWithTitleOverride(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "src"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "clone", "1", "--title", "next sprint"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] next sprint <") {
		t.Fatalf("expected override title, got:\n%s", content)
	}
	if strings.Contains(content, "(copy)") {
		t.Fatalf("override should skip the suffix, got:\n%s", content)
	}
}

func TestCloneNoSuffix(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "exact"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "clone", "1", "--no-suffix"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Count(content, "- [ ] exact ") != 2 {
		t.Fatalf("expected two 'exact' tasks, got:\n%s", content)
	}
}

func TestCloneMultipleN(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "weekly"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "clone", "1", "--n", "3")
	if err != nil {
		t.Fatalf("clone n=3: %v", err)
	}
	if !strings.Contains(stdout, "cloned #1 -> #2, #3, #4") {
		t.Fatalf("expected three new IDs reported, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	// Original + 3 clones = 4 weekly tasks.
	if strings.Count(content, "weekly") != 4 {
		t.Fatalf("expected 4 weekly tasks, got:\n%s", content)
	}
}

func TestCloneRejectsTitleAndNoSuffix(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "clone", "1", "--title", "y", "--no-suffix")
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
}

func TestCloneRejectsBadN(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "clone", "1", "--n", "0")
	if err == nil {
		t.Fatal("expected error for --n 0")
	}
}

func TestCloneAlias(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "dupe", "1"); err != nil {
		t.Fatalf("dupe alias: %v", err)
	}
	if _, _, err := runCmd(t, dir, "duplicate", "1"); err != nil {
		t.Fatalf("duplicate alias: %v", err)
	}
}

func TestCloneUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "clone", "999")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestCloneDoesNotShareTagBackingArray(t *testing.T) {
	// Regression guard: appending to one task's tags must not bleed
	// into the clone's tags.
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "src", "-t", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "clone", "1"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	// Now mutate the source's tags; clone should still have just alpha.
	if _, _, err := runCmd(t, dir, "tag", "1", "+beta"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	// Inspect clone (#2) via show --json — its tags should NOT contain beta.
	stdout, _, err := runCmd(t, dir, "show", "2", "--json")
	if err != nil {
		t.Fatalf("show 2: %v", err)
	}
	if strings.Contains(stdout, `"beta"`) {
		t.Fatalf("clone leaked tag from source: %s", stdout)
	}
	if !strings.Contains(stdout, `"alpha"`) {
		t.Fatalf("clone should still have alpha tag: %s", stdout)
	}
}
