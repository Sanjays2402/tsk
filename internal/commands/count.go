package commands

import (
	"github.com/spf13/cobra"
)

// newCountCmd returns a tiny, shell-prompt-friendly count of matching tasks.
//
// It reuses the same filter flags as `ls` (--done, --all, --today, --overdue,
// --upcoming, --tag, --priority) so a status-bar widget can ask, e.g.:
//
//	tsk count --overdue
//	tsk count --today
//	tsk count --tag work
//
// Output is a single integer followed by a newline — easy to consume from
// shells, tmux, starship, etc.
func newCountCmd() *cobra.Command {
	f := lsFilters{}
	cmd := &cobra.Command{
		Use:   "count",
		Short: "Print the number of tasks matching the given filters",
		Long: `Print the number of tasks matching the given filters.

Designed for shell prompts, tmux status lines, and scripts. Accepts the same
filter flags as 'tsk ls' and emits a single integer on stdout.

Examples:
  tsk count                  # undone tasks (default)
  tsk count --overdue        # how many are overdue
  tsk count --today          # due today
  tsk count --all            # everything in the file
  tsk count --tag work       # undone tasks tagged 'work'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			tasks, err := applyFilters(s.Tasks, f)
			if err != nil {
				return err
			}
			pln(cmd.OutOrStdout(), len(tasks))
			return nil
		},
	}
	cmd.Flags().BoolVar(&f.done, "done", false, "count only done tasks")
	cmd.Flags().BoolVar(&f.all, "all", false, "count all tasks (done + undone)")
	cmd.Flags().BoolVar(&f.today, "today", false, "count only tasks due today")
	cmd.Flags().BoolVar(&f.overdue, "overdue", false, "count only overdue tasks")
	cmd.Flags().BoolVar(&f.upcoming, "upcoming", false, "count only tasks due in the future")
	cmd.Flags().StringVar(&f.tag, "tag", "", "count only tasks with this tag")
	cmd.Flags().StringVar(&f.priorityStr, "priority", "", "count only tasks with this priority")
	return cmd
}
