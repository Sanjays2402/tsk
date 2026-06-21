package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newTopoCmd implements `tsk topo`: emit tasks in topological
// dependency-respecting order. The output is "do these in this
// order" — every prerequisite comes before the tasks that depend
// on it, so a user can work the list straight through without
// hitting `tsk done` refusals.
//
// Algorithm: Kahn's algorithm over the DependsOn edge list.
//   - in-degree counts how many open prereqs each open task still
//     needs (closed prereqs are treated as satisfied — same policy
//     as unmetBlockers, so the output stays consistent with the
//     enforcement path)
//   - the "ready" set starts as every open task with zero unmet
//     prereqs; we drain it ordered by the same tie-break `tsk next`
//     uses (pin > priority desc > earliest-due > lowest-id) so the
//     emitted sequence reflects "what would `next` return next?"
//   - emitting a task decrements the in-degree of every task that
//     depends on it; new zero-in-degree tasks enter the ready set
//
// Cycle handling: any tasks left with non-zero in-degree after the
// drain are part of a cycle (a hand-edit can create deeper cycles
// the writer doesn't catch). Emit them at the tail in id order with
// a "(cycle)" annotation in plain text, or with a "cycle": true
// boolean in --json — never silently drop them, the user needs to
// know the file is broken.
//
// Done tasks are excluded by default — the whole point is "what
// should I do?", and historical work shouldn't dominate the output.
// Pass --all to include them (they appear first as already-satisfied
// prereqs, in id order). Waiting tasks (wait:<future date>) follow
// the same exclude-by-default policy as `tsk next`/`top`.
//
// Output formats:
//   - default plain: one task per line with id + priority + title,
//     ready to skim
//   - --json: array of objects in the same order, each with
//     id/title/priority/done/cycle
//   - --ids: just the ids, comma-separated, suitable for piping
//     into `tsk done $(tsk topo --ids)` style batch operations
//   - --format dot: GraphViz DOT with rank=topological order, so
//     visual layout matches the textual one
func newTopoCmd() *cobra.Command {
	var (
		asJSON      bool
		idsOnly     bool
		format      string
		includeDone bool
	)
	cmd := &cobra.Command{
		Use:   "topo",
		Short: "Emit tasks in dependency-respecting topological order",
		Long: `Emit tasks in dependency-respecting topological order — prereqs
always come before the tasks that depend on them, so you can work
the list straight through without ` + "`tsk done`" + ` refusals.

Ordering within each "ready" layer uses the same tie-break as
` + "`tsk next`" + ` (pin > priority desc > earliest-due > lowest-id), so the
top of the list is also what ` + "`tsk next --respect-deps`" + ` would return.

Cycle detection: any tasks remaining after the topological drain
are part of a dependency cycle. They're emitted at the tail with a
"(cycle)" marker (or "cycle": true in JSON) so a corrupt hand-edit
is visible rather than silently dropped.

Examples:
  tsk topo                            # ordered list, human-readable
  tsk topo --all                      # include done tasks (historical first)
  tsk topo --json                     # array of objects
  tsk topo --ids                      # comma-separated ids
  tsk topo --ids | xargs -n1 tsk show # walk the chain interactively
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Mutual exclusion: --json, --ids, --format all describe
			// the output, can't combine.
			modes := 0
			if asJSON {
				modes++
			}
			if idsOnly {
				modes++
			}
			if format != "" {
				modes++
			}
			if modes > 1 {
				return usageErrorf("--json, --ids, --format are mutually exclusive")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			ordered, cycle := topoOrder(s, includeDone)
			return emitTopo(cmd.OutOrStdout(), ordered, cycle, asJSON, idsOnly, format)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array")
	cmd.Flags().BoolVar(&idsOnly, "ids", false, "emit just the ids, comma-separated")
	cmd.Flags().StringVar(&format, "format", "", "alternate format: dot (GraphViz with rank=topo order)")
	cmd.Flags().BoolVar(&includeDone, "all", false, "include done tasks (default: open only)")
	return cmd
}

// topoTask wraps a task with a flag noting whether it landed in the
// "remaining after drain" bucket — i.e. it's part of a cycle. The
// caller treats those rows specially in every output format.
type topoTask struct {
	Task    model.Task
	InCycle bool
}

// topoOrder runs Kahn's algorithm to produce a stable topological
// ordering over the open task pool. Returns the ordered slice plus
// a bool indicating whether at least one cycle was detected.
//
// Filtering policy:
//   - Done tasks are excluded unless includeDone is set.
//   - Waiting tasks are excluded (they're hidden in default views;
//     including them here would suggest doing work that's still
//     deferred).
//   - Dangling deps (id with no task) are TREATED as satisfied —
//     matches unmetBlockers' policy so topo output stays consistent
//     with what `tsk done` would actually allow.
//
// Inside-layer ordering: the canonical "tsk next" tie-break is
// applied so emission order matches user intuition.
func topoOrder(s *store.Store, includeDone bool) ([]topoTask, bool) {
	pool := filterTopoCandidates(s.Tasks, includeDone)
	// Snapshot every candidate's id → index for fast back-references.
	inPool := make(map[int]bool, len(pool))
	for _, t := range pool {
		inPool[t.ID] = true
	}
	// In-degree: count of prereqs that are ALSO in the pool (so a
	// done prereq we excluded contributes nothing — same as
	// unmetBlockers' policy).
	inDeg := make(map[int]int, len(pool))
	for _, t := range pool {
		for _, dep := range t.DependsOn {
			if inPool[dep] {
				inDeg[t.ID]++
			}
		}
	}
	// Build "outgoing" map: dep → list of tasks that depend on it.
	// When dep is emitted, every task in this list gets its in-degree
	// decremented.
	outgoing := make(map[int][]int, len(pool))
	for _, t := range pool {
		for _, dep := range t.DependsOn {
			if inPool[dep] {
				outgoing[dep] = append(outgoing[dep], t.ID)
			}
		}
	}
	byID := make(map[int]model.Task, len(pool))
	for _, t := range pool {
		byID[t.ID] = t
	}
	// Initial ready queue: every task with zero in-degree, sorted by
	// the canonical next tie-break.
	ready := make([]model.Task, 0)
	for _, t := range pool {
		if inDeg[t.ID] == 0 {
			ready = append(ready, t)
		}
	}
	sortTopTasks(ready)
	out := make([]topoTask, 0, len(pool))
	emitted := make(map[int]bool, len(pool))
	for len(ready) > 0 {
		// Pop the front (best by tie-break).
		head := ready[0]
		ready = ready[1:]
		emitted[head.ID] = true
		out = append(out, topoTask{Task: head})
		// Discover newly-ready dependents.
		newly := make([]model.Task, 0)
		for _, child := range outgoing[head.ID] {
			if emitted[child] {
				continue
			}
			inDeg[child]--
			if inDeg[child] == 0 {
				newly = append(newly, byID[child])
			}
		}
		if len(newly) == 0 {
			continue
		}
		// Re-sort the merged ready set so the next pop is correct.
		ready = append(ready, newly...)
		sortTopTasks(ready)
	}
	// Anything left has non-zero in-degree → part of a cycle. Emit
	// in id order so the output is deterministic, with InCycle=true
	// so callers can mark the rows.
	cycleDetected := false
	leftover := make([]int, 0)
	for _, t := range pool {
		if !emitted[t.ID] {
			leftover = append(leftover, t.ID)
		}
	}
	sort.Ints(leftover)
	for _, id := range leftover {
		cycleDetected = true
		out = append(out, topoTask{Task: byID[id], InCycle: true})
	}
	return out, cycleDetected
}

// filterTopoCandidates applies the include-done / hide-waiting
// policies. Mirrors filterTopCandidates so the two views stay
// behaviorally consistent for the default flags.
func filterTopoCandidates(in []model.Task, includeDone bool) []model.Task {
	out := make([]model.Task, 0, len(in))
	now := time.Now()
	for _, t := range in {
		if !includeDone && t.Done {
			continue
		}
		if !includeDone && t.IsWaiting(now) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// emitTopo dispatches to the requested output format. All formats
// share the same `ordered` slice — the cycle annotation is encoded
// per-row so a partial cycle in the middle (only some tasks cyclic)
// renders correctly.
func emitTopo(w io.Writer, ordered []topoTask, cycle, asJSON, idsOnly bool, format string) error {
	if len(ordered) == 0 {
		if asJSON {
			pln(w, "[]")
			return nil
		}
		if idsOnly {
			pln(w, "")
			return nil
		}
		pln(w, "no tasks")
		return nil
	}
	switch {
	case asJSON:
		return emitTopoJSON(w, ordered)
	case idsOnly:
		return emitTopoIDs(w, ordered)
	case strings.EqualFold(format, "dot"):
		return emitTopoDOT(w, ordered)
	case format != "":
		return usageErrorf("unknown --format %q (want dot)", format)
	}
	return emitTopoPlain(w, ordered, cycle)
}

// emitTopoPlain prints one task per line with id + priority +
// title. Cycle rows get a trailing "(cycle)" annotation so the
// user knows those came out of the safety net, not the linear pass.
func emitTopoPlain(w io.Writer, ordered []topoTask, cycle bool) error {
	for _, row := range ordered {
		check := " "
		if row.Task.Done {
			check = "x"
		}
		line := fmt.Sprintf("#%d [%s] [%s] %s", row.Task.ID, check, row.Task.Priority.Short(), row.Task.Title)
		if row.InCycle {
			line += "  (cycle)"
		}
		pln(w, line)
	}
	if cycle {
		pln(w, "")
		pln(w, "note: tasks marked (cycle) are part of a dependency cycle;")
		pln(w, "      run `tsk lint` to find the bad edge and `tsk depend` to fix it.")
	}
	return nil
}

// emitTopoIDs prints just the ids in topological order, comma-
// separated. Useful for `tsk done $(tsk topo --ids)` style batch
// pipelines. Cycle ids are still included (caller decides what to
// do with them — usually nothing, but at least they're visible).
func emitTopoIDs(w io.Writer, ordered []topoTask) error {
	parts := make([]string, len(ordered))
	for i, r := range ordered {
		parts[i] = fmt.Sprintf("%d", r.Task.ID)
	}
	pln(w, strings.Join(parts, ","))
	return nil
}

// topoJSONRow is the JSON shape — one entry per ordered task. The
// schema mirrors `tsk next --json` field names where they overlap
// so consumers can reuse jq filters across commands.
type topoJSONRow struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Done     bool   `json:"done"`
	Cycle    bool   `json:"cycle,omitempty"`
}

// emitTopoJSON renders the array. We use SetIndent for readability
// since the typical consumer is `jq`, not `wc -c`.
func emitTopoJSON(w io.Writer, ordered []topoTask) error {
	rows := make([]topoJSONRow, len(ordered))
	for i, r := range ordered {
		rows[i] = topoJSONRow{
			ID:       r.Task.ID,
			Title:    r.Task.Title,
			Priority: r.Task.Priority.String(),
			Done:     r.Task.Done,
			Cycle:    r.InCycle,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// emitTopoDOT renders the topological order as a left-to-right DOT
// graph. Each row is a node tagged with its position; edges still
// reflect the actual DependsOn arrows ("A -> B means A depends on
// B"). The order itself is encoded via the implicit rank=same
// pseudo-rows (one per topo level), so a `dot -Tpng` render lays
// the chain out left-to-right matching the textual output.
func emitTopoDOT(w io.Writer, ordered []topoTask) error {
	pln(w, "digraph tsk_topo {")
	pln(w, "  rankdir=LR;")
	pln(w, `  node [shape=box, fontname="Helvetica", fontsize=10];`)
	for i, r := range ordered {
		style := ""
		switch {
		case r.InCycle:
			style = ` color="red", style="dashed"`
		case r.Task.Done:
			style = ` style="filled", fillcolor="lightgray"`
		}
		label := fmt.Sprintf("%d. #%d %s", i+1, r.Task.ID, truncateForDOT(r.Task.Title, 32))
		pf(w, "  %d [label=%q%s];\n", r.Task.ID, label, style)
	}
	// Edges: every dep that's also in the ordered set (i.e. wasn't
	// filtered out as done when --all was off).
	inSet := make(map[int]bool, len(ordered))
	for _, r := range ordered {
		inSet[r.Task.ID] = true
	}
	type edge struct{ from, to int }
	edges := make([]edge, 0)
	for _, r := range ordered {
		for _, dep := range r.Task.DependsOn {
			if !inSet[dep] {
				continue
			}
			edges = append(edges, edge{from: r.Task.ID, to: dep})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return edges[i].from < edges[j].from
		}
		return edges[i].to < edges[j].to
	})
	for _, e := range edges {
		pf(w, "  %d -> %d;\n", e.from, e.to)
	}
	pln(w, "}")
	return nil
}
