package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newRebuildIDsCmd implements `tsk rebuild-ids`: densify the ID space
// after lots of removes.
//
// After a long history of `rm`, IDs become sparse (1, 5, 7, 12). This
// makes `tsk done 7` a guess. rebuild-ids renumbers tasks in current
// file order, starting at 1, so the IDs become contiguous (1, 2, 3, 4).
//
// This is a DESTRUCTIVE operation — the IDs printed by `top`, `next`,
// `last`, exported JSON, and anything bookmarked in a shell history all
// shift. So:
//
//   - Dry-run is the default. The command prints the mapping (old -> new)
//     and exits without writing.
//   - --apply commits the change. Also requires --yes (so a stray
//     `tsk rebuild-ids --apply` from a script doesn't surprise the user).
//   - --since-id N (optional): only renumber tasks whose current ID is
//     >= N, leaving lower IDs alone. Useful when you want to clean up
//     a tail of removes without disturbing IDs people might have noted.
//
// store.Save uses atomic write + .bak snapshot, so `tsk undo-last`
// reverts a surprise renumber.
func newRebuildIDsCmd() *cobra.Command {
	var (
		apply   bool
		yes     bool
		sinceID int
		asJSON  bool
	)
	cmd := &cobra.Command{
		Use:     "rebuild-ids",
		Aliases: []string{"densify-ids", "renumber"},
		Short:   "Densify task IDs so they become contiguous (1,2,3,...)",
		Long: `Renumber tasks so the ID sequence is dense (1, 2, 3, ...).

After many removes, IDs become sparse (1, 5, 7, 12). This makes
'tsk done 7' a guess. rebuild-ids renumbers in current file order
so IDs become contiguous.

This is destructive — IDs printed anywhere ('top', 'next', 'last',
exported JSON, shell history) shift. So:

  - DRY RUN is the default. Prints the (old -> new) mapping and
    exits without writing.
  - --apply commits. ALSO requires --yes (defense against scripts
    accidentally rewriting an entire store).
  - --since-id N renumbers only tasks with id >= N (leaves lower
    IDs alone — useful if you wrote them down).

The save path snapshots the previous file to .tsk.md.bak, so
'tsk undo-last' reverts a surprise rebuild.

Examples:
  tsk rebuild-ids                # preview the mapping
  tsk rebuild-ids --apply --yes  # actually renumber
  tsk rebuild-ids --since-id 100 --apply --yes  # only renumber 100+

  tsk rebuild-ids --json | jq    # scriptable mapping
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if apply && !yes {
				return usageErrorf("rebuild-ids --apply requires --yes (this rewrites every task id)")
			}
			if sinceID < 0 {
				return usageErrorf("--since-id must be >= 0, got %d", sinceID)
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			mapping := planRebuildIDs(s.Tasks, sinceID)
			if asJSON {
				if err := emitRebuildJSON(cmd.OutOrStdout(), mapping, apply); err != nil {
					return err
				}
			} else {
				printRebuildPlain(cmd.OutOrStdout(), mapping, apply)
			}
			if !apply {
				return nil
			}
			applyRebuildIDs(s.Tasks, mapping)
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "rebuilt %d id(s); previous file snapshotted to %s.bak\n",
				countChanged(mapping), s.Path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "actually write the renumbered ids (default: dry run)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm the destructive renumber (required with --apply)")
	cmd.Flags().IntVar(&sinceID, "since-id", 0, "only renumber tasks with current id >= N (0 = all)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// idMapping is one (old -> new) entry. Old == New means unchanged.
type idMapping struct {
	Old int `json:"old"`
	New int `json:"new"`
}

// planRebuildIDs walks tasks in file order and computes the renumber
// plan. Tasks below sinceID keep their current id; their id values are
// also reserved so the new ids don't collide with them.
func planRebuildIDs(tasks []model.Task, sinceID int) []idMapping {
	mapping := make([]idMapping, 0, len(tasks))
	reserved := make(map[int]bool, len(tasks))
	for _, t := range tasks {
		if t.ID > 0 && t.ID < sinceID {
			reserved[t.ID] = true
		}
	}
	next := 1
	for _, t := range tasks {
		if t.ID < sinceID && t.ID > 0 {
			mapping = append(mapping, idMapping{Old: t.ID, New: t.ID})
			continue
		}
		for reserved[next] {
			next++
		}
		mapping = append(mapping, idMapping{Old: t.ID, New: next})
		reserved[next] = true
		next++
	}
	return mapping
}

// applyRebuildIDs mutates tasks in place according to mapping. Caller
// must save the store after.
func applyRebuildIDs(tasks []model.Task, mapping []idMapping) {
	for i := range tasks {
		tasks[i].ID = mapping[i].New
	}
}

// countChanged returns the number of mappings where Old != New.
func countChanged(mapping []idMapping) int {
	n := 0
	for _, m := range mapping {
		if m.Old != m.New {
			n++
		}
	}
	return n
}

// printRebuildPlain renders the mapping for human consumption.
func printRebuildPlain(w io.Writer, mapping []idMapping, apply bool) {
	header := "DRY RUN (pass --apply --yes to commit):"
	if apply {
		header = "APPLYING:"
	}
	pln(w, header)
	if len(mapping) == 0 {
		pln(w, "  (no tasks)")
		return
	}
	// Sort by Old asc for stable, scannable output.
	sorted := make([]idMapping, len(mapping))
	copy(sorted, mapping)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Old < sorted[j].Old })
	for _, m := range sorted {
		marker := "  "
		if m.Old != m.New {
			marker = "* "
		}
		pf(w, "%s#%d -> #%d\n", marker, m.Old, m.New)
	}
	pf(w, "summary: %d task(s), %d will change\n", len(mapping), countChanged(mapping))
}

// emitRebuildJSON writes a stable JSON document for scriptable use.
func emitRebuildJSON(w io.Writer, mapping []idMapping, apply bool) error {
	if mapping == nil {
		mapping = []idMapping{}
	}
	doc := struct {
		Apply   bool        `json:"apply"`
		Mapping []idMapping `json:"mapping"`
		Changed int         `json:"changed"`
	}{
		Apply:   apply,
		Mapping: mapping,
		Changed: countChanged(mapping),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// summaryString is a small helper for tests that want a one-line gist.
func summaryString(mapping []idMapping) string {
	return fmt.Sprintf("%d/%d", countChanged(mapping), len(mapping))
}
