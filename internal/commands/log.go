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

// newLogCmd implements `tsk log`: a recent-completions feed.
//
// The relationship to siblings:
//   - `tsk stats` shows aggregates (counts, completion %, streak,
//     30-day sparkline).
//   - `tsk ls --done` shows everything that's done, with no time-sort
//     (it's the file order — usually creation order).
//   - `tsk log` is the chronological tail: newest completion first,
//     bounded by --limit (default 10) and/or --since (no default).
//
// Why this exists separately from `ls --done`: when you want "what did
// I just finish?" you want it newest-first, capped, and ideally trimmed
// by recency. `ls --done` is the catalog; `log` is the journal.
//
// Tasks without a Completed timestamp (older tsk files, or anything
// flipped done manually in the markdown) are excluded by definition —
// `log` is strictly time-ordered. A footer note tells you when this is
// happening so you're not surprised by a "where's task #N?" question.
func newLogCmd() *cobra.Command {
	var (
		limit  int
		since  string
		asJSON bool
		tag    string
		format string
	)
	cmd := &cobra.Command{
		Use:     "log",
		Aliases: []string{"recent"},
		Short:   "Show recently completed tasks (newest first)",
		Long: `Show recently completed tasks, newest completion first.

Bounds (compose: --since trims first, --limit caps the result):
  --limit N      cap to N rows (default 10; 0 = unlimited)
  --since DUR    only include tasks completed within this duration
                 (7d, 2w, 1m, 72h, 1h30m, ...)
  --tag T        only include tasks tagged T

Tasks without a Completed timestamp are excluded (the log is strictly
time-ordered). A footer line notes how many such tasks were skipped
so you can audit your store.

Output:
  --json         stable JSON array of task objects (same shape as
                 'tsk export --json'); empty -> []
  --format X     plain (default), table, or json

Examples:
  tsk log                       # last 10
  tsk log --limit 50            # last 50
  tsk log --since 7d            # past week
  tsk log --since 1d --tag work
  tsk log --json | jq '.[].Title'
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if limit < 0 {
				return usageErrorf("--limit must be >= 0, got %d", limit)
			}
			var sinceDur time.Duration
			if strings.TrimSpace(since) != "" {
				d, err := parseDurationLocal(since)
				if err != nil {
					return usageErrorf("invalid --since %q: %v", since, err)
				}
				if d <= 0 {
					return usageErrorf("--since must be a positive duration, got %q", since)
				}
				sinceDur = d
			}
			outFormat, err := resolveLsFormat(format, asJSON)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			rows, skipped := collectLogRows(s.Tasks, time.Now(), sinceDur, tag, limit)
			return emitLogRows(cmd.OutOrStdout(), rows, skipped, outFormat)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "cap to N rows (0 = unlimited)")
	cmd.Flags().StringVar(&since, "since", "", "only include tasks completed within this duration (e.g. 7d, 2w, 1m)")
	cmd.Flags().StringVar(&tag, "tag", "", "only include tasks with this tag")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().StringVar(&format, "format", "", "output format: plain, table, or json")
	return cmd
}

// collectLogRows filters tasks to recently completed ones, sorts newest
// first, and caps to limit. Returns (rows, skippedWithoutCompleted) so
// the caller can surface a "N tasks done but missing timestamp" footer.
func collectLogRows(tasks []model.Task, now time.Time, since time.Duration, tag string, limit int) ([]model.Task, int) {
	var cutoff time.Time
	if since > 0 {
		cutoff = now.Add(-since)
	}
	tag = strings.TrimSpace(tag)
	out := make([]model.Task, 0, len(tasks))
	skipped := 0
	for _, t := range tasks {
		if !t.Done {
			continue
		}
		if t.Completed == nil {
			skipped++
			continue
		}
		if since > 0 && t.Completed.Before(cutoff) {
			continue
		}
		if tag != "" && !t.HasTag(tag) {
			continue
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		// Both have non-nil Completed (filtered above). Newer first.
		return out[i].Completed.After(*out[j].Completed)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, skipped
}

// emitLogRows dispatches per format. Plain output is "DATE  #ID title"
// per line, in the active timezone — log is meant for at-a-glance
// reading, not parsing.
func emitLogRows(w io.Writer, rows []model.Task, skipped int, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if rows == nil {
			rows = []model.Task{}
		}
		// JSON mode stays silent about skipped rows — the JSON contract
		// is just the task array. Use plain/table to see the footer.
		return enc.Encode(rows)
	case "table":
		return printTasksTable(w, rows)
	default:
		return printLogPlain(w, rows, skipped)
	}
}

// printLogPlain renders rows as "YYYY-MM-DD HH:MM  #ID title  (#tags)"
// in the active tz. If `skipped > 0`, appends a footer noting the count.
func printLogPlain(w io.Writer, rows []model.Task, skipped int) error {
	if len(rows) == 0 {
		pln(w, "no completed tasks")
		if skipped > 0 {
			pf(w, "(%d done task(s) missing completion timestamp)\n", skipped)
		}
		return nil
	}
	loc := PacificLoc()
	for _, t := range rows {
		when := t.Completed.In(loc).Format("2006-01-02 15:04")
		line := "[" + when + "]  #" + strconv.Itoa(t.ID) + " " + t.Title
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
