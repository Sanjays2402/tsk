package commands

import (
	"fmt"
	"io"

	"github.com/Sanjays2402/tsk/internal/store"
)

// exportGraphDOT renders the store's dependency graph as GraphViz
// DOT source. It's the export-verb sibling of `tsk graph --format dot`:
// same wire format, same node styling (filled-gray for done,
// red-bordered for blocked, default for actionable, dashed-gray for
// dangling, gold-bold for highlight), but routed through `tsk export`
// so callers have ONE stable verb to pipeline data out of tsk.
//
// Why expose it twice? The two surfaces serve different mental
// models:
//
//   - `tsk graph --format dot` lives in the dependency-debugging
//     cluster (alongside `tsk depend --tree`, `tsk topo`, etc).
//     It's where you reach when you're thinking about the graph
//     itself — "what's blocked on what?"
//
//   - `tsk export --graph-dot` lives in the data-out cluster
//     (alongside `tsk export --json/--csv/--markdown`). It's where
//     you reach when you're scripting a "dump everything to disk"
//     pipeline or wiring tsk into a CI step that produces an SVG
//     artifact. The graph is just another shape of the same data.
//
// A regression test asserts byte-identical output between the two
// surfaces so they can't drift. The shared collectGraphEdges /
// filterReachableEdges / printGraphDOT helpers in graph.go do the
// heavy lifting; this function just plumbs the export flags
// through to them.
//
// Reachable validation: if --reachable points at an id with no task
// in the store, surface the same "no task with id X" error
// `tsk graph` does. Highlight validation: same shape, with the
// "--highlight: no task with id X" prefix so the user knows which
// flag was at fault when they pass both.
//
// Empty graphs (no deps anywhere, or the reachable filter excluded
// everything) emit the same "no dependencies" / "no dependencies
// reachable from #N" markers as `tsk graph` — keeps the calling
// experience identical regardless of which verb the user reached
// for.
func exportGraphDOT(w io.Writer, s *store.Store, openOnly bool, reachable int, highlight, highlightTag, dim, dimTag string) error {
	if reachable > 0 && s.ByID(reachable) == nil {
		return fmt.Errorf("no task with id %d in %s", reachable, s.Path)
	}
	highlightSet, err := parseHighlightCSV(s, highlight)
	if err != nil {
		return err
	}
	highlightSet = mergeHighlightTag(s, highlightSet, highlightTag)
	dimSet, err := parseDimCSV(s, dim)
	if err != nil {
		return err
	}
	dimSet = mergeDimTag(s, dimSet, dimTag)
	if err := rejectDimHighlightOverlap(dimSet, highlightSet); err != nil {
		return err
	}
	edges := collectGraphEdges(s, openOnly)
	if reachable > 0 {
		edges = filterReachableEdges(s, edges, reachable)
	}
	// Reuse emitGraph so the "no dependencies" / "no dependencies
	// reachable from #N" empty-state messages match `tsk graph`
	// byte-for-byte. emitGraph dispatches based on the format
	// string; pass "dot" to get DOT output.
	return emitGraph(w, s, edges, "dot", reachable, "reachable", highlightSet, dimSet)
}
