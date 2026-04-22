package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:    "tui",
		Short:  "Launch the interactive TUI",
		Hidden: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTUI(cmd)
		},
	}
}

// runTUI is replaced by the real TUI wiring in tui.go. It returns a stub error
// if the binary was built without the TUI (not a current configuration).
func runTUI(cmd *cobra.Command) error {
	if tuiEntry != nil {
		return tuiEntry(cmd)
	}
	return fmt.Errorf("TUI not wired")
}

// tuiEntry is wired by the tui package's init (via SetTUI).
var tuiEntry func(cmd *cobra.Command) error

// SetTUI injects the TUI entrypoint. Called from main to avoid an import cycle.
func SetTUI(fn func(cmd *cobra.Command) error) {
	tuiEntry = fn
}
