package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/spf13/cobra"
)

// newNextCmd implements `tsk next`: surface the highest-priority
// undone task with the canonical tie-break (pin > priority desc >
// dated-first > earliest-due > lowest-id).
//
// --respect-deps skips tasks that are blocked by at least one open
// prerequisite. That's what most users actually want when they ask
// "what should I work on next?" — a task you can't legally close
// shouldn't be the answer. Defaults to OFF so existing scripts that
// rely on the legacy "pure priority" behavior keep working; opt in
// with the flag (or the future config knob).
//
// When --respect-deps is set and EVERY undone task is blocked, the
// command falls back to surfacing the highest-priority blocked one
// with a "(blocked)" annotation rather than going silent — the user
// likely wants to know "everything's stuck on X" instead of
// "all caught up" (which would be a lie).
//
// --skip <ids> excludes specific tasks from the candidate pool
// without persistently mutating them. The use case is "I know about
// these, give me the NEXT next" — e.g. you just rejected the
// suggestion and want the runner-up without making a wait/freeze
// commitment. Comma-separated, accepts `#7` and `7` forms. Unknown
// ids are silently ignored (the whole point is "do not consider
// these"; erroring would make the flag harder to use from scripts
// that already may have stale ids).
//
// --json emits a structured object so pipelines can branch on
// fields without parsing the human-readable line. Empty-store /
// all-caught-up renders as {"empty": true} so consumers reliably
// detect "no task" via `jq '.empty'` without sentinel string match.
func newNextCmd() *cobra.Command {
	var respectDeps bool
	var asJSON bool
	var skipCSV string
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show the highest-priority undone task",
		Long: `Show the highest-priority undone task.

Pin and priority drive selection; ties break on due date then id.

--respect-deps skips tasks blocked by open prerequisites. That's
usually what you want — a task you can't legally close shouldn't be
the suggested "next thing". When every candidate is blocked, the
command falls back to the highest-priority blocked task with a
"(blocked by #X, #Y)" annotation so you know what's gating progress.

--skip <ids> excludes the named tasks from the candidate pool
without mutating them. Useful when you've already rejected a
suggestion ("I know about #3, give me the NEXT next") without
needing to wait/freeze the task. Comma-separated; ` + "`#7`" + ` and
` + "`7`" + ` both work. Stacks with --respect-deps so you can
combine "skip these AND skip the blocked ones".

--json emits a structured object with id/title/priority/due/pinned/
blocked_by/blocked (the all-blocked fallback flag). Empty store or
caught-up status renders {"empty": true} so scripts can detect it
without sentinel string matching.

Examples:
  tsk next                       # legacy: priority-only
  tsk next --respect-deps        # skip tasks with unmet prereqs
  tsk next --skip 3              # ignore #3 — give me runner-up
  tsk next --skip 3,5,7          # ignore several at once
  tsk next --respect-deps --skip 3
  tsk next --json                # script-friendly object
  tsk next --json | jq -r '.id'  # bare id for the next pipeline stage
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			skipSet, err := parseSkipIDs(skipCSV)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			now := time.Now()
			var best, bestBlocked *model.Task
			var bestBlockers []int
			for i := range s.Tasks {
				t := &s.Tasks[i]
				if t.Done {
					continue
				}
				if t.IsWaiting(now) {
					continue
				}
				if skipSet[t.ID] {
					continue
				}
				if respectDeps {
					blockers := unmetBlockers(s, t, nil)
					if len(blockers) > 0 {
						// Track best blocked candidate as fallback.
						if isBetterNext(t, bestBlocked) {
							bestBlocked = t
							bestBlockers = blockers
						}
						continue
					}
				}
				if isBetterNext(t, best) {
					best = t
				}
			}
			if asJSON {
				return emitNextJSON(cmd, best, bestBlocked, bestBlockers)
			}
			if best == nil && bestBlocked != nil {
				// All candidates blocked — surface the best blocked one
				// with annotation so the user knows what's stuck.
				printNextLine(cmd, bestBlocked, bestBlockers)
				return nil
			}
			if best == nil {
				pln(cmd.OutOrStdout(), "all caught up")
				return nil
			}
			printNextLine(cmd, best, nil)
			return nil
		},
	}
	cmd.Flags().BoolVar(&respectDeps, "respect-deps", false, "skip tasks with unmet prerequisites")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit structured JSON")
	cmd.Flags().StringVar(&skipCSV, "skip", "", "comma-separated task ids to exclude from selection")
	return cmd
}

// parseSkipIDs converts the --skip CSV into a lookup set. Tolerates
// "#N" / "N" notation, dedupes, and silently drops empty tokens (so
// trailing/leading commas don't error). An empty or whitespace-only
// flag value returns an empty set without error — equivalent to
// "no skips", which the user opts into by setting the flag at all.
//
// Returns a usage-coded error on non-numeric tokens so main.go exits
// with code 2 and the user sees the typo before any store work
// happens.
func parseSkipIDs(raw string) (map[int]bool, error) {
	out := make(map[int]bool)
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		tok = strings.TrimPrefix(tok, "#")
		if tok == "" {
			continue
		}
		n, err := strconvAtoiPos(tok)
		if err != nil || n == 0 {
			return nil, usageErrorf("invalid task id %q in --skip", tok)
		}
		out[n] = true
	}
	return out, nil
}

// isBetterNext returns true when t should beat current under the
// canonical next-task ordering (pin > priority desc > dated-first >
// earliest-due > lowest-id). Reuses the same tie-breaks as `tsk top`
// so top[0] and `next` agree when no pins are in play.
func isBetterNext(t, current *model.Task) bool {
	if current == nil {
		return true
	}
	if t.Pinned != current.Pinned {
		return t.Pinned
	}
	if t.Priority != current.Priority {
		return t.Priority > current.Priority
	}
	switch {
	case t.Due != nil && current.Due == nil:
		return true
	case t.Due == nil && current.Due != nil:
		return false
	case t.Due != nil && current.Due != nil:
		if !t.Due.Equal(*current.Due) {
			return t.Due.Before(*current.Due)
		}
	}
	return t.ID < current.ID
}

// printNextLine renders the result row. When blockers is non-empty,
// append " (blocked by #X, #Y)" so the user understands why this
// task came back as the best available even though they asked
// --respect-deps.
func printNextLine(cmd *cobra.Command, t *model.Task, blockers []int) {
	pinMark := ""
	if t.Pinned {
		pinMark = "* "
	}
	line := fmt.Sprintf("%s#%d [%s] %s", pinMark, t.ID, t.Priority, t.Title)
	if t.Due != nil {
		line += "  due:" + t.Due.Format(model.DateLayout)
	}
	if len(blockers) > 0 {
		line += "  (blocked by " + formatBlockerIDs(blockers) + ")"
	}
	pln(cmd.OutOrStdout(), line)
}

// nextJSON is the structured shape returned by `tsk next --json`.
// Empty/all-caught-up encodes as {"empty": true} so consumers can
// branch on a real boolean instead of pattern-matching the text
// "all caught up". When the all-blocked fallback fires, the Blocked
// boolean flips true and BlockedBy lists the open prereqs that
// gated the suggestion — matching the "(blocked by …)" annotation
// that the human-readable path appends.
type nextJSON struct {
	Empty     bool     `json:"empty,omitempty"`
	ID        int      `json:"id,omitempty"`
	Title     string   `json:"title,omitempty"`
	Priority  string   `json:"priority,omitempty"`
	Due       string   `json:"due,omitempty"`
	Pinned    bool     `json:"pinned,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Blocked   bool     `json:"blocked,omitempty"`
	BlockedBy []int    `json:"blocked_by,omitempty"`
}

// emitNextJSON writes the nextJSON document. We deliberately keep a
// stable schema (every field present in the type, omitempty for
// optional ones) so downstream `jq` calls are predictable.
func emitNextJSON(cmd *cobra.Command, best, bestBlocked *model.Task, blockers []int) error {
	doc := nextJSON{}
	t := best
	if t == nil && bestBlocked != nil {
		t = bestBlocked
		doc.Blocked = true
		doc.BlockedBy = blockers
	}
	if t == nil {
		doc.Empty = true
	} else {
		doc.ID = t.ID
		doc.Title = t.Title
		doc.Priority = t.Priority.String()
		if t.Due != nil {
			doc.Due = t.Due.Format(model.DateLayout)
		}
		doc.Pinned = t.Pinned
		if len(t.Tags) > 0 {
			doc.Tags = append([]string(nil), t.Tags...)
		}
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
