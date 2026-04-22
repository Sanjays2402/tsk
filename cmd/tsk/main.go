// Package main is the tsk command-line entry point.
package main

import (
	"fmt"
	"os"

	"github.com/Sanjays2402/tsk/internal/commands"
	"github.com/Sanjays2402/tsk/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	commands.SetVersion(version, commit, date)
	commands.SetTUI(tui.Run)
	if err := commands.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
