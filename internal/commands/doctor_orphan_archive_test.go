package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctorOrphanArchiveCleanPasses: a live store + archive where
// every archived task's DependsOn resolves either in live OR in
// the archive itself passes the orphan check.
func TestDoctorOrphanArchiveCleanPasses(t *testing.T) {
	dir := t.TempDir()
	// Live: tasks #1, #2.
	if _, _, err := runCmd(t, dir, "add", "live one"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "live two"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	// Add a task with no deps then complete it, then archive.
	if _, _, err := runCmd(t, dir, "add", "done task"); err != nil {
		t.Fatalf("add 3: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// NOW add the dep retroactively (depend doesn't reject a
	// done source). The archive will preserve this dep and it
	// will resolve via live #1.
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Archive everything done. --all to bypass --older-than.
	if _, _, err := runCmd(t, dir, "archive", "--all"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	// Run doctor with the orphan check.
	out, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive")
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if strings.Contains(out, "orphan_archive_dep") {
		t.Errorf("clean archive should not flag orphans, got:\n%s", out)
	}
}

// TestDoctorOrphanArchiveDetectsDangling: an archived task whose
// DependsOn references an id missing from both the live store and
// the archive (manually scrubbed) is surfaced as a warning.
func TestDoctorOrphanArchiveDetectsDangling(t *testing.T) {
	dir := t.TempDir()
	// Hand-write an archive with a known-dangling dep.
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] orphan-pointer <!-- id:1 prio:medium depends:99 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	// Live store has its own task.
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive")
	// Warning → still passes (warnings don't flip exit code; only errors do).
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "orphan_archive_dep") {
		t.Errorf("expected orphan_archive_dep warning, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#99") {
		t.Errorf("expected orphan id #99 in detail, got:\n%s", stdout)
	}
}

// TestDoctorOrphanArchiveOnlyFiresWithFlag: without
// --check-orphan-archive, a corrupted archive is NOT inspected;
// doctor passes cleanly because the orphan check is opt-in.
func TestDoctorOrphanArchiveOnlyFiresWithFlag(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] orphan <!-- id:1 prio:medium depends:99 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor")
	if err != nil {
		t.Fatalf("doctor (no flag): %v", err)
	}
	if strings.Contains(stdout, "orphan_archive_dep") {
		t.Errorf("orphan check should NOT fire without flag, got:\n%s", stdout)
	}
}

// TestDoctorOrphanArchiveMissingArchiveIsClean: when there's no
// archive sibling, the orphan check passes silently (nothing to
// corrupt yet). The OK list should include "orphan_archive_check".
func TestDoctorOrphanArchiveMissingArchiveIsClean(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "live"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive")
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if strings.Contains(stdout, "orphan_archive_dep") {
		t.Errorf("missing archive should not raise findings, got:\n%s", stdout)
	}
}

// TestDoctorOrphanArchiveJSONShapeIncludesOrphan: --json passes
// the orphan warnings through the existing DoctorReport.Warnings
// list, so a CI hook can parse them.
func TestDoctorOrphanArchiveJSONShapeIncludesOrphan(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] orphan <!-- id:1 prio:medium depends:42 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "live"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--json")
	if err != nil {
		t.Fatalf("doctor --json: %v\n%s", err, stdout)
	}
	var doc struct {
		Warnings []struct {
			Check  string `json:"check"`
			Detail string `json:"detail"`
			TaskID int    `json:"task_id"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json unmarshal: %v\n%s", err, stdout)
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Check == "orphan_archive_dep" {
			found = true
			if w.TaskID != 1 {
				t.Errorf("orphan_archive_dep task_id: want 1, got %d", w.TaskID)
			}
			if !strings.Contains(w.Detail, "#42") {
				t.Errorf("expected #42 in detail, got %q", w.Detail)
			}
		}
	}
	if !found {
		t.Errorf("orphan_archive_dep missing from warnings: %+v", doc.Warnings)
	}
}

// TestDoctorOrphanArchiveLiveStoreReferencesArchiveID: a live task
// that depends on an ARCHIVED id resolves cleanly via the unified
// id-set (live ∪ archive). The orphan check is for the ARCHIVE
// side (archive task references missing dep), not the live side,
// but verifying this regression keeps the resolvable union honest.
func TestDoctorOrphanArchiveLiveStoreReferencesArchiveID(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	// Archive has id #5 (a real, valid archive task).
	body := "# tsk archive\n\n- [x] historical <!-- id:5 prio:medium -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	// Live store has its own task; no archive-side orphans.
	if _, _, err := runCmd(t, dir, "add", "live"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive")
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, stdout)
	}
	if strings.Contains(stdout, "orphan_archive_dep") {
		t.Errorf("no archive-side orphans expected, got:\n%s", stdout)
	}
}

// TestDoctorOrphanArchiveIgnoresLiveDeps: an archived task that
// depends on a LIVE-store task that still exists is not orphaned.
// (regression: don't accidentally flag healthy cross-store deps.)
func TestDoctorOrphanArchiveIgnoresLiveDeps(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "live"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// archive references id 1 in the live store.
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n- [x] arch <!-- id:1 prio:medium depends:1 -->\n"
	// Note: the archived task's id is 1 in archive space, depends:1 is
	// the live store's id 1. tsk's resolvable set unifies BOTH spaces.
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive")
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, stdout)
	}
	if strings.Contains(stdout, "orphan_archive_dep") {
		t.Errorf("dep resolving in live should NOT be orphan, got:\n%s", stdout)
	}
}

// TestDoctorOrphanArchiveMultipleOrphans: an archive with multiple
// dangling deps surfaces ALL of them, not just the first. CI hooks
// need the complete picture in one pass.
func TestDoctorOrphanArchiveMultipleOrphans(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] orphan-one <!-- id:1 prio:medium depends:99 -->\n" +
		"- [x] orphan-two <!-- id:2 prio:medium depends:88 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "live"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive")
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	for _, want := range []string{"#99", "#88"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %s orphan in report, got:\n%s", want, stdout)
		}
	}
}

// TestDoctorOrphanArchiveResolvableViaArchive: two archived tasks
// where one depends on another in the SAME archive — that's
// resolvable (in-archive references are valid).
func TestDoctorOrphanArchiveResolvableViaArchive(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] prereq <!-- id:1 prio:medium -->\n" +
		"- [x] dependent <!-- id:2 prio:medium depends:1 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "live"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive")
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if strings.Contains(stdout, "orphan_archive_dep") {
		t.Errorf("in-archive dep should resolve, got:\n%s", stdout)
	}
}
