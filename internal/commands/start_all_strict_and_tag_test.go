package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestStartAllStrictAndTagIntersection: --strict-and-tag a,b
// narrows the bulk-start to ONLY open tasks carrying BOTH 'a'
// AND 'b'. Sister of pause --all --strict-and-tag for the
// inverse verb.
func TestStartAllStrictAndTagIntersection(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "both", "-t", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "work-only", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "p0-only", "-t", "p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "neither", "-t", "later"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "--all", "--strict-and-tag", "work,p0"); err != nil {
		t.Fatalf("start --all --strict-and-tag: %v", err)
	}
	// Only #1 (both work and p0) should now be started.
	stdout, _, err := runCmd(t, dir, "wip", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("parse wip: %v\nbody: %s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 in-progress task, got %d:\n%s", len(rows), stdout)
	}
	idF, _ := rows[0]["ID"].(float64)
	if int(idF) != 1 {
		t.Errorf("expected #1 started, got #%d", int(idF))
	}
}

// TestStartAllStrictAndTagMutexWithTag: --strict-and-tag and --tag
// are mutually exclusive (each is a different selector axis).
func TestStartAllStrictAndTagMutexWithTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--strict-and-tag", "work,p0")
	if err == nil {
		t.Fatal("expected error for --tag + --strict-and-tag")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutex error, got %v", err)
	}
}

// TestStartAllStrictAndTagSingleElementCSV: a single-element CSV
// "work" should behave identically to --tag work (intersection of
// one element is the element itself).
func TestStartAllStrictAndTagSingleElementCSV(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "in-work", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "in-other", "-t", "other"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "--all", "--strict-and-tag", "work"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 started, got %d", len(rows))
	}
	idF, _ := rows[0]["ID"].(float64)
	if int(idF) != 1 {
		t.Errorf("expected #1 started, got #%d", int(idF))
	}
}

// TestStartAllStrictAndTagComposesWithPriority: --strict-and-tag
// composes with --priority as AND: a task must carry ALL tags AND
// match the priority.
func TestStartAllStrictAndTagComposesWithPriority(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "both-high", "-t", "work,p0", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "both-low", "-t", "work,p0", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "work-high", "-t", "work", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "--all", "--strict-and-tag", "work,p0", "--priority", "high"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 task (both-high), got %d", len(rows))
	}
	idF, _ := rows[0]["ID"].(float64)
	if int(idF) != 1 {
		t.Errorf("expected #1 (both-high) started, got #%d", int(idF))
	}
}

// TestStartAllStrictAndTagDryRunPreview: --dry-run prints the
// intersection-filtered set without writing.
func TestStartAllStrictAndTagDryRunPreview(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "both", "-t", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "single", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--strict-and-tag", "work,p0", "--dry-run")
	if err != nil {
		t.Fatalf("start --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "would start 1 task") {
		t.Errorf("expected preview to say would start 1 task, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tag=work&p0") {
		t.Errorf("expected filter summary 'tag=work&p0' in preview, got:\n%s", stdout)
	}
	// Verify nothing was actually started.
	wipOut, _, err := runCmd(t, dir, "wip", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	if !strings.Contains(wipOut, "[]") {
		t.Errorf("dry-run should not start anything, but wip shows:\n%s", wipOut)
	}
}

// TestStartAllStrictAndTagDryRunJSON: --dry-run --json emits the
// strict_and_tag field in the envelope and the filter summary
// renders with the &-form.
func TestStartAllStrictAndTagDryRunJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "both", "-t", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--strict-and-tag", "work,p0", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("start --json: %v", err)
	}
	var doc startAllDryRunDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.StrictAndTag != "work,p0" {
		t.Errorf("strict_and_tag: want 'work,p0', got %q", doc.StrictAndTag)
	}
	if doc.Tag != "" {
		t.Errorf("tag should be empty when strict-and-tag is set, got %q", doc.Tag)
	}
	if doc.TotalCount != 1 {
		t.Errorf("total_count: want 1, got %d", doc.TotalCount)
	}
	if !strings.Contains(doc.Filter, "tag=work&p0") {
		t.Errorf("filter summary should contain 'tag=work&p0', got %q", doc.Filter)
	}
}

// TestStartAllStrictAndTagEmptyResult: when no tasks match the
// intersection, the no-match path fires with the &-form filter
// summary in the message.
func TestStartAllStrictAndTagEmptyResult(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "single", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--strict-and-tag", "work,p0")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !strings.Contains(stdout, "no open tasks match") {
		t.Errorf("expected 'no open tasks match', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tag=work&p0") {
		t.Errorf("expected filter summary 'tag=work&p0', got:\n%s", stdout)
	}
}

// TestStartAllStrictAndTagToleratesCSVSpaces: "work, p0" with a
// space after the comma should still split into 2 tags (same
// behavior the other strict-and-tag verbs use).
func TestStartAllStrictAndTagToleratesCSVSpaces(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "both", "-t", "work,p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "--all", "--strict-and-tag", "work, p0"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip", "--json")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 started despite CSV whitespace, got %d", len(rows))
	}
}

// TestStartAllErrorMessageMentionsStrictAndTag: the rejection
// message for `tsk start --all` (with no filter) should now mention
// `strict-and-tag` alongside `tag` and `priority`, so users
// discover the new selector axis from the help text alone.
func TestStartAllErrorMessageMentionsStrictAndTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "--all")
	if err == nil {
		t.Fatal("expected error for --all without filter")
	}
	if !strings.Contains(err.Error(), "strict-and-tag") {
		t.Errorf("error should mention strict-and-tag now (new option), got %v", err)
	}
}
