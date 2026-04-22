package commands

import (
	"fmt"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

func newNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Show the highest-priority undone task",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			var best *model.Task
			for i := range s.Tasks {
				t := &s.Tasks[i]
				if t.Done {
					continue
				}
				if best == nil {
					best = t
					continue
				}
				if t.Priority > best.Priority {
					best = t
					continue
				}
				if t.Priority == best.Priority && t.Due != nil && (best.Due == nil || t.Due.Before(*best.Due)) {
					best = t
				}
			}
			if best == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "all caught up")
				return nil
			}
			line := fmt.Sprintf("#%d [%s] %s", best.ID, best.Priority, best.Title)
			if best.Due != nil {
				line += "  due:" + best.Due.Format(model.DateLayout)
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
			return nil
		},
	}
}
