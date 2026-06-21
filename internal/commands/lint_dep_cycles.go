package commands

import (
	"fmt"
	"sort"

	"github.com/Sanjays2402/tsk/internal/store"
)

// appendDepCycleFindings scans the DependsOn graph for cycles of 3+
// nodes (2-cycles and self-cycles are rejected at write time by
// validateProposedDeps, so they shouldn't appear here — but we don't
// special-case them either; the SCC search finds whatever's there).
//
// Algorithm: Tarjan's strongly-connected-components. Every SCC with
// more than one node is a cycle; the cycle ids are the SCC's nodes
// in topological-discovery order, which gives a readable chain.
// (A pure DFS-cycle search would also work but produces non-canonical
// chains depending on visit order; SCCs give us a stable surface.)
//
// Each finding describes one cycle:
//
//	check  = "dependency_cycle"
//	detail = "cycle: #1 -> #2 -> #3 -> #1 (break any edge to resolve)"
//
// We don't try to be clever about WHICH edge to break — the user
// knows their domain better than we do. The footer just nudges them
// toward `tsk depend <id> --remove`.
//
// Filtering: dangling deps (id with no task) are skipped, matching
// unmetBlockers' tolerant policy. A done task is still considered a
// participant — a cycle that includes done tasks is still a cycle
// in the file even if the runtime never trips on it; surfacing it
// lets the user clean up.
func appendDepCycleFindings(r *LintReport, path string) {
	s, err := store.Load(path)
	if err != nil {
		// runLint already returned the parse error if there was one;
		// if we got here the file parses. If a second load fails it's
		// transient/IO — record a finding so the user sees something.
		r.Findings = append(r.Findings, LintFinding{
			Check:  "dependency_cycle_scan_error",
			Detail: fmt.Sprintf("could not reload store for cycle scan: %v", err),
		})
		return
	}
	cycles := findDepCycles(s)
	for _, cyc := range cycles {
		r.Findings = append(r.Findings, LintFinding{
			Check: "dependency_cycle",
			Detail: fmt.Sprintf("cycle: %s (break any edge to resolve, e.g. `tsk depend %d --remove %d`)",
				formatCycleChain(cyc), cyc[0], cyc[1%len(cyc)]),
		})
	}
}

// findDepCycles returns every >=3-node cycle in the DependsOn graph
// as a list of id slices. Each slice is sorted to start at its
// smallest id (canonical rotation) so the output is reproducible
// regardless of DFS visit order, and the slice list itself is sorted
// by first id ascending.
//
// Self-loops and 2-cycles are SKIPPED — those are caught at write
// time by validateProposedDeps. This scan exists to find what the
// writer can't catch (3+ node cycles forming via independent edits).
func findDepCycles(s *store.Store) [][]int {
	// Build adjacency: id → outgoing edges (DependsOn). Skip dangling
	// edges so we don't try to traverse missing nodes.
	exists := make(map[int]bool, len(s.Tasks))
	for _, t := range s.Tasks {
		exists[t.ID] = true
	}
	adj := make(map[int][]int, len(s.Tasks))
	for _, t := range s.Tasks {
		var outs []int
		for _, dep := range t.DependsOn {
			if dep == t.ID {
				continue // self-loop (rejected at write; skip defensively)
			}
			if !exists[dep] {
				continue // dangling
			}
			outs = append(outs, dep)
		}
		if len(outs) > 0 {
			sort.Ints(outs)
			adj[t.ID] = outs
		}
	}
	// Stable iteration order: process ids ascending so Tarjan's
	// discovery order is reproducible across runs.
	ids := make([]int, 0, len(s.Tasks))
	for _, t := range s.Tasks {
		ids = append(ids, t.ID)
	}
	sort.Ints(ids)

	// Tarjan state.
	idx := 0
	indexOf := make(map[int]int, len(ids))
	lowlink := make(map[int]int, len(ids))
	onStack := make(map[int]bool, len(ids))
	stack := make([]int, 0, len(ids))
	var sccs [][]int

	var strongconnect func(v int)
	strongconnect = func(v int) {
		indexOf[v] = idx
		lowlink[v] = idx
		idx++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range adj[v] {
			if _, seen := indexOf[w]; !seen {
				strongconnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] {
				if indexOf[w] < lowlink[v] {
					lowlink[v] = indexOf[w]
				}
			}
		}
		// Root of an SCC; pop everything down to v.
		if lowlink[v] == indexOf[v] {
			comp := make([]int, 0)
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			// SCC with >=2 nodes is a cycle. We only surface >=3
			// because 2-cycles are blocked at write time and would
			// be noise (the writer guarantees they won't land via
			// the CLI; if one slipped in by hand-edit, the existing
			// direct-cycle policy is the user's hint).
			if len(comp) >= 3 {
				sccs = append(sccs, comp)
			}
		}
	}
	for _, v := range ids {
		if _, seen := indexOf[v]; !seen {
			strongconnect(v)
		}
	}

	// Canonicalize each cycle: rotate to start at the smallest id,
	// keeping the directed traversal intact. SCC nodes from Tarjan
	// come out in reverse-postorder of the popping which is NOT the
	// cycle traversal order, so we rebuild the cycle by following
	// edges from the chosen start.
	out := make([][]int, 0, len(sccs))
	for _, comp := range sccs {
		// Find the smallest id as the cycle anchor.
		start := comp[0]
		for _, n := range comp {
			if n < start {
				start = n
			}
		}
		// Reconstruct the directed cycle by following any edge inside
		// the SCC starting from `start`. Each step takes the smallest
		// in-SCC neighbour so two runs against the same input produce
		// the same chain.
		inSCC := make(map[int]bool, len(comp))
		for _, n := range comp {
			inSCC[n] = true
		}
		chain := []int{start}
		visited := map[int]bool{start: true}
		cur := start
		for len(chain) < len(comp) {
			var next int
			found := false
			for _, n := range adj[cur] {
				if inSCC[n] && !visited[n] {
					next = n
					found = true
					break
				}
			}
			if !found {
				// Shouldn't happen for a true SCC, but stay safe.
				break
			}
			chain = append(chain, next)
			visited[next] = true
			cur = next
		}
		out = append(out, chain)
	}
	// Stable: sort cycles by first id ascending so consumers can rely
	// on the output order.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i][0] < out[j][0]
	})
	return out
}

// formatCycleChain renders a cycle as "#1 -> #2 -> #3 -> #1" — the
// trailing arrow back to the start is what makes it visibly a cycle.
func formatCycleChain(ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	out := fmt.Sprintf("#%d", ids[0])
	for _, n := range ids[1:] {
		out += fmt.Sprintf(" -> #%d", n)
	}
	out += fmt.Sprintf(" -> #%d", ids[0])
	return out
}
