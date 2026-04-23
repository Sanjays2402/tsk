package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var asJSON, asCSV bool
	var format string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export tasks as JSON, CSV, or Markdown",
		Long: `Export tasks in a shareable format.

Formats:
  json      Pretty-printed JSON array of task objects
  csv       CSV with header row (id, done, priority, title, due, ...)
  markdown  Human-readable Markdown grouped by section

Use --format to pick a format explicitly, or the legacy --json / --csv flags.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			chosen, err := resolveExportFormat(format, asJSON, asCSV)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch chosen {
			case "json":
				return exportJSON(out, s.Tasks)
			case "csv":
				return exportCSV(out, s.Tasks)
			case "markdown":
				return exportMarkdown(out, s.Tasks)
			}
			return fmt.Errorf("unreachable: unknown format %q", chosen)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "", "output format: json, csv, or markdown")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().BoolVar(&asCSV, "csv", false, "emit CSV (shortcut for --format=csv)")
	return cmd
}

// resolveExportFormat arbitrates between --format and the legacy boolean
// shortcuts. Supplying more than one wins with a useful error rather than a
// silent priority rule.
func resolveExportFormat(format string, asJSON, asCSV bool) (string, error) {
	chosen := ""
	count := 0
	if asJSON {
		chosen = "json"
		count++
	}
	if asCSV {
		chosen = "csv"
		count++
	}
	if format != "" {
		chosen = strings.ToLower(strings.TrimSpace(format))
		count++
	}
	if count == 0 {
		return "", fmt.Errorf("specify --format=<json|csv|markdown> (or --json / --csv)")
	}
	if count > 1 {
		return "", fmt.Errorf("specify exactly one of --format, --json, --csv")
	}
	switch chosen {
	case "json", "csv", "markdown", "md":
		if chosen == "md" {
			chosen = "markdown"
		}
		return chosen, nil
	}
	return "", fmt.Errorf("unknown --format %q: expected json, csv, or markdown", chosen)
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

// exportMarkdown emits a clean, shareable view: grouped sections, priority
// emoji, tags inline, notes indented. Intentionally NOT round-trippable —
// use the raw .tsk.md file for that. This is for pasting into a PR, a wiki,
// or a status update.
func exportMarkdown(w io.Writer, tasks []model.Task) error {
	// Group into: Undone (with priority sort) / Done.
	var undone, done []model.Task
	for _, t := range tasks {
		if t.Done {
			done = append(done, t)
		} else {
			undone = append(undone, t)
		}
	}
	sort.SliceStable(undone, func(i, j int) bool {
		if undone[i].Priority != undone[j].Priority {
			return undone[i].Priority > undone[j].Priority
		}
		return undone[i].ID < undone[j].ID
	})
	sort.SliceStable(done, func(i, j int) bool { return done[i].ID < done[j].ID })

	bf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(w, format, args...)
		return err
	}

	if err := bf("# Tasks\n\n"); err != nil {
		return err
	}
	if len(undone) > 0 {
		if err := bf("## Todo\n\n"); err != nil {
			return err
		}
		for _, t := range undone {
			if err := writeMarkdownTask(w, t); err != nil {
				return err
			}
		}
		if err := bf("\n"); err != nil {
			return err
		}
	}
	if len(done) > 0 {
		if err := bf("## Done\n\n"); err != nil {
			return err
		}
		for _, t := range done {
			if err := writeMarkdownTask(w, t); err != nil {
				return err
			}
		}
	}
	return nil
}

// priorityGlyph returns a terse inline marker for each priority level. Kept
// ASCII-only to stay greppable and to avoid emoji rendering lottery on older
// wikis/PRs.
func priorityGlyph(p model.Priority) string {
	switch p {
	case model.PriorityUrgent:
		return "[!]"
	case model.PriorityHigh:
		return "[H]"
	case model.PriorityMedium:
		return "[M]"
	case model.PriorityLow:
		return "[L]"
	}
	return ""
}

func writeMarkdownTask(w io.Writer, t model.Task) error {
	box := "[ ]"
	if t.Done {
		box = "[x]"
	}
	line := fmt.Sprintf("- %s %s %s", box, priorityGlyph(t.Priority), t.Title)
	if t.Due != nil {
		line += " (due " + t.Due.Format(model.DateLayout) + ")"
	}
	if len(t.Tags) > 0 {
		line += " #" + strings.Join(t.Tags, " #")
	}
	if _, err := fmt.Fprintln(w, line); err != nil {
		return err
	}
	if strings.TrimSpace(t.Notes) != "" {
		for _, nl := range strings.Split(strings.TrimRight(t.Notes, "\n"), "\n") {
			if _, err := fmt.Fprintf(w, "  > %s\n", nl); err != nil {
				return err
			}
		}
	}
	return nil
}
