package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
)

// newBulkCmd creates the `tsk bulk` command. Bulk applies edits to many tasks
// at once, selected via filter flags (--tag, --priority, --status, --id).
// By default it runs in dry-run mode; --apply commits the changes.
func newBulkCmd() *cobra.Command {
	var (
		// selectors
		filterTags     []string
		filterPriority string
		filterStatus   string
		filterIDs      []int

		// mutations
		setPriority string
		addTags     []string
		removeTags  []string
		setDue      string
		clearDue    bool

		// behavior
		apply bool
	)

	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Bulk edit tasks matched by filter (dry-run by default; use --apply to commit)",
		Long: `Bulk edits multiple tasks selected by filter flags.

Selectors (all that are set must match — AND across selectors):
  --tag <name>       match tasks containing this tag (repeatable, OR within)
  --priority <p>     match tasks with this priority (low|medium|high|urgent)
  --status <s>       match tasks with this status (open|done)
  --id <n>           explicit task id (repeatable)

Mutations:
  --set-priority <p>  set priority on matched tasks
  --add-tag <name>    add a tag (repeatable; idempotent — won't dup)
  --remove-tag <name> remove a tag (repeatable)
  --set-due <date>    set due date (parsed by tsk dateparse)
  --clear-due         clear the due date

Examples:
  tsk bulk --tag old --add-tag legacy --remove-tag old --apply
  tsk bulk --priority low --status open --set-priority medium --apply
  tsk bulk --id 3 --id 7 --set-due tomorrow --apply
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Validate at least one selector and one mutation
			if len(filterTags) == 0 && filterPriority == "" && filterStatus == "" && len(filterIDs) == 0 {
				return usageErrorf("at least one selector required (--tag, --priority, --status, or --id)")
			}
			if setPriority == "" && len(addTags) == 0 && len(removeTags) == 0 && setDue == "" && !clearDue {
				return usageErrorf("at least one mutation required (--set-priority, --add-tag, --remove-tag, --set-due, --clear-due)")
			}
			if setDue != "" && clearDue {
				return usageErrorf("--set-due and --clear-due are mutually exclusive")
			}

			// Parse filter priority
			var wantPrio model.Priority
			var hasWantPrio bool
			if filterPriority != "" {
				p, err := model.ParsePriority(filterPriority)
				if err != nil {
					return usageErrorf("invalid --priority: %s", err.Error())
				}
				wantPrio = p
				hasWantPrio = true
			}

			// Parse filter status
			var wantDone bool
			var hasWantStatus bool
			if filterStatus != "" {
				switch strings.ToLower(filterStatus) {
				case "open", "todo", "pending":
					wantDone = false
					hasWantStatus = true
				case "done", "complete", "completed":
					wantDone = true
					hasWantStatus = true
				default:
					return usageErrorf("invalid --status %q (use open or done)", filterStatus)
				}
			}

			// Parse mutation priority
			var newPrio model.Priority
			var hasNewPrio bool
			if setPriority != "" {
				p, err := model.ParsePriority(setPriority)
				if err != nil {
					return usageErrorf("invalid --set-priority: %s", err.Error())
				}
				newPrio = p
				hasNewPrio = true
			}

			// Parse mutation due
			var newDue *time.Time
			if setDue != "" {
				loc := PacificLoc()
				t, err := dateparse.Parse(setDue, time.Now().In(loc), loc)
				if err != nil {
					return usageErrorf("invalid --set-due: %s", err.Error())
				}
				newDue = &t
			}

			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}

			// ID set for quick lookup
			idSet := map[int]bool{}
			for _, id := range filterIDs {
				idSet[id] = true
			}

			// Match predicate
			match := func(t model.Task) bool {
				if len(idSet) > 0 && !idSet[t.ID] {
					return false
				}
				if hasWantPrio && t.Priority != wantPrio {
					return false
				}
				if hasWantStatus && t.Done != wantDone {
					return false
				}
				if len(filterTags) > 0 {
					if !anyTagMatch(t.Tags, filterTags) {
						return false
					}
				}
				return true
			}

			// Gather matches
			matched := make([]int, 0)
			for _, t := range s.Tasks {
				if match(t) {
					matched = append(matched, t.ID)
				}
			}
			sort.Ints(matched)

			out := cmd.OutOrStdout()
			if len(matched) == 0 {
				pf(out, "no tasks matched\n")
				return nil
			}

			// Build mutation summary line
			mods := bulkMutationSummary(hasNewPrio, newPrio, addTags, removeTags, newDue, clearDue)

			if !apply {
				pf(out, "DRY RUN — %d task(s) would be changed (%s):\n", len(matched), mods)
				for _, id := range matched {
					t := s.ByID(id)
					if t == nil {
						continue
					}
					pf(out, "  #%-3d  %s\n", t.ID, t.Title)
				}
				pf(out, "\nre-run with --apply to commit.\n")
				return nil
			}

			// Apply mutations
			for _, id := range matched {
				t := s.ByID(id)
				if t == nil {
					continue
				}
				if hasNewPrio {
					t.Priority = newPrio
				}
				if len(addTags) > 0 {
					t.Tags = addUniqueTags(t.Tags, addTags)
				}
				if len(removeTags) > 0 {
					t.Tags = removeTagsFrom(t.Tags, removeTags)
				}
				if newDue != nil {
					d := *newDue
					t.Due = &d
				}
				if clearDue {
					t.Due = nil
				}
			}

			if err := s.Save(); err != nil {
				return err
			}
			pf(out, "updated %d task(s): %s\n", len(matched), mods)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&filterTags, "tag", nil, "filter: tasks with this tag (repeatable)")
	cmd.Flags().StringVar(&filterPriority, "priority", "", "filter: tasks with this priority")
	cmd.Flags().StringVar(&filterStatus, "status", "", "filter: open|done")
	cmd.Flags().IntSliceVar(&filterIDs, "id", nil, "filter: specific task id (repeatable)")

	cmd.Flags().StringVar(&setPriority, "set-priority", "", "set priority on matched tasks")
	cmd.Flags().StringArrayVar(&addTags, "add-tag", nil, "add a tag (repeatable)")
	cmd.Flags().StringArrayVar(&removeTags, "remove-tag", nil, "remove a tag (repeatable)")
	cmd.Flags().StringVar(&setDue, "set-due", "", "set due date (parsed like tsk add --due)")
	cmd.Flags().BoolVar(&clearDue, "clear-due", false, "clear the due date")

	cmd.Flags().BoolVar(&apply, "apply", false, "commit changes (default is dry run)")
	return cmd
}

// anyTagMatch returns true if any of want is present in have.
func anyTagMatch(have, want []string) bool {
	if len(have) == 0 {
		return false
	}
	hs := make(map[string]bool, len(have))
	for _, t := range have {
		hs[strings.ToLower(t)] = true
	}
	for _, w := range want {
		if hs[strings.ToLower(w)] {
			return true
		}
	}
	return false
}

// addUniqueTags appends new tags to existing, deduping case-insensitively.
func addUniqueTags(existing, add []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, t := range existing {
		seen[strings.ToLower(t)] = true
	}
	out := make([]string, len(existing))
	copy(out, existing)
	for _, t := range add {
		key := strings.ToLower(t)
		if !seen[key] {
			out = append(out, t)
			seen[key] = true
		}
	}
	return out
}

// removeTagsFrom returns existing minus any tag in remove (case-insensitive).
func removeTagsFrom(existing, remove []string) []string {
	if len(existing) == 0 || len(remove) == 0 {
		return existing
	}
	rs := make(map[string]bool, len(remove))
	for _, t := range remove {
		rs[strings.ToLower(t)] = true
	}
	out := existing[:0:len(existing)]
	for _, t := range existing {
		if !rs[strings.ToLower(t)] {
			out = append(out, t)
		}
	}
	return out
}

// bulkMutationSummary builds a compact human-readable summary of the mutations.
func bulkMutationSummary(hasPrio bool, prio model.Priority, add, remove []string, due *time.Time, clearDue bool) string {
	var parts []string
	if hasPrio {
		parts = append(parts, fmt.Sprintf("priority=%s", prio))
	}
	if len(add) > 0 {
		parts = append(parts, fmt.Sprintf("+tags[%s]", strings.Join(add, ",")))
	}
	if len(remove) > 0 {
		parts = append(parts, fmt.Sprintf("-tags[%s]", strings.Join(remove, ",")))
	}
	if due != nil {
		parts = append(parts, fmt.Sprintf("due=%s", due.Format("2006-01-02")))
	}
	if clearDue {
		parts = append(parts, "clear-due")
	}
	return strings.Join(parts, ", ")
}
