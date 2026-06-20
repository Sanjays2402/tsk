package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newSwapCmd implements `tsk swap <id1> <id2>`: exchange the file-order
// positions of two tasks, in place, preserving everything else.
//
// Why this exists: tsk's primary sort knobs (ls --sort, top, next) are
// always derived views — the underlying file order is what determines
// the TUI list order, what tsk export emits, and what your editor sees
// when you open .tsk.md. Sometimes you just want task #7 to sit above
// task #3 without giving #3 a different priority. swap is that.
//
// Both IDs must exist; they must be different (a self-swap is a usage
// error so the user knows the command did nothing). IDs survive the
// swap — only file order changes.
func newSwapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "swap <id1> <id2>",
		Short: "Exchange the file positions of two tasks (in-place reorder)",
		Long: `Swap the positions of two tasks in the .tsk.md file.

Both tasks keep their IDs, due dates, tags, and every other field —
only their position in the file changes. The TUI, 'tsk ls' default
order, and 'tsk export' all read file order, so swap is the right
tool for "I want this task above that one" without changing
priorities or due dates.

Examples:
  tsk swap 3 7     # task 7 moves to task 3's slot and vice versa
  tsk swap 1 12    # makes 12 the first task in the file

Self-swap (swap N N) is rejected so a typo never silently no-ops.
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id1, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			id2, err := parseSingleID(args[1])
			if err != nil {
				return err
			}
			if id1 == id2 {
				return usageErrorf("swap: ids must differ, got %d and %d", id1, id2)
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			i1, ok1 := findTaskIndex(s.Tasks, id1)
			i2, ok2 := findTaskIndex(s.Tasks, id2)
			if !ok1 {
				return fmt.Errorf("no task with id %d in %s", id1, s.Path)
			}
			if !ok2 {
				return fmt.Errorf("no task with id %d in %s", id2, s.Path)
			}
			s.Tasks[i1], s.Tasks[i2] = s.Tasks[i2], s.Tasks[i1]
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "swapped #%d <-> #%d (positions %d <-> %d)\n",
				id1, id2, i1+1, i2+1)
			return nil
		},
	}
}

// findTaskIndex returns the slice index of the task with the given ID
// and whether it was found. Sibling of store.ByID, which returns a
// pointer — here we need the index for the actual slice swap.
func findTaskIndex(tasks []model.Task, id int) (int, bool) {
	for i, t := range tasks {
		if t.ID == id {
			return i, true
		}
	}
	return -1, false
}
