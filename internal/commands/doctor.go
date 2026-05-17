package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	var asJSON bool
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

Exit codes:
  0  all checks passed
  1  at least one issue found (error)
  2  bad invocation or transient IO failure

Pass --json for machine-readable output (always emitted, exit code still
reflects severity).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := runDoctor(cmd)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				printDoctorReport(cmd.OutOrStdout(), report)
			}
			if report.HasIssues() {
				return silentExit{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
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
func runDoctor(cmd *cobra.Command) DoctorReport {
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
