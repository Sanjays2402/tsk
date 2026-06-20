package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newTopCmd implements `tsk top [N]`: surface the N highest-priority
// undone tasks at once. It's the multi-task companion to `tsk next` —
// when `next` is too narrow and `tsk ls` is too noisy.
//
// Ordering matches `tsk next`'s tie-breaks so the head of `top` is the
// same task `next` would return:
//
//  1. higher Priority first (urgent > high > medium > low)
//  2. then tasks WITH a due date come before those WITHOUT
//  3. then earliest due date first
//  4. then lower ID (stable, deterministic across runs)
//
// Filters mirror `tsk ls` where useful:
//
//	--tag <t>       only consider tasks with this tag
//	--priority <p>  only consider tasks at this exact priority
//	--all           include done tasks (default: undone only)
//
// Output formats match `tsk ls`: plain (default), table, json.
func newTopCmd() *cobra.Command {
	var (
		tagFilter   string
		prioFilter  string
		includeDone bool
		asJSON      bool
		format      string
	)
	cmd := &cobra.Command{
		Use:   "top [N]",
		Short: "Show the N highest-priority undone tasks (default 5)",
		Long: `Show the N highest-priority undone tasks at once.

Same ordering as 'tsk next' (priority desc, then earliest due, then ID),
just multi-task. N defaults to 5; pass 0 or 'all' to show every match.

Filters mirror 'tsk ls' for the common slices.

Examples:
  tsk top              # top 5
  tsk top 10           # top 10
  tsk top all          # everything matching, sorted
  tsk top --tag work   # top 5 #work tasks
  tsk top --priority high
  tsk top --all 3      # top 3 across done + undone
  tsk top --json | jq '.[].Title'
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, err := parseTopLimit(args)
			if err != nil {
				return err
			}
			prio, prioActive, err := resolvePriorityFilter(prioFilter)
			if err != nil {
				return err
			}
			outFormat, err := resolveLsFormat(format, asJSON)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			pool := filterTopCandidates(s.Tasks, tagFilter, prio, prioActive, includeDone)
			sortTopTasks(pool)
			if limit > 0 && len(pool) > limit {
				pool = pool[:limit]
			}
			return emitTopResults(cmd.OutOrStdout(), pool, outFormat)
		},
	}
	cmd.Flags().StringVar(&tagFilter, "tag", "", "only consider tasks with this tag")
	cmd.Flags().StringVar(&prioFilter, "priority", "", "only consider tasks with this priority")
	cmd.Flags().BoolVar(&includeDone, "all", false, "include done tasks in the candidate pool")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().StringVar(&format, "format", "", "output format: plain, table, or json")
	return cmd
}

// parseTopLimit reads the optional N positional argument. Defaults to 5.
// Accepts "0" or "all" as "no limit". Negatives are a usage error.
func parseTopLimit(args []string) (int, error) {
	if len(args) == 0 {
		return 5, nil
	}
	raw := strings.TrimSpace(args[0])
	if raw == "" {
		return 5, nil
	}
	if strings.EqualFold(raw, "all") {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, usageErrorf("N must be an integer or 'all', got %q", raw)
	}
	if n < 0 {
		return 0, usageErrorf("N must be >= 0, got %d", n)
	}
	return n, nil
}

// filterTopCandidates narrows the task pool by done state and the optional
// tag / priority filters. The returned slice is a fresh copy (safe to sort
// without disturbing the store).
func filterTopCandidates(in []model.Task, tag string, prio model.Priority, prioActive, includeDone bool) []model.Task {
	out := make([]model.Task, 0, len(in))
	tag = strings.TrimSpace(tag)
	for _, t := range in {
		if !includeDone && t.Done {
			continue
		}
		if tag != "" && !t.HasTag(tag) {
			continue
		}
		if prioActive && t.Priority != prio {
			continue
		}
		out = append(out, t)
	}
	return out
}

// sortTopTasks applies the canonical ordering: higher priority first, then
// tasks WITH a due date before those without, then earliest due first,
// then lower ID. Matches `tsk next`'s tie-breaks so top[0] == next.
func sortTopTasks(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		switch {
		case a.Due != nil && b.Due == nil:
			return true
		case a.Due == nil && b.Due != nil:
			return false
		case a.Due != nil && b.Due != nil:
			if !a.Due.Equal(*b.Due) {
				return a.Due.Before(*b.Due)
			}
		}
		return a.ID < b.ID
	})
}

// emitTopResults dispatches based on output format. Plain has its own
// numbered-rank rendering; table/json reuse the shared printers so the
// output stays consistent with `tsk ls`.
func emitTopResults(w io.Writer, tasks []model.Task, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tasks)
	case "table":
		return printTasksTable(w, tasks)
	default:
		return printTopPlain(w, tasks)
	}
}

// printTopPlain renders a numbered list — rank in front so it's clear
// which task is #1 even when titles vary in length.
func printTopPlain(w io.Writer, tasks []model.Task) error {
	if len(tasks) == 0 {
		pln(w, "no tasks")
		return nil
	}
	width := digits(len(tasks))
	for i, t := range tasks {
		check := " "
		if t.Done {
			check = "x"
		}
		line := fmt.Sprintf("%*d. [%s] #%d [%s] %s", width, i+1, check, t.ID, t.Priority.Short(), t.Title)
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

// digits returns the number of decimal digits in n (>=1 for n=0).
func digits(n int) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}
