package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newRenameCmd implements `tsk rename <id> <new title...>`: a quick
// single-task title change that doesn't require dropping into the TUI
// or hand-editing the .tsk.md.
//
// The new title is joined from every arg after the id, so you don't have
// to quote multi-word titles:
//
//	tsk rename 3 buy more milk
//
// is equivalent to the (also-accepted) `tsk rename 3 "buy more milk"`.
//
// The title is trimmed; whitespace-only titles are rejected with a usage
// error so main exits 2. Identical-to-current is a no-op (no save, no
// snapshot churn).
func newRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rename <id> <new title...>",
		Aliases: []string{"retitle"},
		Short:   "Change a task's title",
		Long: `Change the title of a single task in-place.

The new title is built by joining every arg after the id with spaces, so
quoting is optional:

  tsk rename 3 buy more milk
  tsk rename 3 "buy more milk"

Both forms produce the same result. Identical titles are a no-op. Empty
or whitespace-only titles are rejected with a usage error (exit 2).

Examples:
  tsk rename 3 ship the autoship loop
  tsk retitle 7 "refactor the parser"
`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			newTitle := strings.TrimSpace(strings.Join(args[1:], " "))
			if newTitle == "" {
				return usageErrorf("rename requires a non-empty title")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			if t.Title == newTitle {
				pf(cmd.OutOrStdout(), "#%d title unchanged\n", id)
				return nil
			}
			old := t.Title
			t.Title = newTitle
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "#%d title %q -> %q\n", id, old, newTitle)
			return nil
		},
	}
}
