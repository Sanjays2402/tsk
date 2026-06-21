package commands

import (
	"github.com/spf13/cobra"
)

// newBlockedCmd implements `tsk blocked`: a discoverable verb for
// "what tasks are stuck waiting on something?". This is `tsk depend
// --list` under a name people actually type at the prompt.
//
// Why a dedicated command rather than another alias? Cobra alias on
// `depend` would surface this as `tsk depend blocked` in help/man
// output, which buries it. A top-level verb makes it appear in
// `tsk --help` directly and gets tab-completion. The runtime is a
// thin call into runDependList so semantics can't drift between the
// two surfaces.
func newBlockedCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "blocked",
		Aliases: []string{"stuck"},
		Short:   "List every task that's blocked by an unmet dependency",
		Long: `List every task that's currently waiting on another task to be done.

This is the discoverable surface for ` + "`tsk depend --list`" + ` — same
output, easier to remember when you're scanning ` + "`tsk --help`" + ` for
"what should I work on next that's NOT stuck?".

A task is "blocked" when at least one id in its DependsOn list refers
to an open (undone) task in the same store. Dangling dependencies (id
with no task) are treated as satisfied — surface those via 'tsk lint'.

Examples:
  tsk blocked              # human-readable list
  tsk blocked --json       # CI signal: are we deadlocked?
  tsk blocked | wc -l      # how many things are stuck?
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			return runDependList(cmd.OutOrStdout(), s, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}
