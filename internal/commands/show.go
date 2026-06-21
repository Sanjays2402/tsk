package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newShowCmd implements `tsk show <id>`: a single-task detail view that
// expands everything tsk knows about one task — title, status, priority,
// due date, tags, full notes, and exact ISO timestamps.
//
// The `tsk ls` output is intentionally terse for scrollability; `tsk show`
// is the "open it" companion for when you want every field. With `--json`
// it emits the same task object `tsk export --json` does, but for a
// single id, so scripts can pluck one field without piping through jq.
//
// `--tree` is the "downstream context" companion: it prints the standard
// snapshot AND appends the recursive prerequisite chain rooted at <id>.
// Saves users from running `tsk show 7 && tsk depend 7 --tree` back-to-back
// when triaging a blocked task. The tree section is suppressed entirely
// when the task has no dependencies (skip the empty "dependencies:" header
// so the output for a leaf task stays identical to a plain `tsk show`).
// In JSON mode, --tree adds a `dependency_tree` field containing the
// same nested shape `tsk depend <id> --tree --json` emits.
//
// `--upstream` is the inverse-direction companion: appends the list of
// tasks that depend on <id>, with the same "(unblocks)/(blocked)/(done)"
// annotations `tsk depend --upstream` uses. Saves users running
// `tsk show 7 && tsk depend 7 --upstream` back-to-back when deciding
// "if I close this, what becomes actionable?". Section is suppressed
// when nothing depends on the task — same idempotency contract as --tree.
// In JSON mode, --upstream adds an `upstream` array.
//
// --tree and --upstream are mutually exclusive — they each render a
// different relationship and combining them would muddle the snapshot's
// layout. Pick one per call.
func newShowCmd() *cobra.Command {
	var (
		asJSON   bool
		tree     bool
		upstream bool
	)
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show full detail for a single task",
		Long: `Show every field tsk knows about one task.

Plain output formats the task across multiple lines with full ISO timestamps,
labelled fields, and the complete notes block. Pass --json for the same task
object as 'tsk export --json' but scoped to one id.

--tree appends the recursive prerequisite chain (the same shape
'tsk depend <id> --tree' would render) under the snapshot. Saves a
follow-up command when triaging a blocked task. Suppressed when the
task has no dependencies.

--upstream appends the list of tasks that depend on this one (the
same view 'tsk depend <id> --upstream' renders), with state
annotations. Saves a follow-up when deciding "if I close this, what
becomes actionable?". Suppressed when nothing depends on the task.

--tree and --upstream are mutually exclusive — each shows a
different relationship.

Examples:
  tsk show 3
  tsk show 3 --json
  tsk show 3 --tree            # snapshot + dep tree (downstream prereqs)
  tsk show 3 --upstream        # snapshot + dependents (upstream)
  tsk show 3 --tree --json     # JSON snapshot + nested dep tree
  tsk show 3 --upstream --json # JSON snapshot + upstream array
  tsk show 3 --json | jq -r '.Notes'
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tree && upstream {
				return usageErrorf("--tree and --upstream are mutually exclusive (each shows a different relationship)")
			}
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
				switch {
				case tree:
					return emitShowJSONWithTree(cmd.OutOrStdout(), s, t)
				case upstream:
					return emitShowJSONWithUpstream(cmd.OutOrStdout(), s, t)
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(t)
			}
			printTaskDetail(cmd.OutOrStdout(), t)
			if tree && t.HasDependencies() {
				pln(cmd.OutOrStdout())
				pln(cmd.OutOrStdout(), "dependencies:")
				printDependTreeText(cmd.OutOrStdout(), s, t, 0, make(map[int]bool))
			}
			if upstream {
				rows := collectUpstreamDependents(s, t)
				if len(rows) > 0 {
					pln(cmd.OutOrStdout())
					pln(cmd.OutOrStdout(), "upstream:")
					for _, r := range rows {
						pf(cmd.OutOrStdout(), "  #%d  %s  (%s)\n", r.ID, r.Title, r.Status)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (single task object)")
	cmd.Flags().BoolVar(&tree, "tree", false, "append the recursive prerequisite chain below the snapshot")
	cmd.Flags().BoolVar(&upstream, "upstream", false, "append the list of tasks that depend on this one")
	return cmd
}

// emitShowJSONWithTree builds the JSON shape for `tsk show <id> --tree
// --json`: the task object embedded with an extra `dependency_tree`
// field carrying the same nested structure `depend --tree --json`
// produces. We can't decorate *model.Task itself because the model
// has no awareness of the tree shape (and shouldn't — it's a view
// concern), so we round-trip the task into a generic map and tack
// the tree on as an extra key.
//
// The tree key is omitted when the task has no dependencies so the
// JSON shape for a leaf task matches plain `--json` exactly (callers
// using a fixed schema don't suddenly see a null/empty field appear).
func emitShowJSONWithTree(w io.Writer, s *store.Store, t *model.Task) error {
	// Marshal the task first so we get the exact same field set the
	// non-tree path produces, then unmarshal into a map to splice on
	// the extra key. Two-step round-trip keeps the schema stable.
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if t.HasDependencies() {
		doc["dependency_tree"] = buildDependTreeNode(s, t, make(map[int]bool))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// emitShowJSONWithUpstream is the mirror of emitShowJSONWithTree for
// the inverse direction. Same round-trip-and-splice technique so the
// existing task field set is preserved exactly: callers that already
// parse `tsk show --json` keep their schema, plus a new top-level
// `upstream` array when there are dependents.
//
// The upstream key is OMITTED entirely when nothing depends on the
// task — schema parity with the --tree variant on a leaf task. A
// dependent-less task and a dependent-having task should be
// distinguishable by key presence (`.upstream | length` works only
// because we set [] when there ARE upstream rows, never omit them
// in that case; absence means "no dependents at all").
func emitShowJSONWithUpstream(w io.Writer, s *store.Store, t *model.Task) error {
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	rows := collectUpstreamDependents(s, t)
	if len(rows) > 0 {
		doc["upstream"] = rows
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
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
	if len(t.DependsOn) > 0 {
		ids := make([]string, 0, len(t.DependsOn))
		for _, id := range t.DependsOn {
			ids = append(ids, fmt.Sprintf("#%d", id))
		}
		pf(w, "depends:   %s\n", strings.Join(ids, ", "))
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
	if t.Started != nil {
		pf(w, "started:   %s\n", t.Started.Format("2006-01-02 15:04:05 -0700"))
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
