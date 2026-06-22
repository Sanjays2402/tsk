package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctorFixOrphansRemovesDanglingRefs: a dangling DependsOn id
// in the archive is scrubbed after --fix-orphans, and a subsequent
// scan reports no orphans.
func TestDoctorFixOrphansRemovesDanglingRefs(t *testing.T) {
	dir := t.TempDir()
	// Hand-write archive with a known-dangling dep.
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] orphan-pointer <!-- id:1 prio:medium depends:99 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	// Live store with one real task.
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Apply --fix-orphans.
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans")
	if err != nil {
		t.Fatalf("doctor --fix-orphans: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "fix-orphans") {
		t.Errorf("expected fix-orphans summary line, got:\n%s", stdout)
	}
	// Re-read the archive on disk: depends:99 should be gone.
	archive := string(mustReadFile(t, archivePath))
	if strings.Contains(archive, "depends:99") {
		t.Errorf("dangling depends:99 should be scrubbed, archive now:\n%s", archive)
	}
	// And the warning should no longer appear on a re-scan.
	stdout2, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive")
	if err != nil {
		t.Fatalf("post-fix doctor: %v\n%s", err, stdout2)
	}
	if strings.Contains(stdout2, "orphan_archive_dep") {
		t.Errorf("post-fix re-scan should not flag orphans, got:\n%s", stdout2)
	}
}

// TestDoctorFixOrphansPreservesValidDeps: scrubbing dangling refs
// must leave VALID DependsOn entries intact (only the orphans go).
func TestDoctorFixOrphansPreservesValidDeps(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	// Archive task #1 with two deps: 99 (orphan) and 2 (live).
	body := "# tsk archive\n\n" +
		"- [x] mixed-deps <!-- id:1 prio:medium depends:99,2 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	// Live store has tasks 1, 2 (after this:1=real, 2=other-real).
	if _, _, err := runCmd(t, dir, "add", "real-1"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "real-2"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans"); err != nil {
		t.Fatalf("doctor --fix-orphans: %v", err)
	}
	archive := string(mustReadFile(t, archivePath))
	if strings.Contains(archive, "depends:99") || strings.Contains(archive, "99,") || strings.Contains(archive, ",99") {
		t.Errorf("dangling 99 should be removed:\n%s", archive)
	}
	// The valid dep on #2 must still be present.
	if !strings.Contains(archive, "depends:2") {
		t.Errorf("valid depends:2 must survive, archive:\n%s", archive)
	}
}

// TestDoctorFixOrphansRequiresCheckFlag: --fix-orphans without
// --check-orphan-archive is rejected at exit 2 with a helpful
// usage error.
func TestDoctorFixOrphansRequiresCheckFlag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "doctor", "--fix-orphans")
	if err == nil {
		t.Fatal("expected error for --fix-orphans without --check-orphan-archive")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
}

// TestDoctorFixOrphansCountsIndividualRefs: when a single archive
// task carries TWO dangling refs, the summary reports 2 scrubbed
// (not 1 — the count is per-ref, matching the warning granularity).
func TestDoctorFixOrphansCountsIndividualRefs(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] double-orphan <!-- id:1 prio:medium depends:99,100 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans")
	if err != nil {
		t.Fatalf("doctor --fix-orphans: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "2 dangling") {
		t.Errorf("expected '2 dangling' in summary (one task, two refs), got:\n%s", stdout)
	}
}

// TestDoctorFixOrphansEmptyArchiveIsNoop: --fix-orphans on a store
// with no archive sibling is silently a no-op (0 scrubbed, no error).
// The summary still prints "0 dangling ref(s) scrubbed" so the user
// sees their explicit --fix-orphans flag was honored.
func TestDoctorFixOrphansEmptyArchiveIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "live"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// No archive exists. fix-orphans should silently pass.
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans")
	if err != nil {
		t.Fatalf("doctor --fix-orphans on empty archive: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "0 dangling") {
		t.Errorf("expected '0 dangling' summary when archive missing, got:\n%s", stdout)
	}
}

// TestDoctorFixOrphansJSONReflectsRepair: --fix-orphans --json
// strips the orphan warnings out of the post-fix report (the
// repair erased the dangling refs, so a truthful report shouldn't
// continue listing them as findings).
func TestDoctorFixOrphansJSONReflectsRepair(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] orphan <!-- id:1 prio:medium depends:99 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans", "--json")
	if err != nil {
		t.Fatalf("doctor --fix-orphans --json: %v\n%s", err, stdout)
	}
	var report DoctorReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse json: %v\n%s", err, stdout)
	}
	for _, w := range report.Warnings {
		if w.Check == "orphan_archive_dep" {
			t.Errorf("post-fix report should not retain orphan_archive_dep warnings, got %+v", w)
		}
	}
	// The OKChecks line should include the scrub summary.
	found := false
	for _, ok := range report.OKChecks {
		if strings.Contains(ok, "fix-orphans") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected fix-orphans summary in OKChecks, got %+v", report.OKChecks)
	}
}

// TestDoctorFixOrphansBakSnapshot: --fix-orphans goes through
// store.Save which produces a .bak snapshot atomically, so
// `tsk undo-last` against the archive can revert the fix.
func TestDoctorFixOrphansBakSnapshot(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] orphan <!-- id:1 prio:medium depends:99 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans"); err != nil {
		t.Fatalf("doctor --fix-orphans: %v", err)
	}
	// The .bak should exist and still contain depends:99 (the
	// pre-fix state).
	bakBytes, err := os.ReadFile(archivePath + ".bak")
	if err != nil {
		t.Fatalf("expected .bak snapshot, got: %v", err)
	}
	if !strings.Contains(string(bakBytes), "depends:99") {
		t.Errorf("expected .bak to preserve pre-fix depends:99, got:\n%s", string(bakBytes))
	}
}

// mustReadFile is a small test helper that fails the test if the
// file cannot be read.
func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
