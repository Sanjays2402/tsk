package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

func newStatsCmd() *cobra.Command {
	var sinceRaw string
	var asJSON bool
	var withGraph bool
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show task counts, completion %, streak, and top tags",
		Long: `Aggregate task metrics: totals, completion rate, current streak, and top tags.

Flags:
  --since <dur>  Restrict completion-derived metrics to tasks completed within
                 the window. Accepts 7d, 30d, 90d, 2w, 1m, 1y, or any Go
                 duration string (e.g. 72h). Total / Undone / Overdue / Today
                 always reflect the whole store.
  --graph        Append a 30-day completion sparkline below the summary.
                 The sparkline is always the trailing 30 days, independent
                 of --since.
  --json         Emit a stable JSON document instead of human output.
                 --json wins when combined with --graph.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			var since time.Duration
			if sinceRaw != "" {
				d, err := parseDurationLocal(sinceRaw)
				if err != nil {
					return usageErrorf("invalid --since %q: %v", sinceRaw, err)
				}
				if d <= 0 {
					return usageErrorf("--since must be a positive duration, got %q", sinceRaw)
				}
				since = d
			}

			now := time.Now()
			summary := computeStats(s.Tasks, now, since)
			out := cmd.OutOrStdout()

			if asJSON {
				return emitStatsJSON(out, summary, since)
			}

			pf(out, "total:       %d\n", summary.Total)
			pf(out, "done:        %d\n", summary.Done)
			pf(out, "undone:      %d\n", summary.Undone)
			pf(out, "overdue:     %d\n", summary.Overdue)
			pf(out, "today:       %d\n", summary.Today)
			pf(out, "completion:  %.0f%%\n", summary.Completion)
			pf(out, "streak:      %d day(s)\n", summary.Streak)
			if since > 0 {
				pf(out, "since:       %s ago\n", sinceRaw)
			}
			if len(summary.TopTags) > 0 {
				pln(out, "top tags:")
				for _, tc := range summary.TopTags {
					pf(out, "  %-16s %d\n", tc.Tag, tc.Count)
				}
			}
			if withGraph {
				pf(out, "30d completions:  %s\n", renderSparkline(summary.CompletionHistory))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceRaw, "since", "", "only consider completions within this duration (e.g. 7d, 2w, 1m, 72h)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (machine-readable, stable schema)")
	cmd.Flags().BoolVar(&withGraph, "graph", false, "append a 30-day completion sparkline")
	return cmd
}

// statsJSON is the stable contract for `tsk stats --json`.
//
// Schema (do not break without a major version):
//
//	{
//	  "total": int, "done": int, "undone": int,
//	  "overdue": int, "today": int,
//	  "completion": float64, "streak": int,
//	  "since_seconds": int,
//	  "top_tags": [{"tag": string, "count": int}, ...],
//	  "completion_history": [{"date": "YYYY-MM-DD", "count": int}, ...]
//	}
//
// `completion_history` is always populated (length 30, oldest-first) so the
// schema is stable regardless of whether the caller passed --graph.
type statsJSON struct {
	Total             int            `json:"total"`
	Done              int            `json:"done"`
	Undone            int            `json:"undone"`
	Overdue           int            `json:"overdue"`
	Today             int            `json:"today"`
	Completion        float64        `json:"completion"`
	Streak            int            `json:"streak"`
	SinceSeconds      int            `json:"since_seconds"`
	TopTags           []tagCountJSON `json:"top_tags"`
	CompletionHistory []dayCountJSON `json:"completion_history"`
}

type tagCountJSON struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type dayCountJSON struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

func emitStatsJSON(w io.Writer, s statsSummary, since time.Duration) error {
	tags := make([]tagCountJSON, 0, len(s.TopTags))
	for _, t := range s.TopTags {
		tags = append(tags, tagCountJSON{Tag: t.Tag, Count: t.Count})
	}
	hist := make([]dayCountJSON, 0, len(s.CompletionHistory))
	for _, d := range s.CompletionHistory {
		hist = append(hist, dayCountJSON{Date: d.Date, Count: d.Count})
	}
	doc := statsJSON{
		Total:             s.Total,
		Done:              s.Done,
		Undone:            s.Undone,
		Overdue:           s.Overdue,
		Today:             s.Today,
		Completion:        s.Completion,
		Streak:            s.Streak,
		SinceSeconds:      int(since / time.Second),
		TopTags:           tags,
		CompletionHistory: hist,
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

// tagCount pairs a tag with its occurrence count for top-tag reporting.
type tagCount struct {
	Tag   string
	Count int
}

// dayCount is one bucket of the 30-day completion history.
type dayCount struct {
	Date  string // YYYY-MM-DD, in `now`'s zone
	Count int
}

// statsSummary holds the aggregated metrics printed by `tsk stats`.
type statsSummary struct {
	Total, Done, Undone, Overdue, Today int
	Completion                          float64
	Streak                              int
	TopTags                             []tagCount
	// CompletionHistory is always the trailing 30 days, oldest-first.
	// It is independent of `--since`: the visualization stays comparable
	// across windows because it always reflects the same 30-day span.
	CompletionHistory []dayCount
}

// computeStats aggregates metrics from the task list.
//
// Total / Undone / Overdue / Today are always whole-store counts.
//
// When `since` > 0, the following are restricted to tasks whose Completed
// timestamp falls within `[now-since, now]`:
//   - Done (within-window count, not whole-store)
//   - Completion (within-window done over whole-store total)
//   - Streak (only counts days inside the window)
//   - TopTags (only tags from completed-within-window tasks count)
func computeStats(tasks []model.Task, now time.Time, since time.Duration) statsSummary {
	var s statsSummary
	s.Total = len(tasks)

	// Whole-store passes: undone, overdue, today.
	for _, t := range tasks {
		if !t.Done {
			s.Undone++
		}
		if t.IsOverdue(now) {
			s.Overdue++
		}
		if t.IsDueToday(now) {
			s.Today++
		}
	}

	// Windowed pass: Done, TopTags, and the streak input.
	cutoff := time.Time{}
	if since > 0 {
		cutoff = now.Add(-since)
	}
	tagMap := map[string]int{}
	windowed := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		if !t.Done {
			continue
		}
		if since > 0 {
			if t.Completed == nil || t.Completed.Before(cutoff) {
				continue
			}
		}
		s.Done++
		windowed = append(windowed, t)
		for _, tag := range t.Tags {
			tagMap[tag]++
		}
	}
	if s.Total > 0 {
		s.Completion = float64(s.Done) / float64(s.Total) * 100
	}
	s.Streak = currentStreak(windowed, now)

	tags := make([]tagCount, 0, len(tagMap))
	for k, v := range tagMap {
		tags = append(tags, tagCount{k, v})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return tags[i].Tag < tags[j].Tag
	})
	if len(tags) > 5 {
		tags = tags[:5]
	}
	s.TopTags = tags

	// 30-day history is always whole-store and independent of `--since`.
	s.CompletionHistory = completionHistory(tasks, now, 30)
	return s
}

// completionHistory builds an oldest-first slice of length `days` covering
// the trailing window ending on `now`'s calendar day. Each bucket counts
// tasks whose `Completed` timestamp falls on that date in `now`'s zone.
func completionHistory(tasks []model.Task, now time.Time, days int) []dayCount {
	if days <= 0 {
		return nil
	}
	loc := now.Location()
	y, m, d := now.In(loc).Date()
	end := time.Date(y, m, d, 0, 0, 0, 0, loc)
	out := make([]dayCount, days)
	for i := 0; i < days; i++ {
		d := end.AddDate(0, 0, -(days - 1 - i))
		out[i] = dayCount{Date: d.Format(model.DateLayout), Count: 0}
	}
	idx := make(map[string]int, days)
	for i, b := range out {
		idx[b.Date] = i
	}
	for _, t := range tasks {
		if !t.Done || t.Completed == nil {
			continue
		}
		key := t.Completed.In(loc).Format(model.DateLayout)
		if i, ok := idx[key]; ok {
			out[i].Count++
		}
	}
	return out
}

// currentStreak counts consecutive days ending at `now` where at least one task
// was completed.
func currentStreak(tasks []model.Task, now time.Time) int {
	days := map[string]bool{}
	for _, t := range tasks {
		if !t.Done || t.Completed == nil {
			continue
		}
		days[t.Completed.Format(model.DateLayout)] = true
	}
	streak := 0
	cur := now
	for {
		if days[cur.Format(model.DateLayout)] {
			streak++
			cur = cur.AddDate(0, 0, -1)
			continue
		}
		// allow today to have no completion yet; streak starts yesterday in that case
		if streak == 0 && cur.Format(model.DateLayout) == now.Format(model.DateLayout) {
			cur = cur.AddDate(0, 0, -1)
			continue
		}
		break
	}
	return streak
}

// renderSparkline maps a slice of day counts onto the 9-rune sparkline
// alphabet, oldest-first. Empty input yields an empty string. Output is
// plain runes only — no ANSI escapes — so it's safe under NO_COLOR.
func renderSparkline(history []dayCount) string {
	if len(history) == 0 {
		return ""
	}
	// 9 characters: space then 8 increasing block heights.
	const alphabet = " ▁▂▃▄▅▆▇█"
	rs := []rune(alphabet)
	max := 0
	for _, b := range history {
		if b.Count > max {
			max = b.Count
		}
	}
	var sb strings.Builder
	sb.Grow(len(history) * 4)
	for _, b := range history {
		var r rune
		switch {
		case max == 0, b.Count == 0:
			r = rs[0]
		default:
			// Map [1..max] -> [1..len(rs)-1] with ceiling division so any
			// nonzero count gets at least the smallest visible block.
			step := (b.Count*(len(rs)-1) + max - 1) / max
			if step < 1 {
				step = 1
			}
			if step > len(rs)-1 {
				step = len(rs) - 1
			}
			r = rs[step]
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// parseDurationLocal accepts the friendly suffixes 7d, 2w, 1m, 1y in addition
// to bare Go duration strings (e.g. "72h", "1h30m"). Months are treated as
// 30 days and years as 365 days — calendar-exact arithmetic isn't required
// for "completions in the last N" semantics, and avoids surprises like a user
// asking for "1y" and getting 365.25 days.
//
// This helper is intentionally local to `stats.go`. A future refactor PR may
// promote it to `internal/util` once a sibling caller materializes; keeping
// it private here keeps this PR independent.
func parseDurationLocal(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	// Friendly suffix path: "<int><unit>" where unit is one of d, w, m, y.
	if n := len(s); n >= 2 {
		last := s[n-1]
		switch last {
		case 'd', 'w', 'm', 'y':
			numPart := s[:n-1]
			// Reject if numPart has any non-digit (so "1.5d" falls through to
			// the Go parser path, not here, where it'd silently drop the
			// fraction).
			allDigits := numPart != ""
			for _, r := range numPart {
				if r < '0' || r > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				v, err := strconv.Atoi(numPart)
				if err != nil {
					return 0, err
				}
				switch last {
				case 'd':
					return time.Duration(v) * 24 * time.Hour, nil
				case 'w':
					return time.Duration(v) * 7 * 24 * time.Hour, nil
				case 'm':
					return time.Duration(v) * 30 * 24 * time.Hour, nil
				case 'y':
					return time.Duration(v) * 365 * 24 * time.Hour, nil
				}
			}
		}
	}
	// Fall back to Go's parser so 72h, 1h30m, etc. still work.
	return time.ParseDuration(s)
}
