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
		fmt.Fprintln(os.Stderr, "error:", err)
		var ec commands.ExitCoder
		if errors.As(err, &ec) {
			os.Exit(ec.ExitCode())
		}
		os.Exit(1)
	}
}
