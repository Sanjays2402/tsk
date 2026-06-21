package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
)

// newSnoozeCmd implements `tsk snooze <id> <date>`: push a task's due date
// forward. The defining trait vs `tsk due`: snooze REFUSES to move a due
// date backward (or to clear it), because the whole point is "I'm not
// dealing with this until later". Use `tsk due` for arbitrary set/clear.
//
// Refusing the backward move is a guard, not a hard wall — pass --force
// to override (e.g. you misread the date and 'next week' was supposed
// to be 'tomorrow').
//
// If the task has no due date yet, snooze just sets it — there's no
// "backward" to refuse. That makes `tsk snooze <id> tomorrow` work as
// a convenient "I'll think about this tomorrow" verb on fresh tasks too.
func newSnoozeCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "snooze <id> <date>",
		Short: "Push a task's due date forward (refuses to move it backward)",
		Long: `Push a task's due date forward.

Same date vocabulary as 'tsk add --due' and 'tsk due': YYYY-MM-DD,
tomorrow, fri, in 3d, jul 4, eow, ...

Snooze refuses to move a due date backward — its whole point is "deal
with this later". Pass --force if you genuinely want to set an earlier
date (e.g. you misread, or you want to use snooze's vocabulary as a
shorthand for 'due').

If the task has no due date set, snooze just sets it (there's nothing
to compare against; nothing to refuse).

Examples:
  tsk snooze 3 tomorrow
  tsk snooze 7 "next monday"
  tsk snooze 12 +1w               # one week from today
  tsk snooze 5 fri --force        # override the no-backward guard
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			raw := strings.TrimSpace(args[1])
			if raw == "" {
				return usageErrorf("snooze requires a non-empty <date>")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			loc := PacificLoc()
			parsed, err := dateparse.Parse(raw, time.Now().In(loc), loc)
			if err != nil {
				return usageErrorf("%s", err.Error())
			}
			return applySnooze(cmd, s, t, parsed, force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "allow moving the due date backward")
	return cmd
}

// applySnooze enforces the no-backward-move rule (unless force), reports
// no-ops, persists changes, and prints the transition line.
func applySnooze(cmd *cobra.Command, s saver, t *model.Task, parsed time.Time, force bool) error {
	newStr := parsed.Format(model.DateLayout)
	// No existing due date: snooze degrades to a plain set.
	if t.Due == nil {
		t.Due = &parsed
		if err := s.Save(); err != nil {
			return err
		}
		pf(cmd.OutOrStdout(), "#%d due - -> %s (initial)\n", t.ID, newStr)
		return nil
	}
	oldStr := t.Due.Format(model.DateLayout)
	if oldStr == newStr {
		pf(cmd.OutOrStdout(), "#%d already due %s (no snooze applied)\n", t.ID, newStr)
		return nil
	}
	if parsed.Before(*t.Due) && !force {
		return usageErrorf(
			"refusing to move #%d due date backward (%s -> %s); use --force to override",
			t.ID, oldStr, newStr,
		)
	}
	t.Due = &parsed
	if err := s.Save(); err != nil {
		return err
	}
	pf(cmd.OutOrStdout(), "#%d due %s -> %s\n", t.ID, oldStr, newStr)
	return nil
}
