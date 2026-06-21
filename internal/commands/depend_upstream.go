package commands

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// runDependUpstream answers the reverse-of-tree question: "what tasks
// depend on ME?"
//
// `tsk depend <id> --tree` walks DOWN the prerequisite chain (the
// prereqs of the task, and their prereqs, etc). `--upstream` walks
// UP (the tasks that name this id in their DependsOn).
//
// Why: closing a task often triggers a follow-up — "did anyone
// depend on this? should I revisit those tasks?" Without
// `--upstream`, the user has to grep the store or scan
// `tsk depend --list` for any blocker entry that references this id.
//
// Direct-dependents only by default. The "transitive upstream" view
// (every task that transitively-via-deps reaches this id) is what
// `tsk graph` already shows; reproducing it here would duplicate
// rendering with no win. The single-step list IS what you want
// before/after closing one task, which is the most common moment
// to ask the upstream question.
//
// Output annotates each row with the dependent's state:
//
//   - "(blocked)" — open and still has other unmet prereqs even
//     once the queried task closes (closing this task won't unblock it)
//   - "(unblocks)" — open and the queried task is its ONLY remaining
//     open blocker (closing this task is the trigger to unblock it)
//   - "(done)"     — already complete (this dep edge is historical)
//
// The "(unblocks)" annotation is the most useful signal — it tells
// the user which tasks would immediately become actionable if they
// closed the queried task. JSON exposes the same info as a stable
// "status" enum.
//
// Sorting: by id asc, stable. Deterministic output for diffs/scripts.
//
// Empty result: human path prints "no tasks depend on #N"; JSON
// emits "[]" (NOT null) for consumers that map straight to an array.
func runDependUpstream(w io.Writer, s *store.Store, target *model.Task, asJSON bool) error {
	rows := collectUpstreamDependents(s, target)
	if asJSON {
		return emitUpstreamJSON(w, target, rows)
	}
	return emitUpstreamPlain(w, target, rows)
}

// upstreamRow describes one direct dependent of the queried task,
// along with the state annotation explaining what closing the
// queried task would do for this dependent.
type upstreamRow struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // "blocked", "unblocks", or "done"
}

// collectUpstreamDependents scans every task in the store, picks the
// ones whose DependsOn list contains the target id, and classifies
// each one's state relative to the target.
//
// Algorithm:
//
//  1. For each candidate t that depends on target.ID:
//  2. If t.Done -> status="done" (historical edge)
//  3. Else compute t's OTHER open blockers (everyone in t.DependsOn
//     except target itself):
//     - if any other blocker is still open -> status="blocked"
//     (the target isn't the only thing gating t)
//     - else -> status="unblocks"
//     (closing target unblocks t)
//
// Important: "other open blockers" must EXCLUDE target.ID itself —
// otherwise every dependent looks blocked-by-target and the
// "unblocks" signal disappears. The exclusion treats target as
// "about to be closed" for the purposes of this what-if analysis.
func collectUpstreamDependents(s *store.Store, target *model.Task) []upstreamRow {
	rows := make([]upstreamRow, 0)
	for _, candidate := range s.Tasks {
		dependsOnTarget := false
		for _, dep := range candidate.DependsOn {
			if dep == target.ID {
				dependsOnTarget = true
				break
			}
		}
		if !dependsOnTarget {
			continue
		}
		row := upstreamRow{ID: candidate.ID, Title: candidate.Title}
		switch {
		case candidate.Done:
			row.Status = "done"
		default:
			// Look at OTHER deps (not the target) to decide if closing
			// the target would actually unblock this dependent.
			otherStillOpen := false
			for _, dep := range candidate.DependsOn {
				if dep == target.ID {
					continue
				}
				bt := s.ByID(dep)
				// Dangling deps treated as satisfied (matches
				// unmetBlockers' policy — surfacing those is
				// `tsk lint`'s job).
				if bt == nil {
					continue
				}
				if !bt.Done {
					otherStillOpen = true
					break
				}
			}
			if otherStillOpen {
				row.Status = "blocked"
			} else {
				row.Status = "unblocks"
			}
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

// emitUpstreamPlain renders the human-readable list. Header line
// names the target so the user has unambiguous context; each row
// is annotated with the status word in parens.
func emitUpstreamPlain(w io.Writer, target *model.Task, rows []upstreamRow) error {
	if len(rows) == 0 {
		pf(w, "no tasks depend on #%d\n", target.ID)
		return nil
	}
	pf(w, "#%d  %s  (upstream: %d)\n", target.ID, target.Title, len(rows))
	for _, r := range rows {
		pf(w, "  #%d  %s  (%s)\n", r.ID, r.Title, r.Status)
	}
	return nil
}

// emitUpstreamJSON writes a list of dependents with stable schema.
// Empty result emits "[]" (not null) — consumers `.length` checking
// or iterating won't crash on an unexpected nil.
func emitUpstreamJSON(w io.Writer, target *model.Task, rows []upstreamRow) error {
	doc := struct {
		ID         int           `json:"id"`
		Title      string        `json:"title"`
		Upstream   []upstreamRow `json:"upstream"`
		TotalCount int           `json:"total_count"`
	}{
		ID:         target.ID,
		Title:      target.Title,
		Upstream:   rows,
		TotalCount: len(rows),
	}
	if doc.Upstream == nil {
		doc.Upstream = []upstreamRow{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
