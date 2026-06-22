package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newDoctorCmd implements `tsk doctor`: a one-shot health check that scans
// the active .tsk.md for parse problems, duplicate IDs, malformed metadata,
// and unresolved timezone configuration. Exits non-zero when issues found,
// so it can be wired into pre-commit / CI hooks.
func newDoctorCmd() *cobra.Command {
	var (
		asJSON             bool
		checkOrphanArchive bool
		fixOrphans         bool
	)
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the active .tsk.md and tsk configuration",
		Long: `Run a health check against the active tsk environment.

Checks performed:
  - File exists and is readable
  - File parses without error
  - No duplicate task IDs
  - All due / created / completed timestamps are valid RFC3339
  - All task IDs are positive integers (or zero for ID-less tasks)
  - Resolved timezone (TSK_TZ override or system default)
  - No tasks with empty titles

Pass --check-orphan-archive to ALSO load the sibling
.tsk.archive.md and scan it for archived tasks whose
DependsOn references resolve in NEITHER the live store NOR
the archive itself — i.e. dangling references that survived
a hand-edit or a partial rollback. This is the corruption
canary for long-running projects: a stale dep id usually
means an archived prereq was deleted (rather than properly
re-archived), leaving an orphan pointer behind. The active
store doesn't surface these because it doesn't see the
archive; doctor unifies the view so the rot is visible.

Pass --fix-orphans WITH --check-orphan-archive to ALSO repair
the dangling references in place: scrub the orphan ids out of
every affected archive task's DependsOn list, then re-save the
archive (with a .bak snapshot, same contract as every other
write path). The archive itself is left otherwise untouched —
only the dangling deps are removed. Mirror of lint
--autofix-all but for the archive's dep graph rather than the
live store's metadata.

Exit codes:
  0  all checks passed
  1  at least one issue found (error)
  2  bad invocation or transient IO failure

Pass --json for machine-readable output (always emitted, exit code still
reflects severity).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fixOrphans && !checkOrphanArchive {
				return usageErrorf("--fix-orphans requires --check-orphan-archive (the orphan scan must run first to produce the repair set)")
			}
			report := runDoctor(cmd, checkOrphanArchive)
			// --fix-orphans repair runs BEFORE we print, so the
			// printed report reflects the post-fix state. The
			// repair count is folded into the report's OKChecks
			// line so the user sees "fix-orphans: N dangling
			// refs scrubbed" alongside the standard checks.
			scrubbed := 0
			if fixOrphans {
				n, err := applyOrphanArchiveFix(cmd, &report)
				if err != nil {
					return fmt.Errorf("fix-orphans: %w", err)
				}
				scrubbed = n
				report.OKChecks = append(report.OKChecks, fmt.Sprintf("fix-orphans: %d dangling ref(s) scrubbed", n))
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				printDoctorReport(cmd.OutOrStdout(), report)
				// Surface the fix-orphans summary line in the
				// human-readable path. The JSON path already
				// carries it in OKChecks; in the human path we
				// print an explicit "REPAIRS:" block so the
				// signal isn't buried.
				if fixOrphans {
					pf(cmd.OutOrStdout(), "REPAIRS:\n  fix-orphans: %d dangling ref(s) scrubbed\n", scrubbed)
				}
			}
			if report.HasIssues() {
				return silentExit{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().BoolVar(&checkOrphanArchive, "check-orphan-archive", false, "also scan the sibling .tsk.archive.md for orphan DependsOn references that resolve in neither the live store nor the archive itself")
	cmd.Flags().BoolVar(&fixOrphans, "fix-orphans", false, "with --check-orphan-archive: scrub the dangling DependsOn refs from the archive and re-save (with .bak snapshot)")
	return cmd
}

// DoctorReport is the full output of `tsk doctor`. Stable schema — used by
// `--json` for scripts and CI gates.
type DoctorReport struct {
	Path     string        `json:"path"`
	Timezone string        `json:"timezone"`
	TaskCnt  int           `json:"task_count"`
	Errors   []DoctorIssue `json:"errors"`
	Warnings []DoctorIssue `json:"warnings"`
	OKChecks []string      `json:"ok_checks"`
}

// DoctorIssue describes a single problem found by a check.
type DoctorIssue struct {
	Check  string `json:"check"`
	Detail string `json:"detail"`
	TaskID int    `json:"task_id,omitempty"`
}

// HasIssues reports whether the report contains any error-level findings.
// Warnings do not flip the exit code.
func (r DoctorReport) HasIssues() bool {
	return len(r.Errors) > 0
}

// runDoctor performs every check and returns the structured report.
func runDoctor(cmd *cobra.Command, checkOrphanArchive bool) DoctorReport {
	tz := ResolveTZ()
	report := DoctorReport{
		Timezone: tz.String(),
		OKChecks: make([]string, 0, 6),
		Errors:   make([]DoctorIssue, 0),
		Warnings: make([]DoctorIssue, 0),
	}

	path, _ := cmd.Flags().GetString("file")
	if path == "" {
		resolved, ok := store.Resolve("")
		if !ok {
			report.Errors = append(report.Errors, DoctorIssue{
				Check:  "file_present",
				Detail: "no .tsk.md found; run `tsk init` first",
			})
			return report
		}
		path = resolved
	}
	report.Path = path

	if _, err := os.Stat(path); err != nil {
		report.Errors = append(report.Errors, DoctorIssue{
			Check:  "file_present",
			Detail: fmt.Sprintf("cannot stat %s: %v", path, err),
		})
		return report
	}
	report.OKChecks = append(report.OKChecks, "file_present")

	s, err := store.Load(path)
	if err != nil {
		report.Errors = append(report.Errors, DoctorIssue{
			Check:  "file_parses",
			Detail: fmt.Sprintf("parse error: %v", err),
		})
		return report
	}
	report.OKChecks = append(report.OKChecks, "file_parses")
	report.TaskCnt = len(s.Tasks)

	checkDuplicateIDs(&report, s.Tasks)
	checkTaskFields(&report, s.Tasks)
	checkTimestamps(&report, s.Tasks)
	checkTimezoneSanity(&report)
	if checkOrphanArchive {
		checkArchiveOrphans(&report, s, path)
	}
	return report
}

func checkDuplicateIDs(r *DoctorReport, tasks []model.Task) {
	seen := make(map[int][]int, len(tasks)) // id -> task indices
	for i, t := range tasks {
		if t.ID == 0 {
			continue
		}
		seen[t.ID] = append(seen[t.ID], i)
	}
	dupes := make([]int, 0)
	for id, idxs := range seen {
		if len(idxs) > 1 {
			dupes = append(dupes, id)
		}
	}
	if len(dupes) == 0 {
		r.OKChecks = append(r.OKChecks, "unique_ids")
		return
	}
	sort.Ints(dupes)
	for _, id := range dupes {
		r.Errors = append(r.Errors, DoctorIssue{
			Check:  "unique_ids",
			Detail: fmt.Sprintf("id #%d appears on %d tasks", id, len(seen[id])),
			TaskID: id,
		})
	}
}

func checkTaskFields(r *DoctorReport, tasks []model.Task) {
	emptyTitles := 0
	negativeIDs := 0
	for _, t := range tasks {
		if strings.TrimSpace(t.Title) == "" {
			emptyTitles++
			r.Warnings = append(r.Warnings, DoctorIssue{
				Check:  "non_empty_title",
				Detail: "task has empty or whitespace-only title",
				TaskID: t.ID,
			})
		}
		if t.ID < 0 {
			negativeIDs++
			r.Errors = append(r.Errors, DoctorIssue{
				Check:  "positive_ids",
				Detail: fmt.Sprintf("task has negative id (%d)", t.ID),
				TaskID: t.ID,
			})
		}
	}
	if emptyTitles == 0 {
		r.OKChecks = append(r.OKChecks, "non_empty_title")
	}
	if negativeIDs == 0 {
		r.OKChecks = append(r.OKChecks, "positive_ids")
	}
}

func checkTimestamps(r *DoctorReport, tasks []model.Task) {
	// The model already parses timestamps with RFC3339; Load() would have
	// errored if any were unparseable. So here we verify they're in a
	// sensible range — flag completion dates before creation, due dates
	// 100+ years in the past or future.
	ok := true
	now := time.Now()
	for _, t := range tasks {
		if t.Completed != nil && !t.Created.IsZero() && t.Completed.Before(t.Created) {
			ok = false
			r.Warnings = append(r.Warnings, DoctorIssue{
				Check: "timestamp_order",
				Detail: fmt.Sprintf("task #%d completed (%s) before created (%s)",
					t.ID, t.Completed.Format(time.RFC3339), t.Created.Format(time.RFC3339)),
				TaskID: t.ID,
			})
		}
		if t.Due != nil {
			yearDelta := t.Due.Year() - now.Year()
			if yearDelta < -100 || yearDelta > 100 {
				ok = false
				r.Warnings = append(r.Warnings, DoctorIssue{
					Check: "due_date_sanity",
					Detail: fmt.Sprintf("task #%d due date %s is implausibly far from now",
						t.ID, t.Due.Format(model.DateLayout)),
					TaskID: t.ID,
				})
			}
		}
	}
	if ok {
		r.OKChecks = append(r.OKChecks, "timestamp_sanity")
	}
}

func checkTimezoneSanity(r *DoctorReport) {
	// If the user set TSK_TZ, verify it parses (already does in ResolveTZ,
	// but record the source for the report).
	envTZ := strings.TrimSpace(os.Getenv("TSK_TZ"))
	if envTZ == "" {
		envTZ = "(system default)"
	}
	r.OKChecks = append(r.OKChecks, fmt.Sprintf("timezone:%s", envTZ))
}

// printDoctorReport renders a human-friendly summary.
func printDoctorReport(w io.Writer, r DoctorReport) {
	if r.Path != "" {
		pf(w, "file:    %s\n", r.Path)
	}
	pf(w, "tz:      %s\n", r.Timezone)
	pf(w, "tasks:   %d\n", r.TaskCnt)
	pln(w)
	if len(r.Errors) > 0 {
		pln(w, "ERRORS:")
		for _, e := range r.Errors {
			pf(w, "  ✗ %s: %s\n", e.Check, e.Detail)
		}
	}
	if len(r.Warnings) > 0 {
		pln(w, "WARNINGS:")
		for _, e := range r.Warnings {
			pf(w, "  ! %s: %s\n", e.Check, e.Detail)
		}
	}
	if len(r.Errors) == 0 && len(r.Warnings) == 0 {
		pln(w, "all checks passed ✓")
	}
}

// checkArchiveOrphans scans the sibling .tsk.archive.md for archived
// tasks whose DependsOn references resolve in NEITHER the live store
// (s) NOR the archive itself — i.e. dangling pointers that should
// have been cleaned up but weren't.
//
// This is the corruption canary for long-running projects:
//
//   - An archived prereq deleted (rather than properly re-archived)
//     leaves a hanging reference behind.
//   - A live-store task referenced by an archived "depends:" can be
//     deleted from the live store without any safeguard, leaving
//     the archive task pointing at nothing.
//   - A hand-edit of the archive file (e.g. an "oops, that one
//     wasn't supposed to be archived") can erase a task that other
//     archive entries pointed at.
//
// Behavior:
//   - Missing archive file: silent OK ("orphan_archive_check"
//     passes, because there's no archive to corrupt yet).
//   - Archive load failure: surfaced as an error (parse problem in
//     .tsk.archive.md is itself a corruption signal worth a
//     non-zero exit).
//   - Live archive: every archived task's DependsOn ids are checked
//     against (live.ByID ∪ archive.ByID); any id missing from BOTH
//     yields a Warning entry. (Warning, not Error, because the
//     archive is a write-rarely store — a dangling pointer is not
//     a parse failure; the user might prefer to leave it as a
//     historical artifact rather than be forced to fix it.)
//
// archivePath resolves to "<dir>/.tsk.archive.md" relative to the
// live-store's directory — the standard archive sibling location.
// We deliberately do NOT honor --merge-into here: doctor is a
// per-store health check, and the merge-into target is a shared
// rollup that a different store owns. Adding per-store inspection
// of merged archives would require an explicit secondary flag and
// a different command shape; the orphan-check use case is "check
// THIS project's archive sibling for rot" which is the 90% case.
func checkArchiveOrphans(r *DoctorReport, s *store.Store, livePath string) {
	archivePath := archiveSiblingPath(livePath)
	if _, err := os.Stat(archivePath); err != nil {
		if os.IsNotExist(err) {
			// No archive yet → no possible orphans. Pass.
			r.OKChecks = append(r.OKChecks, "orphan_archive_check")
			return
		}
		r.Errors = append(r.Errors, DoctorIssue{
			Check:  "orphan_archive_check",
			Detail: fmt.Sprintf("cannot stat archive %s: %v", archivePath, err),
		})
		return
	}
	arch, err := store.Load(archivePath)
	if err != nil {
		r.Errors = append(r.Errors, DoctorIssue{
			Check:  "orphan_archive_check",
			Detail: fmt.Sprintf("archive parse error %s: %v", archivePath, err),
		})
		return
	}
	// Build a single id-set spanning live + archive — any id in
	// either store is "resolvable" for the dangling check.
	resolvable := make(map[int]bool, len(s.Tasks)+len(arch.Tasks))
	for _, t := range s.Tasks {
		if t.ID > 0 {
			resolvable[t.ID] = true
		}
	}
	for _, t := range arch.Tasks {
		if t.ID > 0 {
			resolvable[t.ID] = true
		}
	}
	orphanFound := false
	for _, t := range arch.Tasks {
		for _, dep := range t.DependsOn {
			if !resolvable[dep] {
				orphanFound = true
				r.Warnings = append(r.Warnings, DoctorIssue{
					Check:  "orphan_archive_dep",
					Detail: fmt.Sprintf("archive task #%d (%q) depends on #%d which is missing from both live store and archive", t.ID, t.Title, dep),
					TaskID: t.ID,
				})
			}
		}
	}
	if !orphanFound {
		r.OKChecks = append(r.OKChecks, "orphan_archive_check")
	}
}

// archiveSiblingPath returns the canonical sibling archive path for
// a given live-store path: "<dir>/.tsk.archive.md". Used by the
// orphan-check and matches the default archive layout produced by
// `tsk archive` without --merge-into.
func archiveSiblingPath(livePath string) string {
	return filepath.Join(filepath.Dir(livePath), ".tsk.archive.md")
}

// SilentExitCoder is the interface implemented by errors that carry a
// non-zero exit code but should NOT trigger the default "error: <msg>"
// print in main.go. Used by commands like `doctor` that produce their
// own structured output where a prefix would just be noise.
type SilentExitCoder interface {
	ExitCoder
	silentExitMarker() // unexported — only types in this package qualify
}

// silentExit is the canonical implementation of SilentExitCoder.
type silentExit struct {
	code int
}

func (silentExit) Error() string     { return "" }
func (e silentExit) ExitCode() int   { return e.code }
func (silentExit) silentExitMarker() {}
