package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

// newAddCmd creates `tsk add <title>`: add a new task to the active
// store with optional priority, due date, tags, notes, and (since
// this slice) dependencies.
//
// --depends sets the new task's DependsOn at creation time, saving
// the user a follow-up `tsk depend <id> --on …` call. The follow-up
// pattern was tolerable on its own ("add it, then depend it") but
// awkward in scripts: you'd have to capture the new id from `tsk add`'s
// stdout, parse the "added #N" line, and feed it back into a second
// call. With --depends, the dependency relationship lands in one shot.
//
// The deps are validated up-front against the SAME rules
// `tsk depend --on` enforces: every id must exist; self-deps are
// impossible (the new id isn't known yet); direct cycles are caught
// (if B already lists A, `add C --depends B` is fine, but if we'd
// somehow allow back-refs, we'd still refuse a circular shape).
//
// Validation happens BEFORE Add+Save so a bad --depends value doesn't
// land a half-formed task in the store. The store stays untouched
// when the flag fails parsing.
func newAddCmd() *cobra.Command {
	var (
		priorityStr string
		dueStr      string
		tags        []string
		notes       string
		dependsCSV  string
	)
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a new task",
		Long: `Add a new task to the active .tsk.md store.

Examples:
  tsk add "ship release" -p high -t dev
  tsk add "weekly review" -d fri
  tsk add "follow-up email" --depends 3,5
  tsk add "report" --depends 7 -p urgent -t work
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.TrimSpace(strings.Join(args, " "))
			if title == "" {
				return fmt.Errorf("title required")
			}
			prio, err := model.ParsePriority(priorityStr)
			if err != nil {
				return err
			}
			// Parse --depends BEFORE touching the store so a typo
			// can't half-commit the task.
			depIDs, err := parseDependCSV(dependsCSV)
			if err != nil {
				return err
			}
			task := model.Task{
				Title:    title,
				Priority: prio,
				Tags:     tags,
				Notes:    strings.TrimSpace(notes),
				Created:  time.Now(),
			}
			if dueStr != "" {
				loc := PacificLoc()
				t, err := dateparse.Parse(dueStr, time.Now().In(loc), loc)
				if err != nil {
					return usageErrorf("%s", err.Error())
				}
				task.Due = &t
			}
			s, err := resolveStore(cmd, false)
			if err != nil {
				return err
			}
			// Validate every dep exists. We can't use validateProposedDeps
			// directly (it takes the OWN task to check self-deps and
			// reverse-cycles, but the new task doesn't have an id yet),
			// so we replicate the existence check and rely on the
			// post-id-assignment validator below for the cycle check.
			for _, dep := range depIDs {
				if s.ByID(dep) == nil {
					return usageErrorf("no task with id %d in %s", dep, s.Path)
				}
			}
			task.DependsOn = depIDs
			id := s.Add(task)
			// Final cycle/self-dep check against the now-allocated id.
			// (Self-deps are impossible because we just minted the id;
			// the call is defense-in-depth and lets us share the same
			// validator that `tsk depend --on` uses, so semantics can't
			// drift between creation and post-creation.)
			created := s.ByID(id)
			if err := validateProposedDeps(s, created, depIDs); err != nil {
				// Roll back: drop the freshly-added task.
				s.Remove(id)
				return err
			}
			if err := s.Save(); err != nil {
				return err
			}
			if len(depIDs) > 0 {
				pf(cmd.OutOrStdout(), "added #%d: %s (depends on %s)\n", id, title, formatBlockerIDs(depIDs))
			} else {
				pf(cmd.OutOrStdout(), "added #%d: %s\n", id, title)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&priorityStr, "priority", "p", "medium", "priority (low|medium|high|urgent)")
	cmd.Flags().StringVarP(&dueStr, "due", "d", "", "due date (YYYY-MM-DD, or tomorrow/fri/in 3d/jul 4/eow/...)")
	cmd.Flags().StringArrayVarP(&tags, "tag", "t", nil, "tag (repeatable)")
	cmd.Flags().StringVarP(&notes, "notes", "n", "", "freeform notes")
	cmd.Flags().StringVar(&dependsCSV, "depends", "", "comma-separated task ids the new task depends on")
	return cmd
}
