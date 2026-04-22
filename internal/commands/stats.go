package commands

import (
	"fmt"
	"sort"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show task counts, completion %, streak, and top tags",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			summary := computeStats(s.Tasks, time.Now())
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "total:       %d\n", summary.Total)
			fmt.Fprintf(out, "done:        %d\n", summary.Done)
			fmt.Fprintf(out, "undone:      %d\n", summary.Undone)
			fmt.Fprintf(out, "overdue:     %d\n", summary.Overdue)
			fmt.Fprintf(out, "today:       %d\n", summary.Today)
			fmt.Fprintf(out, "completion:  %.0f%%\n", summary.Completion)
			fmt.Fprintf(out, "streak:      %d day(s)\n", summary.Streak)
			if len(summary.TopTags) > 0 {
				fmt.Fprintln(out, "top tags:")
				for _, tc := range summary.TopTags {
					fmt.Fprintf(out, "  %-16s %d\n", tc.Tag, tc.Count)
				}
			}
			return nil
		},
	}
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

func computeStats(tasks []model.Task, now time.Time) statsSummary {
	var s statsSummary
	s.Total = len(tasks)
	tagMap := map[string]int{}
	for _, t := range tasks {
		if t.Done {
			s.Done++
		} else {
			s.Undone++
		}
		if t.IsOverdue(now) {
			s.Overdue++
		}
		if t.IsDueToday(now) {
			s.Today++
		}
		for _, tag := range t.Tags {
			tagMap[tag]++
		}
	}
	if s.Total > 0 {
		s.Completion = float64(s.Done) / float64(s.Total) * 100
	}
	s.Streak = currentStreak(tasks, now)

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
