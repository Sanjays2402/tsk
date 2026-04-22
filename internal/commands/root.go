// Package commands wires up cobra commands for the tsk CLI.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version metadata plumbed from main via SetVersion.
var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

// SetVersion injects build metadata from main.
func SetVersion(version, commit, date string) {
	buildVersion = version
	buildCommit = commit
	buildDate = date
}

// NewRoot returns the root cobra command. All subcommands attach here.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "tsk",
		Short:         "Fast, keyboard-first markdown todo manager",
		Long:          "tsk is a souped-up TUI + CLI todo manager backed by a human-readable markdown file (.tsk.md).",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("file", "", "path to .tsk.md (default: nearest .tsk.md or ~/.tsk/global.md)")

	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newLsCmd(),
		newDoneCmd(),
		newUndoCmd(),
		newRmCmd(),
		newEditCmd(),
		newNextCmd(),
		newStatsCmd(),
		newExportCmd(),
		newCompletionCmd(),
		newVersionCmd(),
		newTUICmd(),
	)
	attachOptional(root)

	// Run the TUI when invoked with no subcommand and no args.
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runTUI(cmd)
		}
		return fmt.Errorf("unknown command %q", args[0])
	}
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pf(cmd.OutOrStdout(), "tsk %s (commit %s, built %s)\n", buildVersion, buildCommit, buildDate)
			return nil
		},
	}
}
