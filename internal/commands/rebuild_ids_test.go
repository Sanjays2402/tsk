package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
)

func TestRebuildIDsDryRunDoesNotMutate(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "rm", "2"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	before := readFile(t, filepath.Join(dir, ".tsk.md"))
	// Default (no --apply) is a preview.
	stdout, _, err := runCmd(t, dir, "rebuild-ids")
	if err != nil {
		t.Fatalf("rebuild-ids: %v", err)
	}
	if !strings.Contains(stdout, "DRY RUN") {
		t.Fatalf("expected DRY RUN banner, got:\n%s", stdout)
	}
	after := readFile(t, filepath.Join(dir, ".tsk.md"))
	if before != after {
		t.Fatalf("dry run mutated file:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
	}
}

func TestRebuildIDsApplyDensifies(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// rm 2 and rm 3 → ids 1 and 4 remain. rebuild-ids should produce 1, 2.
	if _, _, err := runCmd(t, dir, "rm", "2", "3"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if _, _, err := runCmd(t, dir, "rebuild-ids", "--apply", "--yes"); err != nil {
		t.Fatalf("rebuild-ids --apply: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] a <!-- id:1 ") {
		t.Fatalf("expected a -> id:1, got:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] d <!-- id:2 ") {
		t.Fatalf("expected d -> id:2, got:\n%s", content)
	}
	// No id:3 or id:4 remains.
	if strings.Contains(content, "id:3 ") || strings.Contains(content, "id:4 ") {
		t.Fatalf("old sparse ids should be gone, got:\n%s", content)
	}
}

func TestRebuildIDsApplyRequiresYes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "rebuild-ids", "--apply")
	if err == nil {
		t.Fatal("expected error: --apply requires --yes")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

func TestRebuildIDsSinceIDPreservesLowerIDs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// rm 2, rm 3 → ids 1, 4 remain in positions [a, d].
	if _, _, err := runCmd(t, dir, "rm", "2", "3"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	// --since-id 4 means "leave any id < 4 alone". a (id:1) stays
	// at id:1; d (id:4) gets renumbered to first free slot >= 4,
	// which is... well, since 1 is reserved by a, d gets 2.
	if _, _, err := runCmd(t, dir, "rebuild-ids", "--since-id", "4", "--apply", "--yes"); err != nil {
		t.Fatalf("rebuild-ids: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] a <!-- id:1 ") {
		t.Fatalf("a should keep id:1 (below since-id), got:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] d <!-- id:2 ") {
		t.Fatalf("d should be renumbered to id:2, got:\n%s", content)
	}
}

func TestRebuildIDsNoOpStaysClean(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// No removes — IDs already 1, 2, 3.
	stdout, _, err := runCmd(t, dir, "rebuild-ids")
	if err != nil {
		t.Fatalf("rebuild-ids: %v", err)
	}
	if !strings.Contains(stdout, "0 will change") {
		t.Fatalf("expected 0 changes when already dense, got:\n%s", stdout)
	}
}

func TestRebuildIDsJSONShape(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "rm", "2"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "rebuild-ids", "--json")
	if err != nil {
		t.Fatalf("rebuild-ids --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if doc["apply"] != false {
		t.Fatalf("apply should be false in dry-run JSON, got %v", doc["apply"])
	}
	mapping, ok := doc["mapping"].([]any)
	if !ok {
		t.Fatalf("mapping should be array, got %T", doc["mapping"])
	}
	if len(mapping) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mapping))
	}
}

func TestRebuildIDsRejectsNegativeSinceID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "rebuild-ids", "--since-id", "-1")
	if err == nil {
		t.Fatal("expected error for negative --since-id")
	}
}

func TestPlanRebuildIDsReservation(t *testing.T) {
	// Direct test of the planning algorithm: when sinceID=5 and
	// existing IDs include 2 and 8, the new ids for >= 5 should
	// avoid colliding with 2 (reserved by surviving task).
	tasks := []model.Task{
		{ID: 2, Title: "low-keep"},
		{ID: 8, Title: "renum-1"},
		{ID: 14, Title: "renum-2"},
	}
	mapping := planRebuildIDs(tasks, 5)
	if len(mapping) != 3 {
		t.Fatalf("expected 3 mappings, got %d", len(mapping))
	}
	if mapping[0].New != 2 {
		t.Fatalf("low-keep should stay at 2, got %d", mapping[0].New)
	}
	// next free slot != 2 (reserved); should be 1.
	if mapping[1].New != 1 {
		t.Fatalf("renum-1 should get 1 (first free, skipping reserved 2), got %d", mapping[1].New)
	}
	// then 3.
	if mapping[2].New != 3 {
		t.Fatalf("renum-2 should get 3 (next after 1, skipping reserved 2), got %d", mapping[2].New)
	}
}

func TestSummaryStringHelper(t *testing.T) {
	// Tiny sanity check on the helper used elsewhere.
	mapping := []idMapping{
		{Old: 1, New: 1},
		{Old: 5, New: 2},
		{Old: 7, New: 3},
	}
	if got := summaryString(mapping); got != "2/3" {
		t.Fatalf("summaryString = %q want 2/3", got)
	}
}
