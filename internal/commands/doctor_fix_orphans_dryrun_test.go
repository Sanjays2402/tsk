package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctorFixOrphansDryRunDoesNotMutateArchive: the cornerstone
// guarantee of --dry-run. The same scan + count logic runs, but
// the archive on disk is byte-identical before and after. Sister
// of every other tsk --dry-run preview verb.
func TestDoctorFixOrphansDryRunDoesNotMutateArchive(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] orphan-pointer <!-- id:1 prio:medium depends:99 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	preBytes, _ := os.ReadFile(archivePath)
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans", "--dry-run")
	if err != nil {
		t.Fatalf("doctor --fix-orphans --dry-run: %v\n%s", err, stdout)
	}
	postBytes, _ := os.ReadFile(archivePath)
	if string(preBytes) != string(postBytes) {
		t.Errorf("--dry-run mutated archive\npre:\n%s\npost:\n%s", string(preBytes), string(postBytes))
	}
	// .bak must NOT have been created — dry-run leaves the chain
	// untouched (no rotation, no snapshot).
	if _, err := os.Stat(archivePath + ".bak"); err == nil {
		t.Errorf("--dry-run should not produce .bak, but it exists")
	}
	if !strings.Contains(stdout, "REPAIRS (dry-run)") {
		t.Errorf("expected REPAIRS (dry-run) block, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "would scrub 1 dangling") {
		t.Errorf("expected 'would scrub 1 dangling' summary, got:\n%s", stdout)
	}
}

// TestDoctorFixOrphansDryRunListsEachRef: the per-ref detail rows
// in the REPAIRS (dry-run) block enumerate every (task, missing-
// dep) pair so the user can review before applying.
func TestDoctorFixOrphansDryRunListsEachRef(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	// One task with TWO orphan deps — the preview must list both.
	body := "# tsk archive\n\n" +
		"- [x] double-orphan <!-- id:1 prio:medium depends:99,100 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans", "--dry-run")
	if err != nil {
		t.Fatalf("doctor --fix-orphans --dry-run: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "would scrub 2 dangling") {
		t.Errorf("expected count=2 in summary, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "archive task #1 depends on missing #99") {
		t.Errorf("expected per-ref row for #99, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "archive task #1 depends on missing #100") {
		t.Errorf("expected per-ref row for #100, got:\n%s", stdout)
	}
}

// TestDoctorFixOrphansDryRunCountMatchesApplyPath: the count the
// dry-run reports must EXACTLY equal what a real --fix-orphans run
// would scrub. Honesty is the whole point of dry-run.
func TestDoctorFixOrphansDryRunCountMatchesApplyPath(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	body := "# tsk archive\n\n" +
		"- [x] a <!-- id:1 prio:medium depends:99 -->\n" +
		"- [x] b <!-- id:2 prio:medium depends:98 -->\n" +
		"- [x] c <!-- id:3 prio:medium depends:97,2 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Dry-run preview: capture the count.
	dryStdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v\n%s", err, dryStdout)
	}
	// 3 orphan refs total: 99, 98, 97. (The depends:2 on task #3
	// resolves to in-archive id 2, so it's NOT scrubbed.)
	if !strings.Contains(dryStdout, "would scrub 3 dangling") {
		t.Errorf("expected dry-run count=3, got:\n%s", dryStdout)
	}
	// Fresh scratch dir for the actual apply so we don't compare
	// against state mutated by the dry-run (the dry-run shouldn't
	// have mutated anything, but the cleaner test setup is
	// independent runs).
	dir2 := t.TempDir()
	archivePath2 := filepath.Join(dir2, ".tsk.archive.md")
	if err := os.WriteFile(archivePath2, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive2: %v", err)
	}
	if _, _, err := runCmd(t, dir2, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	applyStdout, _, err := runCmd(t, dir2, "doctor", "--check-orphan-archive", "--fix-orphans")
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, applyStdout)
	}
	if !strings.Contains(applyStdout, "3 dangling ref(s) scrubbed") {
		t.Errorf("expected apply count=3, got:\n%s", applyStdout)
	}
}

// TestDoctorFixOrphansDryRunRequiresFixFlag: --dry-run alone (no
// --fix-orphans) is a usage error — every other doctor path is
// already side-effect-free, so --dry-run would be a no-op alias
// with confusing semantics.
func TestDoctorFixOrphansDryRunRequiresFixFlag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "doctor", "--dry-run")
	if err == nil {
		t.Fatal("expected error for --dry-run without --fix-orphans")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
}

// TestDoctorFixOrphansDryRunJSONEnvelope: --json + --dry-run still
// emits the structured DoctorReport, with the "would be scrubbed"
// preview folded into OKChecks. Critical for CI gates that want
// to gate on the structured output.
func TestDoctorFixOrphansDryRunJSONEnvelope(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("doctor --dry-run --json: %v\n%s", err, stdout)
	}
	var report DoctorReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse json: %v\n%s", err, stdout)
	}
	// Dry-run preview line must be present in OKChecks.
	found := false
	for _, ok := range report.OKChecks {
		if strings.Contains(ok, "fix-orphans (dry-run)") && strings.Contains(ok, "would be scrubbed") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected dry-run preview in OKChecks, got %+v", report.OKChecks)
	}
	// Warnings should STILL contain the orphan flag (dry-run
	// didn't actually scrub anything — the warning is honest).
	stillFlagged := false
	for _, w := range report.Warnings {
		if w.Check == "orphan_archive_dep" {
			stillFlagged = true
			break
		}
	}
	if !stillFlagged {
		t.Errorf("dry-run should NOT strip the orphan warning (nothing was actually fixed), got %+v", report.Warnings)
	}
}

// TestDoctorFixOrphansDryRunEmptyArchive: no archive sibling →
// silent 0, no errors, "no dangling refs to scrub" in the
// REPAIRS block.
func TestDoctorFixOrphansDryRunEmptyArchive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "live"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans", "--dry-run")
	if err != nil {
		t.Fatalf("doctor --dry-run on empty archive: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "no dangling refs to scrub") {
		t.Errorf("expected 'no dangling refs to scrub' for empty archive, got:\n%s", stdout)
	}
	// The OKChecks summary line lives in the JSON envelope (and
	// is checked by TestDoctorFixOrphansDryRunJSONEnvelope); the
	// human path surfaces the REPAIRS block above instead.
}

// TestDoctorFixOrphansDryRunDetailsSortedDeterministically: the
// per-ref preview rows are sorted (task_id asc, then missing_dep
// asc) so CI diffs against the output are stable.
func TestDoctorFixOrphansDryRunDetailsSortedDeterministically(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, ".tsk.archive.md")
	// Deliberately write deps in non-sorted order. The preview
	// must still emit them sorted.
	body := "# tsk archive\n\n" +
		"- [x] a <!-- id:3 prio:medium depends:99 -->\n" +
		"- [x] b <!-- id:1 prio:medium depends:97,98 -->\n" +
		"- [x] c <!-- id:2 prio:medium depends:96 -->\n"
	if err := os.WriteFile(archivePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--check-orphan-archive", "--fix-orphans", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v\n%s", err, stdout)
	}
	// Extract just the per-ref preview lines (those with the
	// "depends on missing" tail-fragment). The doctor scan
	// WARNINGS section also mentions "archive task #" so we use
	// the distinctive REPAIRS-block phrasing to filter.
	lines := strings.Split(stdout, "\n")
	var refRows []string
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if strings.Contains(trim, "depends on missing #") {
			refRows = append(refRows, trim)
		}
	}
	expected := []string{
		"archive task #1 depends on missing #97",
		"archive task #1 depends on missing #98",
		"archive task #2 depends on missing #96",
		"archive task #3 depends on missing #99",
	}
	if len(refRows) != len(expected) {
		t.Fatalf("expected %d ref rows, got %d:\n%v", len(expected), len(refRows), refRows)
	}
	for i, want := range expected {
		if refRows[i] != want {
			t.Errorf("row[%d] = %q, want %q", i, refRows[i], want)
		}
	}
}
