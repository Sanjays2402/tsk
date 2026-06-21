package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newDedupeCmd implements `tsk dedupe`: surface tasks whose titles are
// identical (or near-identical) so the user can review and clean up.
//
// The decision NOT to delete anything automatically is intentional —
// dedupe is a review tool, not a destructive one. Wiring auto-removal
// would too often clobber the wrong copy (the one with notes, the one
// with a due date, the one mid-conversation in a notes thread).
//
// Default scope is UNDONE-ONLY because that's where accidental
// re-adds happen ("did I already add 'pay rent'?"). --done finds
// dupe completions; --all finds both.
//
// Matching modes:
//
//   - exact (default): case-folded + whitespace-normalized title
//     equality. Catches the "Pay rent" / "pay rent" / "pay  rent"
//     family. Cheap.
//   - --near[=N]: normalized Damerau-light Levenshtein distance <= N
//     (default 2). Catches typos ("dpeloy infra" / "deploy infra"),
//     trailing punctuation ("buy milk" / "buy milk."), and missing
//     spaces ("payrent" / "pay rent"). Quadratic, but only over the
//     task set — fine for any human-sized store.
//
// Output groups duplicates so the relationship is visible at a glance:
//
//	group 1 (3 tasks, distance 0):
//	  #3   pay rent
//	  #7   Pay rent
//	  #12  pay  rent
//
//	group 2 (2 tasks, distance 1):
//	  #5   deploy infra
//	  #9   dpeloy infra
//
// --json emits a stable schema (`groups: [{distance, tasks: [...]}]`).
// --files-only / -l prints only the IDs (one per line), grouped by
// blank lines — pipeline-friendly for "review then maybe rm" workflows.
//
// Exit codes:
//
//	0 no duplicate groups
//	1 duplicate groups found (so CI / pre-commit can flag without --json)
//	2 bad invocation
func newDedupeCmd() *cobra.Command {
	var (
		doneOnly  bool
		showAll   bool
		near      int
		asJSON    bool
		filesOnly bool
	)
	cmd := &cobra.Command{
		Use:   "dedupe",
		Short: "Surface duplicate (or near-duplicate) tasks for review",
		Long: `Surface tasks whose titles look like duplicates, so you can review
and clean up. Does NOT delete anything automatically — pair with
'tsk rm <id>' once you've picked the survivor.

Modes:
  default     case-folded + whitespace-normalized exact match
  --near[=N]  edit-distance <= N (default 2); catches typos and
              trailing punctuation. N=0 is equivalent to the default.

Scope (mirrors ls / grep):
  default     undone tasks only
  --done      done tasks only
  --all       union

Output:
  default       grouped human-readable report
  --json        stable {groups: [{distance, tasks: [...]}]}
  --files-only  IDs one per line, groups separated by blank lines

Exit codes:
  0  no duplicates
  1  duplicates found (printed)
  2  bad invocation

Examples:
  tsk dedupe                        # exact duplicates only
  tsk dedupe --near                 # plus typos within distance 2
  tsk dedupe --near=1               # tighter — only off-by-one
  tsk dedupe --all --json
  tsk dedupe --files-only           # IDs only, for piping
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if doneOnly && showAll {
				return usageErrorf("--done and --all are mutually exclusive")
			}
			if near < 0 {
				return usageErrorf("--near must be >= 0, got %d", near)
			}
			if filesOnly && asJSON {
				return usageErrorf("--files-only and --json are mutually exclusive")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			scope := dedupeScope{doneOnly: doneOnly, all: showAll}
			groups := findDupeGroups(s.Tasks, scope, near)
			if err := emitDedupeResult(cmd.OutOrStdout(), groups, asJSON, filesOnly); err != nil {
				return err
			}
			if len(groups) > 0 {
				return silentExit{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&doneOnly, "done", false, "consider done tasks only")
	cmd.Flags().BoolVar(&showAll, "all", false, "consider done + undone")
	cmd.Flags().IntVar(&near, "near", 0, "match titles within edit distance N (omit or 0 = exact only; default 2 when flag passed without value)")
	cmd.Flags().Lookup("near").NoOptDefVal = "2" // `--near` alone implies 2
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit stable JSON")
	cmd.Flags().BoolVarP(&filesOnly, "files-only", "l", false, "print matching IDs only (one per line, groups separated by blank lines)")
	return cmd
}

// dedupeScope is the done-state arbiter, same shape as priStatsScope /
// grepScope. Kept local so unrelated commands can't accidentally bind
// to its layout.
type dedupeScope struct {
	doneOnly bool
	all      bool
}

func (s dedupeScope) allows(t model.Task) bool {
	switch {
	case s.all:
		return true
	case s.doneOnly:
		return t.Done
	default:
		return !t.Done
	}
}

// dupeGroup is one cluster of similar tasks plus the maximum edit
// distance seen INSIDE the group (0 for an exact-match group).
type dupeGroup struct {
	Distance int          // max edit distance from the group's representative title
	Tasks    []model.Task // members in file order (sorted by ID asc)
}

// findDupeGroups runs a union-find over normalized task titles. For
// exact mode (near==0) the "key" is the normalized title itself; for
// near mode we cluster by edit distance with the first task in scope
// acting as the anchor for each cluster.
func findDupeGroups(tasks []model.Task, scope dedupeScope, near int) []dupeGroup {
	in := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		if scope.allows(t) {
			in = append(in, t)
		}
	}
	if near <= 0 {
		return findExactDupeGroups(in)
	}
	return findNearDupeGroups(in, near)
}

// findExactDupeGroups buckets by normalized title and keeps the
// buckets with >= 2 members. O(n) after normalization.
func findExactDupeGroups(in []model.Task) []dupeGroup {
	buckets := map[string][]model.Task{}
	order := []string{}
	for _, t := range in {
		key := normalizeTitle(t.Title)
		if key == "" {
			continue
		}
		if _, ok := buckets[key]; !ok {
			order = append(order, key)
		}
		buckets[key] = append(buckets[key], t)
	}
	out := make([]dupeGroup, 0)
	for _, k := range order {
		group := buckets[k]
		if len(group) < 2 {
			continue
		}
		sortTasksByID(group)
		out = append(out, dupeGroup{Distance: 0, Tasks: group})
	}
	return out
}

// findNearDupeGroups builds clusters greedily: for each task, find the
// first existing cluster whose anchor is within `near` of the task and
// drop it there; otherwise seed a new cluster. Order-dependent — but
// stable for a given input order, which is what tests need.
func findNearDupeGroups(in []model.Task, near int) []dupeGroup {
	type cluster struct {
		anchor   string
		members  []model.Task
		maxDist  int
		distance int
	}
	clusters := make([]cluster, 0)
	for _, t := range in {
		norm := normalizeTitle(t.Title)
		if norm == "" {
			continue
		}
		placed := false
		for i := range clusters {
			d := boundedEditDistance(norm, clusters[i].anchor, near)
			if d <= near {
				clusters[i].members = append(clusters[i].members, t)
				if d > clusters[i].maxDist {
					clusters[i].maxDist = d
				}
				placed = true
				break
			}
		}
		if !placed {
			clusters = append(clusters, cluster{anchor: norm, members: []model.Task{t}})
		}
	}
	out := make([]dupeGroup, 0, len(clusters))
	for _, c := range clusters {
		if len(c.members) < 2 {
			continue
		}
		sortTasksByID(c.members)
		out = append(out, dupeGroup{Distance: c.maxDist, Tasks: c.members})
	}
	return out
}

// normalizeTitle lowercases, trims, and collapses internal whitespace
// runs to a single space. Punctuation is removed for the "buy milk" /
// "buy milk." class of dupes that exact-mode should still catch.
func normalizeTitle(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			if !prevSpace && sb.Len() > 0 {
				sb.WriteRune(' ')
			}
			prevSpace = true
		case unicode.IsPunct(r):
			// Skip punctuation entirely so "buy milk." and "buy milk"
			// collide in exact mode.
			prevSpace = false
		default:
			sb.WriteRune(r)
			prevSpace = false
		}
	}
	out := sb.String()
	return strings.TrimRight(out, " ")
}

// boundedEditDistance is Levenshtein with early termination when the
// running minimum exceeds `cap`. Quadratic in the worst case, but the
// inner loop bails as soon as no row entry can still produce a value
// <= cap. That keeps `--near 2` fast even with thousands of tasks.
//
// We don't bother with Damerau (transpositions) — for the typo class
// dedupe targets, single-edit distance is plenty, and adding the
// transposition row inflates code with little user-visible win.
func boundedEditDistance(a, b string, cap int) int {
	la, lb := len(a), len(b)
	if abs(la-lb) > cap {
		return cap + 1
	}
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(
				cur[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
			if cur[j] < rowMin {
				rowMin = cur[j]
			}
		}
		if rowMin > cap {
			return cap + 1
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func sortTasksByID(ts []model.Task) {
	sort.SliceStable(ts, func(i, j int) bool {
		return ts[i].ID < ts[j].ID
	})
}

// emitDedupeResult dispatches to the chosen output mode.
func emitDedupeResult(w io.Writer, groups []dupeGroup, asJSON, filesOnly bool) error {
	switch {
	case asJSON:
		return emitDedupeJSON(w, groups)
	case filesOnly:
		emitDedupeFilesOnly(w, groups)
		return nil
	default:
		printDedupeHuman(w, groups)
		return nil
	}
}

type dedupeJSONGroup struct {
	Distance int          `json:"distance"`
	Tasks    []model.Task `json:"tasks"`
}
type dedupeJSONDoc struct {
	Groups []dedupeJSONGroup `json:"groups"`
}

func emitDedupeJSON(w io.Writer, groups []dupeGroup) error {
	doc := dedupeJSONDoc{Groups: make([]dedupeJSONGroup, 0, len(groups))}
	for _, g := range groups {
		doc.Groups = append(doc.Groups, dedupeJSONGroup{Distance: g.Distance, Tasks: g.Tasks})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func emitDedupeFilesOnly(w io.Writer, groups []dupeGroup) {
	for i, g := range groups {
		if i > 0 {
			pln(w)
		}
		for _, t := range g.Tasks {
			pf(w, "%d\n", t.ID)
		}
	}
}

func printDedupeHuman(w io.Writer, groups []dupeGroup) {
	if len(groups) == 0 {
		pln(w, "no duplicates")
		return
	}
	for i, g := range groups {
		if i > 0 {
			pln(w)
		}
		distStr := ""
		if g.Distance > 0 {
			distStr = fmt.Sprintf(", distance %d", g.Distance)
		}
		pf(w, "group %d (%d tasks%s):\n", i+1, len(g.Tasks), distStr)
		for _, t := range g.Tasks {
			pf(w, "  #%-3d %s\n", t.ID, t.Title)
		}
	}
}
