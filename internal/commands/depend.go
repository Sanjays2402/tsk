package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newDependCmd implements `tsk depend <id>`: declare or inspect the
// dependency graph for one task.
//
// Three modes (mutually exclusive):
//
//   - `tsk depend <id> --on 3,5`     set DependsOn to {3, 5}, replacing
//     any prior list. Strict replace,
//     not add — mirrors `tsk tag <id>
//     +foo -bar`'s "I'm telling you the
//     final state" model.
//   - `tsk depend <id> --add 7`      add ids without losing existing ones
//   - `tsk depend <id> --remove 3`   drop specific ids
//   - `tsk depend <id> --clear`      drop them all (alias for --remove
//     everything; ergonomic)
//   - `tsk depend <id>`              (no flags) prints the current
//     DependsOn list and the OPEN
//     subset (the actual blockers)
//   - `tsk depend --list`            global view of every blocked
//     task — useful for "what's stuck
//     on what?"
//
// Hard rules:
//
//   - self-deps rejected (cycles trivial → infinite blocking)
//   - direct cycle prevention: if `A --on B` and `B --on A` is the
//     proposed state, refuse with a clear error
//   - referenced ids must exist (no dangling refs from the CLI;
//     hand-edits are tolerated, see lint.go for the eventual surface)
//
// The enforcement (refusing to mark a blocked task done) lives in
// `toggle.go` via unmetBlockers — depend just curates the list.
func newDependCmd() *cobra.Command {
	var (
		onCSV     string
		addCSV    string
		removeCSV string
		clear     bool
		list      bool
		tree      bool
		justify   bool
		upstream  bool
		asJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "depend [<id>]",
		Short: "Track task dependencies (blocks done when prereqs are open)",
		Long: `Track task dependencies. Done is BLOCKED until every dependency is
itself done. Storage: ` + "`depends:1,5,7`" + ` in the task's meta block,
strictly additive so old files round-trip unchanged.

Set/replace:    tsk depend <id> --on 3,5
Add ids:        tsk depend <id> --add 7
Remove ids:     tsk depend <id> --remove 3
Clear all:      tsk depend <id> --clear
Inspect one:    tsk depend <id>
Inspect tree:   tsk depend <id> --tree         (recursive prereq chain)
Justify block:  tsk depend <id> --justify      (why is this blocked?)
Upstream view:  tsk depend <id> --upstream     (what depends on me?)
Inspect all:    tsk depend --list              (every blocked task at once)

Self-dependencies and direct cycles (A↔B) are rejected. Bigger-than-
two cycles are intentionally NOT detected — they're rare in practice
and the user would notice when both ends refuse to close. The --tree
view DOES detect deeper cycles defensively (marks them "(cycle)") so
a corrupt hand-edit can't put the renderer in an infinite loop.

--justify walks the prereq chain and reports a chain of reasons:
"#5 blocked by #3 (open) which is blocked by #7 (open)". Stops at
the first OPEN-with-no-open-prereqs leaf — that's the actionable
task. Tree (--tree) shows structure; justify shows the WHY chain
in plain English, optimized for the "what do I do about this?"
question.

--upstream answers the reverse question: "what tasks have ME as a
prerequisite?" Useful before closing a task: "did anyone depend on
this?" or "should I notify N other tickets that this is unblocked?"
By default emits the DIRECT dependents (tasks listing this id in
their DependsOn) sorted by id. Pass --json for the same set in
structured form.

Examples:
  tsk depend 7 --on 3,5         # 7 needs 3 and 5 done first
  tsk depend 7 --add 9          # 7 also needs 9
  tsk depend 7 --remove 3       # 3 no longer required
  tsk depend 7 --clear          # 7 is fully unblocked
  tsk depend 7                  # show the chain
  tsk depend 7 --tree           # recursive prereq chain, indented
  tsk depend 7 --justify        # plain-English reason chain
  tsk depend 7 --justify --json # structured reason chain for scripts
  tsk depend 7 --upstream       # what depends on #7?
  tsk depend 7 --upstream --json
  tsk depend --list --json      # CI: what's stuck?
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDependFlags(args, onCSV, addCSV, removeCSV, clear, list, tree, justify, upstream); err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			if list {
				return runDependList(cmd.OutOrStdout(), s, asJSON)
			}
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			if tree {
				return runDependTree(cmd.OutOrStdout(), s, t, asJSON)
			}
			if justify {
				return runDependJustify(cmd.OutOrStdout(), s, t, asJSON)
			}
			if upstream {
				return runDependUpstream(cmd.OutOrStdout(), s, t, asJSON)
			}
			// Inspect-mode: no mutation flags.
			if onCSV == "" && addCSV == "" && removeCSV == "" && !clear {
				return runDependInspect(cmd.OutOrStdout(), s, t, asJSON)
			}
			return runDependMutate(cmd.OutOrStdout(), s, t, onCSV, addCSV, removeCSV, clear)
		},
	}
	cmd.Flags().StringVar(&onCSV, "on", "", "set DependsOn to this comma-separated id list (replaces prior)")
	cmd.Flags().StringVar(&addCSV, "add", "", "add the given ids to DependsOn")
	cmd.Flags().StringVar(&removeCSV, "remove", "", "remove the given ids from DependsOn")
	cmd.Flags().BoolVar(&clear, "clear", false, "drop all dependencies")
	cmd.Flags().BoolVar(&list, "list", false, "list every blocked task in the store")
	cmd.Flags().BoolVar(&tree, "tree", false, "print the recursive prerequisite chain (depth-first, indented)")
	cmd.Flags().BoolVar(&justify, "justify", false, "explain why a task is blocked via a chain of reasons")
	cmd.Flags().BoolVar(&upstream, "upstream", false, "list tasks that depend on this one (reverse of --tree)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// validateDependFlags rejects nonsensical combinations up-front so the
// user sees a precise error instead of weird half-applied state.
func validateDependFlags(args []string, onCSV, addCSV, removeCSV string, clear, list, tree, justify, upstream bool) error {
	mutexCount := 0
	if onCSV != "" {
		mutexCount++
	}
	if addCSV != "" {
		mutexCount++
	}
	if removeCSV != "" {
		mutexCount++
	}
	if clear {
		mutexCount++
	}
	if mutexCount > 1 {
		return usageErrorf("--on, --add, --remove, --clear are mutually exclusive")
	}
	// Read-only modes are mutually exclusive: each answers a different
	// question, combining them would muddle the output shape.
	readOnlyCount := 0
	if tree {
		readOnlyCount++
	}
	if justify {
		readOnlyCount++
	}
	if upstream {
		readOnlyCount++
	}
	if readOnlyCount > 1 {
		return usageErrorf("--tree, --justify, --upstream are mutually exclusive (each shows a different view)")
	}
	if list {
		if len(args) > 0 {
			return usageErrorf("--list takes no positional id")
		}
		if mutexCount > 0 {
			return usageErrorf("--list can't be combined with mutation flags")
		}
		if readOnlyCount > 0 {
			return usageErrorf("--list is mutually exclusive with --tree, --justify, --upstream (use --list for the global view)")
		}
		return nil
	}
	if tree {
		if mutexCount > 0 {
			return usageErrorf("--tree is read-only — can't combine with mutation flags")
		}
		if len(args) == 0 {
			return usageErrorf("--tree requires an <id> (e.g. `tsk depend 7 --tree`)")
		}
		return nil
	}
	if justify {
		if mutexCount > 0 {
			return usageErrorf("--justify is read-only — can't combine with mutation flags")
		}
		if len(args) == 0 {
			return usageErrorf("--justify requires an <id> (e.g. `tsk depend 7 --justify`)")
		}
		return nil
	}
	if upstream {
		if mutexCount > 0 {
			return usageErrorf("--upstream is read-only — can't combine with mutation flags")
		}
		if len(args) == 0 {
			return usageErrorf("--upstream requires an <id> (e.g. `tsk depend 7 --upstream`)")
		}
		return nil
	}
	if len(args) == 0 {
		return usageErrorf("missing <id> (or pass --list)")
	}
	return nil
}

// parseDependCSV converts a comma-separated id list to a sorted, deduped
// []int. Rejects 0, negatives, and non-numeric. The leading "#" prefix
// is tolerated (so `tsk depend 7 --on #3,#5` works the same as
// `--on 3,5`).
func parseDependCSV(raw string) ([]int, error) {
	out := make([]int, 0, 4)
	seen := make(map[int]bool, 4)
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		tok = strings.TrimPrefix(tok, "#")
		if tok == "" {
			continue
		}
		n, err := strconvAtoiPos(tok)
		if err != nil || n == 0 {
			return nil, usageErrorf("invalid task id %q in dependency list", tok)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	sort.Ints(out)
	return out, nil
}

// runDependMutate handles --on / --add / --remove / --clear. Validates
// the proposed final state (no self-dep, no direct cycle, every
// referenced id exists), then saves.
func runDependMutate(w io.Writer, s *store.Store, t *model.Task,
	onCSV, addCSV, removeCSV string, clear bool,
) error {
	var proposed []int
	switch {
	case clear:
		proposed = nil
	case onCSV != "":
		ids, err := parseDependCSV(onCSV)
		if err != nil {
			return err
		}
		proposed = ids
	case addCSV != "":
		ids, err := parseDependCSV(addCSV)
		if err != nil {
			return err
		}
		proposed = mergeIntSlices(t.DependsOn, ids)
	case removeCSV != "":
		ids, err := parseDependCSV(removeCSV)
		if err != nil {
			return err
		}
		proposed = subtractIntSlices(t.DependsOn, ids)
	default:
		// Defensive — should be unreachable thanks to the caller's
		// inspect-mode short-circuit.
		return usageErrorf("no mutation requested")
	}
	if err := validateProposedDeps(s, t, proposed); err != nil {
		return err
	}
	t.DependsOn = proposed
	if err := s.Save(); err != nil {
		return err
	}
	if len(proposed) == 0 {
		pf(w, "#%d depends on nothing\n", t.ID)
		return nil
	}
	pf(w, "#%d depends on %s\n", t.ID, formatBlockerIDs(proposed))
	return nil
}

// validateProposedDeps enforces existence + no-self + no-direct-cycle.
func validateProposedDeps(s *store.Store, t *model.Task, proposed []int) error {
	for _, dep := range proposed {
		if dep == t.ID {
			return usageErrorf("#%d cannot depend on itself", t.ID)
		}
		other := s.ByID(dep)
		if other == nil {
			return usageErrorf("no task with id %d in %s", dep, s.Path)
		}
		// Direct cycle: B already lists A as a dep and we're proposing
		// A → B. Block. Deeper cycles (3-node etc.) aren't detected by
		// design — they're rare and require graph traversal that's
		// disproportionate to the value.
		for _, theirs := range other.DependsOn {
			if theirs == t.ID {
				return usageErrorf("direct cycle: #%d already depends on #%d", dep, t.ID)
			}
		}
	}
	return nil
}

// runDependInspect prints the dep list for one task plus the OPEN
// subset (the active blockers — what's actually keeping done from
// landing right now).
func runDependInspect(w io.Writer, s *store.Store, t *model.Task, asJSON bool) error {
	blockers := unmetBlockers(s, t, nil)
	if asJSON {
		doc := struct {
			ID        int   `json:"id"`
			DependsOn []int `json:"depends_on"`
			Blocking  []int `json:"blocking_open"`
		}{
			ID:        t.ID,
			DependsOn: orEmpty(t.DependsOn),
			Blocking:  orEmpty(blockers),
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}
	pf(w, "#%d  %s\n", t.ID, t.Title)
	if !t.HasDependencies() {
		pln(w, "  (no dependencies)")
		return nil
	}
	pf(w, "  depends on:  %s\n", formatBlockerIDs(t.DependsOn))
	if len(blockers) == 0 {
		pln(w, "  open blockers: (none — every prerequisite is done)")
	} else {
		pf(w, "  open blockers: %s\n", formatBlockerIDs(blockers))
	}
	return nil
}

// runDependList lists every blocked task in the store, with their
// open-blockers subset. Useful for "what's stuck on what?" reviews.
func runDependList(w io.Writer, s *store.Store, asJSON bool) error {
	type row struct {
		ID       int    `json:"id"`
		Title    string `json:"title"`
		Blockers []int  `json:"open_blockers"`
	}
	rows := make([]row, 0)
	for _, t := range s.Tasks {
		if t.Done {
			continue
		}
		blockers := unmetBlockers(s, &t, nil)
		if len(blockers) == 0 {
			continue
		}
		rows = append(rows, row{ID: t.ID, Title: t.Title, Blockers: blockers})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if rows == nil {
			rows = []row{}
		}
		return enc.Encode(rows)
	}
	if len(rows) == 0 {
		pln(w, "no blocked tasks")
		return nil
	}
	for _, r := range rows {
		pf(w, "#%d  %s  (blocked by %s)\n", r.ID, r.Title, formatBlockerIDs(r.Blockers))
	}
	return nil
}

// mergeIntSlices returns a sorted, deduped union of a and b.
func mergeIntSlices(a, b []int) []int {
	seen := make(map[int]bool, len(a)+len(b))
	out := make([]int, 0, len(a)+len(b))
	for _, x := range a {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	for _, x := range b {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Ints(out)
	return out
}

// subtractIntSlices returns a minus b, preserving a's order. Missing
// ids in b are silently ignored — removing nothing is not an error.
func subtractIntSlices(a, b []int) []int {
	drop := make(map[int]bool, len(b))
	for _, x := range b {
		drop[x] = true
	}
	out := make([]int, 0, len(a))
	for _, x := range a {
		if !drop[x] {
			out = append(out, x)
		}
	}
	return out
}

// orEmpty replaces a nil slice with a 0-length one so JSON renders [].
func orEmpty(in []int) []int {
	if in == nil {
		return []int{}
	}
	return in
}
