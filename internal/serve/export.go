package serve

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/Sanjays2402/tsk/internal/model"
)

// handleExport streams the task list in a shareable format. It mirrors the CLI
// `tsk export` formats (json, csv, markdown) so the web "Export" buttons hit
// the exact same shapes a user would get from the terminal. The format is
// chosen via ?format=json|csv|markdown (aliases: md). Read-only; no store
// mutation.
//
// A Content-Disposition header names the download (tasks.json/.csv/.md) so the
// browser saves a sensibly-named file when the client triggers a download.
//
// The exporters are reimplemented here rather than imported from
// internal/commands because commands imports serve (for `tsk serve`), so the
// reverse import would be a cycle. The output schemas are kept byte-compatible
// with the CLI by sharing the same model types and field ordering.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format == "md" {
		format = "markdown"
	}
	st, err := s.loadStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="tasks.json"`)
		w.Header().Set("Cache-Control", "no-store")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(st.Tasks)
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="tasks.csv"`)
		w.Header().Set("Cache-Control", "no-store")
		_ = exportCSVTo(w, st.Tasks)
	case "markdown":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="tasks.md"`)
		w.Header().Set("Cache-Control", "no-store")
		_ = exportMarkdownTo(w, st.Tasks)
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown export format %q: expected json, csv, or markdown", format))
	}
}

// exportCSVTo writes the CSV form, header + one row per task. Field order
// matches the CLI's `tsk export --csv`.
func exportCSVTo(w http.ResponseWriter, tasks []model.Task) error {
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

// exportMarkdownTo writes a clean, shareable Markdown view grouped into Todo /
// Done with priority glyphs, mirroring the CLI's `tsk export --markdown`.
func exportMarkdownTo(w http.ResponseWriter, tasks []model.Task) error {
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
			if err := writeMarkdownTaskTo(w, t); err != nil {
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
			if err := writeMarkdownTaskTo(w, t); err != nil {
				return err
			}
		}
	}
	return nil
}

// priorityGlyphFor returns a terse ASCII marker for a priority level, matching
// the CLI's exportMarkdown glyphs.
func priorityGlyphFor(p model.Priority) string {
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

func writeMarkdownTaskTo(w http.ResponseWriter, t model.Task) error {
	box := "[ ]"
	if t.Done {
		box = "[x]"
	}
	line := fmt.Sprintf("- %s %s %s", box, priorityGlyphFor(t.Priority), t.Title)
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
