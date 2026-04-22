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
	var prio model.Priority
	prioFilter := false
	if f.priorityStr != "" {
		p, err := model.ParsePriority(f.priorityStr)
		if err != nil {
			return nil, err
		}
		prio = p
		prioFilter = true
	}
	now := time.Now()
	out := make([]model.Task, 0, len(in))
	for _, t := range in {
		switch {
		case f.all:
			// everything
		case f.done:
			if !t.Done {
				continue
			}
		default:
			if t.Done {
				continue
			}
		}
		if f.today && !t.IsDueToday(now) {
			continue
		}
		if f.overdue && !t.IsOverdue(now) {
			continue
		}
		if f.upcoming && !t.IsUpcoming(now) {
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

func printTasks(w io.Writer, tasks []model.Task, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tasks)
	}
	if len(tasks) == 0 {
		fmt.Fprintln(w, "no tasks")
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
		fmt.Fprintln(w, line)
	}
	return nil
}
