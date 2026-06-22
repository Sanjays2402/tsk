package commands

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newPauseCmd implements `tsk pause <id>`: a discoverable verb that
// pairs visually with `tsk start`. Semantically identical to
// `tsk stop`: clears the started: timestamp.
//
// Why a dedicated command rather than a cobra Alias on `stop`? Same
// reason `blocked` (alias of `depend --list`) and `reachable` (alias
// of `graph --reachable`) got top-level commands instead: a cobra
// Alias surfaces as `tsk stop pause` in help/man output, burying it.
// A top-level verb appears in `tsk --help` and gets its own shell
// completion entry.
//
// The pair start/pause/done reads more naturally than start/stop/done
// for users coming from time-tracker apps (Toggl, Harvest, RescueTime),
// where "stop" usually means "end the timer entirely" and "pause"
// means "I'm coming back to this". In tsk both verbs do the same
// thing (clear started:), but `pause` is the right name when the
// task is still on the active list and you intend to resume.
//
// Runtime delegates to runStartStop(false, nil) — the same body
// `tsk stop` uses — so semantics literally cannot drift between
// the two surfaces (positional id case).
//
// --all is the end-of-day-clear shortcut: pause every task currently
// in-progress at once. Without it the user has to type
// `tsk wip --json | jq '.[].id' | xargs tsk pause` which is hostile
// at the end of a long day. With --all the call is a single verb
// that resolves the wip set internally, in one Save, with the same
// "no in-progress tasks" message wip already prints when the set
// is empty (so the empty case isn't confusing).
//
// --all is mutually exclusive with positional ids: the whole point
// is "don't bother typing them out", and combining the two would
// hide a typo (e.g. `tsk pause --all 3` could plausibly mean
// "pause everything except 3" or "also pause 3 if not started";
// rejecting the combination forces the user to be explicit).
func newPauseCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "pause [<id>...]",
		Aliases: []string{"hold"},
		Short:   "Pause tasks (alias for `stop`; pairs visually with `start`)",
		Long: `Pause one or more tasks. Equivalent to ` + "`tsk stop`" + ` —
clears the started: timestamp — but named to pair visually with
` + "`tsk start`" + ` for users who think in start/pause/resume cycles
(time-tracker muscle memory).

Pair with:
  tsk start <id>       resume work (re-sets started:<now>)
  tsk done  <id>       finish (clears started, sets completed)
  tsk wip              list everything currently in-progress

Pass --all to pause every task currently in-progress at once
(end-of-day clear). Mutually exclusive with positional ids.

Idempotent: pausing a non-started task is a no-op with a "no change"
message.

Examples:
  tsk pause 3
  tsk pause 3 5 7
  tsk hold 3                  # alias
  tsk pause --all             # pause everything currently in-progress
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				if len(args) > 0 {
					return usageErrorf("--all takes no positional ids (pause every in-progress task)")
				}
				return runPauseAll(cmd)
			}
			if len(args) == 0 {
				return usageErrorf("missing <id> (or pass --all)")
			}
			return runStartStop(false, nil)(cmd, args)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "pause every task currently in-progress (end-of-day clear)")
	return cmd
}

// runPauseAll resolves the in-progress set inside the store and
// dispatches to runStartStop with those ids. Keeps the runStartStop
// body the single source of truth for the actual mutation (so any
// future enforcement, e.g. \"done tasks reject pause\", applies here
// too without a parallel guard).
//
// Empty in-progress set is reported with the same \"no in-progress
// tasks\" line `tsk wip` uses, so the empty case is consistent across
// the two verbs (the user sees the same answer whether they checked
// first or just ran pause --all).
func runPauseAll(cmd *cobra.Command) error {
	s, err := resolveStore(cmd, true)
	if err != nil {
		return err
	}
	ids := inProgressIDs(s.Tasks)
	if len(ids) == 0 {
		pln(cmd.OutOrStdout(), "no in-progress tasks")
		return nil
	}
	args := make([]string, len(ids))
	for i, id := range ids {
		args[i] = fmt.Sprintf("%d", id)
	}
	return runStartStop(false, nil)(cmd, args)
}

// inProgressIDs returns the sorted-ascending ids of every task with
// IsInProgress() true. Sorted for reproducible output ordering in
// runStartStop's summary line and to make tests deterministic.
func inProgressIDs(tasks []model.Task) []int {
	ids := make([]int, 0)
	for _, t := range tasks {
		if t.IsInProgress() {
			ids = append(ids, t.ID)
		}
	}
	sort.Ints(ids)
	return ids
}
