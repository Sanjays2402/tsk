package commands

import (
	"github.com/spf13/cobra"
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
// Runtime is a thin call into runStartStop(false, nil) — the same
// function `tsk stop` uses — so semantics literally cannot drift
// between the two surfaces.
func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "pause <id>...",
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

Idempotent: pausing a non-started task is a no-op with a "no change"
message.

Examples:
  tsk pause 3
  tsk pause 3 5 7
  tsk hold 3                  # alias
`,
		Args: cobra.MinimumNArgs(1),
		RunE: runStartStop(false, nil),
	}
}
