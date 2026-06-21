package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
)

// newDueCmd implements `tsk due <id> <date>`: a single-task due-date setter
// that replaces the verbose `tsk bulk --id N --set-due <date> --apply`.
//
// The date arg supports the full natural-language vocabulary of `tsk add
// --due`: YYYY-MM-DD, tomorrow, fri, in 3d, jul 4, eow, etc.
//
// `--clear` removes the due date instead. The two are mutually exclusive
// with the positional date arg.
func newDueCmd() *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "due <id> [<date>]",
		Short: "Set or clear the due date on a single task",
		Long: `Set or clear the due date on a single task.

The date argument supports every spelling tsk add --due does
(YYYY-MM-DD, tomorrow, fri, in 3d, jul 4, eow, ...). Pass --clear to
remove the due date instead of setting one (no date arg required).

Examples:
  tsk due 3 tomorrow
  tsk due 7 2099-12-31
  tsk due 12 "in 3 days"
  tsk due 5 --clear
`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			// Argument shape validation: exactly one of (date arg | --clear)
			// must be provided.
			switch {
			case clear && len(args) == 2:
				return usageErrorf("--clear and a date argument are mutually exclusive")
			case !clear && len(args) == 1:
				return usageErrorf("due requires a <date> argument (or --clear to remove the due date)")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			if clear {
				return runDueClear(cmd, s, t)
			}
			return runDueSet(cmd, s, t, args[1])
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "remove the due date instead of setting one")
	return cmd
}

// runDueClear unsets t.Due and persists. Reports a no-op when already clear.
func runDueClear(cmd *cobra.Command, s saver, t *model.Task) error {
	if t.Due == nil {
		pf(cmd.OutOrStdout(), "#%d already has no due date\n", t.ID)
		return nil
	}
	old := t.Due.Format(model.DateLayout)
	t.Due = nil
	if err := s.Save(); err != nil {
		return err
	}
	pf(cmd.OutOrStdout(), "#%d due %s -> cleared\n", t.ID, old)
	return nil
}

// runDueSet parses the date arg in the active timezone and persists.
func runDueSet(cmd *cobra.Command, s saver, t *model.Task, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return usageErrorf("due requires a non-empty <date>")
	}
	loc := PacificLoc()
	parsed, err := dateparse.Parse(raw, time.Now().In(loc), loc)
	if err != nil {
		return usageErrorf("%s", err.Error())
	}
	newStr := parsed.Format(model.DateLayout)
	if t.Due != nil && t.Due.Format(model.DateLayout) == newStr {
		pf(cmd.OutOrStdout(), "#%d already due %s\n", t.ID, newStr)
		return nil
	}
	old := "-"
	if t.Due != nil {
		old = t.Due.Format(model.DateLayout)
	}
	t.Due = &parsed
	if err := s.Save(); err != nil {
		return err
	}
	pf(cmd.OutOrStdout(), "#%d due %s -> %s\n", t.ID, old, newStr)
	return nil
}

// saver is a tiny interface satisfied by *store.Store so the run helpers
// stay easy to unit-test in isolation if we ever want to.
type saver interface {
	Save() error
}
