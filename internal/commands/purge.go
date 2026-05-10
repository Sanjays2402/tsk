package commands

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	"github.com/spf13/cobra"
)

func newPurgeCmd() *cobra.Command {
	var (
		done      bool
		ids       []string
		olderThan string
		dryRun    bool
	)
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Hard-delete tasks from the active file",
		Long: "Permanently delete tasks. Requires an explicit selection — at least one\n" +
			"of --done or --id must be set. --older-than further restricts --done to\n" +
			"tasks completed more than the given duration ago.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !done && len(ids) == 0 {
				return usageErrorf("purge requires --done or --id; refusing to delete everything")
			}

			idSet := make(map[int]struct{}, len(ids))
			for _, raw := range ids {
				n, err := strconv.Atoi(raw)
				if err != nil {
					return usageErrorf("invalid id %q", raw)
				}
				idSet[n] = struct{}{}
			}

			var cutoff time.Time
			useCutoff := false
			if olderThan != "" {
				d, err := store.ParseDuration(olderThan)
				if err != nil {
					return usageErrorf("%s", err.Error())
				}
				cutoff = time.Now().Add(-d)
				useCutoff = true
			}

			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}

			pred := func(t model.Task) bool {
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
			kept, removed := s.Partition(pred)

			out := cmd.OutOrStdout()

			// Verify every explicit --id matched something so users notice typos.
			matched := make(map[int]bool, len(removed))
			for _, t := range removed {
				matched[t.ID] = true
			}
			for id := range idSet {
				if !matched[id] {
					return fmt.Errorf("no task with id %d", id)
				}
			}

			if len(removed) == 0 {
				pf(out, "no tasks to purge\n")
				return nil
			}

			for _, t := range removed {
				status := "todo"
				if t.Done {
					status = "done"
				}
				prefix := ""
				if dryRun {
					prefix = "would delete "
				} else {
					prefix = "delete "
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
		},
	}
	cmd.Flags().BoolVar(&done, "done", false, "delete all Done tasks")
	cmd.Flags().StringArrayVar(&ids, "id", nil, "delete the task with this id (repeatable)")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "with --done, only delete tasks completed more than this ago")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be deleted without changing files")
	return cmd
}
