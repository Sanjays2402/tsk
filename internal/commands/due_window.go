package commands

import (
	"github.com/spf13/cobra"
)

// newTodayCmd and newOverdueCmd give the two most common `tsk ls` slices
// their own one-word verbs.
//
// They are NOT thin aliases. Each:
//   - defaults to "undone only" (the only sensible default for "today's
//     work" or "what's overdue") while still accepting --done/--all if
//     you want to look at history;
//   - supports the same --format/--json switches as `tsk ls` so scripts
//     get the same shape;
//   - composes with --tag and --priority for further slicing.
//
// The behavior delegates to applyFilters/printTasks so there is exactly
// one source of truth for filter semantics — these commands just preset
// the relevant filter bit.

func newTodayCmd() *cobra.Command {
	return newDueWindowCmd(dueWindowConfig{
		use:   "today",
		short: "List tasks due today (shortcut for `ls --today`)",
		long: `Show every task whose due date falls on today (in the active timezone).

Defaults to undone only; pass --done or --all to expand the candidate set.
Same --format/--json/--tag/--priority flags as 'tsk ls' for scripts and slicing.

Examples:
  tsk today
  tsk today --tag work
  tsk today --json
  tsk today --all       # include already-done tasks due today
`,
		setter: func(f *lsFilters) { f.today = true },
	})
}

func newOverdueCmd() *cobra.Command {
	return newDueWindowCmd(dueWindowConfig{
		use:   "overdue",
		short: "List tasks past their due date (shortcut for `ls --overdue`)",
		long: `Show every undone task whose due date is in the past.

Only undone tasks can be overdue by definition — done tasks are never
shown here even with --all, because "overdue" implies "still needs doing".
The done-state flags are accepted for parity with 'tsk ls' but have no
visible effect (they're filtered out by the overdue predicate itself).

Same --format/--json/--tag/--priority flags as 'tsk ls'.

Examples:
  tsk overdue
  tsk overdue --tag work
  tsk overdue --json | jq '.[].Title'
  tsk overdue --priority urgent
`,
		setter: func(f *lsFilters) { f.overdue = true },
	})
}

// dueWindowConfig holds the per-command knobs.
type dueWindowConfig struct {
	use, short, long string
	// setter flips the relevant filter bit on the shared lsFilters.
	setter func(*lsFilters)
}

// newDueWindowCmd builds a `tsk ls`-flavoured command preset to a single
// date-window filter. Keeps today/overdue (and any future siblings)
// behaviorally identical to `tsk ls --<window>`.
func newDueWindowCmd(cfg dueWindowConfig) *cobra.Command {
	f := lsFilters{}
	cmd := &cobra.Command{
		Use:   cfg.use,
		Short: cfg.short,
		Long:  cfg.long,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg.setter(&f)
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
	cmd.Flags().BoolVar(&f.done, "done", false, "show done tasks (default: undone only)")
	cmd.Flags().BoolVar(&f.all, "all", false, "show all tasks (done + undone) in the window")
	cmd.Flags().StringVar(&f.tag, "tag", "", "only show tasks with this tag")
	cmd.Flags().StringVar(&f.priorityStr, "priority", "", "only show tasks with this priority")
	cmd.Flags().BoolVar(&f.asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().StringVar(&f.format, "format", "", "output format: plain, table, or json")
	return cmd
}
