package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newPriCmd implements `tsk pri`: a single-task priority setter that
// collapses the verbose form
//
//	tsk bulk --id 3 --set-priority high --apply
//
// into the punchy
//
//	tsk pri 3 high
//
// Accepts every priority spelling `tsk add -p` does (low/l, medium/med/m,
// high/h, urgent/u/critical), case-insensitive.
//
// Also supports cycling without naming the priority — useful when you
// just want to bump something up a notch and don't want to remember the
// next step (low->medium->high->urgent):
//
//	tsk pri 3 --up
//	tsk pri 3 --down
//	tsk pri 3 --cycle    # urgent wraps back to low (round-robin)
//
// The cycling flags are mutually exclusive with each other and with the
// positional <priority> arg.
func newPriCmd() *cobra.Command {
	var (
		up    bool
		down  bool
		cycle bool
	)
	cmd := &cobra.Command{
		Use:     "pri <id> [<priority>]",
		Aliases: []string{"priority"},
		Short:   "Set, bump, or cycle priority on a single task",
		Long: `Set priority on a single task.

Direct set (positional arg):
  tsk pri 3 high
  tsk pri 7 urgent
  tsk pri 12 low

Accepts low/l, medium/med/m, high/h, urgent/u/critical (case-insensitive).

Cycle without naming the priority (useful for quick bumps):
  tsk pri 3 --up       # one step toward urgent; urgent is a no-op
  tsk pri 3 --down     # one step toward low; low is a no-op
  tsk pri 3 --cycle    # one step, with urgent wrapping back to low

The cycle flags are mutually exclusive with each other and with a
positional <priority> arg. Identical-to-current is a no-op (no save).
`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			if err := validatePriModeFlags(args, up, down, cycle); err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			next, noop, err := resolvePriTransition(t.Priority, args, up, down, cycle)
			if err != nil {
				return err
			}
			if noop {
				pf(cmd.OutOrStdout(), "#%d already at priority %s\n", id, t.Priority)
				return nil
			}
			old := t.Priority
			t.Priority = next
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "#%d priority %s -> %s\n", id, old, next)
			return nil
		},
	}
	cmd.Flags().BoolVar(&up, "up", false, "bump priority one step toward urgent (no-op at urgent)")
	cmd.Flags().BoolVar(&down, "down", false, "lower priority one step toward low (no-op at low)")
	cmd.Flags().BoolVar(&cycle, "cycle", false, "cycle priority one step; urgent wraps back to low")
	return cmd
}

// validatePriModeFlags enforces the mutually-exclusive surface: a user
// must pass exactly one of {positional <priority>, --up, --down, --cycle}.
func validatePriModeFlags(args []string, up, down, cycle bool) error {
	bumps := 0
	if up {
		bumps++
	}
	if down {
		bumps++
	}
	if cycle {
		bumps++
	}
	if bumps > 1 {
		return usageErrorf("pri: --up, --down, and --cycle are mutually exclusive")
	}
	hasPositional := len(args) == 2
	if hasPositional && bumps == 1 {
		return usageErrorf("pri: positional <priority> and --up/--down/--cycle are mutually exclusive")
	}
	if !hasPositional && bumps == 0 {
		return usageErrorf("pri: need either a <priority> arg or one of --up/--down/--cycle")
	}
	return nil
}

// resolvePriTransition computes the next priority value. Returns
// (next, noop, err): noop=true means current==next (caller skips Save).
func resolvePriTransition(current model.Priority, args []string, up, down, cycle bool) (model.Priority, bool, error) {
	if len(args) == 2 {
		next, err := model.ParsePriority(args[1])
		if err != nil {
			return current, false, usageErrorf("%s", err.Error())
		}
		return next, current == next, nil
	}
	switch {
	case up:
		if current == model.PriorityUrgent {
			return current, true, nil
		}
		return current + 1, false, nil
	case down:
		if current == model.PriorityLow {
			return current, true, nil
		}
		return current - 1, false, nil
	case cycle:
		// urgent wraps back to low; never a no-op unless the store has
		// only one priority value (which we don't expose).
		if current == model.PriorityUrgent {
			return model.PriorityLow, false, nil
		}
		return current + 1, false, nil
	}
	// Unreachable: validatePriModeFlags caught this.
	return current, true, nil
}
