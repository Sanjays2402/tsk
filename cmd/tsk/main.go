// Package main is the tsk command-line entry point.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Sanjays2402/tsk/internal/commands"
	"github.com/Sanjays2402/tsk/internal/tui"
)

var (
	version = "0.2.0"
	commit  = "none"
	date    = "unknown"
)

func main() {
	commands.SetVersion(version, commit, date)
	commands.SetTUI(tui.Run)
	if err := commands.NewRoot().Execute(); err != nil {
		var ec commands.ExitCoder
		if errors.As(err, &ec) {
			// SilentExitCoder skips the "error: <msg>" prefix — used by
			// commands like `doctor` that print their own structured output
			// and only need the non-zero exit code.
			var sec commands.SilentExitCoder
			if !errors.As(err, &sec) {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			os.Exit(ec.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
