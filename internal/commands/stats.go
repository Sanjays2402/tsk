package commands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

func newStatsCmd() *cobra.Command {
	var sinceRaw string
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show task counts, completion %, streak, and top tags",
		Long: `Aggregate task metrics: totals, completion rate, current streak, and top tags.

Flags:
  --since <dur>  Restrict completion-derived metrics to tasks completed within
                 the window. Accepts 7d, 30d, 90d, 2w, 1m, 1y, or any Go
                 duration string (e.g. 72h). Total / Undone / Overdue / Today
                 always reflect the whole store.`,
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
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceRaw, "since", "", "only consider completions within this duration (e.g. 7d, 2w, 1m, 72h)")
	return cmd
}

// tagCount pairs a tag with its occurrence count for top-tag reporting.
type tagCount struct {
	Tag   string
	Count int
}

// statsSummary holds the aggregated metrics printed by `tsk stats`.
type statsSummary struct {
	Total, Done, Undone, Overdue, Today int
	Completion                          float64
	Streak                              int
	TopTags                             []tagCount
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
	return s
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
