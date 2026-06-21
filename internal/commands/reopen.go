package commands

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newReopenCmd implements `tsk reopen [<id>...]`: a more discoverable verb
// for the "this should be open again" workflow that tsk currently spells
// `tsk undo <id>`. Same semantics as undo (clears Done + Completed), with
// two extra ergonomic surfaces undo doesn't have:
//
//   - `tsk reopen --last`        re-open the single most recently completed task
//   - `tsk reopen --since 1h`    re-open every task completed within the window
//
// These cover the two common "wait, no" patterns:
//
//	"I marked the wrong one done"        -> tsk reopen --last
//	"I marked a whole batch done"        -> tsk reopen --since 5m
//	"I know exactly which one"           -> tsk reopen 3
//
// IDs and --last / --since are mutually exclusive (combining them would
// have to invent surprising precedence rules — better to refuse).
func newReopenCmd() *cobra.Command {
	var (
		last  bool
		since string
	)
	cmd := &cobra.Command{
		Use:   "reopen [<id>...]",
		Short: "Mark one or more done tasks as undone (with --last / --since shortcuts)",
		Long: `Re-open done tasks. Functionally identical to 'tsk undo' for the id form;
adds --last and --since shortcuts that 'undo' doesn't have.

Selecting tasks:
  reopen 3                    explicit ids (one or more, same as 'undo 3')
  reopen 3 7 12               explicit ids (multiple)
  reopen --last               re-open the single most recently completed task
  reopen --since 1h           re-open every task completed in the last hour
  reopen --since 30m          minute-precision shortcut window

You must pick exactly one mode (ids OR --last OR --since); they cannot
be combined.

The --since value uses tsk's standard duration parser:
  10m, 1h, 24h, 2d, 1w  (s/m/h/d/w suffixes; bare numbers are seconds)

Examples:
  tsk reopen 3
  tsk reopen 3 7
  tsk reopen --last
  tsk reopen --since 5m
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := pickReopenMode(args, last, since)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			ids, err := mode.resolve(s.Tasks)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				pf(cmd.OutOrStdout(), "no tasks to reopen\n")
				return nil
			}
			for _, id := range ids {
				if !s.SetDone(id, false) {
					return fmt.Errorf("no task with id %d", id)
				}
			}
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "reopened %d task(s): %s\n", len(ids), joinIDs(ids))
			return nil
		},
	}
	cmd.Flags().BoolVar(&last, "last", false, "reopen the most recently completed task")
	cmd.Flags().StringVar(&since, "since", "", "reopen tasks completed within this duration (e.g. 30m, 1h, 2d)")
	return cmd
}

// reopenMode is the resolved selection strategy.
type reopenMode struct {
	explicitIDs []int
	last        bool
	since       *time.Duration
}

// pickReopenMode validates that exactly one mode is selected and parses
// the duration if --since is used. Returns a usage error (exit 2) on
// argument shape problems.
func pickReopenMode(args []string, last bool, since string) (reopenMode, error) {
	var m reopenMode
	chosen := 0
	if len(args) > 0 {
		chosen++
	}
	if last {
		chosen++
	}
	if since != "" {
		chosen++
	}
	switch chosen {
	case 0:
		return m, usageErrorf("reopen requires <id> arg(s), --last, or --since <duration>")
	case 1:
		// ok
	default:
		return m, usageErrorf("reopen accepts exactly one of: ids, --last, --since")
	}
	if len(args) > 0 {
		ids, err := parseTaskIDs(args)
		if err != nil {
			return m, err
		}
		m.explicitIDs = ids
		return m, nil
	}
	if last {
		m.last = true
		return m, nil
	}
	d, err := parseReopenDuration(since)
	if err != nil {
		return m, err
	}
	m.since = &d
	return m, nil
}

// resolve returns the sorted list of task IDs to reopen for this mode,
// inspecting the live task slice. Returns an empty slice when --last or
// --since matches nothing (callers print a friendly "no tasks to reopen").
func (m reopenMode) resolve(tasks []model.Task) ([]int, error) {
	if len(m.explicitIDs) > 0 {
		return m.explicitIDs, nil
	}
	if m.last {
		id := mostRecentlyCompletedID(tasks)
		if id == 0 {
			return nil, nil
		}
		return []int{id}, nil
	}
	if m.since != nil {
		cutoff := time.Now().Add(-*m.since)
		ids := completedSinceIDs(tasks, cutoff)
		return ids, nil
	}
	return nil, nil
}

// mostRecentlyCompletedID returns the ID of the done task with the latest
// Completed timestamp, or 0 if no done task carries a completed timestamp.
func mostRecentlyCompletedID(tasks []model.Task) int {
	var (
		bestID   int
		bestTime time.Time
	)
	for _, t := range tasks {
		if !t.Done || t.Completed == nil {
			continue
		}
		if bestID == 0 || t.Completed.After(bestTime) {
			bestID = t.ID
			bestTime = *t.Completed
		}
	}
	return bestID
}

// completedSinceIDs returns the IDs of every done task whose Completed
// timestamp falls at or after cutoff, sorted ascending by ID for stable
// reporting.
func completedSinceIDs(tasks []model.Task, cutoff time.Time) []int {
	var ids []int
	for _, t := range tasks {
		if !t.Done || t.Completed == nil {
			continue
		}
		if t.Completed.Equal(cutoff) || t.Completed.After(cutoff) {
			ids = append(ids, t.ID)
		}
	}
	sort.Ints(ids)
	return ids
}

// parseReopenDuration parses --since values using tsk's existing duration
// shorthand: a bare number is seconds; s/m/h/d/w suffixes scale up. We
// keep this local (rather than importing internal/store's parser) so the
// command stays self-contained and the error messages are reopen-flavored.
func parseReopenDuration(raw string) (time.Duration, error) {
	if raw == "" {
		return 0, usageErrorf("--since requires a duration (e.g. 30m, 1h)")
	}
	// Try Go's native parser first (handles "1h30m", "500ms", etc.).
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d, nil
	}
	// Then the tsk shorthand: <number><s|m|h|d|w>
	if len(raw) >= 2 {
		suffix := raw[len(raw)-1]
		num, err := strconv.Atoi(raw[:len(raw)-1])
		if err == nil && num > 0 {
			switch suffix {
			case 's':
				return time.Duration(num) * time.Second, nil
			case 'm':
				return time.Duration(num) * time.Minute, nil
			case 'h':
				return time.Duration(num) * time.Hour, nil
			case 'd':
				return time.Duration(num) * 24 * time.Hour, nil
			case 'w':
				return time.Duration(num) * 7 * 24 * time.Hour, nil
			}
		}
	}
	// Bare integer -> seconds.
	if num, err := strconv.Atoi(raw); err == nil && num > 0 {
		return time.Duration(num) * time.Second, nil
	}
	return 0, usageErrorf("invalid --since %q (try 30m, 1h, 2d)", raw)
}

// joinIDs renders IDs as "#3, #7, #12" for the success line.
func joinIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("#%d", id)
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
