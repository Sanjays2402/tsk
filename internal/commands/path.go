package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/store"
)

// newPathCmd implements `tsk path <a> <b>`: find the shortest
// dependency path from task A to task B in the DependsOn graph.
//
// Semantics: "A depends on B" reads as "A → B" (matching
// `tsk depend --on`). So `tsk path 5 1` finds a chain like
// 5 → 3 → 1 that says "to finish 5, you also need 3 and 1
// done first". Useful for two questions:
//
//  1. "Why is #5 blocked by #1?" (#1 might be a deep transitive
//     prereq the user has forgotten about).
//  2. "Is #5 actually waiting on #1?" — the absence of a path is
//     a hard "no" answer.
//
// Algorithm: BFS over the directed DependsOn graph, expanding
// from A. Returns the FIRST path found (BFS guarantees shortest).
// O(V+E) — fast even on large stores. When no path exists, exit 1
// with a clear message; --json emits `{"found": false}` so scripts
// can branch programmatically.
//
// Cycle safety: BFS naturally avoids re-visiting nodes via the
// visited-set, so a hand-edited cycle in the file (which the
// writer doesn't catch for 3+ nodes) can't loop the search.
//
// Direction: by default search A → B (A depends on B transitively).
// To find paths in the reverse direction ("everything that
// depends on #1"), use `tsk graph --reachable`. For "are these two
// related at all?", pass --any-direction: the BFS treats DependsOn
// edges as UNDIRECTED, finding the shortest dependency chain in
// either direction. The reported path is still ordered A → B in
// the output (BFS reconstructs back from B), but the chain may
// include edges that the writer originally laid down in the other
// direction.
func newPathCmd() *cobra.Command {
	var (
		asJSON       bool
		anyDirection bool
	)
	cmd := &cobra.Command{
		Use:   "path <from-id> <to-id>",
		Short: "Find the shortest dependency path between two tasks",
		Long: `Find the shortest dependency path from <from-id> to <to-id>.

"A depends on B" reads as "A → B" (matching ` + "`tsk depend --on`" + `).
So ` + "`tsk path 5 1`" + ` finds a chain like 5 → 3 → 1 — "to finish 5,
you also need 3 and 1 done first".

Returns the FIRST path found (BFS guarantees shortest by edge
count). Exit code 1 + a clear message when no path exists, so
scripts can branch on success/failure cleanly.

By default the search is one-directional (` + "`a -> b`" + ` only). Pass
--any-direction to treat dependency edges as UNDIRECTED, which
answers the related question "are these two tasks related at all?"
even when the chain runs B → A or zig-zags. The result is still
printed as a path from <from-id> to <to-id>.

Use ` + "`tsk graph --reachable <id>`" + ` for the broader "everything
that depends on #X" question; ` + "`tsk path`" + ` is one-to-one only.

Examples:
  tsk path 5 1                       # how does 5 depend on 1?
  tsk path 5 1 --json                # {"found": true, "path": [5, 3, 1]}
  tsk path 5 99 || echo "free"       # exit 1 when no path exists
  tsk path 5 7 --any-direction       # are these two related at all?
  tsk path 5 7 --any-direction --json
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			from, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			to, err := parseSingleID(args[1])
			if err != nil {
				return err
			}
			if from == to {
				return usageErrorf("<from-id> and <to-id> are the same (#%d)", from)
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			if s.ByID(from) == nil {
				return fmt.Errorf("no task with id %d in %s", from, s.Path)
			}
			if s.ByID(to) == nil {
				return fmt.Errorf("no task with id %d in %s", to, s.Path)
			}
			var path []int
			if anyDirection {
				path = findDepPathUndirected(s, from, to)
			} else {
				path = findDepPath(s, from, to)
			}
			return emitPath(cmd.OutOrStdout(), s, from, to, path, asJSON, anyDirection)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().BoolVar(&anyDirection, "any-direction", false, "treat dependency edges as undirected (find any path)")
	return cmd
}

// findDepPath runs BFS over DependsOn edges from `from` toward
// `to` and returns the FIRST path found, or nil if none exists.
//
// Implementation note: we keep a parent map keyed by child id
// (parent[c] = the node that discovered c). When we reach `to`,
// we walk parent backward to reconstruct the path, then reverse.
// This avoids storing full path slices in the queue (O(V) memory
// instead of O(V²) worst case).
//
// Iteration order over t.DependsOn is fixed (sorted ascending)
// so the same input always yields the same path — important for
// tests and human-readable diffs.
func findDepPath(s *store.Store, from, to int) []int {
	if from == to {
		// Same-node case is rejected at the CLI layer; this is
		// defensive in case future callers reach the function
		// directly.
		return []int{from}
	}
	visited := map[int]bool{from: true}
	// parent map: child → discoverer. The from-node has no parent
	// (-1 sentinel).
	parent := map[int]int{from: -1}
	queue := []int{from}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		t := s.ByID(curr)
		if t == nil {
			// Dangling — should be filtered at enqueue time but
			// defensive here too.
			continue
		}
		deps := append([]int(nil), t.DependsOn...)
		sort.Ints(deps)
		for _, dep := range deps {
			if visited[dep] {
				continue
			}
			visited[dep] = true
			parent[dep] = curr
			if dep == to {
				return reconstructPath(parent, from, to)
			}
			// Skip enqueueing dangling refs — a missing task can't
			// have further dependencies to traverse, and treating
			// it as a terminal "visited" node above is sufficient.
			if s.ByID(dep) == nil {
				continue
			}
			queue = append(queue, dep)
		}
	}
	return nil
}

// reconstructPath walks the parent map backward from `to` to
// `from`, then reverses to produce a from → to ordered slice.
func reconstructPath(parent map[int]int, from, to int) []int {
	rev := []int{to}
	curr := to
	for curr != from {
		p, ok := parent[curr]
		if !ok || p == -1 {
			// Defensive — shouldn't happen if BFS found `to`.
			break
		}
		rev = append(rev, p)
		curr = p
	}
	out := make([]int, len(rev))
	for i, v := range rev {
		out[len(rev)-1-i] = v
	}
	return out
}

// findDepPathUndirected is the --any-direction sibling of
// findDepPath. Same BFS shape, same parent-reconstruction trick;
// the difference is the neighbour set, which includes BOTH:
//   - forward edges: every dep in t.DependsOn (t depends on dep)
//   - reverse edges: every other task whose DependsOn includes t
//
// Use case: "are these two tasks related at all?" The directed
// search misses related-via-reverse pairs (B → A when you asked
// A → B). The undirected search treats the dependency graph as a
// connectivity graph: any chain of edges in either direction
// connects them.
//
// Implementation note: the reverse-adjacency map is built once
// up-front from the full store (O(V+E)), then queried per pop.
// This avoids per-pop linear scans of s.Tasks (which would make
// the BFS O(V²+VE)). Memory cost is bounded by the edge count.
//
// Iteration order inside both directions is sorted ascending so
// the resulting path is reproducible (matches the directed
// search's contract).
func findDepPathUndirected(s *store.Store, from, to int) []int {
	if from == to {
		return []int{from}
	}
	// Build the reverse adjacency once: for each dep target id,
	// the list of tasks that name it as a prereq.
	reverse := make(map[int][]int)
	for _, t := range s.Tasks {
		for _, dep := range t.DependsOn {
			reverse[dep] = append(reverse[dep], t.ID)
		}
	}
	// Pre-sort each reverse list so traversal order is
	// deterministic.
	for k := range reverse {
		sort.Ints(reverse[k])
	}
	visited := map[int]bool{from: true}
	parent := map[int]int{from: -1}
	queue := []int{from}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		// Gather neighbours from both directions, then de-dup and
		// sort so the choice between siblings is deterministic.
		// Order matters here only for tie-breaks; correctness
		// (shortest path) is BFS-guaranteed regardless.
		neighbours := make([]int, 0)
		seen := make(map[int]bool)
		if t := s.ByID(curr); t != nil {
			for _, dep := range t.DependsOn {
				if !seen[dep] {
					seen[dep] = true
					neighbours = append(neighbours, dep)
				}
			}
		}
		for _, rev := range reverse[curr] {
			if !seen[rev] {
				seen[rev] = true
				neighbours = append(neighbours, rev)
			}
		}
		sort.Ints(neighbours)
		for _, n := range neighbours {
			if visited[n] {
				continue
			}
			visited[n] = true
			parent[n] = curr
			if n == to {
				return reconstructPath(parent, from, to)
			}
			// Skip enqueueing dangling refs — a missing task can't
			// have further dependencies to traverse, and the
			// reverse adjacency map already accounts for the
			// inbound side.
			if s.ByID(n) == nil {
				continue
			}
			queue = append(queue, n)
		}
	}
	return nil
}

// pathJSON is the structured shape returned by `tsk path --json`.
// `found` is the primary branch flag — `path` is empty when not
// found, populated (length >= 2) when found.
type pathJSON struct {
	From      int               `json:"from"`
	To        int               `json:"to"`
	Found     bool              `json:"found"`
	Path      []int             `json:"path"`
	Hops      int               `json:"hops"`
	Titles    map[string]string `json:"titles,omitempty"`
	Direction string            `json:"direction"`
}

// emitPath dispatches plain vs JSON output. Plain prints a one-line
// chain like "#5 -> #3 -> #1" with the task titles inline; JSON
// emits the structured pathJSON. Not-found exits 1 (silent in plain,
// found=false in JSON) so caller scripts can `tsk path A B || …`.
//
// The `anyDirection` flag is reported in JSON output (so consumers
// know whether the result came from the directed search or the
// undirected one) and adjusts the plain-text "no path" message so
// the user knows widening to --any-direction isn't going to help.
func emitPath(w io.Writer, s *store.Store, from, to int, path []int, asJSON, anyDirection bool) error {
	direction := "directed"
	if anyDirection {
		direction = "any"
	}
	if asJSON {
		doc := pathJSON{
			From:      from,
			To:        to,
			Found:     len(path) > 0,
			Path:      []int{},
			Direction: direction,
		}
		if len(path) > 0 {
			doc.Path = path
			doc.Hops = len(path) - 1
			doc.Titles = make(map[string]string, len(path))
			for _, id := range path {
				if t := s.ByID(id); t != nil {
					doc.Titles[fmt.Sprintf("%d", id)] = t.Title
				}
			}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(doc); err != nil {
			return err
		}
		if len(path) == 0 {
			// Still exit 1 for script-friendliness, even though
			// the JSON document is fully populated with found=false.
			// SilentExit so main.go doesn't print "error: " (the
			// JSON IS the error signal, and main's prefix would
			// pollute pipelines).
			return silentExit{code: 1}
		}
		return nil
	}
	if len(path) == 0 {
		if anyDirection {
			pf(w, "no dependency path from #%d to #%d (even with --any-direction)\n", from, to)
		} else {
			pf(w, "no dependency path from #%d to #%d (try --any-direction)\n", from, to)
		}
		return silentExit{code: 1}
	}
	// Render: "#5 ship -> #3 build -> #1 design"
	parts := make([]string, len(path))
	for i, id := range path {
		title := ""
		if t := s.ByID(id); t != nil {
			title = " " + t.Title
		}
		parts[i] = fmt.Sprintf("#%d%s", id, title)
	}
	pln(w, strings.Join(parts, "  ->  "))
	if len(path) > 2 {
		pf(w, "(%d hops)\n", len(path)-1)
	}
	return nil
}
