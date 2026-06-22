package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveBucketByPriority: archiving tasks with mixed priorities
// produces sections by priority, ordered urgent → high → medium → low.
func TestArchiveBucketByPriority(t *testing.T) {
	dir := t.TempDir()
	// Add tasks with different priorities.
	cases := []struct {
		title string
		prio  string
	}{
		{"low task", "low"},
		{"high task", "high"},
		{"urgent task", "urgent"},
		{"medium task", "medium"},
	}
	for _, c := range cases {
		if _, _, err := runCmd(t, dir, "add", c.title, "-p", c.prio); err != nil {
			t.Fatalf("add %q: %v", c.title, err)
		}
	}
	// Mark all done.
	for i := 1; i <= 4; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done %d: %v", i, err)
		}
	}
	out, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "priority")
	if err != nil {
		t.Fatalf("archive: %v\n%s", err, out)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// Sections should be present in the right order.
	wantOrder := []string{"## urgent", "## high", "## medium", "## low"}
	prev := -1
	for _, w := range wantOrder {
		idx := strings.Index(archive, w)
		if idx < 0 {
			t.Fatalf("missing %q section, archive:\n%s", w, archive)
		}
		if idx <= prev {
			t.Errorf("%q section at offset %d but expected after offset %d:\n%s", w, idx, prev, archive)
		}
		prev = idx
	}
	// Each task should land in its own priority section.
	for _, c := range cases {
		if !strings.Contains(archive, c.title) {
			t.Errorf("missing task %q in archive:\n%s", c.title, archive)
		}
	}
}

// TestArchiveBucketByTag: archiving tasks with various tags produces
// sections keyed off the first tag, plus an "untagged" section.
func TestArchiveBucketByTag(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		title string
		tag   string
	}{
		{"work-thing", "work"},
		{"personal-thing", "personal"},
		{"no-tag-thing", ""},
		{"work-again", "work"},
	}
	for _, c := range cases {
		args := []string{"add", c.title}
		if c.tag != "" {
			args = append(args, "-t", c.tag)
		}
		if _, _, err := runCmd(t, dir, args...); err != nil {
			t.Fatalf("add %q: %v", c.title, err)
		}
	}
	for i := 1; i <= 4; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	for _, want := range []string{"## work", "## personal", "## untagged"} {
		if !strings.Contains(archive, want) {
			t.Errorf("missing section %q in archive:\n%s", want, archive)
		}
	}
	// Both work tasks should be under the same section (one bucket).
	workIdx := strings.Index(archive, "## work")
	if workIdx < 0 {
		t.Fatal("no ## work section")
	}
	// Find the NEXT section after ## work.
	rest := archive[workIdx+len("## work"):]
	nextSection := strings.Index(rest, "\n## ")
	var workSection string
	if nextSection < 0 {
		workSection = rest
	} else {
		workSection = rest[:nextSection]
	}
	for _, want := range []string{"work-thing", "work-again"} {
		if !strings.Contains(workSection, want) {
			t.Errorf("missing %q under ## work, section was:\n%s", want, workSection)
		}
	}
}

// TestArchiveBucketByUnknownKeyRejected: a typo in --bucket-by is
// rejected at exit 2 with a help message listing the supported keys.
func TestArchiveBucketByUnknownKeyRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "made-up-key")
	if err == nil {
		t.Fatal("expected error for unknown --bucket-by key")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
}

// TestArchiveBucketByMutuallyExclusiveWithStrategy: --bucket-by + a
// non-default --strategy is rejected — the two define different
// bucket axes and combining would muddle the layout.
func TestArchiveBucketByMutuallyExclusiveWithStrategy(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "priority", "--strategy", "weekly")
	if err == nil {
		t.Fatal("expected mutex error")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestArchiveBucketByDryRunShowsLabel: --dry-run with --bucket-by
// prints a "bucket-by=<key>" summary line so the user knows which
// axis the preview would use.
func TestArchiveBucketByDryRunShowsLabel(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "priority", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(stdout, "bucket-by=priority") {
		t.Errorf("expected 'bucket-by=priority' summary, got:\n%s", stdout)
	}
	// File should NOT be written.
	if _, err := os.Stat(filepath.Join(dir, ".tsk.archive.md")); err == nil {
		t.Errorf("dry-run should not create archive file")
	}
}

// TestArchiveBucketByPriorityAliasPrio: 'prio' is a synonym for
// 'priority' (matching ParsePriority's flexibility).
func TestArchiveBucketByPriorityAliasPrio(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "prio"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	if !strings.Contains(archive, "## urgent") {
		t.Errorf("expected ## urgent section, got:\n%s", archive)
	}
}

// TestArchiveBucketByTagAliasTags: 'tags' is a synonym for 'tag'.
func TestArchiveBucketByTagAliasTags(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tags"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	if !strings.Contains(archive, "## alpha") {
		t.Errorf("expected ## alpha section, got:\n%s", archive)
	}
}

// TestArchiveBucketByMergeIntoComposes: --bucket-by works with
// --merge-into the same way as --strategy does (orthogonal flags).
func TestArchiveBucketByMergeIntoComposes(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "team.archive.md")
	if _, _, err := runCmd(t, dir, "add", "ship-it", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "priority", "--merge-into", target); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, target)
	if !strings.Contains(archive, "## high") {
		t.Errorf("expected ## high in merged archive, got:\n%s", archive)
	}
	// Default sibling should NOT exist (merge-into took the write).
	if _, err := os.Stat(filepath.Join(dir, ".tsk.archive.md")); err == nil {
		t.Errorf("default sibling archive should NOT exist with --merge-into")
	}
}

// TestArchiveBucketByMultipleTasksSameTagGrouped: 3 tasks all tagged
// 'release' fall into one ## release section, not three separate ones.
func TestArchiveBucketByMultipleTasksSameTagGrouped(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 3; i++ {
		if _, _, err := runCmd(t, dir, "add", "task"+itoa(i), "-t", "release"); err != nil {
			t.Fatalf("add: %v", err)
		}
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	count := strings.Count(archive, "## release")
	if count != 1 {
		t.Errorf("expected exactly 1 '## release' section, got %d in:\n%s", count, archive)
	}
	// All 3 tasks should be present.
	for i := 1; i <= 3; i++ {
		want := "task" + itoa(i)
		if !strings.Contains(archive, want) {
			t.Errorf("missing %q in archive:\n%s", want, archive)
		}
	}
}

// TestArchiveDefaultBehaviorUnchanged: regression — without
// --bucket-by, the default flat strategy is unchanged (no sections).
func TestArchiveDefaultBehaviorUnchanged(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	if strings.Contains(archive, "## urgent") || strings.Contains(archive, "## untagged") {
		t.Errorf("default archive should NOT have bucket sections, got:\n%s", archive)
	}
}
