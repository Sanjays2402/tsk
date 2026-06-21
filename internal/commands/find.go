package commands

import (
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newFindCmd implements `tsk find <regex>`: title-only regex search.
//
// The fast cousin of `tsk grep`. The semantic difference:
//
//   - grep   matches across title + tags + notes; reports the matched field
//   - find   matches against TITLE ONLY; cheap and focused
//
// For a polished `.tsk.md` with many tasks and long notes, `find` is
// the right tool when you remember a phrase from the title: it skips
// the notes scan entirely, which is the bulk of the work in `grep`.
//
// Defaults mirror `grep` so muscle memory carries over:
//
//   - case-insensitive by default (use -i=false for sensitive)
//   - undone-only by default; --done / --all expand scope
//   - prints "#ID  TITLE" one per line — the simplest scriptable shape
//
// Mutually-exclusive output modes:
//
//   - --files-only / -l   prints just IDs, one per line
//   - --count             prints just the match count
//   - --json              JSON array of task objects
//
// Why a separate command instead of `grep --title-only`? Because:
//
//  1. `grep --title-only` exists already (and find DOES delegate to it
//     conceptually). The verb `find` is a separate name people search
//     for ("how do I find tasks?") so adding it makes discovery
//     easier — the help output for `tsk find` is title-only-specific
//     and shorter than `tsk grep`'s.
//  2. The plain output is intentionally simpler: no "matched in: <field>"
//     annotation, since there's only ever one possible field. That's
//     friendlier for `tsk find foo | head` pipelines.
//  3. The default --limit and the field-only scope make find safe to
//     wire into completion / quick-pick UIs later without worrying
//     about notes-scan latency on huge stores.
func newFindCmd() *cobra.Command {
	var (
		ignoreCase bool
		invert     bool
		filesOnly  bool
		justCount  bool
		asJSON     bool
		limit      int
		doneOnly   bool
		showAll    bool
	)
	cmd := &cobra.Command{
		Use:   "find <regex>",
		Short: "Title-only RE2-regex search (faster than grep on big stores)",
		Long: `Search tasks by TITLE only. The fast, focused cousin of 'tsk grep':

  grep  matches across title + tags + notes (with field annotation)
  find  matches TITLE only — skips the notes scan entirely

Use 'find' when you remember a phrase from the title and don't need
to scan task bodies. Plain output is "#ID  TITLE" one per line —
the simplest scriptable shape.

Defaults:
  - case-insensitive (use -i=false for sensitive)
  - undone tasks only (--done / --all expand scope)

Mutually-exclusive output modes: --files-only / --count / --json.

Examples:
  tsk find ^deploy
  tsk find -i=false PR          # case-sensitive
  tsk find rent --all
  tsk find docs --files-only    # IDs only for pipelines
  tsk find urgent --count
  tsk find report --json
  tsk find done --invert        # tasks whose title does NOT match
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern := strings.Join(args, " ")
			if strings.TrimSpace(pattern) == "" {
				return usageErrorf("find requires a non-empty <regex>")
			}
			modeCount := 0
			for _, b := range []bool{filesOnly, justCount, asJSON} {
				if b {
					modeCount++
				}
			}
			if modeCount > 1 {
				return usageErrorf("--files-only, --count, --json are mutually exclusive")
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
			hits := findTitleHits(s.Tasks, re, doneOnly, showAll, invert, limit)
			return emitFindResults(cmd.OutOrStdout(), hits, justCount, filesOnly, asJSON)
		},
	}
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", true, "case-insensitive match (-i=false for sensitive)")
	cmd.Flags().BoolVar(&invert, "invert", false, "return tasks whose title does NOT match (like grep -v)")
	cmd.Flags().BoolVarP(&filesOnly, "files-only", "l", false, "print matching IDs only (one per line)")
	cmd.Flags().BoolVar(&justCount, "count", false, "print only the count of matching titles")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array of matching tasks")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap to N matches (0 = unlimited)")
	cmd.Flags().BoolVar(&doneOnly, "done", false, "search done tasks only")
	cmd.Flags().BoolVar(&showAll, "all", false, "search done + undone")
	return cmd
}

// findTitleHits walks the store, applies scope+invert+limit, and
// returns matching tasks in file order. Implementation is intentionally
// flatter than grep's: no field-annotation tracking because the
// matched field is always Title.
func findTitleHits(tasks []model.Task, re *regexp.Regexp, doneOnly, showAll, invert bool, limit int) []model.Task {
	out := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		switch {
		case showAll:
			// nothing — pass
		case doneOnly:
			if !t.Done {
				continue
			}
		default:
			if t.Done {
				continue
			}
		}
		matched := re.MatchString(t.Title)
		if invert {
			matched = !matched
		}
		if !matched {
			continue
		}
		out = append(out, t)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// emitFindResults dispatches between the output modes. Precedence
// matches the mutex guards in newFindCmd.
func emitFindResults(w io.Writer, hits []model.Task, justCount, filesOnly, asJSON bool) error {
	if justCount {
		pln(w, len(hits))
		return nil
	}
	if asJSON {
		if hits == nil {
			hits = []model.Task{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(hits)
	}
	if filesOnly {
		for _, t := range hits {
			pf(w, "%d\n", t.ID)
		}
		return nil
	}
	if len(hits) == 0 {
		pln(w, "no matches")
		return nil
	}
	for _, t := range hits {
		pf(w, "#%d  %s\n", t.ID, t.Title)
	}
	return nil
}

// (No additional helpers here — compileGrepPattern is reused from
// grep.go to keep the case-fold semantics identical across the two
// verbs.)
