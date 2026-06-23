package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveBucketByTagStrictAndIntersection: with --strict-and,
// only tasks carrying ALL listed tags land in the call-out bucket
// (intersection). Without it, ANY listed tag suffices (default
// union). Same input — different membership outcome.
func TestArchiveBucketByTagStrictAndIntersection(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha-only", "-t", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta-only", "-t", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "both-tagged", "-t", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Make #3 also tagged "beta" so it carries BOTH.
	if _, _, err := runCmd(t, dir, "tag", "3", "+beta"); err != nil {
		t.Fatalf("tag both: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "neither"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for i := 1; i <= 4; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:alpha,beta", "--strict-and"); err != nil {
		t.Fatalf("archive --strict-and: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// Label should be "## tag:&alpha,beta" — the & marker
	// distinguishes intersection from union sections.
	inIdx := strings.Index(archive, "## tag:&alpha,beta")
	otherIdx := strings.Index(archive, "## other")
	if inIdx < 0 {
		t.Fatalf("missing '## tag:&alpha,beta' section, got:\n%s", archive)
	}
	if otherIdx < 0 {
		t.Fatalf("missing '## other' section, got:\n%s", archive)
	}
	intersectSection := archive[inIdx:otherIdx]
	// Only both-tagged should match intersection.
	if !strings.Contains(intersectSection, "both-tagged") {
		t.Errorf("expected 'both-tagged' in intersection section, got:\n%s", intersectSection)
	}
	for _, unwanted := range []string{"alpha-only", "beta-only", "neither"} {
		if strings.Contains(intersectSection, unwanted) {
			t.Errorf("did NOT expect %q in intersection (it doesn't carry BOTH tags), got:\n%s", unwanted, intersectSection)
		}
	}
	// The other bucket should have everything else.
	otherSection := archive[otherIdx:]
	for _, want := range []string{"alpha-only", "beta-only", "neither"} {
		if !strings.Contains(otherSection, want) {
			t.Errorf("expected %q in '## other', got:\n%s", want, otherSection)
		}
	}
}

// TestArchiveBucketByTagStrictAndInverseIntersection: combines the
// inverse predicate with --strict-and. Tasks NOT carrying ALL
// listed tags land in the call-out bucket. Label gains the
// "tag:!&" prefix marking inverse + intersection.
func TestArchiveBucketByTagStrictAndInverseIntersection(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "has-both", "-t", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// #1 needs both a and b.
	if _, _, err := runCmd(t, dir, "tag", "1", "+b"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "only-a", "-t", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "neither"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:!a,!b", "--strict-and"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// Inverse + intersection label: "## tag:!&a,b"
	if !strings.Contains(archive, "## tag:!&a,b") {
		t.Fatalf("expected '## tag:!&a,b' label, got:\n%s", archive)
	}
	inIdx := strings.Index(archive, "## tag:!&a,b")
	otherIdx := strings.Index(archive, "## other")
	if inIdx < 0 || otherIdx < 0 {
		t.Fatalf("section indices: in=%d other=%d in:\n%s", inIdx, otherIdx, archive)
	}
	inSection := archive[inIdx:otherIdx]
	// Inverse-intersection = NOT (carries BOTH a AND b).
	//   has-both: matched -> NOT matched -> false -> other
	//   only-a:   not matched -> NOT matched -> true -> in
	//   neither:  not matched -> NOT matched -> true -> in
	for _, want := range []string{"only-a", "neither"} {
		if !strings.Contains(inSection, want) {
			t.Errorf("expected %q in inverse-intersection, got:\n%s", want, inSection)
		}
	}
	otherSection := archive[otherIdx:]
	if !strings.Contains(otherSection, "has-both") {
		t.Errorf("expected 'has-both' in '## other' (it matches AND so flips to other), got:\n%s", otherSection)
	}
}

// TestArchiveBucketByTagStrictAndRejectsWithoutBucketBy: --strict-and
// without --bucket-by has no meaningful semantic — surface a usage
// error rather than silently ignoring the flag.
func TestArchiveBucketByTagStrictAndRejectsWithoutBucketBy(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--strict-and")
	if err == nil {
		t.Fatal("expected error for --strict-and without --bucket-by")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "--strict-and requires --bucket-by") {
		t.Fatalf("expected requires-bucket-by error, got: %v", err)
	}
}

// TestArchiveBucketByTagStrictAndRejectsWithPriority: --strict-and
// with --bucket-by priority (or any non-tag axis) is meaningless —
// the union/intersection distinction is a TAG concept. Reject
// loudly so users don't expect strict-and to influence non-tag
// bucketing.
func TestArchiveBucketByTagStrictAndRejectsWithPriority(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "priority", "--strict-and")
	if err == nil {
		t.Fatal("expected error for --strict-and with non-tag bucket-by")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "only applies to --bucket-by tag") {
		t.Fatalf("expected non-tag-axis error, got: %v", err)
	}
}

// TestArchiveBucketByTagStrictAndSingleTagNoLabelChange: --strict-and
// on a single-tag input has no effect (one tag has no union/
// intersection distinction). Label should NOT gain the "&" marker
// — that'd be label noise. Behavior matches positive single-tag.
func TestArchiveBucketByTagStrictAndSingleTagNoLabelChange(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "alpha"); err != nil {
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
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:alpha", "--strict-and"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// Single-tag label: plain "## tag:alpha" — no "&" decoration.
	if !strings.Contains(archive, "## tag:alpha") {
		t.Errorf("expected '## tag:alpha' label, got:\n%s", archive)
	}
	if strings.Contains(archive, "tag:&alpha") {
		t.Errorf("did NOT expect '&' marker on single-tag form, got:\n%s", archive)
	}
}

// TestArchiveBucketByTagStrictAndUnionBackwardCompat: omitting
// --strict-and on a CSV-tag bucket keeps the historical union
// behavior — regression guard so the new intersection plumbing
// doesn't subtly change the default.
func TestArchiveBucketByTagStrictAndUnionBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "only-a", "-t", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "only-b", "-t", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "neither"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if _, _, err := runCmd(t, dir, "done", itoa(i)); err != nil {
			t.Fatalf("done: %v", err)
		}
	}
	// No --strict-and: default union — only-a OR only-b match.
	if _, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:a,b"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// Label is "## tag:a,b" — no "&" marker.
	if !strings.Contains(archive, "## tag:a,b") {
		t.Errorf("expected '## tag:a,b' label (union), got:\n%s", archive)
	}
	if strings.Contains(archive, "tag:&") {
		t.Errorf("did NOT expect '&' marker (default is union), got:\n%s", archive)
	}
	tagIdx := strings.Index(archive, "## tag:a,b")
	otherIdx := strings.Index(archive, "## other")
	if tagIdx < 0 || otherIdx < 0 {
		t.Fatalf("section indices: tag=%d other=%d in:\n%s", tagIdx, otherIdx, archive)
	}
	tagSection := archive[tagIdx:otherIdx]
	for _, want := range []string{"only-a", "only-b"} {
		if !strings.Contains(tagSection, want) {
			t.Errorf("expected %q in union section, got:\n%s", want, tagSection)
		}
	}
	if !strings.Contains(archive[otherIdx:], "neither") {
		t.Errorf("expected 'neither' in '## other', got:\n%s", archive[otherIdx:])
	}
}

// TestArchiveBucketByTagStrictAndSummaryLabel: the dry-run + success
// summary lines carry "(strict-and)" so scripts watching the
// output can tell intersection from union without parsing flags.
func TestArchiveBucketByTagStrictAndSummaryLabel(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "tag", "1", "+b"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:a,b", "--strict-and")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "(strict-and)") {
		t.Errorf("expected '(strict-and)' in summary, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "bucket-by=tag:a,b") {
		t.Errorf("expected 'bucket-by=tag:a,b' in summary, got:\n%s", stdout)
	}
}

// TestArchiveBucketByTagStrictAndDryRunSummaryLabel: dry-run also
// surfaces the strict-and marker so previews don't lie about the
// mode. Sister of the success path.
func TestArchiveBucketByTagStrictAndDryRunSummaryLabel(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "tag", "1", "+b"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:a,b", "--strict-and", "--dry-run")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "(strict-and)") {
		t.Errorf("expected '(strict-and)' in dry-run summary, got:\n%s", stdout)
	}
}
