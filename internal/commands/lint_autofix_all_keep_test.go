package commands

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestLintAutofixAllKeepRequiresBackup: --keep without --backup is a
// usage error (it operates on the backup chain — without a chain
// there's nothing to prune).
func TestLintAutofixAllKeepRequiresBackup(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] x <!-- id:1 prio:medium -->\n")
	_, _, err := runCmd(t, dir, "lint", "--autofix-all", "--keep", "3")
	if err == nil {
		t.Fatal("expected error for --keep without --backup")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestLintAutofixAllKeepRejectsNegative: --keep with a negative
// integer is a usage error. Zero (the default) means "keep all".
func TestLintAutofixAllKeepRejectsNegative(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "bak")
	writeRawFile(t, dir, "# tasks\n\n- [ ] x <!-- id:1 prio:medium -->\n")
	_, _, err := runCmd(t, dir, "lint", "--autofix-all", "--backup", backupDir, "--keep", "-1")
	if err == nil {
		t.Fatal("expected error for negative --keep")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestPruneBackupChainTrimsOldest: build a directory with 5 fake
// snapshots having distinct timestamps; prune keep=2; verify only
// the 2 NEWEST remain and the 3 oldest are gone. Direct unit test
// of pruneBackupChain — no test-time sleep needed because we
// fabricate the chain ourselves.
func TestPruneBackupChainTrimsOldest(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "bak")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	srcPath := filepath.Join(dir, ".tsk.md")
	// 5 distinct stamps in chronological order.
	stamps := []string{
		"20260101-120000",
		"20260102-120000",
		"20260103-120000",
		"20260104-120000",
		"20260105-120000",
	}
	for _, s := range stamps {
		name := ".tsk.md.bak." + s
		if err := os.WriteFile(filepath.Join(backupDir, name), []byte("snapshot "+s), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := pruneBackupChain(srcPath, backupDir, 2); err != nil {
		t.Fatalf("prune: %v", err)
	}
	left, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	names := make([]string, 0, len(left))
	for _, e := range left {
		names = append(names, e.Name())
	}
	if len(names) != 2 {
		t.Fatalf("want 2 remaining, got %d: %v", len(names), names)
	}
	// Newest 2 are 20260105-120000 and 20260104-120000.
	want := []string{
		".tsk.md.bak.20260104-120000",
		".tsk.md.bak.20260105-120000",
	}
	sort.Strings(names)
	for i, w := range want {
		if names[i] != w {
			t.Errorf("kept[%d]: want %s, got %s", i, w, names[i])
		}
	}
}

// TestPruneBackupChainNoOpWhenUnderLimit: 2 snapshots, keep=5 →
// nothing removed.
func TestPruneBackupChainNoOpWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "bak")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, s := range []string{"20260101-120000", "20260102-120000"} {
		if err := os.WriteFile(filepath.Join(backupDir, ".tsk.md.bak."+s), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	srcPath := filepath.Join(dir, ".tsk.md")
	if err := pruneBackupChain(srcPath, backupDir, 5); err != nil {
		t.Fatalf("prune: %v", err)
	}
	left, _ := os.ReadDir(backupDir)
	if len(left) != 2 {
		t.Errorf("want 2 untouched, got %d", len(left))
	}
}

// TestPruneBackupChainKeepZeroIsNoOp: keep=0 means "keep all"
// (historical default behavior). The chain is untouched.
func TestPruneBackupChainKeepZeroIsNoOp(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "bak")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, s := range []string{"20260101-120000", "20260102-120000", "20260103-120000"} {
		if err := os.WriteFile(filepath.Join(backupDir, ".tsk.md.bak."+s), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	srcPath := filepath.Join(dir, ".tsk.md")
	if err := pruneBackupChain(srcPath, backupDir, 0); err != nil {
		t.Fatalf("prune keep=0: %v", err)
	}
	left, _ := os.ReadDir(backupDir)
	if len(left) != 3 {
		t.Errorf("want 3 untouched on keep=0, got %d", len(left))
	}
}

// TestPruneBackupChainIgnoresUnrelatedFiles: a non-matching file
// (e.g. a README, or another tool's backup) in the backup dir is
// preserved even when the chain is over-limit. We only prune our
// own naming pattern.
func TestPruneBackupChainIgnoresUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "bak")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// 3 of ours, plus README.md and someone-elses.bak.
	for _, s := range []string{"20260101-120000", "20260102-120000", "20260103-120000"} {
		if err := os.WriteFile(filepath.Join(backupDir, ".tsk.md.bak."+s), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(backupDir, "README.md"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "other-tool.bak"), []byte("foreign"), 0o644); err != nil {
		t.Fatalf("write foreign: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, ".tsk.md.bak.keep-forever"), []byte("user-renamed"), 0o644); err != nil {
		t.Fatalf("write user-renamed: %v", err)
	}
	srcPath := filepath.Join(dir, ".tsk.md")
	if err := pruneBackupChain(srcPath, backupDir, 1); err != nil {
		t.Fatalf("prune: %v", err)
	}
	left, _ := os.ReadDir(backupDir)
	names := make([]string, 0, len(left))
	for _, e := range left {
		names = append(names, e.Name())
	}
	// Should have: 1 of ours (newest) + README.md + other-tool.bak +
	// .tsk.md.bak.keep-forever = 4 total.
	if len(names) != 4 {
		t.Fatalf("want 4 surviving (1 ours + 3 unrelated), got %d: %v", len(names), names)
	}
	// Verify the unrelated files survived.
	for _, want := range []string{"README.md", "other-tool.bak", ".tsk.md.bak.keep-forever", ".tsk.md.bak.20260103-120000"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %q from surviving files: %v", want, names)
		}
	}
	// The two older snapshots should be gone.
	for _, gone := range []string{".tsk.md.bak.20260101-120000", ".tsk.md.bak.20260102-120000"} {
		for _, n := range names {
			if n == gone {
				t.Errorf("expected %q pruned, still present", gone)
			}
		}
	}
}

// TestPruneBackupChainMissingDirNoOp: pruning a nonexistent
// backupDir is a silent no-op (not an error). Matches the
// "first run bootstraps the dir" ergonomic.
func TestPruneBackupChainMissingDirNoOp(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, ".tsk.md")
	missing := filepath.Join(dir, "does-not-exist")
	if err := pruneBackupChain(srcPath, missing, 3); err != nil {
		t.Errorf("missing dir should be silent no-op, got %v", err)
	}
}

// TestIsStampSuffixValidation: only "YYYYMMDD-HHMMSS" passes (the
// exact 15-char digit/hyphen shape). Other shapes are rejected so
// a user-renamed file ".tsk.md.bak.keep-forever" or
// ".tsk.md.bak.OLD" is preserved.
func TestIsStampSuffixValidation(t *testing.T) {
	good := []string{
		"20260101-120000",
		"20991231-235959",
	}
	for _, s := range good {
		if !isStampSuffix(s) {
			t.Errorf("expected %q to be a valid stamp", s)
		}
	}
	bad := []string{
		"",
		"abc",
		"20260101120000",   // missing hyphen
		"20260101-12000",   // 14 chars
		"20260101-1200000", // 16 chars
		"keep-forever",
		"YYYYMMDD-HHMMSS",
	}
	for _, s := range bad {
		if isStampSuffix(s) {
			t.Errorf("expected %q to be REJECTED", s)
		}
	}
}

// TestSortStringsDescending: in-place sort, newest (lex-max) first.
func TestSortStringsDescending(t *testing.T) {
	in := []string{"b", "a", "c"}
	sortStringsDescending(in)
	want := []string{"c", "b", "a"}
	for i, w := range want {
		if in[i] != w {
			t.Errorf("at[%d]: want %s, got %s", i, w, in[i])
		}
	}
}

// TestLintAutofixAllKeepEndToEnd: an end-to-end test that goes
// through the CLI surface: fabricate 4 fake backups, then run
// autofix-all with --keep 2. The new snapshot is added (=5 total
// before prune), then pruned to 2 retained. We pre-populate the
// chain to avoid needing sleeps between successive autofixes.
func TestLintAutofixAllKeepEndToEnd(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "bak")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// 4 fake older snapshots.
	for _, s := range []string{
		"20260101-120000",
		"20260102-120000",
		"20260103-120000",
		"20260104-120000",
	} {
		if err := os.WriteFile(filepath.Join(backupDir, ".tsk.md.bak."+s), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	// Create a fresh .tsk.md with a finding so autofix runs.
	writeRawFile(t, dir, "# tasks\n\n- [ ] no-created <!-- id:1 prio:medium -->\n")
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all", "--backup", backupDir, "--keep", "2"); err != nil {
		t.Fatalf("autofix --keep 2: %v", err)
	}
	left, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(left) != 2 {
		names := make([]string, 0, len(left))
		for _, e := range left {
			names = append(names, e.Name())
		}
		t.Fatalf("want 2 remaining after --keep 2, got %d: %v", len(left), names)
	}
	// One of the survivors should be the NEW snapshot (today's date),
	// the other should be 20260104 (the newest of the pre-fabricated
	// chain). Older 20260101-20260103 should be pruned.
	hasJan04 := false
	for _, e := range left {
		if strings.Contains(e.Name(), "20260104-120000") {
			hasJan04 = true
		}
		// The 3 oldest must NOT remain.
		for _, gone := range []string{"20260101-120000", "20260102-120000", "20260103-120000"} {
			if strings.Contains(e.Name(), gone) {
				t.Errorf("%q should be pruned, still present", e.Name())
			}
		}
	}
	if !hasJan04 {
		t.Errorf("newest pre-fab snapshot 20260104 should survive --keep 2")
	}
}
