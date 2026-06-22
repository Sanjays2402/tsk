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
//   - `tsk depend <id> --remove-all` GLOBAL sweep: scrub <id> out of
//     every OTHER task's DependsOn. Use when <id> is going away
//     (about to be removed/merged) and you want every dependent to
//     forget about it in one shot. Pairs cleanly with `tsk rm` /
//     `tsk merge` so callers don't have to spelunk for dependents.
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
		onCSV           string
		addCSV          string
		removeCSV       string
		clear           bool
		removeAll       bool
		list            bool
		tree            bool
		justify         bool
		upstream        bool
		pending         bool
		pendingSince    string
		pendingTag      string
		pendingPriority string
		asJSON          bool
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
Remove globally:tsk depend <id> --remove-all  (scrub <id> from every other task)
Inspect one:    tsk depend <id>
Inspect tree:   tsk depend <id> --tree         (recursive prereq chain)
Justify block:  tsk depend <id> --justify      (why is this blocked?)
Upstream view:  tsk depend <id> --upstream     (what depends on me?)
Inspect all:    tsk depend --list              (every blocked task at once)
Now-unblocked:  tsk depend --pending           (tasks whose prereqs JUST closed)

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

--pending is the "now-unblocked notification queue": open tasks
whose every dependency is satisfied AND at least one of those
dependencies was recently completed (default: last 24h). It's the
"what just became actionable?" view — perfect for morning standup
("which tasks got unblocked overnight?") or after closing a batch
of prereqs ("what's freshly free?"). Tasks unblocked long ago are
excluded; pass --since to widen the window (1h, 7d, 30d, etc).
Pair with --tag to narrow the feed to one project's tag (e.g.
` + "`tsk depend --pending --tag work --since 7d`" + ` for "what's freshly
unblocked on work this week, leaving home stuff out"), and/or
with --priority to focus the feed on just the urgent/high pull
(e.g. ` + "`tsk depend --pending --priority urgent`" + ` for the
"what's freshly unblocked AND on fire?" view). --tag and
--priority compose — they intersect, not union.

Examples:
  tsk depend 7 --on 3,5         # 7 needs 3 and 5 done first
  tsk depend 7 --add 9          # 7 also needs 9
  tsk depend 7 --remove 3       # 3 no longer required
  tsk depend 7 --clear          # 7 is fully unblocked
  tsk depend 7 --remove-all     # GLOBAL: scrub #7 from every other task
  tsk depend 7                  # show the chain
  tsk depend 7 --tree           # recursive prereq chain, indented
  tsk depend 7 --justify        # plain-English reason chain
  tsk depend 7 --justify --json # structured reason chain for scripts
  tsk depend 7 --upstream       # what depends on #7?
  tsk depend 7 --upstream --json
  tsk depend --list --json      # CI: what's stuck?
  tsk depend --pending          # what just became actionable?
  tsk depend --pending --since 7d
  tsk depend --pending --tag work --since 7d  # narrow by tag
  tsk depend --pending --priority urgent      # only urgent freshly-unblocked
  tsk depend --pending --tag work --priority high  # tag AND priority
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDependFlags(args, onCSV, addCSV, removeCSV, clear, removeAll, list, tree, justify, upstream, pending); err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			if list {
				return runDependList(cmd.OutOrStdout(), s, asJSON)
			}
			if pending {
				return runDependPending(cmd.OutOrStdout(), s, pendingSince, pendingTag, pendingPriority, asJSON)
			}
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			if removeAll {
				return runDependRemoveAll(cmd.OutOrStdout(), s, id, asJSON)
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
	cmd.Flags().BoolVar(&removeAll, "remove-all", false, "GLOBAL: scrub this id from every other task's DependsOn")
	cmd.Flags().BoolVar(&list, "list", false, "list every blocked task in the store")
	cmd.Flags().BoolVar(&tree, "tree", false, "print the recursive prerequisite chain (depth-first, indented)")
	cmd.Flags().BoolVar(&justify, "justify", false, "explain why a task is blocked via a chain of reasons")
	cmd.Flags().BoolVar(&upstream, "upstream", false, "list tasks that depend on this one (reverse of --tree)")
	cmd.Flags().BoolVar(&pending, "pending", false, "list open tasks whose prereqs were recently completed")
	cmd.Flags().StringVar(&pendingSince, "since", "24h", "for --pending: how recent the unblocking completion must be (e.g. 1h, 7d)")
	cmd.Flags().StringVar(&pendingTag, "tag", "", "for --pending: restrict to tasks carrying this tag")
	cmd.Flags().StringVar(&pendingPriority, "priority", "", "for --pending: restrict to tasks at this priority (low/medium/high/urgent)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// validateDependFlags rejects nonsensical combinations up-front so the
// user sees a precise error instead of weird half-applied state.
func validateDependFlags(args []string, onCSV, addCSV, removeCSV string, clear, removeAll, list, tree, justify, upstream, pending bool) error {
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
	if removeAll {
		// --remove-all is the global sweep of <id> from every other
		// task. It needs a positional id (the id to scrub) but
		// rejects every other mutation flag and every read-only
		// flag — each one represents a different intent.
		if len(args) == 0 {
			return usageErrorf("--remove-all requires an <id> to scrub from every other task's DependsOn")
		}
		if mutexCount > 0 {
			return usageErrorf("--remove-all is mutually exclusive with --on, --add, --remove, --clear (different scopes)")
		}
		if readOnlyCount > 0 {
			return usageErrorf("--remove-all is mutually exclusive with --tree, --justify, --upstream (different intents)")
		}
		if list {
			return usageErrorf("--remove-all is mutually exclusive with --list (different shapes)")
		}
		if pending {
			return usageErrorf("--remove-all is mutually exclusive with --pending (different intents)")
		}
		return nil
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
		if pending {
			return usageErrorf("--list and --pending are mutually exclusive (each is a different global view)")
		}
		return nil
	}
	if pending {
		if len(args) > 0 {
			return usageErrorf("--pending takes no positional id (it covers every freshly-unblocked task)")
		}
		if mutexCount > 0 {
			return usageErrorf("--pending can't be combined with mutation flags")
		}
		if readOnlyCount > 0 {
			return usageErrorf("--pending is mutually exclusive with --tree, --justify, --upstream (each is a per-id view)")
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
		return usageErrorf("missing <id> (or pass --list / --pending)")
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

// runDependRemoveAll sweeps the given id out of EVERY task's
// DependsOn list. The use case: id is being removed/merged/deleted
// and you want every dependent task to forget about it in one shot
// rather than spelunking the store for dependents and editing one
// at a time.
//
// Algorithm:
//  1. Iterate every task in the store.
//  2. For each task whose DependsOn contains id, drop it (preserve
//     the order of the remaining ids — no re-sort, no dedupe; the
//     caller's other invariants are unchanged).
//  3. Save ONCE if anything changed. Skip Save on a no-op so the
//     .bak chain doesn't grow for nothing.
//  4. Report a count of touched tasks plus the ids that were
//     changed so the user can audit.
//
// Idempotent: running on an id that nothing depends on is a no-op
// with a clear message. Running on a missing id (not present in
// the store) is ALSO treated as a no-op rather than an error —
// the intent is "make sure nothing depends on this id", and a
// missing id satisfies that vacuously. (This matches `tsk rm`'s
// liberal acceptance of already-gone ids; rejecting the
// "scrub a freshly-removed task" case would force callers to
// existence-check first, defeating the ergonomic gain.)
//
// JSON shape: {"id": N, "touched": [<task ids>...]} — array (not
// object map) because the order is the iteration order of the
// store, which is already id-ascending; callers can index/count
// without re-sorting. Empty case = {"id": N, "touched": []} so
// `jq '.touched | length'` reads zero without crashing.
//
// Why a separate runner instead of folding into runDependMutate?
// The shapes are too different: runDependMutate operates on ONE
// task's list (curate per-task); --remove-all is a global SWEEP
// (curate the whole store via one id). Folding them would require
// runDependMutate to accept an "operate on every task" flag, which
// is a different mental model that would muddle the per-task code.
func runDependRemoveAll(w io.Writer, s *store.Store, id int, asJSON bool) error {
	touched := make([]int, 0)
	for i := range s.Tasks {
		t := &s.Tasks[i]
		if !t.HasDependencies() {
			continue
		}
		filtered := make([]int, 0, len(t.DependsOn))
		dropped := false
		for _, dep := range t.DependsOn {
			if dep == id {
				dropped = true
				continue
			}
			filtered = append(filtered, dep)
		}
		if dropped {
			t.DependsOn = filtered
			touched = append(touched, t.ID)
		}
	}
	if len(touched) > 0 {
		if err := s.Save(); err != nil {
			return err
		}
	}
	if asJSON {
		doc := struct {
			ID      int   `json:"id"`
			Touched []int `json:"touched"`
		}{ID: id, Touched: orEmpty(touched)}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}
	if len(touched) == 0 {
		pf(w, "no tasks depend on #%d (nothing to scrub)\n", id)
		return nil
	}
	pf(w, "scrubbed #%d from %d task(s): %s\n", id, len(touched), formatBlockerIDs(touched))
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
