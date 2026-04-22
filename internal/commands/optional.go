package commands

import "github.com/spf13/cobra"

// optionalFactories holds subcommand constructors wired by later commits.
var optionalFactories []func() *cobra.Command

// RegisterCommand allows later-stage packages (or test code) to add subcommands
// without modifying root construction.
func RegisterCommand(f func() *cobra.Command) {
	optionalFactories = append(optionalFactories, f)
}

func attachOptional(root *cobra.Command) {
	for _, f := range optionalFactories {
		root.AddCommand(f())
	}
}
