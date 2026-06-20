package commands

import (
	"fmt"
	"time"

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
			now := time.Now()
			var best *model.Task
			for i := range s.Tasks {
				t := &s.Tasks[i]
				if t.Done {
					continue
				}
				if t.IsWaiting(now) {
					continue
				}
				if best == nil {
					best = t
					continue
				}
				// Pinned beats everything else, including higher priority.
				if t.Pinned != best.Pinned {
					if t.Pinned {
						best = t
					}
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
				pln(cmd.OutOrStdout(), "all caught up")
				return nil
			}
			pinMark := ""
			if best.Pinned {
				pinMark = "* "
			}
			line := fmt.Sprintf("%s#%d [%s] %s", pinMark, best.ID, best.Priority, best.Title)
			if best.Due != nil {
				line += "  due:" + best.Due.Format(model.DateLayout)
			}
			pln(cmd.OutOrStdout(), line)
			return nil
		},
	}
}
