package commands

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	"github.com/spf13/cobra"
)

type purgeOpts struct {
	done      bool
	ids       []string
	olderThan string
	dryRun    bool
}

func newPurgeCmd() *cobra.Command {
	o := &purgeOpts{}
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Hard-delete tasks from the active file",
		Long: "Permanently delete tasks. Requires an explicit selection — at least one\n" +
			"of --done or --id must be set. --older-than further restricts --done to\n" +
			"tasks completed more than the given duration ago.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runPurge(cmd, o) },
	}
	cmd.Flags().BoolVar(&o.done, "done", false, "delete all Done tasks")
	cmd.Flags().StringArrayVar(&o.ids, "id", nil, "delete the task with this id (repeatable)")
	cmd.Flags().StringVar(&o.olderThan, "older-than", "", "with --done, only delete tasks completed more than this ago")
	cmd.Flags().BoolVar(&o.dryRun, "dry-run", false, "print what would be deleted without changing files")
	return cmd
}

func runPurge(cmd *cobra.Command, o *purgeOpts) error {
	if !o.done && len(o.ids) == 0 {
		return usageErrorf("purge requires --done or --id; refusing to delete everything")
	}
	idSet, err := parsePurgeIDs(o.ids)
	if err != nil {
		return err
	}
	cutoff, useCutoff, err := resolvePurgeCutoff(o.olderThan)
	if err != nil {
		return err
	}
	s, err := resolveStore(cmd, true)
	if err != nil {
		return err
	}
	pred := purgePredicate(o.done, idSet, useCutoff, cutoff)
	kept, removed := s.Partition(pred)
	if err := verifyPurgeIDsMatched(idSet, removed); err != nil {
		return err
	}
	return applyPurge(cmd, s, kept, removed, o.dryRun)
}

func parsePurgeIDs(ids []string) (map[int]struct{}, error) {
	idSet := make(map[int]struct{}, len(ids))
	for _, raw := range ids {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, usageErrorf("invalid id %q", raw)
		}
		idSet[n] = struct{}{}
	}
	return idSet, nil
}

func resolvePurgeCutoff(olderThan string) (time.Time, bool, error) {
	if olderThan == "" {
		return time.Time{}, false, nil
	}
	d, err := store.ParseDuration(olderThan)
	if err != nil {
		return time.Time{}, false, usageErrorf("%s", err.Error())
	}
	return time.Now().Add(-d), true, nil
}

func purgePredicate(done bool, idSet map[int]struct{}, useCutoff bool, cutoff time.Time) func(model.Task) bool {
	return func(t model.Task) bool {
		if _, ok := idSet[t.ID]; ok {
			return true
		}
		if !done || !t.Done {
			return false
		}
		if !useCutoff {
			return true
		}
		if t.Completed == nil {
			return false
		}
		return t.Completed.Before(cutoff)
	}
}

func verifyPurgeIDsMatched(idSet map[int]struct{}, removed []model.Task) error {
	matched := make(map[int]bool, len(removed))
	for _, t := range removed {
		matched[t.ID] = true
	}
	for id := range idSet {
		if !matched[id] {
			return fmt.Errorf("no task with id %d", id)
		}
	}
	return nil
}

func applyPurge(cmd *cobra.Command, s *store.Store, kept, removed []model.Task, dryRun bool) error {
	out := cmd.OutOrStdout()
	if len(removed) == 0 {
		pf(out, "no tasks to purge\n")
		return nil
	}
	for _, t := range removed {
		status := "todo"
		if t.Done {
			status = "done"
		}
		prefix := "delete "
		if dryRun {
			prefix = "would delete "
		}
		pf(out, "%s#%d %s (%s, %s)\n", prefix, t.ID, t.Title, t.Priority, status)
	}
	if dryRun {
		pf(out, "would purge %d task(s) from %s\n", len(removed), s.Path)
		return nil
	}
	s.ReplaceTasks(kept)
	if err := s.Save(); err != nil {
		return err
	}
	pf(out, "purged %d task(s) from %s\n", len(removed), s.Path)
	return nil
}
