package commands

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
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

// recurSpawn captures an instance that was just completed and needs a
// follow-up task created with a new due date.
type recurSpawn struct {
	completedID int
	newID       int
	due         time.Time
}

func runToggle(done bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		s, err := resolveStore(cmd, true)
		if err != nil {
			return err
		}
		var spawns []recurSpawn
		for _, arg := range args {
			id, err := strconv.Atoi(arg)
			if err != nil {
				return fmt.Errorf("invalid id %q", arg)
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d", id)
			}
			if !s.SetDone(id, done) {
				return fmt.Errorf("no task with id %d", id)
			}
			if !done || t.Recur == nil {
				continue
			}
			loc := PacificLoc()
			var anchor time.Time
			if t.Due != nil {
				// Anchor on the Due calendar date in loc, regardless of how the
				// stored time was parsed (UTC midnight vs loc midnight).
				y, m, d := t.Due.Date()
				anchor = time.Date(y, m, d, 0, 0, 0, 0, loc)
			} else {
				anchor = time.Now().In(loc)
			}
			nextDue := t.Recur.Next(anchor)
			recurCopy := *t.Recur
			next := model.Task{
				Title:    t.Title,
				Priority: t.Priority,
				Due:      &nextDue,
				Tags:     append([]string(nil), t.Tags...),
				Notes:    t.Notes,
				Recur:    &recurCopy,
				Created:  time.Now(),
			}
			newID := s.Add(next)
			spawns = append(spawns, recurSpawn{completedID: id, newID: newID, due: nextDue})
		}
		if err := s.Save(); err != nil {
			return err
		}
		verb := "done"
		if !done {
			verb = "undone"
		}
		pf(cmd.OutOrStdout(), "marked %d task(s) %s\n", len(args), verb)
		for _, sp := range spawns {
			pf(cmd.OutOrStdout(), "→ recurring: created #%d due:%s\n", sp.newID, sp.due.Format(model.DateLayout))
		}
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
			pf(cmd.OutOrStdout(), "removed %d task(s)\n", len(args))
			return nil
		},
	}
}
