package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// applyLintAutofixAll is the multi-step repair pipeline for `tsk
// lint --autofix-all`. It builds on `applyLintFix`'s canonical
// round-trip (which already fixes non-canonical bullets, unknown
// meta keys, and stray notes-shaped lines) by ALSO repairing
// findings that the round-trip can't address by itself.
//
// Currently the only semantic-repair finding is
// missing_created_timestamp — a task with no created: meta key,
// usually because someone hand-edited the file or imported tasks
// from a tool that doesn't track creation time. The fix is to
// stamp every such task with time.Now() and re-save through the
// canonical writer. That makes the task visible to time-bucketed
// views (tsk last, tsk log, tsk yesterday) that currently skip
// tasks with zero Created.
//
// Why now() and not file mtime? Because the very next thing we do
// is save the file, which overwrites mtime to now() anyway. Either
// value would converge on the same outcome; now() is the simpler,
// more honest choice (the backfill happened JUST NOW). The
// resulting timestamps are NOT actual creation times — they're
// best-effort "we have no idea, here's at least something for
// time-bucketing to work" stamps. The lint finding for
// missing_created_timestamp is documented as a soft warning, so
// this is the natural autofix.
//
// Returns the count of repairs applied so the calling command can
// report a useful summary (e.g. "autofixed: 3 repair(s) applied").
// The canonical re-render itself counts as one repair if there
// were any round-trippable findings (non_canonical_task_line,
// unknown_meta_key, stray_notes_before_task) — without it the
// "fixed: 0 repairs" output would be misleading after the user
// asked for autofix-all on a file that did have findings.
//
// backupDir, when non-empty, redirects the pre-write snapshot
// from the default in-place "<Path>.bak" to a timestamped file
// inside backupDir. The in-place .bak that s.Save still creates
// (because Save's snapshot is unconditional) is removed afterward
// so the working tree stays clean — useful in pre-commit setups
// where stray .bak files would show up as untracked. This trades
// off the single-step "tsk undo-last" path (which reads the
// in-place .bak) for working-tree cleanliness; pre-commit users
// don't need undo-last (the commit itself is the rollback handle),
// so the trade is an explicit opt-in via --backup.
//
// The save path passes through s.Save, which takes a .bak snapshot
// before writing — so `tsk undo-last` can revert the autofix in
// one call if it did something the user didn't want.
func applyLintAutofixAll(path string, report LintReport, backupDir string) (int, error) {
	s, err := store.Load(path)
	if err != nil {
		return 0, fmt.Errorf("reload for autofix-all: %w", err)
	}
	repairs := 0
	// Count the semantic repairs FIRST, before re-saving (the
	// canonical writer would otherwise hide a finding we should
	// still credit). Backfill missing_created_timestamp findings
	// by stamping the matching task with now().
	now := time.Now()
	for _, finding := range report.Findings {
		if finding.Check != "missing_created_timestamp" || finding.TaskID == 0 {
			continue
		}
		t := s.ByID(finding.TaskID)
		if t == nil {
			// Defensive: a finding should always have a valid task
			// id when Check=missing_created_timestamp (scan
			// produced it from the same store), but if the store
			// changed between lint and autofix (unlikely; we hold
			// no lock), skip rather than crash.
			continue
		}
		if !t.Created.IsZero() {
			// Already fixed somehow between lint and autofix —
			// don't re-stamp.
			continue
		}
		t.Created = now
		repairs++
	}
	// Round-trippable findings — any of these counts as one
	// "canonicalize" repair regardless of count (a single save
	// fixes them all). The user-visible message will still read
	// truthfully because we only credit one repair for the whole
	// re-render.
	if hasRoundTrippableFindings(report) {
		repairs++
	}
	// If --backup is set, snapshot the pre-save contents to the
	// timestamped explicit-backup path FIRST. Done before Save()
	// so the explicit backup captures the same bytes Save() will
	// snapshot into the in-place .bak — both reflect the source-
	// of-truth pre-fix state of the file.
	if backupDir != "" {
		if err := snapshotToBackupDir(path, backupDir, now); err != nil {
			return repairs, fmt.Errorf("snapshot to --backup dir: %w", err)
		}
	}
	// Save unconditionally if there were any findings (or any
	// semantic repairs queued) — even a clean re-render produces a
	// .bak snapshot that lets the user revert.
	if err := s.Save(); err != nil {
		return repairs, fmt.Errorf("save autofix-all: %w", err)
	}
	// If --backup was set, remove the in-place .bak that Save just
	// produced so the working tree stays clean. The explicit
	// backup (taken above) is the user's rollback handle.
	if backupDir != "" {
		_ = os.Remove(path + ".bak")
	}
	return repairs, nil
}

// snapshotToBackupDir copies the current on-disk contents of path
// into backupDir as a timestamped .bak file. The dir is created
// (with parents) if it doesn't exist — that's almost always what
// pre-commit setups want (the dir might be project-relative and
// the first run is the bootstrap).
//
// Naming: "<base>.bak.YYYYMMDD-HHMMSS". The timestamp prefix sorts
// lexicographically by time so `ls backup/` reads in chronological
// order, and the inclusion of date+time means consecutive autofix-
// all calls within the same minute don't collide (different
// timestamps even at second granularity).
//
// Returns nil if the source path doesn't exist (mirrors Save()'s
// snapshotPrevious policy: nothing to back up is not an error).
// Other IO failures DO surface — the caller decides whether to
// abort the autofix or proceed without a backup; current callers
// abort, because a silent missing backup defeats the whole point
// of the flag.
func snapshotToBackupDir(path, backupDir string, now time.Time) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read source for backup: %w", err)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("create backup dir %q: %w", backupDir, err)
	}
	base := filepath.Base(path)
	stamp := now.Format("20060102-150405")
	dest := filepath.Join(backupDir, fmt.Sprintf("%s.bak.%s", base, stamp))
	if err := store.AtomicWriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("write backup %q: %w", dest, err)
	}
	return nil
}

// hasRoundTrippableFindings returns true if the report contains
// any finding the canonical writer would resolve on its own (the
// same checks `tsk lint --fix` addresses). Used to decide whether
// to credit a "canonicalize" repair in the autofix-all summary.
func hasRoundTrippableFindings(r LintReport) bool {
	for _, f := range r.Findings {
		switch f.Check {
		case "non_canonical_task_line", "unknown_meta_key", "stray_notes_before_task":
			return true
		}
	}
	return false
}

// lintAutofixDoc is the JSON envelope for `tsk lint --autofix-all
// --json`. Stable schema combining BOTH the read-side findings
// scan AND the write-side repair summary in one document, so a
// pre-commit / CI hook reads a single coherent signal:
//
//   - path             : the .tsk.md that was scanned/repaired
//   - findings_count   : how many issues the scan found
//   - findings         : the full LintFinding list (same shape
//     as plain --json so consumers of the
//     read-only path can reuse selectors)
//   - repairs_applied  : how many repairs the autofix-all pass
//     wrote (0 when findings was empty)
//   - backup_dir       : the explicit backup directory if --backup
//     was set; omitted otherwise. Useful so a
//     post-hook can locate the rollback handle
//     without re-parsing argv.
//
// Empty findings emits `findings: []` (not null) so jq pipelines
// don't crash. repairs_applied is always emitted even when zero
// (it's the headline number; omitting it would hide success).
type lintAutofixDoc struct {
	Path           string        `json:"path"`
	FindingsCount  int           `json:"findings_count"`
	Findings       []LintFinding `json:"findings"`
	RepairsApplied int           `json:"repairs_applied"`
	BackupDir      string        `json:"backup_dir,omitempty"`
}

// emitLintAutofixJSON renders the combined findings+repairs envelope
// for `tsk lint --autofix-all --json`. The schema is one document
// so the entire pre-commit signal can be consumed by a single jq
// pass — no need to interleave the read-only --json output and
// the per-line "autofixed: ... (N repair(s) applied)" summary,
// which would require fragile text-parsing on the consumer side.
//
// findings is the PRE-fix list — what the scan saw before the
// repair pass ran. That's the useful signal for CI: \"the file
// had these problems and we auto-corrected them.\" Re-scanning
// after the fix would yield an empty list (the point of fix is
// to clear the report), which would be a less useful surface for
// pre-commit verification.
func emitLintAutofixJSON(w io.Writer, report LintReport, applied int, backupDir string) error {
	findings := report.Findings
	if findings == nil {
		findings = []LintFinding{}
	}
	doc := lintAutofixDoc{
		Path:           report.Path,
		FindingsCount:  len(findings),
		Findings:       findings,
		RepairsApplied: applied,
		BackupDir:      backupDir,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
