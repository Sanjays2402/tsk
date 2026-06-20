package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newStartCmd implements `tsk start <id>`: mark a task as in-progress
// by stamping `started:<now>` into its meta. Pairs with `tsk stop`
// (clear) and `tsk done` (which auto-clears it on completion).
//
// In-progress is a NEW state — it sits between "open" and "done":
//
//	open      → start → in-progress → done    (normal flow)
//	open      → done                          (didn't track work)
//	in-progress → stop → open                 (paused, will resume later)
//	in-progress → done                        (finished; started: dropped
//	                                           because Completed wins)
//
// The persisted form is `started:<RFC3339>` in the existing meta
// comment — strictly additive, the way `pin:true` and `wait:<date>`
// were. Old files round-trip unchanged. Tasks with a `started:` meta
// from a hand-edit also work (just like pin/wait).
//
// Multi-id: `tsk start 3 5 7` flips all three on in a single Save.
// Idempotent: starting an already-started task is a no-op with a
// "no change" message, NOT an error (matches pin/freeze conventions).
//
// One subtle interaction worth flagging:
//
//   - Restarting an already-started task: `tsk start --reset` lets the
//     user explicitly bump the started: timestamp to NOW. Without
//     --reset the existing start time is preserved (so "I forgot I
//     had started this an hour ago" doesn't accidentally zero the
//     elapsed time).
//
// `tsk done` clears Started on completion (Completed is the more
// useful timestamp at that point — the task moved past in-progress).
func newStartCmd() *cobra.Command {
	var reset bool
	cmd := &cobra.Command{
		Use:   "start <id>...",
		Short: "Mark tasks as in-progress (stamp started:<now>)",
		Long: `Mark one or more tasks as in-progress. Stamps started:<now> into
the task's meta block, persisted alongside other metadata.

The "in-progress" state sits between open and done. Pair with:
  tsk stop <id>       pause work (clear started:)
  tsk done <id>       finish (Completed wins; started: is dropped)
  tsk show <id>       see started: in the detail view
  tsk in-progress     list everything currently being worked on

By default, starting an already-started task is a no-op so you can
re-run safely. Pass --reset to bump started: to right now.

Examples:
  tsk start 3
  tsk start 3 5 7              # several at once
  tsk start 3 --reset          # bump started: even if already started
`,
		Args: cobra.MinimumNArgs(1),
		RunE: runStartStop(true, &reset),
	}
	cmd.Flags().BoolVar(&reset, "reset", false, "bump started: to now even if the task is already started")
	return cmd
}

// newStopCmd implements `tsk stop <id>`: clear the started: timestamp.
// Pure inverse of start, with the same multi-id + idempotent shape.
func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <id>...",
		Short: "Mark tasks as no longer in-progress (clear started:)",
		Long: `Mark one or more tasks as no longer in-progress. Clears the
started: meta key.

Use this when you've put a task down and don't expect to pick it up
again soon. (For "done", use 'tsk done'; for "later", 'tsk snooze'
or 'tsk wait'.)

Examples:
  tsk stop 3
  tsk stop 3 5 7
`,
		Args: cobra.MinimumNArgs(1),
		RunE: runStartStop(false, nil),
	}
}

// runStartStop is the shared body of start/stop. Mirrors the
// pin/unpin / freeze/thaw idempotency contract: every requested id
// must exist (validated up-front so no partial state lands), then
// flip whatever's different. Reports "no change" when nothing
// happened.
func runStartStop(starting bool, reset *bool) func(*cobra.Command, []string) error {
	verb := "started"
	if !starting {
		verb = "stopped"
	}
	return func(cmd *cobra.Command, args []string) error {
		ids, err := parseTaskIDs(args)
		if err != nil {
			return err
		}
		s, err := resolveStore(cmd, true)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if s.ByID(id) == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
		}
		now := time.Now()
		changed := 0
		for _, id := range ids {
			t := s.ByID(id)
			if t.Done {
				// Done tasks aren't in-progress candidates; surfacing
				// the conflict here is more helpful than silently
				// flipping a meta key on a completed task.
				return usageErrorf("#%d is already done — start/stop don't apply (use `tsk reopen %d` first)", id, id)
			}
			if starting {
				if t.Started != nil && (reset == nil || !*reset) {
					continue
				}
				ts := now
				t.Started = &ts
				changed++
				continue
			}
			// stop
			if t.Started == nil {
				continue
			}
			t.Started = nil
			changed++
		}
		if changed == 0 {
			pf(cmd.OutOrStdout(), "no change (%d already %s)\n", len(ids), verb)
			return nil
		}
		if err := s.Save(); err != nil {
			return err
		}
		pf(cmd.OutOrStdout(), "%s %d task(s)\n", verb, changed)
		return nil
	}
}

// newInProgressCmd implements `tsk in-progress`: list every task
// currently being worked on (Started != nil && !Done), sorted by
// most-recent start first. Designed for "what am I in the middle
// of?" answers. Aliased `wip` for muscle memory.
func newInProgressCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "in-progress",
		Aliases: []string{"wip", "inprogress"},
		Short:   "List tasks currently in-progress (started: set, not done)",
		Long: `List every task currently marked in-progress. Sorted by most-recent
started: timestamp first, with the elapsed time shown so you can see
which tasks are getting stale.

Examples:
  tsk in-progress
  tsk wip                     # alias
  tsk in-progress --json
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			now := time.Now()
			out := make([]model.Task, 0)
			for _, t := range s.Tasks {
				if t.IsInProgress() {
					out = append(out, t)
				}
			}
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].Started.After(*out[j].Started)
			})
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if out == nil {
					out = []model.Task{}
				}
				return enc.Encode(out)
			}
			if len(out) == 0 {
				pln(cmd.OutOrStdout(), "no in-progress tasks")
				return nil
			}
			for _, t := range out {
				elapsed := humanizeElapsed(now.Sub(*t.Started))
				pf(cmd.OutOrStdout(), "#%d [%s] %s  (started %s ago)\n",
					t.ID, t.Priority.Short(), t.Title, elapsed)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array of in-progress tasks")
	return cmd
}

// humanizeElapsed renders a duration as the largest non-zero unit
// (NNNd, NNNh, NNNm). Anything under a minute reads "<1m" because
// "0m ago" looks broken. Used only for the in-progress display;
// kept local so it doesn't grow into a util pile.
func humanizeElapsed(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
