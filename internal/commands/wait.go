package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
)

// newWaitCmd implements `tsk wait <id> <until>`: hide a task from default
// views until the specified date passes.
//
// Wait differs from due in semantics:
//   - due:  "you should finish this BY this date" — overdue is a warning
//   - wait: "don't even SHOW me this until this date arrives" — declutter
//
// Common use: deferred work, follow-ups blocked on someone else, ideas
// you want to revisit later but don't need cluttering today's view.
//
// Once the wait-until date passes, the task reappears in `tsk ls`,
// `tsk top`, `tsk next` automatically — no manual unhide step.
//
// Modes:
//
//	tsk wait <id> <until>     set the wait-until date
//	tsk wait <id> --clear     clear it (task reappears immediately)
//	tsk wait --list           show currently-waiting tasks (with dates)
//
// The list mode is the deliberate escape hatch for "wait, what did I
// defer?". Pair with `tsk wait <id> --clear` to unhide one.
func newWaitCmd() *cobra.Command {
	var (
		clear  bool
		list   bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "wait <id> [<until>]",
		Short: "Hide a task from default views until the given date",
		Long: `Hide a task from default views until the specified date.

Wait is the "deal with this later, but I don't want to see it now"
verb. Once the wait-until date passes, the task reappears in 'tsk ls',
'tsk top', and 'tsk next' automatically.

The wait date uses the same vocabulary as 'tsk add --due': YYYY-MM-DD,
tomorrow, fri, in 3d, jul 4, eow, ...

Modes:
  tsk wait 3 monday      # hide #3 until next Monday
  tsk wait 3 --clear     # un-wait #3 (it reappears immediately)
  tsk wait --list        # show all currently-waiting tasks
  tsk wait --list --json # scriptable wait queue

Waiting tasks remain visible via:
  tsk ls --all                 # everything
  tsk ls --include-waiting     # undone + waiting
  tsk wait --list              # waiting only
  tsk show <id>                # direct lookup
`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWait(cmd, args, clear, list, asJSON)
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "clear the wait date (task reappears immediately)")
	cmd.Flags().BoolVar(&list, "list", false, "list currently-waiting tasks instead of setting one")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (only valid with --list)")
	return cmd
}

// runWait dispatches based on which mode the user asked for and runs
// the right code path. Flag validation lives here to keep the cobra
// RunE clean.
func runWait(cmd *cobra.Command, args []string, clear, list, asJSON bool) error {
	if list && (clear || len(args) > 0) {
		return usageErrorf("--list cannot be combined with an id or --clear")
	}
	if list {
		return runWaitList(cmd, asJSON)
	}
	if asJSON {
		return usageErrorf("--json is only valid with --list")
	}
	if len(args) == 0 {
		return usageErrorf("wait requires an <id> (or --list)")
	}
	id, err := parseSingleID(args[0])
	if err != nil {
		return err
	}
	switch {
	case clear && len(args) == 2:
		return usageErrorf("--clear and a date argument are mutually exclusive")
	case !clear && len(args) == 1:
		return usageErrorf("wait requires a <date> argument (or --clear)")
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
		return runWaitClear(cmd, s, t)
	}
	return runWaitSet(cmd, s, t, args[1])
}

// runWaitClear unsets t.WaitUntil and persists. No-op when already clear.
func runWaitClear(cmd *cobra.Command, s saver, t *model.Task) error {
	if t.WaitUntil == nil {
		pf(cmd.OutOrStdout(), "#%d is not waiting\n", t.ID)
		return nil
	}
	old := t.WaitUntil.Format(model.DateLayout)
	t.WaitUntil = nil
	if err := s.Save(); err != nil {
		return err
	}
	pf(cmd.OutOrStdout(), "#%d wait %s -> cleared\n", t.ID, old)
	return nil
}

// runWaitSet parses the date arg and persists. Same date vocabulary as
// 'tsk add --due'.
func runWaitSet(cmd *cobra.Command, s saver, t *model.Task, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return usageErrorf("wait requires a non-empty <until> date")
	}
	loc := PacificLoc()
	parsed, err := dateparse.Parse(raw, time.Now().In(loc), loc)
	if err != nil {
		return usageErrorf("%s", err.Error())
	}
	newStr := parsed.Format(model.DateLayout)
	if t.WaitUntil != nil && t.WaitUntil.Format(model.DateLayout) == newStr {
		pf(cmd.OutOrStdout(), "#%d already waiting until %s\n", t.ID, newStr)
		return nil
	}
	old := "-"
	if t.WaitUntil != nil {
		old = t.WaitUntil.Format(model.DateLayout)
	}
	t.WaitUntil = &parsed
	if err := s.Save(); err != nil {
		return err
	}
	pf(cmd.OutOrStdout(), "#%d wait %s -> %s\n", t.ID, old, newStr)
	return nil
}

// runWaitList prints the currently-waiting tasks (sorted by wait-until
// ASC so the next-to-unhide appears first).
func runWaitList(cmd *cobra.Command, asJSON bool) error {
	s, err := resolveStore(cmd, true)
	if err != nil {
		return err
	}
	now := time.Now()
	waiting := make([]model.Task, 0)
	for _, t := range s.Tasks {
		if t.IsWaiting(now) {
			waiting = append(waiting, t)
		}
	}
	sort.SliceStable(waiting, func(i, j int) bool {
		if waiting[i].WaitUntil.Equal(*waiting[j].WaitUntil) {
			return waiting[i].ID < waiting[j].ID
		}
		return waiting[i].WaitUntil.Before(*waiting[j].WaitUntil)
	})
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if waiting == nil {
			waiting = []model.Task{}
		}
		return enc.Encode(waiting)
	}
	return printWaitList(cmd.OutOrStdout(), waiting)
}

// printWaitList renders the waiting tasks in a labelled list. The
// wait-until column is what makes this view useful — the user wants
// to know WHEN each task reappears.
func printWaitList(w io.Writer, tasks []model.Task) error {
	if len(tasks) == 0 {
		pln(w, "no waiting tasks")
		return nil
	}
	for _, t := range tasks {
		line := fmt.Sprintf("#%d [%s] %s  (waiting until %s)",
			t.ID, t.Priority.Short(), t.Title, t.WaitUntil.Format(model.DateLayout))
		pln(w, line)
	}
	return nil
}
