package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestDependRemoveAllDryRunNoWrite: --dry-run reports what WOULD
// happen and does NOT mutate the store. Critical contract: zero
// on-disk drift after a dry-run sweep.
func TestDependRemoveAllDryRunNoWrite(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	for _, id := range []string{"2", "3", "4"} {
		if _, _, err := runCmd(t, dir, "depend", id, "--on", "1"); err != nil {
			t.Fatalf("depend %s: %v", id, err)
		}
	}
	before := readFile(t, filepath.Join(dir, ".tsk.md"))
	// .bak already exists from the earlier depend mutations; snapshot
	// it so we can assert dry-run leaves it untouched.
	bakPath := filepath.Join(dir, ".tsk.md.bak")
	bakBefore, bakErr := os.ReadFile(bakPath)
	if bakErr != nil {
		t.Fatalf("expected .bak from earlier mutations: %v", bakErr)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--remove-all", "--dry-run")
	if err != nil {
		t.Fatalf("depend 1 --remove-all --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "would scrub") {
		t.Fatalf("expected 'would scrub' in dry-run output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "from 3 task(s)") {
		t.Fatalf("expected 'from 3 task(s)', got:\n%s", stdout)
	}
	after := readFile(t, filepath.Join(dir, ".tsk.md"))
	if before != after {
		t.Fatalf("dry-run mutated the store!\nbefore:\n%s\nafter:\n%s", before, after)
	}
	// Critical: dry-run must not bump the .bak chain either —
	// silently rotating .bak would surprise `undo-last` users (the
	// "what would change?" preview shouldn't leave breadcrumbs the
	// user has to clean up).
	bakAfter, bakErr := os.ReadFile(bakPath)
	if bakErr != nil {
		t.Fatalf("read .bak after dry-run: %v", bakErr)
	}
	if string(bakBefore) != string(bakAfter) {
		t.Fatalf("dry-run rotated .bak!\nbefore:\n%s\nafter:\n%s", string(bakBefore), string(bakAfter))
	}
}

// TestDependRemoveAllDryRunReportsTouchedSet: the dry-run text
// surfaces WHICH tasks would be touched so the user can decide
// whether to proceed.
func TestDependRemoveAllDryRunReportsTouchedSet(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "x", "y", "z"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	for _, id := range []string{"2", "3", "4"} {
		if _, _, err := runCmd(t, dir, "depend", id, "--on", "1"); err != nil {
			t.Fatalf("depend %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--remove-all", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	// Must list every touched task id by number so the user can
	// audit before committing.
	for _, want := range []string{"#2", "#3", "#4"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected touched id %s in dry-run output, got:\n%s", want, stdout)
		}
	}
}

// TestDependRemoveAllDryRunEmptyMessage: a dry-run that finds
// nothing to scrub uses the SAME "no tasks depend on" message as
// the live no-op — symmetry keeps the empty case predictable.
func TestDependRemoveAllDryRunEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--remove-all", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run on no-op: %v", err)
	}
	if !strings.Contains(stdout, "no tasks depend on #1") {
		t.Fatalf("expected same no-op message as live run, got:\n%s", stdout)
	}
}

// TestDependRemoveAllDryRunJSONMarker: --dry-run --json surfaces
// the dry_run flag so scripted callers can branch on it.
func TestDependRemoveAllDryRunJSONMarker(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--remove-all", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run --json: %v", err)
	}
	var doc struct {
		IDs     []int `json:"ids"`
		Touched []int `json:"touched"`
		DryRun  bool  `json:"dry_run"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("bad json: %v\n%s", err, stdout)
	}
	if !doc.DryRun {
		t.Fatalf("expected dry_run=true in JSON, got %+v", doc)
	}
	if len(doc.Touched) != 2 {
		t.Fatalf("dry-run must still report touched set, got %v", doc.Touched)
	}
	// Verify state really wasn't changed by the dry-run JSON path.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	for _, t2 := range s.Tasks {
		if t2.ID == 1 {
			continue
		}
		if len(t2.DependsOn) != 1 || t2.DependsOn[0] != 1 {
			t.Fatalf("dry-run --json mutated #%d deps: %v", t2.ID, t2.DependsOn)
		}
	}
}

// TestDependRemoveAllDryRunMultiID: dry-run + CSV multi-id combine
// cleanly — no Save, header reports both ids, the touched set
// covers tasks affected by any of them.
func TestDependRemoveAllDryRunMultiID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "x", "y", "z"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "2"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "5", "--on", "1,2"); err != nil {
		t.Fatalf("depend 5: %v", err)
	}
	before := readFile(t, filepath.Join(dir, ".tsk.md"))
	stdout, _, err := runCmd(t, dir, "depend", "1,2", "--remove-all", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run --remove-all 1,2: %v", err)
	}
	if !strings.Contains(stdout, "would scrub #1, #2") {
		t.Fatalf("expected 'would scrub #1, #2' header, got:\n%s", stdout)
	}
	// Tasks 3, 4, 5 all touched.
	for _, want := range []string{"#3", "#4", "#5"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected touched %s, got:\n%s", want, stdout)
		}
	}
	after := readFile(t, filepath.Join(dir, ".tsk.md"))
	if before != after {
		t.Fatal("dry-run --remove-all CSV must not mutate file")
	}
}
