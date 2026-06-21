package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/store"
)

// newGraphCmd implements `tsk graph`: render the dependency graph
// across the whole store.
//
// `tsk depend <id> --tree` walks ONE prerequisite chain rooted at a
// task. `tsk graph` is the bird's-eye view: every dependency edge in
// the store, summarized in one screen.
//
// Two formats:
//
//   - default (`--format ascii`, or no flag): an indented adjacency
//     listing — one line per task that has dependencies, with the
//     ids it depends on shown after a "->" arrow. Open tasks come
//     first (the actionable ones) then a "(done)" section so the
//     graph isn't dominated by historical completed work.
//
//   - `--format dot`: GraphViz DOT source on stdout. Pipe into
//     `dot -Tpng > graph.png` or paste into https://dreampuf.github.io/GraphvizOnline
//     for a real visual. Done tasks render as filled gray nodes;
//     open tasks render as outlined boxes; blocked tasks (open with
//     at least one open prereq) get a red border so the chokepoints
//     stand out.
//
// Filters:
//
//	--open               only include open tasks AND the open deps that
//	                     actually block them (filters out the done-history
//	                     noise; the most useful default for active work)
//	--reachable <id>     only include the subgraph reachable from <id>
//	                     via DependsOn edges (the transitive prereqs of
//	                     one root + the root itself). Pairs nicely with
//	                     `tsk depend <id> --tree`: tree shows one chain
//	                     in depth-first form, --reachable shows the full
//	                     fan-in/out subgraph in DOT layout.
//
// Empty graphs (no deps anywhere) print "no dependencies" rather than
// emitting a blank DOT skeleton — both shapes are still parseable but
// the explicit message saves a "why is this empty?" diagnostic loop.
func newGraphCmd() *cobra.Command {
	var (
		format    string
		open      bool
		reachable int
	)
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Render the dependency graph (ascii or GraphViz DOT)",
		Long: `Render the whole-store dependency graph.

Two output formats:
  --format ascii     adjacency listing (default; one line per task)
  --format dot       GraphViz DOT source for piping to ` + "`dot -Tpng`" + `

Filters:
  --open             skip done tasks and edges to done prereqs
  --reachable <id>   restrict to the subgraph reachable from <id>
                     via DependsOn (transitive prereqs + root)

Use ` + "`tsk depend <id> --tree`" + ` instead if you want one branch in
depth-first form; this command is the bird's-eye view.
` + "`--reachable`" + ` is the in-between view: every transitive prereq of
one root, in the same DOT layout used for the whole-store graph.

Examples:
  tsk graph                              # quick text adjacency view
  tsk graph --open                       # only show what's still blocking
  tsk graph --reachable 7                # the subgraph rooted at #7
  tsk graph --reachable 7 --open         # …filtered to active work
  tsk graph --format dot | dot -Tpng -o deps.png
  tsk graph --format dot --reachable 7 | dot -Tsvg > sub.svg
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmtChoice, err := resolveGraphFormat(format)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			edges := collectGraphEdges(s, open)
			if reachable > 0 {
				if s.ByID(reachable) == nil {
					return fmt.Errorf("no task with id %d in %s", reachable, s.Path)
				}
				edges = filterReachableEdges(s, edges, reachable)
			}
			return emitGraph(cmd.OutOrStdout(), s, edges, fmtChoice, reachable)
		},
	}
	cmd.Flags().StringVar(&format, "format", "ascii", "output format: ascii or dot")
	cmd.Flags().BoolVar(&open, "open", false, "only include open tasks and the open deps that block them")
	cmd.Flags().IntVar(&reachable, "reachable", 0, "restrict to the subgraph reachable from this task id via DependsOn")
	return cmd
}

// graphEdge represents a single from->to dependency arrow. Sorted
// (from asc, then to asc) before rendering so output is reproducible.
type graphEdge struct {
	from int
	to   int
}

// collectGraphEdges walks every task in the store and returns the full
// edge list. With openOnly, we skip done tasks AND any edges that
// point to done tasks (the dep is already satisfied — no longer
// constraining the graph). This is the "show me what's actively
// blocking real work" mode.
//
// Dangling refs (dep id with no task in the store) are tolerated:
// the edge is emitted in the default mode so the user can see "this
// id is referenced but missing", but dropped in openOnly mode (a
// missing dep is treated as satisfied per toggle.go's semantics).
func collectGraphEdges(s *store.Store, openOnly bool) []graphEdge {
	edges := make([]graphEdge, 0)
	for _, t := range s.Tasks {
		if openOnly && t.Done {
			continue
		}
		if !t.HasDependencies() {
			continue
		}
		for _, dep := range t.DependsOn {
			if openOnly {
				bt := s.ByID(dep)
				if bt == nil || bt.Done {
					continue
				}
			}
			edges = append(edges, graphEdge{from: t.ID, to: dep})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return edges[i].from < edges[j].from
		}
		return edges[i].to < edges[j].to
	})
	return edges
}

// filterReachableEdges keeps only the edges that participate in the
// subgraph reachable from `root` via DependsOn. Algorithm:
//
//  1. BFS from `root` over the source->target edges to compute the
//     set of every transitively-reachable node (the root itself
//     plus every prereq, every prereq's prereq, etc).
//  2. Drop every edge whose source is NOT in that set.
//
// Note: this is the "downstream" reachability (where `root` is the
// source). It answers "what does #X transitively depend on?" — the
// matching question for the user typing `--reachable 7`. The reverse
// ("what transitively depends on #X?") is a separate filter we don't
// add here; users wanting that should reverse the edge direction by
// piping `tsk graph --format dot` through external tooling, or use
// `tsk path` for a one-shot lookup.
//
// Edges already sorted by collectGraphEdges; we preserve that order
// so DOT/ASCII output stays deterministic regardless of which root
// the user picks.
func filterReachableEdges(s *store.Store, edges []graphEdge, root int) []graphEdge {
	// Build outgoing adjacency from the edge list.
	out := make(map[int][]int)
	for _, e := range edges {
		out[e.from] = append(out[e.from], e.to)
	}
	// BFS to find the reachable node set.
	visited := map[int]bool{root: true}
	queue := []int{root}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, next := range out[curr] {
			if visited[next] {
				continue
			}
			visited[next] = true
			queue = append(queue, next)
		}
	}
	// Filter the edge list — keep only edges whose source is in the
	// reachable set. (Target reachability is implied: if from is
	// reachable and we kept the edge, the BFS already added the
	// target to visited.)
	filtered := make([]graphEdge, 0, len(edges))
	for _, e := range edges {
		if visited[e.from] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// resolveGraphFormat normalizes --format to the canonical lowercase
// keyword. Empty defaults to ascii. Unknown values are rejected
// up-front (usage-coded so main.go exits 2).
func resolveGraphFormat(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "ascii", "text", "txt":
		return "ascii", nil
	case "dot", "graphviz":
		return "dot", nil
	}
	return "", usageErrorf("unknown --format %q (want ascii or dot)", raw)
}

// emitGraph dispatches based on the resolved format. When reachable
// is set (>0) and the filter produced zero edges, the message is
// more specific so the user understands "the root has no prereqs"
// vs the whole store being empty.
func emitGraph(w io.Writer, s *store.Store, edges []graphEdge, format string, reachable int) error {
	if len(edges) == 0 {
		if reachable > 0 {
			pf(w, "no dependencies reachable from #%d\n", reachable)
			return nil
		}
		pln(w, "no dependencies")
		return nil
	}
	if format == "dot" {
		return printGraphDOT(w, s, edges)
	}
	return printGraphASCII(w, s, edges)
}

// printGraphASCII prints an adjacency listing — one line per source
// task with all its dep ids and a short title for the source.
//
// Layout:
//
//	#3 -> #1, #2    research the API (3 prereqs)
//	#5 -> #3        ship the feature
//
// Open tasks come first (the actionable ones); done tasks land in a
// "(done)" section so the active work isn't visually buried.
func printGraphASCII(w io.Writer, s *store.Store, edges []graphEdge) error {
	bySource := groupEdgesBySource(edges)
	openSources := make([]int, 0)
	doneSources := make([]int, 0)
	for from := range bySource {
		t := s.ByID(from)
		if t != nil && t.Done {
			doneSources = append(doneSources, from)
		} else {
			openSources = append(openSources, from)
		}
	}
	sort.Ints(openSources)
	sort.Ints(doneSources)
	for _, from := range openSources {
		printGraphRow(w, s, from, bySource[from])
	}
	if len(doneSources) > 0 {
		if len(openSources) > 0 {
			pln(w, "")
		}
		pln(w, "(done):")
		for _, from := range doneSources {
			printGraphRow(w, s, from, bySource[from])
		}
	}
	return nil
}

// groupEdgesBySource collapses a flat edge list into a map keyed by
// the source id. The dep-id slices stay sorted (edges came in sorted).
func groupEdgesBySource(edges []graphEdge) map[int][]int {
	out := make(map[int][]int)
	for _, e := range edges {
		out[e.from] = append(out[e.from], e.to)
	}
	return out
}

// printGraphRow renders one "#N -> #X, #Y  title" line. The title is
// included so the user doesn't have to cross-reference ids to know
// what the line is about.
func printGraphRow(w io.Writer, s *store.Store, from int, deps []int) {
	t := s.ByID(from)
	depStrs := make([]string, len(deps))
	for i, d := range deps {
		depStrs[i] = fmt.Sprintf("#%d", d)
	}
	title := ""
	if t != nil {
		title = "  " + t.Title
	}
	pf(w, "#%d -> %s%s\n", from, strings.Join(depStrs, ", "), title)
}

// printGraphDOT emits GraphViz DOT syntax. The directed-graph
// convention here is "A -> B means A depends on B" — i.e. the arrow
// points TOWARDS the prerequisite, matching how `tsk depend <id>
// --on X` reads ("id depends on X").
//
// Node styling:
//   - done tasks: filled gray (the dep is satisfied)
//   - open with at least one open prereq (blocked): red outline
//   - open with no open prereqs (actionable): default outline
//
// Long titles are truncated to 40 chars at the node level so the
// rendered graph stays readable.
func printGraphDOT(w io.Writer, s *store.Store, edges []graphEdge) error {
	pln(w, "digraph tsk {")
	pln(w, "  rankdir=LR;")
	pln(w, `  node [shape=box, fontname="Helvetica", fontsize=10];`)
	// Compute the set of unique node ids we'll emit (sources + targets).
	used := make(map[int]bool)
	for _, e := range edges {
		used[e.from] = true
		used[e.to] = true
	}
	ids := make([]int, 0, len(used))
	for id := range used {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	// Pre-compute blocked-set for styling (open task with at least one open prereq).
	blocked := make(map[int]bool)
	for _, e := range edges {
		from := s.ByID(e.from)
		to := s.ByID(e.to)
		if from == nil || from.Done {
			continue
		}
		if to == nil || !to.Done {
			blocked[e.from] = true
		}
	}
	for _, id := range ids {
		t := s.ByID(id)
		label := fmt.Sprintf("#%d", id)
		style := ""
		if t == nil {
			label = fmt.Sprintf("#%d (missing)", id)
			style = ` color="gray", style="dashed", fontcolor="gray"`
		} else {
			label = fmt.Sprintf("#%d %s", id, truncateForDOT(t.Title, 40))
			switch {
			case t.Done:
				style = ` style="filled", fillcolor="lightgray"`
			case blocked[id]:
				style = ` color="red"`
			}
		}
		pf(w, "  %d [label=%q%s];\n", id, label, style)
	}
	for _, e := range edges {
		pf(w, "  %d -> %d;\n", e.from, e.to)
	}
	pln(w, "}")
	return nil
}

// truncateForDOT shortens a title to max runes with an ellipsis, and
// escapes the few characters DOT's quoted-string syntax cares about.
// fmt's %q already handles \\ and \" so we only need length capping.
func truncateForDOT(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
