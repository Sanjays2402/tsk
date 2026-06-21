package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// freezeUntil is the sentinel "indefinite" wait date. 2099-12-31 is
// far enough in the future that no realistic task survives that long,
// while still being a real parseable date (so it round-trips through
// store.Save → store.Load unchanged and lints clean). Hand-editing
// the meta to anything earlier silently un-freezes the task on that
// date — by design.
//
// The value is exposed for tests (and any future caller that wants
// to detect a frozen task) via the IsFrozen helper.
var freezeUntilDate = time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)

// newFreezeCmd implements `tsk freeze <id>`: shim over `tsk wait` for
// the very common "I don't want to see this for the foreseeable
// future" case. Equivalent to `tsk wait <id> 2099-12-31` plus the
// usability win of not having to remember a magic date.
//
// Frozen tasks behave identically to any waiting task:
//   - hidden from default `tsk ls`, `tsk top`, `tsk next`, `tsk daily`
//   - surfaced by `tsk ls --all`, `tsk ls --include-waiting`, and
//     `tsk wait --list`
//   - cleared with either `tsk thaw <id>` or `tsk wait <id> --clear`
//
// Why not just use `wait` directly?
//
//	tsk wait 3 2099-12-31        # works, but easy to mistype
//	tsk freeze 3                 # impossible to mistype, and the
//	                             # verb tells future-you what you
//	                             # meant
//
// `tsk freeze` does NOT introduce a new persisted state — the meta
// comment still says `wait:2099-12-31`. That means existing wait
// machinery (filters, listing, --clear) all work without changes.
func newFreezeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "freeze <id>...",
		Short: "Hide tasks indefinitely (shorthand for `wait <id> 2099-12-31`)",
		Long: `Hide one or more tasks indefinitely. The "I don't ever want to see
this again unless I go looking for it" verb.

Equivalent to running 'tsk wait <id> 2099-12-31' on each id. Frozen
tasks are hidden from default views (ls, top, next, daily) but stay
in the file; surface them with 'tsk wait --list', 'tsk ls --all', or
'tsk ls --include-waiting'.

To bring a frozen task back, use 'tsk thaw <id>' or 'tsk wait <id>
--clear'.

Examples:
  tsk freeze 3
  tsk freeze 3 5 7      # freeze several at once
  tsk thaw 3            # inverse
`,
		Args: cobra.MinimumNArgs(1),
		RunE: runFreezeToggle(true),
	}
	return cmd
}

// newThawCmd implements `tsk thaw <id>`: the inverse of `freeze`. A
// pure wait-clear, but with a verb that pairs naturally with freeze.
func newThawCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "thaw <id>...",
		Short: "Un-freeze tasks (clear their wait-until date)",
		Long: `Un-freeze one or more tasks. Clears the wait-until date so the task
reappears in default views immediately.

This is an alias for 'tsk wait <id> --clear' that pairs with
'tsk freeze'. Works on any waiting task, not just ones frozen via
'tsk freeze' (any task with a wait date can be thawed).

Examples:
  tsk thaw 3
  tsk thaw 3 5 7
`,
		Args: cobra.MinimumNArgs(1),
		RunE: runFreezeToggle(false),
	}
}

// runFreezeToggle is the shared body of freeze/thaw: parse ids,
// validate they exist, mutate WaitUntil, save once. Idempotent —
// already-frozen / already-thawed ids report as a no-op and contribute
// 0 to the change count.
func runFreezeToggle(freezing bool) func(*cobra.Command, []string) error {
	verb := "frozen"
	if !freezing {
		verb = "thawed"
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
		// Validate every id up-front so we never half-apply.
		for _, id := range ids {
			if s.ByID(id) == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
		}
		changed := 0
		for _, id := range ids {
			t := s.ByID(id)
			if freezing {
				if t.WaitUntil != nil && t.WaitUntil.Equal(freezeUntilDate) {
					continue
				}
				d := freezeUntilDate
				t.WaitUntil = &d
				changed++
				continue
			}
			// thaw
			if t.WaitUntil == nil {
				continue
			}
			t.WaitUntil = nil
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

// IsFrozen reports whether a task is using the freeze sentinel date.
// Exposed for callers that want to render frozen tasks differently
// from regular waiting ones (e.g. `tsk wait --list` could prefix a
// freeze marker). Currently used only by tests; the freeze/wait
// distinction is otherwise invisible to commands.
func IsFrozen(t model.Task) bool {
	if t.WaitUntil == nil {
		return false
	}
	return t.WaitUntil.Equal(freezeUntilDate)
}
