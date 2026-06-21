package commands

import (
	"encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// runDependTree renders the recursive prerequisite chain rooted at t as
// an indented, depth-first ASCII tree. Each node shows id, title, done
// state, and (when relevant) why it isn't a leaf.
//
// Cycle handling: while `tsk depend --on` rejects self-deps and direct
// A↔B cycles at write time, deeper cycles (A→B→C→A) are NOT rejected
// because detecting them requires a graph traversal that we'd skip on
// the write path anyway. The tree renderer DOES need cycle protection
// — without it a corrupt hand-edit would loop forever. Visit-set
// guarded recursion: when we re-enter an already-seen id on the
// current branch, mark the node "(cycle)" and stop descending.
//
// JSON shape: a nested object with id/title/done/dependencies; the
// dependencies list is empty for leaves. Sorted by id so output is
// deterministic regardless of DependsOn slice ordering. A "(cycle)"
// branch emits {"id":N, "cycle":true} so consumers can short-circuit.
//
// Indent style: two spaces per level, prefixed with "└─ " for depth>0
// to make the structure scannable in a 80-col terminal. The root is
// flush-left without a connector so it's visually distinct.
func runDependTree(w io.Writer, s *store.Store, root *model.Task, asJSON bool) error {
	if asJSON {
		node := buildDependTreeNode(s, root, make(map[int]bool))
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(node)
	}
	printDependTreeText(w, s, root, 0, make(map[int]bool))
	return nil
}

// dependTreeJSON is the recursive JSON shape for --tree --json. The
// `Cycle` field is set true for nodes whose subtree was pruned because
// we're already inside their visit chain — the consumer knows there's
// a real task with that id, but its dependencies aren't repeated here.
type dependTreeJSON struct {
	ID           int              `json:"id"`
	Title        string           `json:"title,omitempty"`
	Done         bool             `json:"done"`
	Cycle        bool             `json:"cycle,omitempty"`
	Missing      bool             `json:"missing,omitempty"`
	Dependencies []dependTreeJSON `json:"dependencies"`
}

// buildDependTreeNode constructs the JSON shape recursively. The visit
// set is mutated on entry and rolled back on exit so siblings (and
// later sub-trees) can re-visit the same id legally — what we're
// protecting against is reaching the same id on the CURRENT path.
func buildDependTreeNode(s *store.Store, t *model.Task, visiting map[int]bool) dependTreeJSON {
	node := dependTreeJSON{
		ID:           t.ID,
		Title:        t.Title,
		Done:         t.Done,
		Dependencies: []dependTreeJSON{},
	}
	if !t.HasDependencies() {
		return node
	}
	visiting[t.ID] = true
	defer delete(visiting, t.ID)
	// Sort deps by id for deterministic output.
	ids := append([]int(nil), t.DependsOn...)
	sort.Ints(ids)
	for _, dep := range ids {
		if visiting[dep] {
			node.Dependencies = append(node.Dependencies, dependTreeJSON{
				ID:           dep,
				Cycle:        true,
				Dependencies: []dependTreeJSON{},
			})
			continue
		}
		child := s.ByID(dep)
		if child == nil {
			node.Dependencies = append(node.Dependencies, dependTreeJSON{
				ID:           dep,
				Missing:      true,
				Dependencies: []dependTreeJSON{},
			})
			continue
		}
		node.Dependencies = append(node.Dependencies, buildDependTreeNode(s, child, visiting))
	}
	return node
}

// printDependTreeText emits the indented text rendering. We do NOT
// share buildDependTreeNode here because the text path wants to emit
// each line as it walks (cheap, streaming) rather than build a full
// JSON tree first.
func printDependTreeText(w io.Writer, s *store.Store, t *model.Task, depth int, visiting map[int]bool) {
	prefix := dependTreeIndent(depth)
	check := " "
	if t.Done {
		check = "x"
	}
	pf(w, "%s#%d [%s] %s\n", prefix, t.ID, check, t.Title)
	if !t.HasDependencies() {
		return
	}
	visiting[t.ID] = true
	defer delete(visiting, t.ID)
	ids := append([]int(nil), t.DependsOn...)
	sort.Ints(ids)
	for _, dep := range ids {
		if visiting[dep] {
			pf(w, "%s#%d (cycle)\n", dependTreeIndent(depth+1), dep)
			continue
		}
		child := s.ByID(dep)
		if child == nil {
			pf(w, "%s#%d (missing — referenced but no task with this id)\n",
				dependTreeIndent(depth+1), dep)
			continue
		}
		printDependTreeText(w, s, child, depth+1, visiting)
	}
}

// dependTreeIndent returns the leading whitespace + connector for a
// given depth. Depth 0 (the root) has no prefix; deeper levels use a
// box-drawing connector so the tree shape is visually obvious.
func dependTreeIndent(depth int) string {
	if depth == 0 {
		return ""
	}
	return strings.Repeat("  ", depth-1) + "└─ "
}
