package commands

import (
	"fmt"
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
// The save path passes through s.Save, which takes a .bak snapshot
// before writing — so `tsk undo-last` can revert the autofix in
// one call if it did something the user didn't want.
func applyLintAutofixAll(path string, report LintReport) (int, error) {
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
	// Save unconditionally if there were any findings (or any
	// semantic repairs queued) — even a clean re-render produces a
	// .bak snapshot that lets the user revert.
	if err := s.Save(); err != nil {
		return repairs, fmt.Errorf("save autofix-all: %w", err)
	}
	return repairs, nil
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
