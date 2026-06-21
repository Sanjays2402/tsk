package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newMergeCmd implements `tsk merge <survivor> <victim>`: collapse two
// tasks into one. The victim's notes are appended to the survivor's,
// the union of both tag sets becomes the survivor's tags, every
// dangling reference to the victim's id (from other tasks' DependsOn
// lists) is rewritten to point at the survivor, and the victim is
// removed from the store.
//
// Use case: dedupe surfaces near-identical tasks; merge lets you
// pick which one survives and roll the other's content into it
// without losing notes or dep links.
//
// Conflicting fields (priority, due, wait, started, pinned, done
// state, completed timestamp) are resolved by the --prefer flag:
//
//   - --prefer survivor  (default) keep the survivor's existing value
//   - --prefer victim    take the victim's value when it differs
//   - --prefer newer     take whichever was Created later
//
// Tags are always union-merged (no "prefer" mode applies — keeping
// both makes sense in every case).
//
// Notes are always concatenated survivor-first with a "--- merged
// from #N ---" separator so the provenance is obvious. Use --note-only
// to skip the field merge entirely (when you just want to fold the
// notes in without touching anything else).
//
// Refusals (usage errors, exit 2):
//   - survivor == victim
//   - either id missing
//   - either id has the other in DependsOn (would create a self-dep
//     after the rewrite — caller should clear the relationship first)
//
// Safety:
//   - normal .bak snapshot via store.Save, so `undo-last` reverts the
//     whole merge in one step
//   - dry-run mode (--dry-run) prints the planned merge without writing
func newMergeCmd() *cobra.Command {
	var (
		prefer   string
		noteOnly bool
		dryRun   bool
		yes      bool
	)
	cmd := &cobra.Command{
		Use:   "merge <survivor-id> <victim-id>",
		Short: "Merge one task into another (notes, tags, dep refs combined)",
		Long: `Merge two tasks into one. The SURVIVOR keeps its id; the VICTIM is
removed after its content has been folded into the survivor.

What gets merged:
  notes:        survivor || "--- merged from #N ---" || victim
  tags:         union of both sets
  dependencies: victim's DependsOn ids added to survivor's
  back-refs:    every DependsOn list elsewhere in the store that
                pointed at the victim is rewritten to point at the
                survivor (so existing dep chains don't break)

Conflicting scalar fields (priority, due, wait, started, pinned, done,
completed) resolve via --prefer:
  survivor   keep the survivor's value (default)
  victim     take the victim's value
  newer      take whichever task was Created later

Pass --note-only to skip the scalar merge entirely (notes + tags +
dep refs still merge; the survivor's priority/due/etc are untouched).

--dry-run prints the planned merge without writing.

Examples:
  tsk merge 3 7                    # fold 7 into 3
  tsk merge 3 7 --prefer victim    # take 7's priority/due/etc
  tsk merge 3 7 --note-only        # just glue notes + tags + deps
  tsk merge 3 7 --dry-run          # preview, no write

Refused: self-merge, missing id, or if the two tasks depend on each
other (clear one direction first with 'tsk depend X --remove Y').
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			survivorID, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			victimID, err := parseSingleID(args[1])
			if err != nil {
				return err
			}
			if survivorID == victimID {
				return usageErrorf("merge: survivor and victim must differ, got %d and %d",
					survivorID, victimID)
			}
			preferMode, err := resolveMergePrefer(prefer)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			survivor := s.ByID(survivorID)
			if survivor == nil {
				return fmt.Errorf("no task with id %d (survivor) in %s", survivorID, s.Path)
			}
			victim := s.ByID(victimID)
			if victim == nil {
				return fmt.Errorf("no task with id %d (victim) in %s", victimID, s.Path)
			}
			if dependsOn(survivor, victimID) || dependsOn(victim, survivorID) {
				return usageErrorf(
					"merge: #%d and #%d depend on each other; clear the dep first (`tsk depend X --remove Y`)",
					survivorID, victimID)
			}
			plan := planMerge(survivor, victim, preferMode, noteOnly)
			if dryRun {
				printMergePlan(cmd, plan, survivor, victim)
				return nil
			}
			applyMerge(s, survivor, victim, plan, noteOnly)
			if err := s.Save(); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "merged #%d into #%d (notes: +%d chars, tags: +%d, deps: +%d, back-refs: %d)\n",
				victimID, survivorID, plan.notesAdded, plan.tagsAdded, plan.depsAdded, plan.backRefs)
			_ = yes // reserved for future "skip confirmation" gating
			return nil
		},
	}
	cmd.Flags().StringVar(&prefer, "prefer", "survivor",
		"conflict resolution for scalar fields: survivor (default), victim, or newer")
	cmd.Flags().BoolVar(&noteOnly, "note-only", false,
		"only merge notes/tags/deps; leave survivor's priority/due/etc untouched")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"preview the merge without writing")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation (reserved)")
	return cmd
}

// mergePrefer enumerates the supported conflict-resolution modes.
type mergePrefer int

const (
	preferSurvivor mergePrefer = iota
	preferVictim
	preferNewer
)

// resolveMergePrefer turns the --prefer string into a mergePrefer
// enum value (or a usage error).
func resolveMergePrefer(raw string) (mergePrefer, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "survivor":
		return preferSurvivor, nil
	case "victim":
		return preferVictim, nil
	case "newer":
		return preferNewer, nil
	}
	return 0, usageErrorf("unknown --prefer %q (want survivor, victim, or newer)", raw)
}

// mergePlan captures what the merge will do, mostly so dry-run can
// report it AND so the summary line after a real apply has accurate
// counts. We compute the plan first, then apply — keeps the apply
// step idempotent and easier to reason about.
type mergePlan struct {
	mergedNotes string
	mergedTags  []string
	mergedDeps  []int

	notesAdded int
	tagsAdded  int
	depsAdded  int
	backRefs   int

	// Scalar fields the apply step will write to the survivor when
	// !noteOnly. Each is computed by chooseField(prefer, ...). Zero
	// values are "no change".
	priority      *model.Priority
	due           *time.Time
	dueClear      bool
	wait          *time.Time
	waitClear     bool
	started       *time.Time
	startedClear  bool
	completed     *time.Time
	completedClea bool
	pinned        *bool
	done          *bool
}

// planMerge inspects both tasks and produces the merge plan. The
// survivor and victim are NOT mutated here — that's applyMerge's job.
func planMerge(survivor, victim *model.Task, prefer mergePrefer, noteOnly bool) mergePlan {
	plan := mergePlan{}
	// Notes: survivor first, separator, victim. If either side is
	// empty, the separator is dropped to avoid awkward orphan lines.
	plan.mergedNotes = mergeNotes(survivor.Notes, victim.Notes, victim.ID)
	plan.notesAdded = len(plan.mergedNotes) - len(survivor.Notes)

	// Tags: case-insensitive union (NormalizeTags will sort + dedup
	// again on Save, but doing it here makes the dry-run output match).
	plan.mergedTags = unionTags(survivor.Tags, victim.Tags)
	plan.tagsAdded = len(plan.mergedTags) - len(survivor.Tags)

	// Deps: survivor's deps + victim's deps (minus any self-refs that
	// would result from the back-ref rewrite). Sorted, deduped.
	plan.mergedDeps = unionInts(survivor.DependsOn, victim.DependsOn)
	plan.mergedDeps = removeInt(plan.mergedDeps, survivor.ID) // can't depend on self
	plan.depsAdded = len(plan.mergedDeps) - len(survivor.DependsOn)

	if noteOnly {
		return plan
	}
	// Scalar fields — pick per --prefer.
	if survivor.Priority != victim.Priority {
		picked := chooseScalar(prefer, survivor, victim,
			func(t *model.Task) any { return t.Priority }).(model.Priority)
		if picked != survivor.Priority {
			plan.priority = &picked
		}
	}
	plan.due, plan.dueClear = chooseTimePtr(prefer, survivor, victim, survivor.Due, victim.Due)
	plan.wait, plan.waitClear = chooseTimePtr(prefer, survivor, victim, survivor.WaitUntil, victim.WaitUntil)
	plan.started, plan.startedClear = chooseTimePtr(prefer, survivor, victim, survivor.Started, victim.Started)
	plan.completed, plan.completedClea = chooseTimePtr(prefer, survivor, victim, survivor.Completed, victim.Completed)
	if survivor.Pinned != victim.Pinned {
		picked := chooseScalar(prefer, survivor, victim,
			func(t *model.Task) any { return t.Pinned }).(bool)
		if picked != survivor.Pinned {
			plan.pinned = &picked
		}
	}
	if survivor.Done != victim.Done {
		picked := chooseScalar(prefer, survivor, victim,
			func(t *model.Task) any { return t.Done }).(bool)
		if picked != survivor.Done {
			plan.done = &picked
		}
	}
	return plan
}

// applyMerge mutates the survivor in place per the plan, rewrites
// back-refs, removes the victim, and tallies the back-ref count.
func applyMerge(s *store.Store, survivor, victim *model.Task,
	plan mergePlan, noteOnly bool) {
	survivor.Notes = plan.mergedNotes
	survivor.Tags = plan.mergedTags
	survivor.DependsOn = plan.mergedDeps
	if !noteOnly {
		if plan.priority != nil {
			survivor.Priority = *plan.priority
		}
		if plan.dueClear {
			survivor.Due = nil
		} else if plan.due != nil {
			survivor.Due = plan.due
		}
		if plan.waitClear {
			survivor.WaitUntil = nil
		} else if plan.wait != nil {
			survivor.WaitUntil = plan.wait
		}
		if plan.startedClear {
			survivor.Started = nil
		} else if plan.started != nil {
			survivor.Started = plan.started
		}
		if plan.completedClea {
			survivor.Completed = nil
		} else if plan.completed != nil {
			survivor.Completed = plan.completed
		}
		if plan.pinned != nil {
			survivor.Pinned = *plan.pinned
		}
		if plan.done != nil {
			survivor.Done = *plan.done
		}
	}
	// Back-refs: rewrite any DependsOn entry pointing at victim.ID
	// to point at survivor.ID. Skip survivor itself (its DependsOn
	// list has already been merged) and skip the victim (about to
	// be removed). Dedup the resulting slice so the rewrite can't
	// produce a duplicate (e.g. task X depended on BOTH survivor and
	// victim — after rewrite it would have survivor twice).
	for i := range s.Tasks {
		t := &s.Tasks[i]
		if t.ID == survivor.ID || t.ID == victim.ID {
			continue
		}
		if !t.HasDependencies() {
			continue
		}
		if !containsInt(t.DependsOn, victim.ID) {
			continue
		}
		plan.backRefs++
		rewritten := make([]int, 0, len(t.DependsOn))
		seen := make(map[int]bool, len(t.DependsOn))
		for _, dep := range t.DependsOn {
			if dep == victim.ID {
				dep = survivor.ID
			}
			if dep == t.ID {
				// Pointing at self after the rewrite — drop it.
				continue
			}
			if seen[dep] {
				continue
			}
			seen[dep] = true
			rewritten = append(rewritten, dep)
		}
		sort.Ints(rewritten)
		t.DependsOn = rewritten
	}
	s.Remove(victim.ID)
}

// printMergePlan emits the dry-run summary. Shows the survivor's
// "before" snapshot, the changes that would apply, and the resulting
// "after" snapshot.
func printMergePlan(cmd *cobra.Command, plan mergePlan, survivor, victim *model.Task) {
	w := cmd.OutOrStdout()
	pf(w, "DRY RUN — would merge #%d (victim) into #%d (survivor)\n\n",
		victim.ID, survivor.ID)
	pf(w, "  survivor: #%d %s\n", survivor.ID, survivor.Title)
	pf(w, "  victim:   #%d %s\n\n", victim.ID, victim.Title)
	pf(w, "  notes:    +%d chars\n", plan.notesAdded)
	pf(w, "  tags:     +%d (final: %v)\n", plan.tagsAdded, plan.mergedTags)
	pf(w, "  deps:     +%d (final: %v)\n", plan.depsAdded, plan.mergedDeps)
	if plan.priority != nil {
		pf(w, "  priority: -> %s\n", *plan.priority)
	}
	if plan.due != nil {
		pf(w, "  due:      -> %s\n", plan.due.Format(model.DateLayout))
	} else if plan.dueClear {
		pln(w, "  due:      -> (cleared)")
	}
	if plan.pinned != nil {
		pf(w, "  pinned:   -> %t\n", *plan.pinned)
	}
	if plan.done != nil {
		pf(w, "  done:     -> %t\n", *plan.done)
	}
	pln(w, "")
	pln(w, "  (use without --dry-run to apply, or rerun and undo-last to revert)")
}

// mergeNotes concatenates survivor and victim notes with a provenance
// separator, skipping the separator when either side is empty.
func mergeNotes(a, b string, victimID int) string {
	a = strings.TrimRight(a, "\n")
	b = strings.TrimRight(b, "\n")
	switch {
	case a == "" && b == "":
		return ""
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "\n\n" + fmt.Sprintf("--- merged from #%d ---", victimID) + "\n" + b
}

// unionTags returns the case-insensitive union of two tag sets,
// preserving the survivor's original ordering for any tags it already
// owned and appending new victim-only tags in their original order
// (deduped, lowercased). NormalizeTags will sort on Save, so this is
// just for the dry-run preview and for any test that inspects tag
// order pre-Save.
func unionTags(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, tag := range a {
		k := strings.ToLower(strings.TrimSpace(tag))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	for _, tag := range b {
		k := strings.ToLower(strings.TrimSpace(tag))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// unionInts returns a sorted, deduped union of two int slices.
func unionInts(a, b []int) []int {
	seen := make(map[int]bool, len(a)+len(b))
	out := make([]int, 0, len(a)+len(b))
	for _, x := range a {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	for _, x := range b {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Ints(out)
	return out
}

// removeInt returns s with target removed (if present).
func removeInt(s []int, target int) []int {
	out := s[:0]
	for _, x := range s {
		if x != target {
			out = append(out, x)
		}
	}
	return append([]int(nil), out...)
}

// containsInt reports whether s contains target.
func containsInt(s []int, target int) bool {
	for _, x := range s {
		if x == target {
			return true
		}
	}
	return false
}

// dependsOn reports whether t has dep in its DependsOn list. Helper
// for the "tasks depend on each other" guard.
func dependsOn(t *model.Task, dep int) bool {
	for _, d := range t.DependsOn {
		if d == dep {
			return true
		}
	}
	return false
}

// chooseScalar returns the field picked per `prefer`. survivor's
// value is returned for preferSurvivor; victim's for preferVictim;
// for preferNewer, whichever task has the later Created time wins.
// Generic-as-any so the same logic works for Priority and bool.
func chooseScalar(prefer mergePrefer, survivor, victim *model.Task,
	get func(*model.Task) any) any {
	switch prefer {
	case preferVictim:
		return get(victim)
	case preferNewer:
		if victim.Created.After(survivor.Created) {
			return get(victim)
		}
		return get(survivor)
	default:
		return get(survivor)
	}
}

// chooseTimePtr resolves a *time.Time field per --prefer. Returns
// (newValue, clear). `clear=true` means "the picked value is nil,
// overwriting a non-nil survivor"; newValue=nil + clear=false means
// "no change". This is needed because nil is ambiguous in plain
// chooseScalar — we can't tell "the picked task had no due" from
// "no decision".
func chooseTimePtr(prefer mergePrefer, survivor, victim *model.Task,
	sv, vv *time.Time) (*time.Time, bool) {
	// Build a tiny helper: when prefer points at a side, return that
	// side's value (even if nil).
	pick := func(side *time.Time) (*time.Time, bool) {
		switch {
		case side == nil && sv == nil:
			return nil, false // both nil — no change
		case side == nil && sv != nil:
			return nil, true // overwrite survivor with nil
		default:
			return side, false
		}
	}
	switch prefer {
	case preferVictim:
		// Compare against survivor's value to detect no-op.
		if timesEqual(sv, vv) {
			return nil, false
		}
		return pick(vv)
	case preferNewer:
		if victim.Created.After(survivor.Created) {
			if timesEqual(sv, vv) {
				return nil, false
			}
			return pick(vv)
		}
		return nil, false // survivor wins → no change
	default:
		return nil, false // preferSurvivor → no change
	}
}

// timesEqual reports whether two *time.Time pointers represent the
// same moment, treating both-nil as equal.
func timesEqual(a, b *time.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.Equal(*b)
	}
}
