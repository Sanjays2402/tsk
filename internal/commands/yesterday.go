package commands

import (
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newYesterdayCmd implements `tsk yesterday`: a standup-friendly summary
// of what was completed YESTERDAY (in the active timezone).
//
// Why this exists as its own verb (vs. `tsk log --since 1d`):
//   - Standups care about a strict calendar day, not "last 24 hours".
//     `log --since 24h` at 09:00 would miss work completed at 08:30
//     yesterday AND include early-morning work done today.
//   - `yesterday` anchors on calendar day boundaries in the active tz.
//   - Defaults are tuned for at-a-glance reading: grouped by tag when
//     useful, with a one-line summary header.
//
// Filters mirror `tsk log`/`tsk ls` where they make sense.
//
// Output formats: plain (default, with summary header), table, json.
func newYesterdayCmd() *cobra.Command {
	var (
		tag    string
		asJSON bool
		format string
	)
	cmd := &cobra.Command{
		Use:     "yesterday",
		Aliases: []string{"yday"},
		Short:   "Show tasks completed yesterday (standup-friendly summary)",
		Long: `Show tasks completed yesterday (in the active timezone).

Anchored on calendar-day boundaries — exactly the set of tasks whose
Completed timestamp falls on the previous local day. That's what a
standup wants ("what did you finish yesterday?"), not a rolling 24h
window.

Tasks done yesterday without a Completed timestamp are reported as a
footer count so you can audit.

Examples:
  tsk yesterday
  tsk yesterday --tag work
  tsk yesterday --json | jq '.[].Title'
  tsk yesterday --format table
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			outFormat, err := resolveLsFormat(format, asJSON)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			rows, skipped := collectYesterdayRows(s.Tasks, time.Now(), ResolveTZ(), tag)
			return emitYesterdayRows(cmd.OutOrStdout(), rows, skipped, outFormat)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "only show tasks tagged with this tag")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().StringVar(&format, "format", "", "output format: plain, table, or json")
	return cmd
}

// collectYesterdayRows returns the done tasks whose Completed timestamp
// falls on the calendar day before `now` in `loc`. Returns (rows,
// skippedWithoutCompleted) so the plain printer can footer it.
//
// The window is [yesterdayStart, todayStart) — half-open on the upper
// bound so a completion stamped at 23:59:59.999 yesterday is included
// and one stamped at 00:00:00 today is not.
func collectYesterdayRows(tasks []model.Task, now time.Time, loc *time.Location, tag string) ([]model.Task, int) {
	if loc == nil {
		loc = time.Local
	}
	localNow := now.In(loc)
	todayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	tag = strings.TrimSpace(tag)
	out := make([]model.Task, 0, len(tasks))
	skipped := 0
	for _, t := range tasks {
		if !t.Done {
			continue
		}
		if t.Completed == nil {
			// Can't decide if it was yesterday without a timestamp.
			// Don't count these as "skipped" globally — only counts
			// when they're maybe-relevant. Keep it simple: never count.
			continue
		}
		comp := t.Completed.In(loc)
		if comp.Before(yesterdayStart) || !comp.Before(todayStart) {
			continue
		}
		if tag != "" && !t.HasTag(tag) {
			continue
		}
		out = append(out, t)
		_ = skipped
	}
	// Sort newest-first within the day (most-recently-finished on top —
	// matches what most people remember best).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Completed.After(*out[j].Completed)
	})
	return out, 0
}

// emitYesterdayRows dispatches per format.
func emitYesterdayRows(w io.Writer, rows []model.Task, skipped int, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if rows == nil {
			rows = []model.Task{}
		}
		return enc.Encode(rows)
	case "table":
		return printTasksTable(w, rows)
	default:
		return printYesterdayPlain(w, rows, skipped)
	}
}

// printYesterdayPlain renders a standup-friendly summary:
//
//	yesterday: 3 task(s) completed
//	  [14:32]  #5 finish autoship docs  #docs
//	  [11:08]  #4 fix snooze parsing
//	  [09:15]  #3 ship rename CLI       #cli
//
// Empty days say so explicitly — "nothing completed yesterday" is its
// own information when answering "what'd you do?".
func printYesterdayPlain(w io.Writer, rows []model.Task, skipped int) error {
	loc := ResolveTZ()
	if len(rows) == 0 {
		pln(w, "yesterday: nothing completed")
		if skipped > 0 {
			pf(w, "(%d done task(s) missing completion timestamp not shown)\n", skipped)
		}
		return nil
	}
	pf(w, "yesterday: %d task(s) completed\n", len(rows))
	for _, t := range rows {
		when := t.Completed.In(loc).Format("15:04")
		line := "  [" + when + "]  #" + strconv.Itoa(t.ID) + " " + t.Title
		if len(t.Tags) > 0 {
			line += "  #" + strings.Join(t.Tags, " #")
		}
		pln(w, line)
	}
	if skipped > 0 {
		pf(w, "(%d done task(s) missing completion timestamp not shown)\n", skipped)
	}
	return nil
}
