package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestDependRemoveAllCSVScrubsEveryListedID: --remove-all 1,2 must
// drop BOTH ids from every other task's DependsOn list in a single
// sweep. This is the "two prereqs are gone, unblock everyone" form
// of the global scrub.
func TestDependRemoveAllCSVScrubsEveryListedID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereqA", "prereqB", "x", "y", "z"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #3, #4, #5 each depend on 1 AND 2.
	for _, id := range []string{"3", "4", "5"} {
		if _, _, err := runCmd(t, dir, "depend", id, "--on", "1,2"); err != nil {
			t.Fatalf("depend %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "depend", "1,2", "--remove-all")
	if err != nil {
		t.Fatalf("depend 1,2 --remove-all: %v", err)
	}
	// Header should mention both ids; touched count = 3.
	if !strings.Contains(stdout, "scrubbed #1, #2") {
		t.Fatalf("expected 'scrubbed #1, #2' header, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "from 3 task(s)") {
		t.Fatalf("expected 'from 3 task(s)', got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	for _, t2 := range s.Tasks {
		for _, dep := range t2.DependsOn {
			if dep == 1 || dep == 2 {
				t.Fatalf("task #%d still references #1/#2 after --remove-all 1,2: deps=%v", t2.ID, t2.DependsOn)
			}
		}
	}
}

// TestDependRemoveAllCSVPreservesOtherDeps: dropping 1,2 from a
// task that depends on [1,2,3,5] must leave [3,5] in that order
// — other prereqs untouched, original order preserved.
func TestDependRemoveAllCSVPreservesOtherDeps(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d", "e", "consumer"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #6 depends on 1,2,3,5 (sorted by --on).
	if _, _, err := runCmd(t, dir, "depend", "6", "--on", "1,2,3,5"); err != nil {
		t.Fatalf("depend 6: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "1,2", "--remove-all"); err != nil {
		t.Fatalf("depend --remove-all 1,2: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t6 := s.ByID(6)
	if t6 == nil {
		t.Fatal("#6 missing")
	}
	if len(t6.DependsOn) != 2 || t6.DependsOn[0] != 3 || t6.DependsOn[1] != 5 {
		t.Fatalf("expected #6 deps [3,5] after scrubbing 1,2; got %v", t6.DependsOn)
	}
}

// TestDependRemoveAllCSVHashPrefixTolerated: --remove-all #3,#5
// works the same as 3,5 — matches the convention every other CSV
// flag follows.
func TestDependRemoveAllCSVHashPrefixTolerated(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "#1,#2", "--remove-all"); err != nil {
		t.Fatalf("depend #1,#2 --remove-all: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t4 := s.ByID(4)
	if len(t4.DependsOn) != 0 {
		t.Fatalf("expected #4 deps empty after scrubbing #1,#2, got %v", t4.DependsOn)
	}
}

// TestDependRemoveAllCSVSingleSave: scrubbing N ids that touch M
// tasks must result in exactly ONE Save (and thus ONE .bak entry).
// The whole point of folding the multi-id path into a single
// scan is to avoid per-id Save churn.
func TestDependRemoveAllCSVSingleSave(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d", "e", "f"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #4,#5,#6 each depend on 1,2,3.
	for _, id := range []string{"4", "5", "6"} {
		if _, _, err := runCmd(t, dir, "depend", id, "--on", "1,2,3"); err != nil {
			t.Fatalf("depend %s: %v", id, err)
		}
	}
	preRun := readFile(t, filepath.Join(dir, ".tsk.md"))
	if _, _, err := runCmd(t, dir, "depend", "1,2,3", "--remove-all"); err != nil {
		t.Fatalf("depend 1,2,3 --remove-all: %v", err)
	}
	bak := readFile(t, filepath.Join(dir, ".tsk.md.bak"))
	if bak != preRun {
		t.Fatalf("expected .bak to match pre-run (single Save), got drift:\nPRE:\n%s\nBAK:\n%s", preRun, bak)
	}
}

// TestDependRemoveAllCSVJSONShape: the multi-id JSON output exposes
// the input ids array AND the touched set. No legacy "id" field
// when ids has more than one element (it's ambiguous which to pick).
func TestDependRemoveAllCSVJSONShape(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1,2", "--remove-all", "--json")
	if err != nil {
		t.Fatalf("depend 1,2 --remove-all --json: %v", err)
	}
	var doc struct {
		IDs     []int `json:"ids"`
		ID      *int  `json:"id"`
		Touched []int `json:"touched"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("bad json: %v\n%s", err, stdout)
	}
	if len(doc.IDs) != 2 || doc.IDs[0] != 1 || doc.IDs[1] != 2 {
		t.Fatalf("expected ids=[1,2], got %v", doc.IDs)
	}
	if doc.ID != nil {
		t.Fatalf("multi-id JSON must not carry the legacy 'id' field, got %v", *doc.ID)
	}
	if len(doc.Touched) != 2 || doc.Touched[0] != 3 || doc.Touched[1] != 4 {
		t.Fatalf("expected touched=[3,4], got %v", doc.Touched)
	}
}

// TestDependRemoveAllCSVSingleIDLegacyJSON: passing exactly one id
// (the original surface) keeps emitting the legacy "id" field
// alongside the new "ids" array — backward compat for existing
// JSON consumers.
func TestDependRemoveAllCSVSingleIDLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--remove-all", "--json")
	if err != nil {
		t.Fatalf("depend 1 --remove-all --json: %v", err)
	}
	var doc struct {
		IDs     []int `json:"ids"`
		ID      *int  `json:"id"`
		Touched []int `json:"touched"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("bad json: %v\n%s", err, stdout)
	}
	if doc.ID == nil || *doc.ID != 1 {
		t.Fatalf("expected legacy id=1 in single-id JSON, got %v", doc.ID)
	}
	if len(doc.IDs) != 1 || doc.IDs[0] != 1 {
		t.Fatalf("expected ids=[1], got %v", doc.IDs)
	}
}

// TestDependRemoveAllCSVRejectsInvalidToken: malformed CSV must
// fail at parse with exit-2, not silently degrade.
func TestDependRemoveAllCSVRejectsInvalidToken(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "1,abc", "--remove-all")
	if err == nil {
		t.Fatal("expected error for non-numeric id in CSV")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestDependRemoveAllCSVDuplicatesCollapse: --remove-all 3,3,5,3
// must behave the same as --remove-all 3,5. The CSV parser already
// dedupes; this guards the contract end-to-end.
func TestDependRemoveAllCSVDuplicatesCollapse(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	// Run with duplicate ids; should drop 1,3 from #4's deps cleanly.
	stdout, _, err := runCmd(t, dir, "depend", "1,3,1,3,3", "--remove-all")
	if err != nil {
		t.Fatalf("depend 1,3,1,3,3 --remove-all: %v", err)
	}
	if !strings.Contains(stdout, "scrubbed #1, #3") {
		t.Fatalf("expected dedup header 'scrubbed #1, #3', got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t4 := s.ByID(4)
	if len(t4.DependsOn) != 1 || t4.DependsOn[0] != 2 {
		t.Fatalf("expected #4 deps [2], got %v", t4.DependsOn)
	}
}
