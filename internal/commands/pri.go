package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newPriCmd implements `tsk pri <id> <priority>`: a single-task priority
// setter that collapses the verbose form
//
//	tsk bulk --id 3 --set-priority high --apply
//
// into the punchy
//
//	tsk pri 3 high
//
// Accepts every priority spelling `tsk add -p` does (low/l, medium/med/m,
// high/h, urgent/u/critical), case-insensitive.
func newPriCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "pri <id> <priority>",
		Aliases: []string{"priority"},
		Short:   "Set priority on a single task",
		Long: `Set priority on a single task. Accepts low/l, medium/med/m, high/h,
urgent/u/critical (case-insensitive).

Examples:
  tsk pri 3 high
  tsk pri 7 urgent
  tsk pri 12 low
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			prio, err := model.ParsePriority(args[1])
			if err != nil {
				return usageErrorf("%s", err.Error())
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			if t.Priority == prio {
				pf(cmd.OutOrStdout(), "#%d already at priority %s\n", id, prio)
				return nil
			}
			old := t.Priority
			t.Priority = prio
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "#%d priority %s -> %s\n", id, old, prio)
			return nil
		},
	}
}
