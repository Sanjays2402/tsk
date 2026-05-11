package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/util"
)

// newSearchCmd implements `tsk search <query>`: fuzzy match across tasks
// using title + tags + notes as the search key. The TUI already has
// instant search; this brings the same affordance to scripts and pipelines.
func newSearchCmd() *cobra.Command {
	var (
		limit     int
		asJSON    bool
		format    string
		showDone  bool
		showAll   bool
		titleOnly bool
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Fuzzy-search tasks by title, tags, and notes",
		Long: `Search tasks with sahilm/fuzzy (same scoring the TUI uses).

The default index is "title #tag1 #tag2 notes". Pass --title-only to
restrict matching to titles.

Examples:
  tsk search milk            # find tasks mentioning milk
  tsk search "fix login"     # multi-word fuzzy
  tsk search dev --done      # search done tasks only
  tsk search api --limit 5   # top 5 hits
  tsk search docs --json     # for jq / scripts
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErrorf("search requires a non-empty query")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			// Filter candidate set by done-state first, mirroring `tsk ls`.
			candidates := make([]model.Task, 0, len(s.Tasks))
			for _, t := range s.Tasks {
				switch {
				case showAll:
					candidates = append(candidates, t)
				case showDone:
					if t.Done {
						candidates = append(candidates, t)
					}
				default:
					if !t.Done {
						candidates = append(candidates, t)
					}
				}
			}
			// Build searchable keys.
			keys := make([]string, len(candidates))
			for i, t := range candidates {
				if titleOnly {
					keys[i] = t.Title
				} else {
					keys[i] = searchKey(t)
				}
			}
			matches := util.Fuzzy(keys, query)
			// Pull matched tasks in score order.
			hits := make([]model.Task, 0, len(matches))
			for _, m := range matches {
				hits = append(hits, candidates[m.Index])
				if limit > 0 && len(hits) >= limit {
					break
				}
			}
			format, err = resolveSearchFormat(format, asJSON)
			if err != nil {
				return err
			}
			return printSearchResults(cmd.OutOrStdout(), hits, query, format)
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "max results (0 = unlimited)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().StringVar(&format, "format", "", "output format: plain, table, or json")
	cmd.Flags().BoolVar(&showDone, "done", false, "search done tasks only")
	cmd.Flags().BoolVar(&showAll, "all", false, "search done + undone")
	cmd.Flags().BoolVar(&titleOnly, "title-only", false, "match against task title only (skip tags/notes)")
	return cmd
}

// searchKey builds the fuzzy index for a task: title, tags as #tag, then notes.
func searchKey(t model.Task) string {
	var b strings.Builder
	b.WriteString(t.Title)
	for _, tag := range t.Tags {
		b.WriteString(" #")
		b.WriteString(tag)
	}
	if t.Notes != "" {
		b.WriteString(" ")
		b.WriteString(t.Notes)
	}
	return b.String()
}

// resolveSearchFormat mirrors resolveLsFormat for the search command.
func resolveSearchFormat(format string, asJSON bool) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "" && asJSON {
		return "", usageErrorf("--format and --json are mutually exclusive")
	}
	if asJSON {
		return "json", nil
	}
	switch format {
	case "", "plain":
		return "plain", nil
	case "table":
		return "table", nil
	case "json":
		return "json", nil
	}
	return "", usageErrorf("unknown --format %q (want plain, table, or json)", format)
}

func printSearchResults(w io.Writer, hits []model.Task, query, format string) error {
	if len(hits) == 0 {
		// Stay quiet in JSON mode (emit empty array), explicit message otherwise.
		if format == "json" {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return enc.Encode([]model.Task{})
		}
		fmt.Fprintf(w, "no matches for %q\n", query)
		return nil
	}
	return printTasks(w, hits, format)
}
