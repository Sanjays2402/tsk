package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

func newSetRecurCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-recur <id> <rule|none>",
		Short: "Set or clear a task's recurrence rule",
		Long: "Set the recurrence rule on a task (daily, weekly, monthly, yearly, weekdays, " +
			"every:Nd|Nw|Nm|Ny). Pass 'none' to clear the rule.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErrorf("invalid id %q", args[0])
			}
			rule := strings.TrimSpace(args[1])
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d", id)
			}
			if strings.EqualFold(rule, "none") || rule == "" {
				t.Recur = nil
				if err := s.Save(); err != nil {
					return err
				}
				pf(cmd.OutOrStdout(), "cleared recurrence on #%d\n", id)
				return nil
			}
			rc, err := model.ParseRecurrence(rule)
			if err != nil {
				return usageErrorf("%s", err.Error())
			}
			t.Recur = &rc
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "set recurrence on #%d: %s\n", id, rc.String())
			return nil
		},
	}
}
