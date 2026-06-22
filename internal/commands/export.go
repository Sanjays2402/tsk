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
	var asJSON, asCSV, asJSONL, asGraphDot bool
	var format string
	var graphReachable int
	var graphOpen bool
	var graphHighlight string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export tasks as JSON, JSONL, CSV, Markdown, or GraphViz DOT",
		Long: `Export tasks in a shareable format.

Formats:
  json       Pretty-printed JSON array of task objects
  jsonl      Streaming JSON-lines (one task per line, no array wrapper) —
             ideal for pipelines: each line is independently parsable so
             jq/mlr/awk can process them without buffering the whole set.
  csv        CSV with header row (id, done, priority, title, due, ...)
  markdown   Human-readable Markdown grouped by section
  graph-dot  GraphViz DOT source of the dependency graph — same shape as
             ` + "`tsk graph --format dot`" + ` but lives under the central
             export verb. Pairs with --reachable <id>, --open, and
             --highlight <id> to scope and decorate the graph; pipe into
             ` + "`dot -Tpng > deps.png`" + ` for a real visual.

Use --format to pick a format explicitly, or the legacy --json / --jsonl /
--csv / --graph-dot shortcuts.

JSONL pipeline example:
  tsk export --jsonl | jq -c 'select(.Done == false) | {ID, Title}'

Graph examples:
  tsk export --graph-dot | dot -Tpng > deps.png
  tsk export --graph-dot --open                       # only currently-blocking edges
  tsk export --graph-dot --reachable 7                # subgraph rooted at #7
  tsk export --graph-dot --highlight 7                # draw the eye to #7
  tsk export --graph-dot --reachable 7 --highlight 7  # subgraph + spotlight
  tsk export --graph-dot --reachable 7 --open | dot -Tsvg > sub.svg
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			chosen, err := resolveExportFormat(format, asJSON, asCSV, asJSONL, asGraphDot)
			if err != nil {
				return err
			}
			// Graph-shaping flags are only meaningful for graph-dot.
			// Surface the misuse loudly rather than silently ignoring.
			if (graphReachable > 0 || graphOpen) && chosen != "graph-dot" {
				return fmt.Errorf("--reachable / --open only apply to --graph-dot (got format %q)", chosen)
			}
			if graphHighlight != "" && chosen != "graph-dot" {
				return fmt.Errorf("--highlight only applies to --graph-dot (got format %q)", chosen)
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch chosen {
			case "json":
				return exportJSON(out, s.Tasks)
			case "jsonl":
				return exportJSONL(out, s.Tasks)
			case "csv":
				return exportCSV(out, s.Tasks)
			case "markdown":
				return exportMarkdown(out, s.Tasks)
			case "graph-dot":
				return exportGraphDOT(out, s, graphOpen, graphReachable, graphHighlight)
			}
			return fmt.Errorf("unreachable: unknown format %q", chosen)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "", "output format: json, jsonl, csv, markdown, or graph-dot")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().BoolVar(&asJSONL, "jsonl", false, "emit JSON-lines (shortcut for --format=jsonl)")
	cmd.Flags().BoolVar(&asCSV, "csv", false, "emit CSV (shortcut for --format=csv)")
	cmd.Flags().BoolVar(&asGraphDot, "graph-dot", false, "emit GraphViz DOT of the dependency graph (shortcut for --format=graph-dot)")
	cmd.Flags().IntVar(&graphReachable, "reachable", 0, "for --graph-dot: restrict to the subgraph reachable from this task id")
	cmd.Flags().BoolVar(&graphOpen, "open", false, "for --graph-dot: only include open tasks and the open deps that block them")
	cmd.Flags().StringVar(&graphHighlight, "highlight", "", "for --graph-dot: comma-separated task ids to draw with a distinct fill+border")
	return cmd
}

// resolveExportFormat arbitrates between --format and the legacy boolean
// shortcuts. Supplying more than one wins with a useful error rather than a
// silent priority rule.
func resolveExportFormat(format string, asJSON, asCSV, asJSONL, asGraphDot bool) (string, error) {
	chosen := ""
	count := 0
	if asJSON {
		chosen = "json"
		count++
	}
	if asJSONL {
		chosen = "jsonl"
		count++
	}
	if asCSV {
		chosen = "csv"
		count++
	}
	if asGraphDot {
		chosen = "graph-dot"
		count++
	}
	if format != "" {
		chosen = strings.ToLower(strings.TrimSpace(format))
		count++
	}
	if count == 0 {
		return "", fmt.Errorf("specify --format=<json|jsonl|csv|markdown|graph-dot> (or --json / --jsonl / --csv / --graph-dot)")
	}
	if count > 1 {
		return "", fmt.Errorf("specify exactly one of --format, --json, --jsonl, --csv, --graph-dot")
	}
	switch chosen {
	case "json", "csv", "markdown", "md":
		if chosen == "md" {
			chosen = "markdown"
		}
		return chosen, nil
	case "jsonl", "ndjson":
		// ndjson is the same wire format under a different name —
		// accept it as an alias because half the world calls it that.
		return "jsonl", nil
	case "graph-dot", "graphdot", "dot":
		// "dot" is the short alias users coming from `tsk graph
		// --format dot` will reach for. graphdot is the no-dash
		// form some shells/scripts prefer.
		return "graph-dot", nil
	}
	return "", fmt.Errorf("unknown --format %q: expected json, jsonl, csv, markdown, or graph-dot", chosen)
}

func exportJSON(w io.Writer, tasks []model.Task) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(tasks)
}

// exportJSONL emits one task per line, no array wrapper, no indent.
// Each line is independently parseable so a downstream `jq` / `mlr` /
// `awk` pipeline can stream through them without buffering the entire
// result set — useful when piping into a long-running stage.
//
// The per-line schema is identical to one element of exportJSON's
// array, so a consumer can switch between the two formats without
// changing field accessors. The contract: every line is exactly one
// valid JSON object terminated by '\n'; on zero tasks the output is
// empty (NOT "[]" and NOT a single empty line — that's the jsonlines
// convention).
func exportJSONL(w io.Writer, tasks []model.Task) error {
	enc := json.NewEncoder(w)
	// NewEncoder writes '\n' after each Encode by default — perfect
	// for jsonlines. Do NOT call SetIndent here: indented JSONL is a
	// contradiction (lines would have embedded newlines).
	for i := range tasks {
		// Encode one element at a time so the writer can flush between
		// records — streaming-friendly.
		if err := enc.Encode(tasks[i]); err != nil {
			return err
		}
	}
	return nil
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
