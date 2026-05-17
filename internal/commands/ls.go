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
	format                              string
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
			format, err := resolveLsFormat(f.format, f.asJSON)
			if err != nil {
				return err
			}
			return printTasks(cmd.OutOrStdout(), tasks, format)
		},
	}
	cmd.Flags().BoolVar(&f.done, "done", false, "only show done tasks")
	cmd.Flags().BoolVar(&f.all, "all", false, "show all tasks (done + undone)")
	cmd.Flags().BoolVar(&f.today, "today", false, "only show tasks due today")
	cmd.Flags().BoolVar(&f.overdue, "overdue", false, "only show overdue tasks")
	cmd.Flags().BoolVar(&f.upcoming, "upcoming", false, "only show tasks due in the future")
	cmd.Flags().StringVar(&f.tag, "tag", "", "only show tasks with this tag")
	cmd.Flags().StringVar(&f.priorityStr, "priority", "", "only show tasks with this priority")
	cmd.Flags().BoolVar(&f.asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().StringVar(&f.format, "format", "", "output format: plain, table, or json")
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

// resolveLsFormat arbitrates between --format and the legacy --json shortcut.
// Returns one of "plain", "table", "json". Empty --format with --json=false
// defaults to "plain".
func resolveLsFormat(format string, asJSON bool) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "" && asJSON {
		return "", usageErrorf("--format and --json are mutually exclusive")
	}
	if asJSON {
		return "json", nil
	}
	switch format {
	case "", "plain":
		return "plain", nil
	case "table":
		return "table", nil
	case "json":
		return "json", nil
	}
	return "", usageErrorf("unknown --format %q (want plain, table, or json)", format)
}

func printTasks(w io.Writer, tasks []model.Task, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tasks)
	case "table":
		return printTasksTable(w, tasks)
	default:
		return printTasksPlain(w, tasks)
	}
}

func printTasksPlain(w io.Writer, tasks []model.Task) error {
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

// printTasksTable renders tasks as a fixed-column, left-aligned table.
// Columns: ID, Done, Priority, Due, Title, Tags. Designed for terminal width
// 80+; long titles are truncated with an ellipsis at column boundary.
func printTasksTable(w io.Writer, tasks []model.Task) error {
	if len(tasks) == 0 {
		pln(w, "no tasks")
		return nil
	}
	rows := make([]tableRow, 0, len(tasks))
	for _, t := range tasks {
		check := " "
		if t.Done {
			check = "x"
		}
		due := ""
		if t.Due != nil {
			due = t.Due.Format(model.DateLayout)
		}
		tags := ""
		if len(t.Tags) > 0 {
			tags = "#" + strings.Join(t.Tags, " #")
		}
		rows = append(rows, tableRow{
			id:    fmt.Sprintf("#%d", t.ID),
			done:  "[" + check + "]",
			prio:  t.Priority.Short(),
			due:   due,
			title: t.Title,
			tags:  tags,
		})
	}
	headers := tableRow{id: "ID", done: "DONE", prio: "P", due: "DUE", title: "TITLE", tags: "TAGS"}
	widths := computeColumnWidths(headers, rows)
	// Cap title at 40 runes so wide terminals don't get wrapped lines.
	const titleCap = 40
	if widths.title > titleCap {
		widths.title = titleCap
	}
	pln(w, formatTableRow(headers, widths))
	for _, r := range rows {
		pln(w, formatTableRow(r, widths))
	}
	return nil
}

// tableRow is the per-task column data used by the table formatter.
type tableRow struct {
	id, done, prio, due, title, tags string
}

// columnWidths holds the rune width of each column.
type columnWidths struct {
	id, done, prio, due, title, tags int
}

func computeColumnWidths(header tableRow, rows []tableRow) columnWidths {
	w := columnWidths{
		id:    runeLen(header.id),
		done:  runeLen(header.done),
		prio:  runeLen(header.prio),
		due:   runeLen(header.due),
		title: runeLen(header.title),
		tags:  runeLen(header.tags),
	}
	for _, r := range rows {
		if l := runeLen(r.id); l > w.id {
			w.id = l
		}
		if l := runeLen(r.done); l > w.done {
			w.done = l
		}
		if l := runeLen(r.prio); l > w.prio {
			w.prio = l
		}
		if l := runeLen(r.due); l > w.due {
			w.due = l
		}
		if l := runeLen(r.title); l > w.title {
			w.title = l
		}
		if l := runeLen(r.tags); l > w.tags {
			w.tags = l
		}
	}
	return w
}

// runeLen returns rune count (handles unicode in titles/tags correctly).
func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func formatTableRow(r tableRow, w columnWidths) string {
	title := r.title
	if runeLen(title) > w.title {
		runes := []rune(title)
		title = string(runes[:w.title-1]) + "…"
	}
	return fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s  %s",
		w.id, r.id,
		w.done, r.done,
		w.prio, r.prio,
		w.due, r.due,
		w.title, title,
		r.tags,
	)
}
