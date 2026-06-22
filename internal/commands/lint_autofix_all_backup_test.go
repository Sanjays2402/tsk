package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLintAutofixAllBackupWritesTimestampedSnapshot: with --backup
// <dir>, the pre-fix snapshot lands in <dir> as a timestamped .bak
// file (YYYYMMDD-HHMMSS suffix). The in-place .bak alongside the
// source file is REMOVED so the working tree stays clean — the
// whole point of --backup is pre-commit setups where stray .bak
// files would show up as untracked.
func TestLintAutofixAllBackupWritesTimestampedSnapshot(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	writeRawFile(t, dir, "# tasks\n\n- [ ] no-created <!-- id:1 prio:medium -->\n")
	out, _, err := runCmd(t, dir, "lint", "--autofix-all", "--backup", backupDir)
	if err != nil {
		t.Fatalf("autofix-all --backup: %v", err)
	}
	if !strings.Contains(out, "backup ->") {
		t.Fatalf("expected 'backup ->' annotation in summary, got:\n%s", out)
	}
	if !strings.Contains(out, backupDir) {
		t.Fatalf("expected backup dir path in summary, got:\n%s", out)
	}
	// In-place .bak must NOT exist (the whole point).
	if _, err := os.Stat(dir + "/.tsk.md.bak"); err == nil {
		t.Fatalf(".tsk.md.bak should NOT exist after --backup (working tree must stay clean)")
	}
	// Backup dir should now contain exactly one timestamped .bak.
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup file, got %d entries", len(entries))
	}
	name := entries[0].Name()
	if !strings.HasPrefix(name, ".tsk.md.bak.") {
		t.Fatalf("expected '.tsk.md.bak.<ts>' name, got %q", name)
	}
	// Timestamp suffix should be 8 digits + dash + 6 digits (15 chars).
	suffix := strings.TrimPrefix(name, ".tsk.md.bak.")
	if len(suffix) != 15 || suffix[8] != '-' {
		t.Fatalf("expected YYYYMMDD-HHMMSS suffix, got %q", suffix)
	}
}

// TestLintAutofixAllBackupContentIsPreFix: the backup file contains
// the PRE-fix bytes (matching the in-place .bak contract). The fix
// itself updates the source path; the backup is the rollback handle.
func TestLintAutofixAllBackupContentIsPreFix(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	preContent := "# tasks\n\n- [ ] no-created <!-- id:1 prio:medium -->\n"
	writeRawFile(t, dir, preContent)
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all", "--backup", backupDir); err != nil {
		t.Fatalf("autofix-all --backup: %v", err)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup file, got %d", len(entries))
	}
	backupPath := filepath.Join(backupDir, entries[0].Name())
	got, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	// Backup must reflect the pre-fix bytes (no created: stamp).
	if string(got) != preContent {
		t.Fatalf("backup is NOT pre-fix bytes\nWANT:\n%s\nGOT:\n%s", preContent, got)
	}
}

// TestLintAutofixAllBackupCreatesParentDir: --backup dir doesn't
// exist yet — autofix should create it (with parents). That's the
// natural pre-commit ergonomics ("first run bootstraps the backup
// directory"), not a hard error.
func TestLintAutofixAllBackupCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	// Nested non-existent path.
	backupDir := filepath.Join(dir, "deep", "nested", "backups")
	writeRawFile(t, dir, "# tasks\n\n- [ ] x <!-- id:1 prio:medium -->\n")
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all", "--backup", backupDir); err != nil {
		t.Fatalf("autofix-all --backup: %v", err)
	}
	if _, err := os.Stat(backupDir); err != nil {
		t.Fatalf("backup dir should have been created, got: %v", err)
	}
}

// TestLintAutofixAllBackupRequiresAutofixAll: --backup without
// --autofix-all is a usage error — there's nothing to back up
// because no write happens on the read-only lint path.
func TestLintAutofixAllBackupRequiresAutofixAll(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	writeRawFile(t, dir, "# tasks\n\n- [ ] x <!-- id:1 prio:medium -->\n")
	_, _, err := runCmd(t, dir, "lint", "--backup", backupDir)
	if err == nil {
		t.Fatal("expected error for --backup without --autofix-all")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestLintAutofixAllBackupRequiresAutofixAllEvenWithFix: --backup
// + --fix (without --autofix-all) is still rejected. --fix doesn't
// take a backup parameter; we don't want to silently accept a
// flag that has no effect.
func TestLintAutofixAllBackupRequiresAutofixAllEvenWithFix(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	writeRawFile(t, dir, "# tasks\n\n* [ ] x <!-- id:1 prio:medium created:2026-01-01T00:00:00Z -->\n")
	_, _, err := runCmd(t, dir, "lint", "--fix", "--backup", backupDir)
	if err == nil {
		t.Fatal("expected error for --backup with --fix (but no --autofix-all)")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestLintAutofixAllBackupTwoRunsTwoFiles: consecutive --backup
// autofixes (each on a NEW source file with findings) produce two
// distinct .bak files in the dir, so a backup chain accumulates
// without overwriting older snapshots. Same-source successive runs
// require findings each time to trigger a backup; we test the
// distinct-content case to avoid timing/idempotency interference.
func TestLintAutofixAllBackupTwoRunsTwoFiles(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	writeRawFile(t, dir, "# tasks\n\n- [ ] one <!-- id:1 prio:medium -->\n")
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all", "--backup", backupDir); err != nil {
		t.Fatalf("first autofix: %v", err)
	}
	// Wait a beat to guarantee a different timestamp (the suffix
	// resolves to seconds; consecutive calls in <1s could collide
	// in tight test environments). Force a second of difference.
	time.Sleep(1100 * time.Millisecond)
	// Re-introduce a finding by rewriting the file with a fresh
	// missing-created task.
	writeRawFile(t, dir, "# tasks\n\n- [ ] two <!-- id:2 prio:medium -->\n")
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all", "--backup", backupDir); err != nil {
		t.Fatalf("second autofix: %v", err)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected 2 distinct backups, got %d: %v", len(entries), names)
	}
}

// TestLintAutofixAllNoBackupDefaultBakStays: WITHOUT --backup, the
// behavior is unchanged — in-place .tsk.md.bak still exists for
// `tsk undo-last` to read. Regression guard against the --backup
// removal logic accidentally firing in the default code path.
func TestLintAutofixAllNoBackupDefaultBakStays(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "# tasks\n\n- [ ] no-created <!-- id:1 prio:medium -->\n")
	if _, _, err := runCmd(t, dir, "lint", "--autofix-all"); err != nil {
		t.Fatalf("autofix-all (no backup): %v", err)
	}
	if _, err := os.Stat(dir + "/.tsk.md.bak"); err != nil {
		t.Fatalf(".tsk.md.bak SHOULD exist without --backup, got: %v", err)
	}
}
