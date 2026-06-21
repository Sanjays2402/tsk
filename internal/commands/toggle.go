package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
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
		// Parse all ids up-front so we can dependency-check before
		// touching anything. parseTaskIDs handles "#3"/"3", dedup, and
		// the empty-arg case.
		ids := make([]int, 0, len(args))
		for _, arg := range args {
			id, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
			if err != nil {
				return fmt.Errorf("invalid id %q", arg)
			}
			ids = append(ids, id)
		}
		if done {
			// Pre-flight every id: if any has an unmet dependency, refuse
			// the whole batch (no partial done state). This mirrors the
			// pin/freeze "validate first" pattern so a failed batch
			// leaves the file untouched.
			for _, id := range ids {
				t := s.ByID(id)
				if t == nil {
					return fmt.Errorf("no task with id %d", id)
				}
				if blockers := unmetBlockers(s, t, ids); len(blockers) > 0 {
					return usageErrorf(
						"#%d is blocked by %s — finish those first or use `tsk depend %d --clear`",
						id, formatBlockerIDs(blockers), id,
					)
				}
			}
		}
		for _, id := range ids {
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
		pf(cmd.OutOrStdout(), "marked %d task(s) %s\n", len(args), verb)
		return nil
	}
}

// unmetBlockers returns the subset of t.DependsOn that still refers to
// open tasks in the store. `batchIDs` is the set of ids being marked
// done in the SAME call — a dependency that's about to be closed in
// the same batch is considered satisfied (so `tsk done 1 2` works when
// task 2 depends on task 1, without forcing the user to order args).
func unmetBlockers(s *store.Store, t *model.Task, batchIDs []int) []int {
	if !t.HasDependencies() {
		return nil
	}
	batch := make(map[int]bool, len(batchIDs))
	for _, id := range batchIDs {
		batch[id] = true
	}
	out := make([]int, 0)
	for _, dep := range t.DependsOn {
		if batch[dep] {
			// Will be closed in this same batch — count as satisfied.
			continue
		}
		bt := s.ByID(dep)
		if bt == nil {
			// Dangling dependency. Treat as satisfied (no task to
			// wait on) — surfacing it is `tsk lint`'s job.
			continue
		}
		if !bt.Done {
			out = append(out, dep)
		}
	}
	return out
}

// formatBlockerIDs renders a small id list as "#1, #5, #7" for error
// messages. Kept local to commands so the model package stays import-
// free of any rendering choices.
func formatBlockerIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("#%d", id)
	}
	return strings.Join(parts, ", ")
}

// filterBlockedTasks drops any task that has at least one unmet
// prerequisite. Used by --respect-deps in next/top/ls so users
// planning the next batch of work don't see tasks they can't
// actually close.
//
// Operates on the value-typed []model.Task slice that the view
// commands already work with — no shared mutable state, the filter
// returns a fresh slice. Dangling deps are tolerated (matching
// unmetBlockers' policy — a missing id is treated as satisfied).
func filterBlockedTasks(s *store.Store, in []model.Task) []model.Task {
	out := make([]model.Task, 0, len(in))
	for _, t := range in {
		if !t.HasDependencies() {
			out = append(out, t)
			continue
		}
		// unmetBlockers takes *model.Task and uses store.ByID, so we
		// can pass the loop value's address — no aliasing issue
		// because we're done with t before the next iteration.
		t := t
		if len(unmetBlockers(s, &t, nil)) == 0 {
			out = append(out, t)
		}
	}
	return out
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
