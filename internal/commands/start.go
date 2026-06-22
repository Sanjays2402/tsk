package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
//
// --all is the bulk-start sibling of `tsk pause --all`. It REQUIRES
// a scope flag (--tag or --priority) because "start every open
// task in the store" is almost never what a user wants — that
// would flood `tsk wip` with dozens of irrelevant rows. Forcing
// a filter is the only way to keep the verb honest:
//
//	tsk start --all --tag standup     # start every open standup task
//	tsk start --all --priority urgent # start every open urgent task
//	tsk start --all --tag x --priority high  # the AND of both
//
// Done tasks are excluded (start/done is meaningless once a task
// is done — same guard runStartStop enforces per-id). Already-
// started tasks are silently skipped (idempotent, matching the
// per-id contract). Empty result set ("no open tasks match")
// is a clean no-op with a clear message, NOT an error — a typo
// in --tag should surface as "no matches", not as a non-zero
// exit; the user knows quickly to fix the filter.
//
// Why a required scope rather than allowing `tsk start --all`
// to mean "literally everything"? The pause sister has the
// natural scope of "every wip task", which is usually a small
// set the user just curated themselves. start --all has no
// such natural scope — the "every open task" interpretation
// would start dozens of items the user has no current context
// for, which is the opposite of what start: is for. Requiring
// a filter forces the verb to mean what it should: "start a
// curated subset I'm about to focus on".
func newStartCmd() *cobra.Command {
	var (
		reset     bool
		all       bool
		startTag  string
		startPrio string
	)
	cmd := &cobra.Command{
		Use:   "start [<id>...]",
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

Pass --all with --tag and/or --priority to start every open task
matching the filter. Sister of ` + "`tsk pause --all`" + `: bulk-start
a curated subset rather than typing ids one at a time. Requires
at least one filter (no "start literally every open task" form).

Examples:
  tsk start 3
  tsk start 3 5 7                          # several at once
  tsk start 3 --reset                      # bump started: even if already started
  tsk start --all --tag standup            # start every open standup task
  tsk start --all --priority urgent        # start every open urgent task
  tsk start --all --tag work --priority high
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				if len(args) > 0 {
					return usageErrorf("--all takes no positional ids (resolves the filtered set internally)")
				}
				return runStartAll(cmd, startTag, startPrio, reset)
			}
			if len(args) == 0 {
				return usageErrorf("missing <id> (or pass --all with --tag/--priority)")
			}
			return runStartStop(true, &reset)(cmd, args)
		},
	}
	cmd.Flags().BoolVar(&reset, "reset", false, "bump started: to now even if the task is already started")
	cmd.Flags().BoolVar(&all, "all", false, "start every open task matching --tag and/or --priority")
	cmd.Flags().StringVar(&startTag, "tag", "", "for --all: only start tasks carrying this tag (case-insensitive)")
	cmd.Flags().StringVar(&startPrio, "priority", "", "for --all: only start tasks at this priority (low/medium/high/urgent)")
	return cmd
}

// runStartAll resolves the open-task set matching the requested
// filter scope and dispatches to runStartStop for the actual
// transition. Keeps the runStartStop body the single source of
// truth so future invariants (e.g. "don't start tasks past a wait
// date") get applied uniformly.
//
// Requires at least one filter — see the doc comment on newStartCmd
// for why. The empty-set case ("no open tasks match") is a clean
// no-op so a typo in --tag exits 0 with a clear message rather
// than firing a non-zero exit that could trip a wrapper script.
func runStartAll(cmd *cobra.Command, tag, prioRaw string, reset bool) error {
	tag = strings.TrimSpace(tag)
	prio, prioActive, err := parsePendingPriority(prioRaw)
	if err != nil {
		return err
	}
	if tag == "" && !prioActive {
		return usageErrorf("--all requires --tag and/or --priority (refusing to start every open task in the store)")
	}
	s, err := resolveStore(cmd, true)
	if err != nil {
		return err
	}
	ids := filterStartAllIDs(s.Tasks, tag, prio, prioActive)
	if len(ids) == 0 {
		filters := buildStartAllFilterSummary(tag, prioRaw, prioActive)
		pf(cmd.OutOrStdout(), "no open tasks match (%s)\n", filters)
		return nil
	}
	args := make([]string, len(ids))
	for i, id := range ids {
		args[i] = fmt.Sprintf("%d", id)
	}
	return runStartStop(true, &reset)(cmd, args)
}

// filterStartAllIDs returns the sorted-ascending ids of every OPEN
// task matching the filter. Done tasks are excluded (start/done
// is meaningless); already-started tasks STAY in the set so the
// idempotent-skip in runStartStop covers them with the standard
// "no change" message (no special-casing here).
func filterStartAllIDs(tasks []model.Task, tag string, prio model.Priority, prioActive bool) []int {
	ids := make([]int, 0)
	for _, t := range tasks {
		if t.Done {
			continue
		}
		if tag != "" && !t.HasTag(tag) {
			continue
		}
		if prioActive && t.Priority != prio {
			continue
		}
		ids = append(ids, t.ID)
	}
	sort.Ints(ids)
	return ids
}

// buildStartAllFilterSummary mirrors the depend --pending filter
// summary shape so the two bulk-action verbs read the same way
// when they print an empty-result line. Deterministic ordering:
// tag first, then priority.
func buildStartAllFilterSummary(tag, prioRaw string, prioActive bool) string {
	parts := make([]string, 0, 2)
	if tag != "" {
		parts = append(parts, "tag="+tag)
	}
	if prioActive {
		parts = append(parts, "priority="+strings.ToLower(strings.TrimSpace(prioRaw)))
	}
	return strings.Join(parts, ", ")
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
