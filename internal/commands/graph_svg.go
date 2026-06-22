package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Sanjays2402/tsk/internal/store"
)

// printGraphSVG renders the dependency graph as a self-contained
// SVG document. The point is to give `tsk graph` a viewer-ready
// output without requiring GraphViz on the host — useful for CI
// containers, fresh dev boxes, or just a one-line "show me the
// graph" without installing anything.
//
// Layout: a simple layered (Sugiyama-ish) algorithm.
//   - Layer 0 holds nodes with no in-graph DependsOn (the leaf
//     prerequisites). Each subsequent layer holds nodes whose
//     deps are all assigned to earlier layers.
//   - Nodes inside a layer are ordered by id ascending for a
//     deterministic, reproducible render (id order matches how
//     `tsk ls` and the DOT renderer order things).
//   - Cycles (theoretically impossible — `tsk depend` rejects them
//     — but defensively handled here) are broken by promoting any
//     remaining nodes to the next layer regardless of unsatisfied
//     deps. The graph still renders; the offending back-edge just
//     draws as a left-going arrow.
//
// The arrow direction matches DOT: from -> to means "from depends
// on to" (the arrow head points at the prerequisite). Same logical
// convention as printGraphDOT so users moving between formats see
// the same shape.
//
// Styling mirrors the DOT renderer's intent:
//   - done tasks: light-gray fill
//   - blocked open tasks (at least one open prereq): red border
//   - actionable open tasks: plain border
//   - dim targets (in dimSet): muted gray fill + dashed border
//   - highlight targets (in highlightSet): gold fill + thick black
//     border, OVERRIDES every other style
//
// The renderer is intentionally simple — for production-quality
// graph rendering, `--format dot | dot -Tsvg` still wins (GraphViz
// has decades of layout heuristics). The embedded path is great
// for quick visual inspection in environments without GraphViz,
// and for users who want a tsk-native SVG without a binary dep.
//
// highlightSet / dimSet have the same semantics as in printGraphDOT
// (callers reject overlap up-front, so we don't double-check here).
// Empty edges produces a tiny empty SVG with a single text label,
// matching emitGraph's empty-edges contract for the other formats.
func printGraphSVG(w io.Writer, s *store.Store, edges []graphEdge, highlightSet, dimSet map[int]bool) error {
	// Collect every node id that appears (sources + targets).
	used := make(map[int]bool)
	for _, e := range edges {
		used[e.from] = true
		used[e.to] = true
	}
	// Stable id order for the deterministic layout.
	ids := make([]int, 0, len(used))
	for id := range used {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	// Build adjacency: outDeps[from] = []to (what each task depends on).
	outDeps := make(map[int][]int, len(ids))
	for _, e := range edges {
		outDeps[e.from] = append(outDeps[e.from], e.to)
	}
	for id := range outDeps {
		sort.Ints(outDeps[id])
	}

	// Compute layer per node via iterative deepening:
	//   layer 0 = no in-graph deps
	//   layer L = max(layer(deps)) + 1
	// Cycle-safe via a fallback "everything remaining gets a layer"
	// pass after a fixed number of iterations.
	layer := assignSVGLayers(ids, outDeps)

	// Group ids by layer, sorted asc by id inside each layer.
	byLayer := make(map[int][]int)
	maxLayer := 0
	for _, id := range ids {
		l := layer[id]
		byLayer[l] = append(byLayer[l], id)
		if l > maxLayer {
			maxLayer = l
		}
	}
	for l := range byLayer {
		sort.Ints(byLayer[l])
	}

	// Pre-compute blocked set the same way printGraphDOT does so
	// the SVG styling stays consistent with the DOT styling.
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

	// Compute node positions. Layers are columns (left -> right),
	// matching DOT's rankdir=LR convention. Nodes within a column
	// stack top -> bottom in id order.
	const (
		nodeW     = 220
		nodeH     = 40
		colGapX   = 80
		rowGapY   = 20
		marginX   = 30
		marginY   = 30
		textOff   = 14
		titleMax  = 24 // chars before truncate-with-ellipsis
		fontSize  = 12
	)
	pos := make(map[int][2]int, len(ids)) // id -> [x, y] of top-left corner

	// Width depends on max layer; height depends on tallest column.
	maxCol := 1
	for l := 0; l <= maxLayer; l++ {
		if n := len(byLayer[l]); n > maxCol {
			maxCol = n
		}
	}
	for l := 0; l <= maxLayer; l++ {
		col := byLayer[l]
		x := marginX + l*(nodeW+colGapX)
		// Vertically center each column in the canvas so short
		// columns sit beside their tall neighbors without
		// hugging the top.
		colHeight := len(col)*nodeH + (len(col)-1)*rowGapY
		if colHeight < 0 {
			colHeight = 0
		}
		canvasHeight := maxCol*nodeH + (maxCol-1)*rowGapY
		startY := marginY + (canvasHeight-colHeight)/2
		for i, id := range col {
			y := startY + i*(nodeH+rowGapY)
			pos[id] = [2]int{x, y}
		}
	}
	svgW := marginX*2 + (maxLayer+1)*nodeW + maxLayer*colGapX
	svgH := marginY*2 + maxCol*nodeH + (maxCol-1)*rowGapY
	if maxCol == 0 {
		// No nodes — emit a minimal SVG with a placeholder.
		fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="240" height="60" viewBox="0 0 240 60">`+"\n")
		fmt.Fprintf(w, `  <text x="10" y="35" font-family="Helvetica" font-size="14" fill="#888">no dependencies</text>`+"\n")
		fmt.Fprintf(w, `</svg>`+"\n")
		return nil
	}

	// SVG preamble + arrowhead marker defs.
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="Helvetica" font-size="%d">`+"\n", svgW, svgH, svgW, svgH, fontSize)
	fmt.Fprintln(w, `  <defs>`)
	fmt.Fprintln(w, `    <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">`)
	fmt.Fprintln(w, `      <path d="M 0 0 L 10 5 L 0 10 z" fill="#333"/>`)
	fmt.Fprintln(w, `    </marker>`)
	fmt.Fprintln(w, `  </defs>`)

	// Draw edges FIRST so nodes paint on top of arrows.
	for _, e := range edges {
		from, ok1 := pos[e.from]
		to, ok2 := pos[e.to]
		if !ok1 || !ok2 {
			continue
		}
		// from -> to: arrow head at to (the prerequisite).
		// Connect right edge of from to left edge of to when
		// to is to the right; otherwise fall back to centers.
		fx, fy := from[0]+nodeW, from[1]+nodeH/2
		tx, ty := to[0], to[1]+nodeH/2
		if to[0] < from[0] {
			// back-edge (cycle or layered-cross): connect via centers.
			fx, fy = from[0]+nodeW/2, from[1]+nodeH
			tx, ty = to[0]+nodeW/2, to[1]
		}
		fmt.Fprintf(w, `  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#444" stroke-width="1.5" marker-end="url(#arrow)"/>`+"\n", fx, fy, tx, ty)
	}

	// Draw nodes.
	for _, id := range ids {
		p, ok := pos[id]
		if !ok {
			continue
		}
		t := s.ByID(id)
		fill := "#ffffff"
		stroke := "#333333"
		strokeWidth := "1"
		dasharray := ""
		textColor := "#111111"
		labelTitle := ""
		if t == nil {
			labelTitle = "(missing)"
			fill = "#f5f5f5"
			stroke = "#999"
			dasharray = "4 2"
			textColor = "#666"
		} else {
			labelTitle = truncateForSVG(t.Title, titleMax)
			switch {
			case t.Done:
				fill = "#dddddd"
			case blocked[id]:
				stroke = "#d33"
				strokeWidth = "2"
			}
		}
		// Dim sits BETWEEN default styling and highlight — same
		// priority as in the DOT renderer.
		if dimSet[id] {
			fill = "#eeeeee"
			stroke = "#999"
			dasharray = "4 2"
			textColor = "#666"
		}
		// Highlight overrides every other style. Gold fill + bold
		// black border keeps spotlighted tasks readable regardless
		// of done/blocked state. Same intent as DOT's highlight.
		if highlightSet[id] {
			fill = "#ffd700"
			stroke = "#000000"
			strokeWidth = "2.5"
			dasharray = ""
			textColor = "#111111"
		}
		// Node rect.
		dashAttr := ""
		if dasharray != "" {
			dashAttr = fmt.Sprintf(` stroke-dasharray="%s"`, dasharray)
		}
		fmt.Fprintf(w, `  <rect x="%d" y="%d" width="%d" height="%d" rx="4" ry="4" fill="%s" stroke="%s" stroke-width="%s"%s/>`+"\n",
			p[0], p[1], nodeW, nodeH, fill, stroke, strokeWidth, dashAttr)
		// Node label: "#N  <title>" — id in bold, title in regular.
		idLabel := fmt.Sprintf("#%d", id)
		fmt.Fprintf(w, `  <text x="%d" y="%d" fill="%s"><tspan font-weight="bold">%s</tspan> %s</text>`+"\n",
			p[0]+10, p[1]+nodeH/2+textOff/3, textColor, idLabel, escapeXML(labelTitle))
	}

	fmt.Fprintln(w, `</svg>`)
	return nil
}

// assignSVGLayers performs a deterministic layering of the graph.
// A node's layer is one greater than the max layer of any of its
// in-graph DependsOn targets. Nodes with no in-graph deps land in
// layer 0.
//
// Cycle safety: the algorithm iterates a bounded number of times.
// On each pass we promote every node whose deps are all already
// assigned. After len(ids) passes, any node still unassigned (only
// possible if there's a cycle, which `tsk depend` rejects) is
// force-placed at the current max layer + 1 so the renderer never
// stalls.
func assignSVGLayers(ids []int, outDeps map[int][]int) map[int]int {
	layer := make(map[int]int, len(ids))
	assigned := make(map[int]bool, len(ids))
	for pass := 0; pass < len(ids)+1; pass++ {
		progress := false
		for _, id := range ids {
			if assigned[id] {
				continue
			}
			deps := outDeps[id]
			ready := true
			maxDep := -1
			for _, d := range deps {
				if !assigned[d] {
					ready = false
					break
				}
				if layer[d] > maxDep {
					maxDep = layer[d]
				}
			}
			if ready {
				layer[id] = maxDep + 1
				assigned[id] = true
				progress = true
			}
		}
		if !progress {
			break
		}
	}
	// Fallback for any unassigned nodes (cycle path): place them
	// one layer beyond the current max so the render finishes.
	maxL := 0
	for _, l := range layer {
		if l > maxL {
			maxL = l
		}
	}
	for _, id := range ids {
		if !assigned[id] {
			layer[id] = maxL + 1
		}
	}
	return layer
}

// truncateForSVG shortens a title to max runes with an ellipsis.
// Sibling of truncateForDOT — same intent, separate function so
// the two renderers can tune their length budgets independently
// (DOT lets GraphViz handle node sizing; SVG has fixed-width nodes
// so we truncate tighter).
func truncateForSVG(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// escapeXML escapes the five XML-reserved characters so a task
// title containing &, <, >, ', or " renders correctly inside SVG
// text content. The set is the strict XML 1.0 subset (no need for
// the HTML named-entity zoo here — SVG is XML).
func escapeXML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
