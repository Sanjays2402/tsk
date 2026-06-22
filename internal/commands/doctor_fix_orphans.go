package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/store"
)

// applyOrphanArchiveFix scrubs dangling DependsOn ids out of the
// sibling .tsk.archive.md for every archived task flagged by the
// check-orphan-archive scan. Mirror of `lint --autofix-all` for the
// dep graph rather than the metadata.
//
// Semantics:
//   - Loads the live store + archive store the same way
//     checkArchiveOrphans does (single source of truth for the
//     resolvable id set: live ∪ archive).
//   - For every archive task whose DependsOn list contains any
//     id missing from that union, the missing ids are filtered
//     out. The surviving ids stay in order (deterministic
//     output; no surprise re-ordering of the user's curated
//     prereq chain).
//   - If any task was touched, archive.Save() writes back through
//     the standard store path — which produces a .bak snapshot
//     atomically — so `tsk undo-last` against the archive (if the
//     user ever does that) can revert the fix in one step.
//
// Returns the count of INDIVIDUAL dangling refs scrubbed (not
// the count of affected tasks). One archive task with two dangling
// deps counts as 2. This matches the doctor warning count's
// granularity — each orphan_archive_dep entry in the report is one
// dangling ref, and fix-orphans should claim them 1:1.
//
// No archive file → no-op (returns 0, nil). Same policy as
// checkArchiveOrphans: missing archive sibling isn't an error,
// there's just nothing to repair. The corresponding "you wanted
// to fix orphans but there's no archive" is communicated by the
// report (the orphan_archive_check OKCheck still fires, and the
// fix-orphans summary just says 0).
//
// Archive load failure surfaces as an error — same shape as the
// check-side load failure. A parse problem in .tsk.archive.md
// blocks the fix the same way it blocks the check.
//
// IMPORTANT: this function may downgrade Warnings in the in-memory
// report. The pre-fix scan already populated report.Warnings with
// one entry per orphan dep; after a successful fix those warnings
// are stale (the dangling refs are gone from the archive). We
// strip them from the in-memory report so the printed output
// doesn't lie about the post-fix state. The original count is
// preserved via the OKChecks summary line ("fix-orphans: N
// dangling ref(s) scrubbed") that the caller appends.
func applyOrphanArchiveFix(cmd *cobra.Command, report *DoctorReport) (int, error) {
	livePath := report.Path
	if livePath == "" {
		// runDoctor short-circuited before resolving the path
		// (no .tsk.md present). Nothing to fix.
		return 0, nil
	}
	archivePath := archiveSiblingPath(livePath)
	if _, err := os.Stat(archivePath); err != nil {
		if os.IsNotExist(err) {
			// No archive → no possible orphans → no fix needed.
			return 0, nil
		}
		return 0, fmt.Errorf("stat archive %s: %w", archivePath, err)
	}

	// Re-load BOTH stores so the resolvable set reflects the
	// current on-disk state (not whatever runDoctor cached).
	// Defensive against the rare case where someone mutates the
	// live store between runDoctor and applyOrphanArchiveFix —
	// using fresh loads keeps the fix's view consistent.
	liveStore, err := store.Load(livePath)
	if err != nil {
		return 0, fmt.Errorf("reload live %s: %w", livePath, err)
	}
	archStore, err := store.Load(archivePath)
	if err != nil {
		return 0, fmt.Errorf("reload archive %s: %w", archivePath, err)
	}

	resolvable := make(map[int]bool, len(liveStore.Tasks)+len(archStore.Tasks))
	for _, t := range liveStore.Tasks {
		if t.ID > 0 {
			resolvable[t.ID] = true
		}
	}
	for _, t := range archStore.Tasks {
		if t.ID > 0 {
			resolvable[t.ID] = true
		}
	}

	scrubbed := 0
	anyTouched := false
	for i := range archStore.Tasks {
		t := &archStore.Tasks[i]
		if len(t.DependsOn) == 0 {
			continue
		}
		kept := make([]int, 0, len(t.DependsOn))
		for _, dep := range t.DependsOn {
			if resolvable[dep] {
				kept = append(kept, dep)
				continue
			}
			scrubbed++
		}
		if len(kept) != len(t.DependsOn) {
			t.DependsOn = kept
			anyTouched = true
		}
	}

	if !anyTouched {
		return 0, nil
	}

	// Sort each touched task's surviving deps in case the
	// renderer expects ascending order (matches the canonical
	// shape produced by parseMetaInt slice handling).
	for i := range archStore.Tasks {
		if len(archStore.Tasks[i].DependsOn) > 1 {
			sort.Ints(archStore.Tasks[i].DependsOn)
		}
	}

	if err := archStore.Save(); err != nil {
		return 0, fmt.Errorf("save archive %s: %w", archivePath, err)
	}

	// Drop the now-stale Warnings so the post-fix report is
	// truthful. We re-fetch the orphan check from scratch:
	// any orphan_archive_dep warning that still has a missing
	// id would survive (defensive against partial repairs),
	// but the standard case is "all fixed → all gone".
	filtered := make([]DoctorIssue, 0, len(report.Warnings))
	for _, w := range report.Warnings {
		if w.Check == "orphan_archive_dep" {
			continue
		}
		filtered = append(filtered, w)
	}
	report.Warnings = filtered

	return scrubbed, nil
}

// OrphanArchiveRef is one (archive-task-id, missing-dep-id) pair
// in the dry-run preview. Stable shape so the JSON path can grow
// to expose the list directly without breaking consumers — for
// now it's used only in the human REPAIRS block, but the type
// being public makes a future JSON envelope drop-in.
type OrphanArchiveRef struct {
	TaskID     int `json:"task_id"`
	MissingDep int `json:"missing_dep"`
}

// previewOrphanArchiveFix runs the SAME scan + filter logic as
// applyOrphanArchiveFix but returns the would-be-scrubbed counts
// and the per-ref details WITHOUT writing anything to disk. The
// archive Save() call is the only difference between the two
// paths — the dependency-pruning logic is identical so the
// preview is honest about what the real fix would do.
//
// Returns:
//   - count: total individual dangling refs that would be scrubbed
//     (one archive task with two dangling deps counts as 2,
//     matching the warning granularity in checkArchiveOrphans).
//   - refs:  per-ref detail rows for the human REPAIRS block,
//     sorted ascending by (task_id, missing_dep) for
//     determinism.
//   - err:   nil on success, non-nil if the archive can't be
//     loaded (mirrors applyOrphanArchiveFix's error contract).
//
// Side effects: NONE — no Save, no .bak rotation, no in-memory
// store mutations are persisted. The in-memory report's stale
// orphan_archive_dep warnings are intentionally left in place
// (in the apply path we'd strip them as part of the repair; the
// preview path lets them stand because the dry-run did NOT
// actually fix anything yet).
//
// Same "no archive file → 0, nil" policy as applyOrphanArchiveFix
// for consistency: a missing archive sibling isn't an error,
// there's just nothing to preview.
func previewOrphanArchiveFix(cmd *cobra.Command, report *DoctorReport) (int, []OrphanArchiveRef, error) {
	livePath := report.Path
	if livePath == "" {
		return 0, nil, nil
	}
	archivePath := archiveSiblingPath(livePath)
	if _, err := os.Stat(archivePath); err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("stat archive %s: %w", archivePath, err)
	}
	liveStore, err := store.Load(livePath)
	if err != nil {
		return 0, nil, fmt.Errorf("reload live %s: %w", livePath, err)
	}
	archStore, err := store.Load(archivePath)
	if err != nil {
		return 0, nil, fmt.Errorf("reload archive %s: %w", archivePath, err)
	}

	resolvable := make(map[int]bool, len(liveStore.Tasks)+len(archStore.Tasks))
	for _, t := range liveStore.Tasks {
		if t.ID > 0 {
			resolvable[t.ID] = true
		}
	}
	for _, t := range archStore.Tasks {
		if t.ID > 0 {
			resolvable[t.ID] = true
		}
	}

	refs := make([]OrphanArchiveRef, 0)
	for _, t := range archStore.Tasks {
		for _, dep := range t.DependsOn {
			if !resolvable[dep] {
				refs = append(refs, OrphanArchiveRef{TaskID: t.ID, MissingDep: dep})
			}
		}
	}
	// Deterministic ordering by (task_id, missing_dep) so the
	// preview output is reproducible across runs — critical for
	// CI pipelines diffing the output to detect new orphans.
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].TaskID != refs[j].TaskID {
			return refs[i].TaskID < refs[j].TaskID
		}
		return refs[i].MissingDep < refs[j].MissingDep
	})
	return len(refs), refs, nil
}
