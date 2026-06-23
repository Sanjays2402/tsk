package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestArchiveJSONFlatSuccess: a basic flat-strategy archive run with
// --json emits the stable envelope shape with archived rows, both
// id halves (active_id + archive_id), and the resolved archive
// path. The bucket field is omitted (flat strategy) so the envelope
// stays minimal.
func TestArchiveJSONFlatSuccess(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// Mark all done so they qualify.
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--json")
	if err != nil {
		t.Fatalf("archive --all --json: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.TotalCount != 3 {
		t.Errorf("total_count: want 3, got %d", doc.TotalCount)
	}
	if doc.Strategy != "flat" {
		t.Errorf("strategy: want flat, got %q", doc.Strategy)
	}
	if doc.DryRun {
		t.Error("dry_run should be false on real archive run")
	}
	if doc.ActiveCount != 0 {
		t.Errorf("active_count: want 0, got %d", doc.ActiveCount)
	}
	if !strings.HasSuffix(doc.ArchivePath, "/.tsk.archive.md") {
		t.Errorf("archive_path: want .../.tsk.archive.md, got %q", doc.ArchivePath)
	}
	if len(doc.Archived) != 3 {
		t.Fatalf("archived: want 3 rows, got %d", len(doc.Archived))
	}
	// active_ids should be 1,2,3; archive_ids should also be 1,2,3
	// (fresh archive file)
	for i, row := range doc.Archived {
		if row.ActiveID != i+1 {
			t.Errorf("row %d active_id: want %d, got %d", i, i+1, row.ActiveID)
		}
		if row.ArchiveID != i+1 {
			t.Errorf("row %d archive_id: want %d, got %d", i, i+1, row.ArchiveID)
		}
		if row.Bucket != "" {
			t.Errorf("row %d bucket should be empty for flat strategy, got %q", i, row.Bucket)
		}
	}
}

// TestArchiveJSONDryRunSimulates: with --dry-run --json the
// envelope mirrors the real-run shape but archived items carry
// SIMULATED archive_ids. Nothing is written: the active store
// retains its tasks; the archive file is not created (or stays
// unchanged if it existed).
func TestArchiveJSONDryRunSimulates(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("archive --dry-run --json: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if !doc.DryRun {
		t.Error("dry_run should be true")
	}
	if doc.TotalCount != 2 {
		t.Errorf("total_count: want 2, got %d", doc.TotalCount)
	}
	if doc.ActiveCount != 0 {
		t.Errorf("active_count: kept after dry-run partition is 0, got %d", doc.ActiveCount)
	}
	// Verify the active store still has both tasks (dry-run wrote
	// nothing) by reading it back via `ls`.
	ls, _, err := runCmd(t, dir, "ls", "--done")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(ls, "a") || !strings.Contains(ls, "b") {
		t.Errorf("dry-run shouldn't have removed tasks, ls says:\n%s", ls)
	}
}

// TestArchiveJSONNoTasksEmitsEmptyArray: when nothing matches the
// selector the JSON envelope still emits a stable shape with
// archived: [] and total_count: 0. Important for jq pipelines —
// hitting `.archived[]` on a null would crash; on [] it's a no-op.
func TestArchiveJSONNoTasksEmitsEmptyArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "still open"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--json")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.TotalCount != 0 {
		t.Errorf("total_count: want 0, got %d", doc.TotalCount)
	}
	if doc.Archived == nil {
		t.Error("archived should be an empty array, not null")
	}
	// Raw text check: ensure "archived":[] appears (not "archived":null).
	if !strings.Contains(stdout, "\"archived\": []") {
		t.Errorf("expected empty array literal in body, got:\n%s", stdout)
	}
}

// TestArchiveJSONBucketByTagPopulatesBucket: with --bucket-by tag
// each row's bucket field reflects the section that task was
// placed in (per the same bucketFn the on-disk writer uses).
// Tasks with no first tag land in "untagged".
func TestArchiveJSONBucketByTagPopulatesBucket(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship-a", "-t", "release"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "ship-b", "-t", "bugfix"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "scratch"); err != nil {
		t.Fatalf("add 3: %v", err)
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag", "--json")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.BucketBy != "tag" {
		t.Errorf("bucket_by: want tag, got %q", doc.BucketBy)
	}
	if doc.Strategy != "flat" {
		t.Errorf("strategy: want flat, got %q", doc.Strategy)
	}
	buckets := map[string]string{}
	for _, row := range doc.Archived {
		buckets[row.Title] = row.Bucket
	}
	if buckets["ship-a"] != "release" {
		t.Errorf("ship-a bucket: want release, got %q", buckets["ship-a"])
	}
	if buckets["ship-b"] != "bugfix" {
		t.Errorf("ship-b bucket: want bugfix, got %q", buckets["ship-b"])
	}
	if buckets["scratch"] != "untagged" {
		t.Errorf("scratch bucket: want untagged, got %q", buckets["scratch"])
	}
}

// TestArchiveJSONStrategyWeekly: with --strategy weekly the JSON
// envelope carries strategy="weekly" and each row's bucket is the
// "YYYY-W##" ISO week key bucketByISOWeek would emit. Same key
// the on-disk weekly section header uses, so JSON and file agree.
func TestArchiveJSONStrategyWeekly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "weekly", "--json")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.Strategy != "weekly" {
		t.Errorf("strategy: want weekly, got %q", doc.Strategy)
	}
	if len(doc.Archived) != 1 {
		t.Fatalf("expected 1 archived row, got %d", len(doc.Archived))
	}
	// Bucket should be a YYYY-W## token. We can't predict the
	// exact week the test runs in, so just assert the shape.
	b := doc.Archived[0].Bucket
	if !strings.Contains(b, "-W") {
		t.Errorf("bucket: want YYYY-W## shape, got %q", b)
	}
}

// TestArchiveJSONStrictAndPropagates: when --strict-and is set
// alongside a CSV-tag bucket-by, the envelope reports strict_and:
// true so consumers can tell union from intersection without
// re-parsing the bucket label.
func TestArchiveJSONStrictAndPropagates(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "a,b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--bucket-by", "tag:a,b", "--strict-and", "--json")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !doc.StrictAnd {
		t.Errorf("strict_and: want true, got false; body: %s", stdout)
	}
	if doc.BucketBy != "tag:a,b" {
		t.Errorf("bucket_by: want tag:a,b, got %q", doc.BucketBy)
	}
}

// TestArchiveJSONActiveAndArchiveIDsDiffer: when the archive file
// has existing tasks, the new archive_ids continue from the
// archive's max+1 — so they DIFFER from the active_ids the tasks
// carried before the move. Both halves appear in the row.
func TestArchiveJSONActiveAndArchiveIDsDiffer(t *testing.T) {
	dir := t.TempDir()
	// Seed: add+done+archive a first batch so the archive file
	// has tasks taking ids 1..3.
	for _, title := range []string{"seed1", "seed2", "seed3"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("seed add: %v", err)
		}
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("seed done: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "archive", "--all"); err != nil {
		t.Fatalf("seed archive: %v", err)
	}
	// New batch: add 2 more tasks, mark done, archive with --json.
	for _, title := range []string{"new-a", "new-b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--json")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(doc.Archived) != 2 {
		t.Fatalf("expected 2 archived rows, got %d", len(doc.Archived))
	}
	for _, row := range doc.Archived {
		if row.ActiveID <= 0 {
			t.Errorf("active_id should be positive, got %d", row.ActiveID)
		}
		if row.ArchiveID <= 3 {
			t.Errorf("archive_id should be > 3 (after the seed batch), got %d", row.ArchiveID)
		}
		if row.ActiveID == row.ArchiveID {
			t.Errorf("expected active_id != archive_id, both = %d", row.ActiveID)
		}
	}
}

// TestArchiveJSONPriorityField: each row carries the canonical
// priority string. Catches drift in the model.Priority.String()
// side or a future field-rename in archivedRow.
func TestArchiveJSONPriorityField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "high-task", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--json")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Archived) != 1 {
		t.Fatalf("expected 1 row, got %d", len(doc.Archived))
	}
	if doc.Archived[0].Priority != "high" {
		t.Errorf("priority: want high, got %q", doc.Archived[0].Priority)
	}
}

// TestArchiveJSONIndentedByDefault: the JSON envelope uses the
// two-space indented form for human-readable output. The encoder
// adds a trailing newline; we assert the body contains internal
// newlines (vs. the compact-json single-line shape graph uses).
func TestArchiveJSONIndentedByDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--json")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "\n  ") {
		t.Errorf("expected indented JSON (internal newlines + 2-space indent), got:\n%s", stdout)
	}
}

// TestArchiveJSONActiveCountAfterPartial: when only SOME tasks
// qualify for archive, active_count reflects the post-archive
// active store size. Useful for "did this run drop us below N
// open tasks?" CI gates.
func TestArchiveJSONActiveCountAfterPartial(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"keep1", "keep2", "done1"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--json")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.TotalCount != 1 {
		t.Errorf("total_count: want 1, got %d", doc.TotalCount)
	}
	if doc.ActiveCount != 2 {
		t.Errorf("active_count: want 2 (two open tasks kept), got %d", doc.ActiveCount)
	}
}

// TestArchiveJSONNoTasksDryRun: --dry-run --json on a no-match
// store still emits the stable empty envelope (archived: [],
// total_count: 0, dry_run: true). Same shape consumers see in
// the no-match real-run case.
func TestArchiveJSONNoTasksDryRun(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "still open"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	var doc archiveDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if !doc.DryRun {
		t.Error("dry_run: want true")
	}
	if doc.TotalCount != 0 {
		t.Errorf("total_count: want 0, got %d", doc.TotalCount)
	}
	if doc.Archived == nil {
		t.Error("archived should be empty array, not null")
	}
}
