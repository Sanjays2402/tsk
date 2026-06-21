package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newCloneCmd implements `tsk clone <id>` (aka `tsk dupe`): duplicate a
// task with a fresh ID. Useful for recurring-ish work (the same checklist
// every Friday) before tsk grows real recurrence, and for splitting one
// task into two parallel branches.
//
// What carries over:
//   - title (with an optional " (copy)" suffix; opt out with --no-suffix)
//   - priority
//   - due date
//   - tags
//   - notes
//
// What does NOT carry over:
//   - id (always freshly assigned by the store)
//   - done state (clones start open)
//   - created timestamp (clones get a fresh "now")
//   - completed timestamp (cleared)
//
// Useful flags:
//
//	--title "..."   override the cloned title entirely (skips the suffix)
//	--no-suffix     keep the title byte-identical to the source
//	--n <N>         create N copies at once (default 1; >=1)
func newCloneCmd() *cobra.Command {
	var (
		titleOverride string
		noSuffix      bool
		count         int
	)
	cmd := &cobra.Command{
		Use:     "clone <id>",
		Aliases: []string{"dupe", "duplicate"},
		Short:   "Duplicate a task with a fresh ID",
		Long: `Duplicate a single task. The clone gets a fresh ID and a fresh
'created' timestamp, starts open (even if the source is done), and inherits
priority, due date, tags, and notes from the source.

By default the clone's title gets a " (copy)" suffix so the two are easy
to tell apart in 'tsk ls'. Pass --no-suffix to keep the title identical,
or --title to override it entirely.

Use --n to create several clones at once (e.g. a weekly checklist split
into per-day instances). Each clone is a separate save; if any clone
fails partway through, prior clones stay.

Examples:
  tsk clone 3                     # adds "<title> (copy)"
  tsk clone 3 --no-suffix         # exact-title duplicate
  tsk clone 3 --title "next sprint version"
  tsk clone 3 --n 5               # five clones in one call
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			if count < 1 {
				return usageErrorf("--n must be >= 1, got %d", count)
			}
			titleOverride = strings.TrimSpace(titleOverride)
			if titleOverride != "" && noSuffix {
				return usageErrorf("--title and --no-suffix are mutually exclusive")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			src := s.ByID(id)
			if src == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			newIDs := make([]int, 0, count)
			for i := 0; i < count; i++ {
				clone := cloneTask(*src, titleOverride, noSuffix)
				newIDs = append(newIDs, s.Add(clone))
			}
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "cloned #%d -> %s\n", id, formatNewIDs(newIDs))
			return nil
		},
	}
	cmd.Flags().StringVar(&titleOverride, "title", "", "override the cloned title (skips the (copy) suffix)")
	cmd.Flags().BoolVar(&noSuffix, "no-suffix", false, "do not append ' (copy)' to the cloned title")
	cmd.Flags().IntVar(&count, "n", 1, "number of clones to create")
	return cmd
}

// cloneTask returns a fresh model.Task derived from src, ready to be passed
// to store.Add (which will assign a new ID).
func cloneTask(src model.Task, titleOverride string, noSuffix bool) model.Task {
	c := model.Task{
		Title:    chooseCloneTitle(src.Title, titleOverride, noSuffix),
		Priority: src.Priority,
		Notes:    src.Notes,
		Created:  time.Now(),
	}
	// Copy tags by value so later mutations on the source don't bleed into
	// the clone (or vice versa).
	if len(src.Tags) > 0 {
		c.Tags = append([]string(nil), src.Tags...)
	}
	// Copy due time by value (Task.Due is *time.Time; we need our own
	// pointer so clearing the clone's due doesn't clear the source's).
	if src.Due != nil {
		d := *src.Due
		c.Due = &d
	}
	// Clones explicitly start open. Done/Completed are zero-value by
	// construction here, but document the contract.
	c.Done = false
	c.Completed = nil
	return c
}

// chooseCloneTitle resolves the clone's title from the three sources:
// explicit --title override wins, otherwise we add the " (copy)" suffix
// unless --no-suffix is set.
func chooseCloneTitle(srcTitle, override string, noSuffix bool) string {
	if override != "" {
		return override
	}
	if noSuffix {
		return srcTitle
	}
	return srcTitle + " (copy)"
}

// formatNewIDs renders 1+ new IDs as "#5" or "#5, #6, #7".
func formatNewIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("#%d", id)
	}
	return strings.Join(parts, ", ")
}
