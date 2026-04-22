package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var asJSON, asCSV bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export tasks as JSON or CSV",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if asJSON == asCSV {
				return fmt.Errorf("specify exactly one of --json or --csv")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				return exportJSON(out, s.Tasks)
			}
			return exportCSV(out, s.Tasks)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().BoolVar(&asCSV, "csv", false, "emit CSV")
	return cmd
}

func exportJSON(w io.Writer, tasks []model.Task) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(tasks)
}

func exportCSV(w io.Writer, tasks []model.Task) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"id", "done", "priority", "title", "due", "tags", "created", "completed", "notes"}); err != nil {
		return err
	}
	for _, t := range tasks {
		due, completed := "", ""
		if t.Due != nil {
			due = t.Due.Format(model.DateLayout)
		}
		if t.Completed != nil {
			completed = t.Completed.Format("2006-01-02T15:04:05Z07:00")
		}
		if err := cw.Write([]string{
			fmt.Sprintf("%d", t.ID),
			fmt.Sprintf("%t", t.Done),
			t.Priority.String(),
			t.Title,
			due,
			strings.Join(t.Tags, ","),
			t.Created.Format("2006-01-02T15:04:05Z07:00"),
			completed,
			t.Notes,
		}); err != nil {
			return err
		}
	}
	return cw.Error()
}
