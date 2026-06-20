package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newShowCmd implements `tsk show <id>`: a single-task detail view that
// expands everything tsk knows about one task — title, status, priority,
// due date, tags, full notes, and exact ISO timestamps.
//
// The `tsk ls` output is intentionally terse for scrollability; `tsk show`
// is the "open it" companion for when you want every field. With `--json`
// it emits the same task object `tsk export --json` does, but for a
// single id, so scripts can pluck one field without piping through jq.
func newShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show full detail for a single task",
		Long: `Show every field tsk knows about one task.

Plain output formats the task across multiple lines with full ISO timestamps,
labelled fields, and the complete notes block. Pass --json for the same task
object as 'tsk export --json' but scoped to one id.

Examples:
  tsk show 3
  tsk show 3 --json
  tsk show 3 --json | jq -r '.Notes'
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(t)
			}
			printTaskDetail(cmd.OutOrStdout(), t)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (single task object)")
	return cmd
}

// printTaskDetail renders one task across labelled lines. Stable column
// width keeps the output scannable in a terminal.
func printTaskDetail(w io.Writer, t *model.Task) {
	status := "open"
	if t.Done {
		status = "done"
	}
	pf(w, "id:        %d\n", t.ID)
	pf(w, "title:     %s\n", t.Title)
	pf(w, "status:    %s\n", status)
	pf(w, "priority:  %s\n", t.Priority)
	if t.Pinned {
		pf(w, "pinned:    yes\n")
	}
	if t.Due != nil {
		pf(w, "due:       %s\n", t.Due.Format(model.DateLayout))
	} else {
		pf(w, "due:       -\n")
	}
	if t.WaitUntil != nil {
		pf(w, "wait:      %s\n", t.WaitUntil.Format(model.DateLayout))
	}
	if len(t.Tags) > 0 {
		pf(w, "tags:      #%s\n", strings.Join(t.Tags, " #"))
	} else {
		pf(w, "tags:      -\n")
	}
	if !t.Created.IsZero() {
		pf(w, "created:   %s\n", t.Created.Format("2006-01-02 15:04:05 -0700"))
	}
	if t.Completed != nil {
		pf(w, "completed: %s\n", t.Completed.Format("2006-01-02 15:04:05 -0700"))
	}
	if strings.TrimSpace(t.Notes) != "" {
		pln(w)
		pln(w, "notes:")
		for _, line := range strings.Split(t.Notes, "\n") {
			pf(w, "  %s\n", line)
		}
	}
}

// parseSingleID converts a positional "<id>" arg into an int.
// Accepts "#3" or "3"; rejects 0, negatives, and non-numeric input with a
// usage-coded error so main.go exits 2 (not 1) on user-input mistakes.
func parseSingleID(arg string) (int, error) {
	ids, err := parseTaskIDs([]string{arg})
	if err != nil {
		return 0, err
	}
	if len(ids) != 1 {
		return 0, usageErrorf("expected exactly one task id, got %d", len(ids))
	}
	return ids[0], nil
}
