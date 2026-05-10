package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	"github.com/spf13/cobra"
)

// archiveFileName is the sibling file that receives archived tasks. It lives
// in the same directory as the active .tsk.md so it travels with the project.
const archiveFileName = ".tsk.archive.md"

// archiveHeader is the one-line header written when the archive file is
// created for the first time.
const archiveHeader = "# tsk archive\n"

func newArchiveCmd() *cobra.Command {
	var (
		olderThan string
		all       bool
		dryRun    bool
	)
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Move completed tasks to a sibling .tsk.archive.md file",
		Long: "Move Done tasks out of the active .tsk.md and into a sibling .tsk.archive.md.\n" +
			"Archived tasks get fresh sequential IDs in the archive file, continuing\n" +
			"from the archive's existing max ID. Active task IDs do not change.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}

			cutoff, useCutoff, err := resolveCutoff(olderThan, all)
			if err != nil {
				return err
			}

			pred := func(t model.Task) bool {
				if !t.Done {
					return false
				}
				if !useCutoff {
					return true // --all
				}
				// No completion timestamp on a done task → can't prove age,
				// be conservative and skip.
				if t.Completed == nil {
					return false
				}
				return t.Completed.Before(cutoff)
			}
			kept, archived := s.Partition(pred)

			out := cmd.OutOrStdout()
			archivePath := filepath.Join(filepath.Dir(s.Path), archiveFileName)

			if len(archived) == 0 {
				pf(out, "no tasks to archive\n")
				return nil
			}

			if dryRun {
				pf(out, "would archive %d task(s) → %s\n", len(archived), archivePath)
				for _, t := range archived {
					pf(out, "  #%d %s\n", t.ID, t.Title)
				}
				return nil
			}

			// Load (or initialize) the archive file and continue IDs from
			// its current max.
			arch, err := store.Load(archivePath)
			if err != nil {
				return fmt.Errorf("load archive: %w", err)
			}
			if _, statErr := os.Stat(archivePath); os.IsNotExist(statErr) {
				arch.Header = archiveHeader
			}
			next := maxTaskID(arch.Tasks) + 1
			for i := range archived {
				archived[i].ID = next
				next++
			}
			arch.ReplaceTasks(append(arch.Tasks, archived...))
			if err := arch.Save(); err != nil {
				return fmt.Errorf("save archive: %w", err)
			}

			s.ReplaceTasks(kept)
			if err := s.Save(); err != nil {
				return fmt.Errorf("save active: %w", err)
			}

			pf(out, "archived %d task(s) → %s\n", len(archived), archivePath)
			pf(out, "active tasks: %d\n", len(kept))
			return nil
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", "30d", "only archive tasks completed more than this ago (e.g. 7d, 2w, 1m)")
	cmd.Flags().BoolVar(&all, "all", false, "archive every Done task regardless of age")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be archived without changing files")
	return cmd
}

// resolveCutoff returns the time before which a task is "old enough" to
// archive, plus whether the cutoff should be applied at all (--all bypasses
// it). With --all set, useCutoff is false. Otherwise the duration string is
// parsed and subtracted from time.Now().
func resolveCutoff(olderThan string, all bool) (cutoff time.Time, useCutoff bool, err error) {
	if all {
		return time.Time{}, false, nil
	}
	d, err := store.ParseDuration(olderThan)
	if err != nil {
		return time.Time{}, false, usageErrorf("%s", err.Error())
	}
	return time.Now().Add(-d), true, nil
}

func maxTaskID(tasks []model.Task) int {
	m := 0
	for _, t := range tasks {
		if t.ID > m {
			m = t.ID
		}
	}
	return m
}
