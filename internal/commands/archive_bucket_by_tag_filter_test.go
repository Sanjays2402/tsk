package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveBucketByTagFilterBasic: --bucket-by tag:work groups
// tagged-work tasks under "## tag:work" and everything else under
// "## other", with tag:work appearing first in the output.
func TestArchiveBucketByTagFilterBasic(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		title string
		tag   string
	}{
		{"work-1", "work"},
		{"personal-1", "personal"},
		{"work-2", "work"},
		{"random-1", ""},
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
			t.Fatalf("done %d: %v", i, err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:work"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// Both sections present.
	tagIdx := strings.Index(archive, "## tag:work")
	otherIdx := strings.Index(archive, "## other")
	if tagIdx < 0 {
		t.Fatalf("missing '## tag:work' section:\n%s", archive)
	}
	if otherIdx < 0 {
		t.Fatalf("missing '## other' section:\n%s", archive)
	}
	// tag:work appears BEFORE other (sort key 1 < 2).
	if otherIdx < tagIdx {
		t.Fatalf("expected ## tag:work before ## other, got tag@%d other@%d:\n%s", tagIdx, otherIdx, archive)
	}
	// Find the tag:work section content (up to "## other").
	workSection := archive[tagIdx:otherIdx]
	for _, want := range []string{"work-1", "work-2"} {
		if !strings.Contains(workSection, want) {
			t.Errorf("expected %q under ## tag:work, got:\n%s", want, workSection)
		}
	}
	// "personal-1" and "random-1" should be in the other section.
	otherSection := archive[otherIdx:]
	for _, want := range []string{"personal-1", "random-1"} {
		if !strings.Contains(otherSection, want) {
			t.Errorf("expected %q under ## other, got:\n%s", want, otherSection)
		}
	}
}

// TestArchiveBucketByTagFilterCaseInsensitive: tag matching is
// case-insensitive (same convention as `tsk ls --tag`). Asking
// for "WORK" matches a task tagged "work".
func TestArchiveBucketByTagFilterCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Capitalized filter matches lowercase tag.
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:WORK"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// Section label preserves the user-supplied case ("tag:WORK").
	if !strings.Contains(archive, "## tag:WORK") {
		t.Fatalf("expected '## tag:WORK' (preserves user case), got:\n%s", archive)
	}
	// alpha should still match into tag:WORK despite case difference.
	tagIdx := strings.Index(archive, "## tag:WORK")
	otherIdx := strings.Index(archive, "## other")
	if tagIdx < 0 || otherIdx < 0 {
		t.Fatalf("both sections required:\n%s", archive)
	}
	workSection := archive[tagIdx:otherIdx]
	if !strings.Contains(workSection, "alpha") {
		t.Errorf("alpha (tagged 'work') should be in tag:WORK section:\n%s", workSection)
	}
}

// TestArchiveBucketByTagFilterRejectsEmptyTag: --bucket-by tag:
// (no tag name after colon) surfaces a clear usage error.
func TestArchiveBucketByTagFilterRejectsEmptyTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:")
	if err == nil {
		t.Fatal("expected error for empty tag")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
	if !strings.Contains(err.Error(), "requires a tag name") {
		t.Fatalf("error should explain empty tag, got: %v", err)
	}
}

// TestArchiveBucketByTagFilterAllInTag: when EVERY task in the
// batch carries the filter tag, only the "## tag:X" section
// appears — no empty "## other" section is emitted.
func TestArchiveBucketByTagFilterAllInTag(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		if _, _, err := runCmd(t, dir, "add", "t"+itoa(i+1), "-t", "work"); err != nil {
			t.Fatalf("add: %v", err)
		}
		if _, _, err := runCmd(t, dir, "done", itoa(i+1)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:work"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	if !strings.Contains(archive, "## tag:work") {
		t.Fatalf("expected '## tag:work' section, got:\n%s", archive)
	}
	if strings.Contains(archive, "## other") {
		t.Fatalf("did not expect '## other' when all tasks match tag:\n%s", archive)
	}
}

// TestArchiveBucketByTagFilterSummary: the success summary line
// uses the bucket-by label with the full "tag:X" spec preserved.
func TestArchiveBucketByTagFilterSummary(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "release"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:release")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "bucket-by=tag:release") {
		t.Fatalf("expected 'bucket-by=tag:release' in summary, got:\n%s", stdout)
	}
}
