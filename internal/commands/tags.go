package commands

import (
	"encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newTagsCmd implements `tsk tags`: a full per-tag usage report.
//
// This is the extended cousin of `stats`'s "top tags" block — that one
// caps at 5 and only counts COMPLETED tasks (the whole point of `stats`
// is the activity it tracks). `tags` is for the catalog:
//
//   - default scope is UNDONE tasks (the working set)
//   - --all expands to undone + done
//   - --done restricts to done-only
//   - results are sorted by count desc, then alphabetically for ties
//   - `--json` emits [{tag, count}, ...]
//   - `--limit N` caps the list (0 = unlimited; default unlimited)
//   - `--min N` hides tags used fewer than N times (useful for noisy stores)
//
// Output ordering choice: count-desc with alpha tiebreak matches what a
// human scanning "which tags do I actually use?" expects. Pass --sort
// alpha to get pure alphabetical (handy for completion menus etc.).
func newTagsCmd() *cobra.Command {
	var (
		includeAll bool
		doneOnly   bool
		asJSON     bool
		limit      int
		minCount   int
		sortMode   string
	)
	cmd := &cobra.Command{
		Use:     "tags",
		Aliases: []string{"taglist"},
		Short:   "List all tags with usage counts",
		Long: `List every tag in the store with its usage count.

Scope (mutually exclusive):
  default    undone tasks only (the working set)
  --all      undone + done
  --done     done tasks only

Output:
  --json     emit JSON [{"tag": string, "count": int}, ...]
  --limit N  cap to N rows (0 = unlimited; default unlimited)
  --min N    only show tags used at least N times
  --sort X   "count" (default: count desc, alpha tiebreak) or "alpha"

Examples:
  tsk tags                    # what tags am I using right now?
  tsk tags --all              # historical catalog
  tsk tags --json | jq -r '.[].tag'
  tsk tags --limit 10         # top 10
  tsk tags --min 3            # only tags used 3+ times
  tsk tags --sort alpha       # for completion menus
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			scope, err := pickTagScope(includeAll, doneOnly)
			if err != nil {
				return err
			}
			if limit < 0 {
				return usageErrorf("--limit must be >= 0, got %d", limit)
			}
			if minCount < 0 {
				return usageErrorf("--min must be >= 0, got %d", minCount)
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			rows := buildTagRows(s.Tasks, scope, minCount, sortMode, limit)
			return emitTagRows(cmd.OutOrStdout(), rows, asJSON)
		},
	}
	cmd.Flags().BoolVar(&includeAll, "all", false, "include done tasks in the count (default: undone only)")
	cmd.Flags().BoolVar(&doneOnly, "done", false, "only count done tasks")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap to N rows (0 = unlimited)")
	cmd.Flags().IntVar(&minCount, "min", 1, "hide tags used fewer than this many times")
	cmd.Flags().StringVar(&sortMode, "sort", "count", "sort order: 'count' (desc, alpha tiebreak) or 'alpha'")
	return cmd
}

// tagScope selects which tasks count toward the tally.
type tagScope int

const (
	tagScopeUndone tagScope = iota
	tagScopeAll
	tagScopeDone
)

// pickTagScope enforces the mutual exclusion of --all / --done and picks
// the resulting scope. Default (neither flag) is undone.
func pickTagScope(all, done bool) (tagScope, error) {
	if all && done {
		return 0, usageErrorf("--all and --done are mutually exclusive")
	}
	switch {
	case all:
		return tagScopeAll, nil
	case done:
		return tagScopeDone, nil
	default:
		return tagScopeUndone, nil
	}
}

// tagRow is the per-tag tally returned by buildTagRows. JSON-tagged so
// it can serialize directly via --json.
type tagRow struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// buildTagRows tallies tags across the in-scope tasks, drops anything
// below minCount, sorts per sortMode, and caps to limit.
func buildTagRows(tasks []model.Task, scope tagScope, minCount int, sortMode string, limit int) []tagRow {
	counts := map[string]int{}
	for _, t := range tasks {
		if !taskInScope(t, scope) {
			continue
		}
		for _, tg := range t.Tags {
			counts[tg]++
		}
	}
	rows := make([]tagRow, 0, len(counts))
	for tg, c := range counts {
		if c < minCount {
			continue
		}
		rows = append(rows, tagRow{Tag: tg, Count: c})
	}
	sortTagRows(rows, sortMode)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// taskInScope returns whether the task should be counted under scope.
func taskInScope(t model.Task, scope tagScope) bool {
	switch scope {
	case tagScopeAll:
		return true
	case tagScopeDone:
		return t.Done
	default:
		return !t.Done
	}
}

// sortTagRows applies the sort mode. Unknown mode falls back to "count"
// so an honest typo doesn't crash the command — the help text documents
// the supported values.
func sortTagRows(rows []tagRow, mode string) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "alpha":
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Tag < rows[j].Tag
		})
	default:
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Count != rows[j].Count {
				return rows[i].Count > rows[j].Count
			}
			return rows[i].Tag < rows[j].Tag
		})
	}
}

// emitTagRows renders the result set, either as JSON or as a labelled
// two-column human table.
func emitTagRows(w io.Writer, rows []tagRow, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if rows == nil {
			rows = []tagRow{} // emit [] not null
		}
		return enc.Encode(rows)
	}
	if len(rows) == 0 {
		pln(w, "no tags")
		return nil
	}
	width := tagColumnWidth(rows)
	for _, r := range rows {
		pf(w, "%-*s  %d\n", width, "#"+r.Tag, r.Count)
	}
	return nil
}

// tagColumnWidth computes the longest "#<tag>" string for column alignment.
func tagColumnWidth(rows []tagRow) int {
	w := 0
	for _, r := range rows {
		l := len(r.Tag) + 1 // "#"
		if l > w {
			w = l
		}
	}
	return w
}
