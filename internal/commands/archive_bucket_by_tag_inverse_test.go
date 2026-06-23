package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveBucketByTagInverseSingleTag: `--bucket-by tag:!X`
// flips the predicate — tasks NOT tagged X land in the call-out
// bucket ("## tag:!X"), tasks tagged X land in "## other".
// Sister of tag:X with inverted semantics.
func TestArchiveBucketByTagInverseSingleTag(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		title string
		tag   string
	}{
		{"release-1", "release"},
		{"normal-1", "normal"},
		{"normal-2", ""},
		{"release-2", "release"},
		{"misc", "wip"},
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
	for i := 1; i <= 5; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done %d: %v", i, err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:!release"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	inverseIdx := strings.Index(archive, "## tag:!release")
	otherIdx := strings.Index(archive, "## other")
	if inverseIdx < 0 {
		t.Fatalf("missing '## tag:!release' section:\n%s", archive)
	}
	if otherIdx < 0 {
		t.Fatalf("missing '## other' section:\n%s", archive)
	}
	if otherIdx < inverseIdx {
		t.Fatalf("expected '## tag:!release' BEFORE '## other', got inv@%d other@%d", inverseIdx, otherIdx)
	}
	// The non-release tasks (normal-1, normal-2, misc) should be
	// in the call-out bucket; the release tasks in "other".
	inverseSection := archive[inverseIdx:otherIdx]
	for _, want := range []string{"normal-1", "normal-2", "misc"} {
		if !strings.Contains(inverseSection, want) {
			t.Errorf("expected %q in inverse section, got:\n%s", want, inverseSection)
		}
	}
	for _, unwanted := range []string{"release-1", "release-2"} {
		if strings.Contains(inverseSection, unwanted) {
			t.Errorf("did not expect %q in inverse section, got:\n%s", unwanted, inverseSection)
		}
	}
	otherSection := archive[otherIdx:]
	for _, want := range []string{"release-1", "release-2"} {
		if !strings.Contains(otherSection, want) {
			t.Errorf("expected %q in '## other', got:\n%s", want, otherSection)
		}
	}
}

// TestArchiveBucketByTagInverseCSV: `--bucket-by tag:!a,!b`
// extends the inverse to multi-tag CSV — tasks NOT tagged ANY of
// a/b land in the call-out bucket. Mirrors the positive CSV form
// (tag:a,b matches any-of) inverted.
func TestArchiveBucketByTagInverseCSV(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a-task", "-t", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b-task", "-t", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "c-task", "-t", "gamma"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "untagged"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for i := 1; i <= 4; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:!alpha,!beta"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	inverseIdx := strings.Index(archive, "## tag:!alpha,!beta")
	otherIdx := strings.Index(archive, "## other")
	if inverseIdx < 0 {
		t.Fatalf("missing '## tag:!alpha,!beta' section:\n%s", archive)
	}
	if otherIdx < 0 {
		t.Fatalf("missing '## other' section:\n%s", archive)
	}
	inverseSection := archive[inverseIdx:otherIdx]
	// c-task (gamma) and untagged should land in inverse bucket;
	// a-task (alpha) and b-task (beta) should land in other.
	for _, want := range []string{"c-task", "untagged"} {
		if !strings.Contains(inverseSection, want) {
			t.Errorf("expected %q in inverse section, got:\n%s", want, inverseSection)
		}
	}
	otherSection := archive[otherIdx:]
	for _, want := range []string{"a-task", "b-task"} {
		if !strings.Contains(otherSection, want) {
			t.Errorf("expected %q in '## other', got:\n%s", want, otherSection)
		}
	}
}

// TestArchiveBucketByTagInverseRejectsMixedSenses: "tag:!a,b" is
// a mixed inversion sense — half inverted, half not. We reject
// rather than silently picking one direction. The user must
// commit to one sense.
func TestArchiveBucketByTagInverseRejectsMixedSenses(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:!a,b")
	if err == nil {
		t.Fatal("expected error for mixed inversion CSV")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
	if !strings.Contains(err.Error(), "inversion sense") {
		t.Fatalf("expected 'inversion sense' in error, got: %v", err)
	}
}

// TestArchiveBucketByTagInverseRejectsBangAlone: "tag:!" (no tag
// name after the bang) is a usage error. The bang must precede an
// actual tag name; an empty inverse predicate is meaningless.
func TestArchiveBucketByTagInverseRejectsBangAlone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:!")
	if err == nil {
		t.Fatal("expected error for tag:!")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestArchiveBucketByTagInversePreservesCase: the inverse label
// keeps the user's case ("tag:!Release") for the bucket header but
// matches case-insensitively against task tags — same as the
// positive form's contract.
func TestArchiveBucketByTagInversePreservesCase(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "release"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "y", "-t", "other"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	// User filter PascalCase; task tags lowercase. Label should
	// preserve PascalCase; match should still be case-insensitive.
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:!Release"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	if !strings.Contains(archive, "## tag:!Release") {
		t.Errorf("expected '## tag:!Release' label, got:\n%s", archive)
	}
	inverseIdx := strings.Index(archive, "## tag:!Release")
	otherIdx := strings.Index(archive, "## other")
	if inverseIdx < 0 || otherIdx < 0 {
		t.Fatalf("section indices: inv=%d other=%d in:\n%s", inverseIdx, otherIdx, archive)
	}
	// y (other-tagged) should be in inverse; x (release-tagged)
	// in "other" — case-insensitive match did the right thing.
	inverseSection := archive[inverseIdx:otherIdx]
	if !strings.Contains(inverseSection, "y") {
		t.Errorf("expected 'y' (non-release) in inverse, got:\n%s", inverseSection)
	}
	otherSection := archive[otherIdx:]
	if !strings.Contains(otherSection, "x") {
		t.Errorf("expected 'x' (release) in '## other', got:\n%s", otherSection)
	}
}

// TestArchiveBucketByTagInverseBackwardCompatPositive: a plain
// "tag:X" (no bang) still uses the union path with no surprise —
// the new inversion-aware factory must be backward-compatible.
// Regression guard so the inversion plumbing doesn't subtly
// change the positive partition's behavior.
func TestArchiveBucketByTagInverseBackwardCompatPositive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "y"); err != nil {
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
	// Positive form: x in "## tag:work", y in "## other". No
	// "!" anywhere.
	if !strings.Contains(archive, "## tag:work") {
		t.Errorf("expected '## tag:work', got:\n%s", archive)
	}
	if strings.Contains(archive, "## tag:!") {
		t.Errorf("did NOT expect any '## tag:!' label in positive form, got:\n%s", archive)
	}
}

// TestArchiveBucketByTagInverseSummaryLabel: the success-summary
// line carries "bucket-by=tag:!X" so scripts watching the output
// can distinguish the inverse variant from the positive one.
func TestArchiveBucketByTagInverseSummaryLabel(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:!x")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "bucket-by=tag:!x") {
		t.Fatalf("expected 'bucket-by=tag:!x' in summary, got:\n%s", stdout)
	}
}
