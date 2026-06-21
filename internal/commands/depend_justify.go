package commands

import (
	"encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// runDependJustify renders a chain of reasons explaining why a task
// can't be marked done — a depth-first walk down the dependency
// graph, picking the FIRST (lowest-id) open prereq at each step.
//
// The point: --tree shows the full structure (great when you want to
// know everything that's pending), --justify shows the single chain
// of bottleneck reasons you need to fix to unblock the root. Each
// line annotates state — done, open, missing, cycle — so the user
// can scan and see exactly where the chain terminates.
//
// Output shape (plain):
//
//	#5 (open) blocked by:
//	  - #3 (open) blocked by:
//	    - #7 (open, no prereqs) — START HERE
//
// Done tasks at the root print a different one-liner ("#5 done, not
// blocked"). Tasks with no prereqs render "#5 has no dependencies"
// — distinguishable from blocked-but-unblockable so the user
// understands the result wasn't truncated.
//
// JSON shape is a flat array of reason steps (order = chain order),
// each {id, title, status, blocked_by?}, terminated with the
// actionable leaf at the tail. Consumers can `jq -r '.[-1].id'` to
// grab the "what should I do next?" answer programmatically.
//
// Cycle handling: same visit-set guard as --tree. If the chain
// re-enters an already-visited id, emit "(cycle)" and stop — never
// loop a corrupt hand-edit.
//
// Tie-break: when a task has multiple open prereqs, --justify
// follows the LOWEST id (deterministic). This is intentional — the
// `tsk next` selector handles "what's the best next thing"; justify
// is a debugging tool answering "trace one chain to the bottom".
// Picking by id keeps output reproducible across runs and
// transparent in tests.
func runDependJustify(w io.Writer, s *store.Store, root *model.Task, asJSON bool) error {
	chain := buildJustifyChain(s, root)
	if asJSON {
		return emitJustifyJSON(w, chain)
	}
	return emitJustifyPlain(w, chain)
}

// justifyStep is one row in the reason chain. Status is one of:
// "done", "open", "open-leaf" (actionable: open with no open
// prereqs), "missing" (id with no task), "cycle" (re-entered a
// node already on the stack).
type justifyStep struct {
	ID        int    `json:"id"`
	Title     string `json:"title,omitempty"`
	Status    string `json:"status"`
	BlockedBy int    `json:"blocked_by,omitempty"`
}

// buildJustifyChain walks the dependency graph from `root` depth-
// first, always picking the lowest-id open prereq at each step.
// Stops on the first task that's either done, has no prereqs, has
// no OPEN prereqs (actionable leaf), is missing, or closes a cycle.
//
// Returns the chain as an ordered list — caller renders it as
// plain text or JSON.
//
// Status semantics:
//   - "done": root is already done (terminal; can only appear at chain[0])
//   - "no-deps": root has no DependsOn entries (terminal; chain[0] only)
//   - "open-leaf": this task is actionable — open with no unmet prereqs.
//     Terminal in chain; rendered "START HERE" in plain text. Appears
//     at chain[0] when root is actionable, or at the tail of a deeper
//     chain when the bottleneck is found.
//   - "blocked": this task has unmet prereqs; blocked_by names the
//     prereq we descended into (always the lowest-id open one).
//   - "cycle": the next hop was already on the visit stack; terminal,
//     defensive against corrupt hand-edits.
//   - "missing": referenced id with no task; terminal, defensive
//     (unmetBlockers normally filters these out so this is a safety net).
func buildJustifyChain(s *store.Store, root *model.Task) []justifyStep {
	chain := make([]justifyStep, 0, 4)
	visiting := make(map[int]bool)
	curr := root
	isRoot := true
	for {
		visiting[curr.ID] = true
		step := justifyStep{ID: curr.ID, Title: curr.Title}
		switch {
		case curr.Done:
			step.Status = "done"
			chain = append(chain, step)
			return chain
		case !curr.HasDependencies():
			// No DependsOn at all. At the root this is "no-deps"
			// (distinct messaging — the user wonders if they
			// forgot to set deps); mid-chain it's the actionable
			// leaf (the bottom of the prereq chain).
			if isRoot {
				step.Status = "no-deps"
			} else {
				step.Status = "open-leaf"
			}
			chain = append(chain, step)
			return chain
		}
		// Compute open blockers (dangling refs treated as
		// satisfied — matches unmetBlockers).
		blockers := unmetBlockers(s, curr, nil)
		if len(blockers) == 0 {
			// All prereqs satisfied — this task IS the actionable
			// next thing. Always "open-leaf", whether at root or
			// mid-chain; the renderer distinguishes the root case
			// by checking chain length.
			step.Status = "open-leaf"
			chain = append(chain, step)
			return chain
		}
		// Pick the lowest-id open prereq for the next hop.
		sort.Ints(blockers)
		next := blockers[0]
		step.Status = "blocked"
		step.BlockedBy = next
		chain = append(chain, step)
		if visiting[next] {
			// Cycle — terminate with a sentinel row so the caller
			// can render it correctly.
			chain = append(chain, justifyStep{ID: next, Status: "cycle"})
			return chain
		}
		child := s.ByID(next)
		if child == nil {
			// Defensive — unmetBlockers already filters dangling
			// refs, so this should be unreachable. Treat as
			// terminal "missing" for safety.
			chain = append(chain, justifyStep{ID: next, Status: "missing"})
			return chain
		}
		curr = child
		isRoot = false
	}
}

// emitJustifyPlain renders the chain as indented one-liners. Each
// non-terminal step ends with "blocked by:"; the terminal step's
// rendering depends on its status (done / open-leaf / missing /
// cycle / no-deps). Indentation grows two spaces per hop so the
// chain shape is visible.
func emitJustifyPlain(w io.Writer, chain []justifyStep) error {
	if len(chain) == 0 {
		// Defensive — buildJustifyChain always returns at least
		// one row.
		pln(w, "no chain")
		return nil
	}
	// Handle root special cases first for clear messaging.
	root := chain[0]
	switch root.Status {
	case "done":
		pf(w, "#%d done, not blocked\n", root.ID)
		return nil
	case "no-deps":
		pf(w, "#%d has no dependencies — already actionable\n", root.ID)
		return nil
	case "open-leaf":
		pf(w, "#%d is open with no unmet prereqs — already actionable\n", root.ID)
		return nil
	}
	// General case: at least one "blocked by" hop.
	for i, step := range chain {
		indent := strings.Repeat("  ", i)
		switch step.Status {
		case "blocked":
			pf(w, "%s#%d %s (open) blocked by:\n", indent, step.ID, step.Title)
		case "open-leaf":
			pf(w, "%s- #%d %s (open, no open prereqs) — START HERE\n", indent, step.ID, step.Title)
		case "done":
			// Shouldn't happen mid-chain (we stop on done at the
			// root only — mid-chain done means buildJustifyChain
			// kept walking, which it doesn't). Defensive render.
			pf(w, "%s- #%d %s (done)\n", indent, step.ID, step.Title)
		case "missing":
			pf(w, "%s- #%d (missing — referenced but no task with this id)\n", indent, step.ID)
		case "cycle":
			pf(w, "%s- #%d (cycle — already on this chain)\n", indent, step.ID)
		case "no-deps":
			pf(w, "%s- #%d %s (no prereqs)\n", indent, step.ID, step.Title)
		default:
			pf(w, "%s- #%d %s (%s)\n", indent, step.ID, step.Title, step.Status)
		}
	}
	return nil
}

// emitJustifyJSON renders the chain as an array of step objects.
// Schema is stable and follows omitempty for optional fields so
// `jq` consumers can rely on key presence (e.g. .blocked_by is
// only present for "blocked" rows; tail rows never have it).
func emitJustifyJSON(w io.Writer, chain []justifyStep) error {
	if chain == nil {
		chain = []justifyStep{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(chain)
}
