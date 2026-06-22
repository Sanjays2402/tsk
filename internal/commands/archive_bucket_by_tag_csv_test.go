package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveBucketByTagFilterCSVUnion: --bucket-by tag:a,b groups
// any task tagged a OR b under the "## tag:a,b" section. Mirrors
// the union semantics of `tsk graph --highlight-tag a,b`.
func TestArchiveBucketByTagFilterCSVUnion(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		title string
		tag   string
	}{
		{"release-task", "release"},
		{"p0-task", "p0"},
		{"both-task", "release"}, // also tagged p0 below
		{"normal-task", "normal"},
		{"untagged", ""},
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
	// Make the third task (id 3 "both-task") also carry p0.
	if _, _, err := runCmd(t, dir, "tag", "3", "+p0"); err != nil {
		t.Fatalf("tag both-task with p0: %v", err)
	}
	for i := 1; i <= 5; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done %d: %v", i, err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:release,p0"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	tagIdx := strings.Index(archive, "## tag:release,p0")
	otherIdx := strings.Index(archive, "## other")
	if tagIdx < 0 {
		t.Fatalf("missing '## tag:release,p0' section:\n%s", archive)
	}
	if otherIdx < 0 {
		t.Fatalf("missing '## other' section:\n%s", archive)
	}
	if otherIdx < tagIdx {
		t.Fatalf("expected '## tag:release,p0' before '## other', got tag@%d other@%d", tagIdx, otherIdx)
	}
	matchSection := archive[tagIdx:otherIdx]
	for _, want := range []string{"release-task", "p0-task", "both-task"} {
		if !strings.Contains(matchSection, want) {
			t.Errorf("expected %q under '## tag:release,p0', got:\n%s", want, matchSection)
		}
	}
	otherSection := archive[otherIdx:]
	for _, want := range []string{"normal-task", "untagged"} {
		if !strings.Contains(otherSection, want) {
			t.Errorf("expected %q under '## other', got:\n%s", want, otherSection)
		}
	}
}

// TestArchiveBucketByTagFilterCSVPreservesCase: the bucket label
// keeps the user-supplied tag names exactly (case + order). Useful
// when the archive holds rollups from different filters.
func TestArchiveBucketByTagFilterCSVPreservesCase(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "release"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-t", "p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:Release,P0"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	if !strings.Contains(archive, "## tag:Release,P0") {
		t.Errorf("expected label '## tag:Release,P0' preserving case + order, got:\n%s", archive)
	}
	// Despite the case mismatch in the user filter (lowercase task
	// tags vs. PascalCase filter), the union should STILL match —
	// matching is case-insensitive even though the label is
	// case-preserving.
	tagIdx := strings.Index(archive, "## tag:Release,P0")
	if tagIdx < 0 {
		t.Fatal("section missing")
	}
	end := strings.Index(archive[tagIdx+1:], "## ")
	matchSection := archive[tagIdx:]
	if end > 0 {
		matchSection = archive[tagIdx : tagIdx+1+end]
	}
	for _, want := range []string{"a", "b"} {
		if !strings.Contains(matchSection, want) {
			t.Errorf("case-insensitive union should match %q, got section:\n%s", want, matchSection)
		}
	}
}

// TestArchiveBucketByTagFilterCSVTolerantOfWhitespace: spaces and
// double-commas in the CSV are forgiven — "tag:a , , b" parses as
// {a, b}.
func TestArchiveBucketByTagFilterCSVTolerantOfWhitespace(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "y", "-t", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "z"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	// Note: the CLI itself trims leading whitespace from the flag
	// value before splitting, so we test the internal whitespace.
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:a, ,b"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// The label keeps the user's CSV (with the empty token dropped
	// to whitespace) — implementation detail. The important
	// behavior is that x AND y match while z doesn't.
	tagIdx := strings.Index(archive, "## tag:a,")
	otherIdx := strings.Index(archive, "## other")
	if tagIdx < 0 {
		t.Fatalf("missing tag-filter section, got:\n%s", archive)
	}
	if otherIdx < 0 {
		t.Fatalf("missing '## other' section, got:\n%s", archive)
	}
	matchSection := archive[tagIdx:otherIdx]
	for _, want := range []string{"x", "y"} {
		if !strings.Contains(matchSection, want) {
			t.Errorf("expected %q under match section, got:\n%s", want, matchSection)
		}
	}
	otherSection := archive[otherIdx:]
	if !strings.Contains(otherSection, "z") {
		t.Errorf("expected 'z' under '## other', got:\n%s", otherSection)
	}
}

// TestArchiveBucketByTagFilterCSVRejectsAllEmpty: "tag:,," (only
// empty tokens) is a usage error — same shape as the empty-tag
// single-form rejection.
func TestArchiveBucketByTagFilterCSVRejectsAllEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:,,")
	if err == nil {
		t.Fatal("expected error for all-empty CSV")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
}

// TestArchiveBucketByTagFilterCSVSummaryLabel: the success-summary
// line carries the full CSV so the user sees which set was filtered.
func TestArchiveBucketByTagFilterCSVSummaryLabel(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "release"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:release,p0")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "bucket-by=tag:release,p0") {
		t.Fatalf("expected 'bucket-by=tag:release,p0' in summary, got:\n%s", stdout)
	}
}

// TestArchiveBucketByTagFilterCSVSingleTagBackwardCompat: a single-
// tag spec ("tag:work") through the new CSV path still produces
// "## tag:work" (not "## tag:work," or any other CSV-shaped label)
// so existing scripts/recipes don't break.
func TestArchiveBucketByTagFilterCSVSingleTagBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:work"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	if !strings.Contains(archive, "## tag:work") {
		t.Errorf("single-tag form should still produce '## tag:work', got:\n%s", archive)
	}
	// And specifically NOT "## tag:work," or similar.
	if strings.Contains(archive, "## tag:work,") {
		t.Errorf("single-tag form should not emit trailing comma, got:\n%s", archive)
	}
}
