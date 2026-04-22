package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>...",
		Short: "Mark one or more tasks as done",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runToggle(true),
	}
}

func newUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo <id>...",
		Short: "Mark one or more done tasks as undone",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runToggle(false),
	}
}

func runToggle(done bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		s, err := resolveStore(cmd, true)
		if err != nil {
			return err
		}
		for _, arg := range args {
			id, err := strconv.Atoi(arg)
			if err != nil {
				return fmt.Errorf("invalid id %q", arg)
			}
			if !s.SetDone(id, done) {
				return fmt.Errorf("no task with id %d", id)
			}
		}
		if err := s.Save(); err != nil {
			return err
		}
		verb := "done"
		if !done {
			verb = "undone"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "marked %d task(s) %s\n", len(args), verb)
		return nil
	}
}

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <id>...",
		Aliases: []string{"remove", "delete"},
		Short:   "Remove one or more tasks",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			for _, arg := range args {
				id, err := strconv.Atoi(arg)
				if err != nil {
					return fmt.Errorf("invalid id %q", arg)
				}
				if !s.Remove(id) {
					return fmt.Errorf("no task with id %d", id)
				}
			}
			if err := s.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d task(s)\n", len(args))
			return nil
		},
	}
}
