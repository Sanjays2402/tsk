// Package main is the tsk command-line entry point.
package main

import "fmt"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	fmt.Printf("tsk %s (%s) %s\n", version, commit, date)
}
