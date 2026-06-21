package commands

import (
	"fmt"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

// newNextCmd implements `tsk next`: surface the highest-priority
// undone task with the canonical tie-break (pin > priority desc >
// dated-first > earliest-due > lowest-id).
//
// --respect-deps skips tasks that are blocked by at least one open
// prerequisite. That's what most users actually want when they ask
// "what should I work on next?" — a task you can't legally close
// shouldn't be the answer. Defaults to OFF so existing scripts that
// rely on the legacy "pure priority" behavior keep working; opt in
// with the flag (or the future config knob).
//
// When --respect-deps is set and EVERY undone task is blocked, the
// command falls back to surfacing the highest-priority blocked one
// with a "(blocked)" annotation rather than going silent — the user
// likely wants to know "everything's stuck on X" instead of
// "all caught up" (which would be a lie).
func newNextCmd() *cobra.Command {
	var respectDeps bool
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show the highest-priority undone task",
		Long: `Show the highest-priority undone task.

Pin and priority drive selection; ties break on due date then id.

--respect-deps skips tasks blocked by open prerequisites. That's
usually what you want — a task you can't legally close shouldn't be
the suggested "next thing". When every candidate is blocked, the
command falls back to the highest-priority blocked task with a
"(blocked by #X, #Y)" annotation so you know what's gating progress.

Examples:
  tsk next                       # legacy: priority-only
  tsk next --respect-deps        # skip tasks with unmet prereqs
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			now := time.Now()
			var best, bestBlocked *model.Task
			var bestBlockers []int
			for i := range s.Tasks {
				t := &s.Tasks[i]
				if t.Done {
					continue
				}
				if t.IsWaiting(now) {
					continue
				}
				if respectDeps {
					blockers := unmetBlockers(s, t, nil)
					if len(blockers) > 0 {
						// Track best blocked candidate as fallback.
						if isBetterNext(t, bestBlocked) {
							bestBlocked = t
							bestBlockers = blockers
						}
						continue
					}
				}
				if isBetterNext(t, best) {
					best = t
				}
			}
			if best == nil && bestBlocked != nil {
				// All candidates blocked — surface the best blocked one
				// with annotation so the user knows what's stuck.
				printNextLine(cmd, bestBlocked, bestBlockers)
				return nil
			}
			if best == nil {
				pln(cmd.OutOrStdout(), "all caught up")
				return nil
			}
			printNextLine(cmd, best, nil)
			return nil
		},
	}
	cmd.Flags().BoolVar(&respectDeps, "respect-deps", false, "skip tasks with unmet prerequisites")
	return cmd
}

// isBetterNext returns true when t should beat current under the
// canonical next-task ordering (pin > priority desc > dated-first >
// earliest-due > lowest-id). Reuses the same tie-breaks as `tsk top`
// so top[0] and `next` agree when no pins are in play.
func isBetterNext(t, current *model.Task) bool {
	if current == nil {
		return true
	}
	if t.Pinned != current.Pinned {
		return t.Pinned
	}
	if t.Priority != current.Priority {
		return t.Priority > current.Priority
	}
	switch {
	case t.Due != nil && current.Due == nil:
		return true
	case t.Due == nil && current.Due != nil:
		return false
	case t.Due != nil && current.Due != nil:
		if !t.Due.Equal(*current.Due) {
			return t.Due.Before(*current.Due)
		}
	}
	return t.ID < current.ID
}

// printNextLine renders the result row. When blockers is non-empty,
// append " (blocked by #X, #Y)" so the user understands why this
// task came back as the best available even though they asked
// --respect-deps.
func printNextLine(cmd *cobra.Command, t *model.Task, blockers []int) {
	pinMark := ""
	if t.Pinned {
		pinMark = "* "
	}
	line := fmt.Sprintf("%s#%d [%s] %s", pinMark, t.ID, t.Priority, t.Title)
	if t.Due != nil {
		line += "  due:" + t.Due.Format(model.DateLayout)
	}
	if len(blockers) > 0 {
		line += "  (blocked by " + formatBlockerIDs(blockers) + ")"
	}
	pln(cmd.OutOrStdout(), line)
}
