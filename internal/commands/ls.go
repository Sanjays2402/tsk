package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

type lsFilters struct {
	done, all, today, overdue, upcoming bool
	tag                                 string
	priorityStr                         string
	asJSON                              bool
}

func newLsCmd() *cobra.Command {
	f := lsFilters{}
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tasks (undone by default)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			tasks, err := applyFilters(s.Tasks, f)
			if err != nil {
				return err
			}
			return printTasks(cmd.OutOrStdout(), tasks, f.asJSON)
		},
	}
	cmd.Flags().BoolVar(&f.done, "done", false, "only show done tasks")
	cmd.Flags().BoolVar(&f.all, "all", false, "show all tasks (done + undone)")
	cmd.Flags().BoolVar(&f.today, "today", false, "only show tasks due today")
	cmd.Flags().BoolVar(&f.overdue, "overdue", false, "only show overdue tasks")
	cmd.Flags().BoolVar(&f.upcoming, "upcoming", false, "only show tasks due in the future")
	cmd.Flags().StringVar(&f.tag, "tag", "", "only show tasks with this tag")
	cmd.Flags().StringVar(&f.priorityStr, "priority", "", "only show tasks with this priority")
	cmd.Flags().BoolVar(&f.asJSON, "json", false, "emit JSON")
	return cmd
}

func applyFilters(in []model.Task, f lsFilters) ([]model.Task, error) {
	prio, prioFilter, err := resolvePriorityFilter(f.priorityStr)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]model.Task, 0, len(in))
	for _, t := range in {
		if !passStateFilter(t, f) {
			continue
		}
		if !passDueFilter(t, f, now) {
			continue
		}
		if f.tag != "" && !t.HasTag(f.tag) {
			continue
		}
		if prioFilter && t.Priority != prio {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func resolvePriorityFilter(s string) (model.Priority, bool, error) {
	if s == "" {
		return model.PriorityMedium, false, nil
	}
	p, err := model.ParsePriority(s)
	if err != nil {
		return 0, false, err
	}
	return p, true, nil
}

func passStateFilter(t model.Task, f lsFilters) bool {
	switch {
	case f.all:
		return true
	case f.done:
		return t.Done
	default:
		return !t.Done
	}
}

func passDueFilter(t model.Task, f lsFilters, now time.Time) bool {
	if f.today && !t.IsDueToday(now) {
		return false
	}
	if f.overdue && !t.IsOverdue(now) {
		return false
	}
	if f.upcoming && !t.IsUpcoming(now) {
		return false
	}
	return true
}

func printTasks(w io.Writer, tasks []model.Task, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tasks)
	}
	if len(tasks) == 0 {
		pln(w, "no tasks")
		return nil
	}
	for _, t := range tasks {
		check := " "
		if t.Done {
			check = "x"
		}
		line := fmt.Sprintf("[%s] #%d %s  (%s)", check, t.ID, t.Title, t.Priority)
		if t.Due != nil {
			line += "  due:" + t.Due.Format(model.DateLayout)
		}
		if len(t.Tags) > 0 {
			line += "  #" + strings.Join(t.Tags, " #")
		}
		pln(w, line)
	}
	return nil
}
