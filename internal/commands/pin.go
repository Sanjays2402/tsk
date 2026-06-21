package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newPinCmd implements `tsk pin <id>`: sticky-flag a task so it floats
// to the top of `tsk top` and `tsk next` regardless of its priority.
//
// The pin is persisted as `pin:true` in the task's metadata comment so
// it survives across runs and is hand-editable (set `pin:false` or
// delete the key to clear).
//
// Pin is multi-task aware: `tsk pin 3 5 7` flips all three on. Use
// `tsk unpin` for the inverse.
func newPinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pin <id>...",
		Short: "Sticky-flag tasks so they float to the top of top/next",
		Long: `Sticky-flag one or more tasks. Pinned tasks appear FIRST in 'tsk top'
and become the winner of 'tsk next' regardless of priority — useful for
"this is what I'm working on right now, don't make me re-find it".

The flag is persisted as 'pin:true' in the .tsk.md metadata comment, so
it survives across runs and is hand-editable.

Examples:
  tsk pin 3
  tsk pin 3 5 7        # pin several at once
  tsk unpin 3          # clear it
`,
		Args: cobra.MinimumNArgs(1),
		RunE: runPinToggle(true),
	}
}

// newUnpinCmd implements `tsk unpin <id>`: the inverse of `pin`.
func newUnpinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unpin <id>...",
		Short: "Clear the pin flag on one or more tasks",
		Long: `Clear the pin flag on one or more tasks. The inverse of 'tsk pin'.

Examples:
  tsk unpin 3
  tsk unpin 3 5 7
`,
		Args: cobra.MinimumNArgs(1),
		RunE: runPinToggle(false),
	}
}

// runPinToggle is the shared body of pin/unpin: parse ids, flip the flag,
// save once. Idempotent — already-pinned tasks pass through unchanged
// and are reported as a no-op.
func runPinToggle(pin bool) func(*cobra.Command, []string) error {
	verb := "pinned"
	if !pin {
		verb = "unpinned"
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
		// Verify every id exists FIRST so we never half-apply.
		for _, id := range ids {
			if s.ByID(id) == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
		}
		changed := 0
		for _, id := range ids {
			t := s.ByID(id)
			if t.Pinned == pin {
				continue
			}
			t.Pinned = pin
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
