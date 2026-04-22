package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var (
		priorityStr string
		dueStr      string
		tags        []string
		notes       string
	)
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a new task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.TrimSpace(strings.Join(args, " "))
			if title == "" {
				return fmt.Errorf("title required")
			}
			prio, err := model.ParsePriority(priorityStr)
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
			id := s.Add(task)
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "added #%d: %s\n", id, title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&priorityStr, "priority", "p", "medium", "priority (low|medium|high|urgent)")
	cmd.Flags().StringVarP(&dueStr, "due", "d", "", "due date (YYYY-MM-DD, or tomorrow/fri/in 3d/jul 4/eow/...)")
	cmd.Flags().StringArrayVarP(&tags, "tag", "t", nil, "tag (repeatable)")
	cmd.Flags().StringVarP(&notes, "notes", "n", "", "freeform notes")
	return cmd
}
