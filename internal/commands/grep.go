package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newGrepCmd implements `tsk grep <regex>`: exact regex search across
// task fields. The cousin of `tsk search` (which uses sahilm/fuzzy):
//
//   - search   ranked fuzzy match, forgiving spelling, returns scored hits
//   - grep     exact RE2 regex, no scoring, returns matches in file order
//
// When you know exactly what you want, fuzzy is noise. When you want
// every task mentioning "TODO\(.+\):", grep is the right tool.
//
// Defaults are POSIX-grep-shaped:
//   - case-INSENSITIVE by default (Go regex `(?i)` prefix)
//   - matches against title + tags + notes (the same "search key" the
//     fuzzy search uses); pass --title-only / --notes-only to narrow
//   - undone-only by default (matches `ls` semantics); --done / --all
//     expand scope just like search
//   - prints "ID  TITLE  (matched-field)" per line so you can see WHY
//     it matched; --quiet skips the field annotation
//
// Flags:
//
//	-i, --ignore-case   (default true — pass `-i=false` for case-sensitive)
//	    --invert        invert match (return non-matching tasks)
//	    --title-only    only consider Title
//	    --notes-only    only consider Notes
//	-l, --files-only    print matching IDs only (one per line) — useful in pipelines
//	    --count         print the match count and exit (like `grep -c`)
//	    --json          JSON array of task objects
//	    --limit N       cap result count (0 = unlimited)
//	    --done          search done tasks only
//	    --all           search done + undone
//
// Note we DELIBERATELY don't add `-v` as a short flag for --invert
// because `-v` collides with the version verb in many cobra setups
// and we want to keep the command surface unambiguous.
func newGrepCmd() *cobra.Command {
	var (
		ignoreCase bool
		invert     bool
		titleOnly  bool
		notesOnly  bool
		filesOnly  bool
		justCount  bool
		asJSON     bool
		limit      int
		doneOnly   bool
		showAll    bool
	)
	cmd := &cobra.Command{
		Use:   "grep <regex>",
		Short: "Exact RE2-regex search across title, tags, and notes",
		Long: `Exact regex search across tasks. RE2 syntax (Go's regexp package).

By default:
  - case-insensitive (pass --ignore-case=false or -i=false for sensitive)
  - matches against title + tags + notes
  - undone tasks only (use --done or --all to expand)
  - prints "<id>  <title>  (matched in: <field>)" per line

Composes with --limit to cap, --count for just the number, --files-only
to print just IDs (great for pipelines: 'tsk grep PR | xargs ...').

Examples:
  tsk grep 'TODO\(.+\)'
  tsk grep -i=false PR              # case-sensitive
  tsk grep deploy --notes-only
  tsk grep '^fix' --files-only      # IDs only, scriptable
  tsk grep urgent --count
  tsk grep doc --all --json
  tsk grep meeting --invert         # tasks that DON'T match
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern := strings.Join(args, " ")
			if strings.TrimSpace(pattern) == "" {
				return usageErrorf("grep requires a non-empty <regex>")
			}
			if titleOnly && notesOnly {
				return usageErrorf("--title-only and --notes-only are mutually exclusive")
			}
			if filesOnly && justCount {
				return usageErrorf("--files-only and --count are mutually exclusive")
			}
			if filesOnly && asJSON {
				return usageErrorf("--files-only and --json are mutually exclusive")
			}
			if justCount && asJSON {
				return usageErrorf("--count and --json are mutually exclusive")
			}
			if limit < 0 {
				return usageErrorf("--limit must be >= 0, got %d", limit)
			}
			if doneOnly && showAll {
				return usageErrorf("--done and --all are mutually exclusive")
			}
			re, err := compileGrepPattern(pattern, ignoreCase)
			if err != nil {
				return usageErrorf("invalid regex %q: %v", pattern, err)
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			scope := grepScope{titleOnly: titleOnly, notesOnly: notesOnly, doneOnly: doneOnly, showAll: showAll}
			hits := grepTasks(s.Tasks, re, scope, invert, limit)
			return emitGrepResults(cmd.OutOrStdout(), hits, justCount, filesOnly, asJSON)
		},
	}
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", true, "case-insensitive match (pass -i=false for sensitive)")
	cmd.Flags().BoolVar(&invert, "invert", false, "return tasks that DON'T match (like grep -v)")
	cmd.Flags().BoolVar(&titleOnly, "title-only", false, "match against task title only")
	cmd.Flags().BoolVar(&notesOnly, "notes-only", false, "match against task notes only")
	cmd.Flags().BoolVarP(&filesOnly, "files-only", "l", false, "print matching IDs only (one per line)")
	cmd.Flags().BoolVar(&justCount, "count", false, "print only the count of matching tasks")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array of matching tasks")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap to N matches (0 = unlimited)")
	cmd.Flags().BoolVar(&doneOnly, "done", false, "search done tasks only")
	cmd.Flags().BoolVar(&showAll, "all", false, "search done + undone")
	return cmd
}

// grepHit is one matching task plus the field that matched, used for
// the labelled plain output.
type grepHit struct {
	Task  model.Task
	Field string // "title", "tag", "notes" — the first field that matched
}

// grepScope holds the filter knobs that narrow what we search over.
type grepScope struct {
	titleOnly, notesOnly bool
	doneOnly, showAll    bool
}

// compileGrepPattern compiles the RE2 pattern with optional case-folding.
// Pre-pending `(?i)` keeps the original pattern preserved in error
// messages — and skips any user-supplied `(?i)` collision.
func compileGrepPattern(pattern string, ignoreCase bool) (*regexp.Regexp, error) {
	if ignoreCase && !strings.HasPrefix(pattern, "(?i)") {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

// grepTasks walks the store, applies scope, invert, and limit, and
// returns the (task, matched-field) hits in file order.
func grepTasks(tasks []model.Task, re *regexp.Regexp, scope grepScope, invert bool, limit int) []grepHit {
	out := make([]grepHit, 0, len(tasks))
	for _, t := range tasks {
		if !grepStateAllows(t, scope) {
			continue
		}
		field, matched := grepFindField(t, re, scope)
		if invert {
			matched = !matched
			if matched {
				field = ""
			}
		}
		if !matched {
			continue
		}
		out = append(out, grepHit{Task: t, Field: field})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// grepStateAllows mirrors the done-state arbitration used by `ls`.
func grepStateAllows(t model.Task, scope grepScope) bool {
	switch {
	case scope.showAll:
		return true
	case scope.doneOnly:
		return t.Done
	default:
		return !t.Done
	}
}

// grepFindField returns the first field that matched and whether any
// did. Title is checked first, then tags, then notes — this ordering
// keeps the most useful annotation when multiple fields would match.
func grepFindField(t model.Task, re *regexp.Regexp, scope grepScope) (string, bool) {
	if !scope.notesOnly && re.MatchString(t.Title) {
		return "title", true
	}
	if !scope.titleOnly && !scope.notesOnly {
		for _, tg := range t.Tags {
			if re.MatchString(tg) {
				return "tag", true
			}
		}
	}
	if !scope.titleOnly && t.Notes != "" && re.MatchString(t.Notes) {
		return "notes", true
	}
	return "", false
}

// emitGrepResults dispatches to the right output mode. Order of checks
// matches the precedence rejected by the mutex guards in newGrepCmd.
func emitGrepResults(w io.Writer, hits []grepHit, justCount, filesOnly, asJSON bool) error {
	if justCount {
		pln(w, len(hits))
		return nil
	}
	if asJSON {
		tasks := make([]model.Task, len(hits))
		for i, h := range hits {
			tasks[i] = h.Task
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tasks)
	}
	if filesOnly {
		for _, h := range hits {
			pf(w, "%d\n", h.Task.ID)
		}
		return nil
	}
	if len(hits) == 0 {
		pln(w, "no matches")
		return nil
	}
	for _, h := range hits {
		annot := ""
		if h.Field != "" {
			annot = fmt.Sprintf("  (matched in: %s)", h.Field)
		}
		pf(w, "#%d  %s%s\n", h.Task.ID, h.Task.Title, annot)
	}
	return nil
}
