package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// bulkOpts is the parsed/validated form of the flag values.
type bulkOpts struct {
	// selectors
	filterTags     []string
	filterPriority *model.Priority
	filterDone     *bool
	filterIDs      []int

	// mutations
	setPriority *model.Priority
	addTags     []string
	removeTags  []string
	setDue      *time.Time
	clearDue    bool

	// behavior
	apply bool
}

// newBulkCmd creates the `tsk bulk` command. Bulk applies edits to many tasks
// at once, selected via filter flags (--tag, --priority, --status, --id).
// By default it runs in dry-run mode; --apply commits the changes.
func newBulkCmd() *cobra.Command {
	var raw bulkRawFlags
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
			opts, err := raw.toOpts()
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			return runBulk(cmd.OutOrStdout(), s, opts)
		},
	}
	raw.bind(cmd)
	return cmd
}

// bulkRawFlags is the raw string-form of the flags before parsing.
type bulkRawFlags struct {
	filterTags     []string
	filterPriority string
	filterStatus   string
	filterIDs      []int

	setPriority string
	addTags     []string
	removeTags  []string
	setDue      string
	clearDue    bool

	apply bool
}

// bind registers the flag definitions on cmd.
func (r *bulkRawFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringArrayVar(&r.filterTags, "tag", nil, "filter: tasks with this tag (repeatable)")
	cmd.Flags().StringVar(&r.filterPriority, "priority", "", "filter: tasks with this priority")
	cmd.Flags().StringVar(&r.filterStatus, "status", "", "filter: open|done")
	cmd.Flags().IntSliceVar(&r.filterIDs, "id", nil, "filter: specific task id (repeatable)")

	cmd.Flags().StringVar(&r.setPriority, "set-priority", "", "set priority on matched tasks")
	cmd.Flags().StringArrayVar(&r.addTags, "add-tag", nil, "add a tag (repeatable)")
	cmd.Flags().StringArrayVar(&r.removeTags, "remove-tag", nil, "remove a tag (repeatable)")
	cmd.Flags().StringVar(&r.setDue, "set-due", "", "set due date (parsed like tsk add --due)")
	cmd.Flags().BoolVar(&r.clearDue, "clear-due", false, "clear the due date")

	cmd.Flags().BoolVar(&r.apply, "apply", false, "commit changes (default is dry run)")
}

// toOpts validates and parses the raw flags into a bulkOpts.
func (r *bulkRawFlags) toOpts() (bulkOpts, error) {
	var o bulkOpts
	if !r.hasSelector() {
		return o, usageErrorf("at least one selector required (--tag, --priority, --status, or --id)")
	}
	if !r.hasMutation() {
		return o, usageErrorf("at least one mutation required (--set-priority, --add-tag, --remove-tag, --set-due, --clear-due)")
	}
	if r.setDue != "" && r.clearDue {
		return o, usageErrorf("--set-due and --clear-due are mutually exclusive")
	}

	prio, err := parseOptionalPriority(r.filterPriority, "--priority")
	if err != nil {
		return o, err
	}
	done, err := parseOptionalStatus(r.filterStatus)
	if err != nil {
		return o, err
	}
	setPrio, err := parseOptionalPriority(r.setPriority, "--set-priority")
	if err != nil {
		return o, err
	}
	due, err := parseOptionalDue(r.setDue)
	if err != nil {
		return o, err
	}

	o.filterTags = r.filterTags
	o.filterPriority = prio
	o.filterDone = done
	o.filterIDs = r.filterIDs
	o.setPriority = setPrio
	o.addTags = r.addTags
	o.removeTags = r.removeTags
	o.setDue = due
	o.clearDue = r.clearDue
	o.apply = r.apply
	return o, nil
}

func (r *bulkRawFlags) hasSelector() bool {
	return len(r.filterTags) > 0 || r.filterPriority != "" || r.filterStatus != "" || len(r.filterIDs) > 0
}

func (r *bulkRawFlags) hasMutation() bool {
	return r.setPriority != "" || len(r.addTags) > 0 || len(r.removeTags) > 0 || r.setDue != "" || r.clearDue
}

// parseOptionalPriority returns nil if s is empty, else a parsed Priority pointer.
func parseOptionalPriority(s, flagName string) (*model.Priority, error) {
	if s == "" {
		return nil, nil
	}
	p, err := model.ParsePriority(s)
	if err != nil {
		return nil, usageErrorf("invalid %s: %s", flagName, err.Error())
	}
	return &p, nil
}

// parseOptionalStatus returns nil if s is empty, else a pointer to whether
// the status means "done".
func parseOptionalStatus(s string) (*bool, error) {
	if s == "" {
		return nil, nil
	}
	switch strings.ToLower(s) {
	case "open", "todo", "pending":
		v := false
		return &v, nil
	case "done", "complete", "completed":
		v := true
		return &v, nil
	}
	return nil, usageErrorf("invalid --status %q (use open or done)", s)
}

// parseOptionalDue returns nil if s is empty, else the parsed time.
func parseOptionalDue(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	loc := PacificLoc()
	t, err := dateparse.Parse(s, time.Now().In(loc), loc)
	if err != nil {
		return nil, usageErrorf("invalid --set-due: %s", err.Error())
	}
	return &t, nil
}

// runBulk is the high-level orchestrator: select, then either preview or apply.
func runBulk(out io.Writer, s *store.Store, opts bulkOpts) error {
	matched := selectBulkMatches(s, opts)
	if len(matched) == 0 {
		pf(out, "no tasks matched\n")
		return nil
	}
	summary := bulkSummary(opts)
	if !opts.apply {
		printBulkPreview(out, s, matched, summary)
		return nil
	}
	applyBulkMutations(s, matched, opts)
	if err := s.Save(); err != nil {
		return err
	}
	pf(out, "updated %d task(s): %s\n", len(matched), summary)
	return nil
}

// selectBulkMatches returns the sorted IDs of tasks that match opts.
func selectBulkMatches(s *store.Store, opts bulkOpts) []int {
	idSet := make(map[int]bool, len(opts.filterIDs))
	for _, id := range opts.filterIDs {
		idSet[id] = true
	}
	matched := make([]int, 0)
	for _, t := range s.Tasks {
		if !bulkMatches(t, opts, idSet) {
			continue
		}
		matched = append(matched, t.ID)
	}
	sort.Ints(matched)
	return matched
}

// bulkMatches returns true if task t passes every selector in opts.
func bulkMatches(t model.Task, opts bulkOpts, idSet map[int]bool) bool {
	if len(idSet) > 0 && !idSet[t.ID] {
		return false
	}
	if opts.filterPriority != nil && t.Priority != *opts.filterPriority {
		return false
	}
	if opts.filterDone != nil && t.Done != *opts.filterDone {
		return false
	}
	if len(opts.filterTags) > 0 && !anyTagMatch(t.Tags, opts.filterTags) {
		return false
	}
	return true
}

// applyBulkMutations mutates every task in matched according to opts.
func applyBulkMutations(s *store.Store, matched []int, opts bulkOpts) {
	for _, id := range matched {
		t := s.ByID(id)
		if t == nil {
			continue
		}
		mutateTask(t, opts)
	}
}

// mutateTask applies the mutation set in opts to a single task.
func mutateTask(t *model.Task, opts bulkOpts) {
	if opts.setPriority != nil {
		t.Priority = *opts.setPriority
	}
	if len(opts.addTags) > 0 {
		t.Tags = addUniqueTags(t.Tags, opts.addTags)
	}
	if len(opts.removeTags) > 0 {
		t.Tags = removeTagsFrom(t.Tags, opts.removeTags)
	}
	if opts.setDue != nil {
		d := *opts.setDue
		t.Due = &d
	}
	if opts.clearDue {
		t.Due = nil
	}
}

// printBulkPreview prints the dry-run preview.
func printBulkPreview(out io.Writer, s *store.Store, matched []int, summary string) {
	pf(out, "DRY RUN — %d task(s) would be changed (%s):\n", len(matched), summary)
	for _, id := range matched {
		t := s.ByID(id)
		if t == nil {
			continue
		}
		pf(out, "  #%-3d  %s\n", t.ID, t.Title)
	}
	pf(out, "\nre-run with --apply to commit.\n")
}

// bulkSummary builds a compact human-readable summary of the mutations.
func bulkSummary(opts bulkOpts) string {
	var parts []string
	if opts.setPriority != nil {
		parts = append(parts, fmt.Sprintf("priority=%s", *opts.setPriority))
	}
	if len(opts.addTags) > 0 {
		parts = append(parts, fmt.Sprintf("+tags[%s]", strings.Join(opts.addTags, ",")))
	}
	if len(opts.removeTags) > 0 {
		parts = append(parts, fmt.Sprintf("-tags[%s]", strings.Join(opts.removeTags, ",")))
	}
	if opts.setDue != nil {
		parts = append(parts, fmt.Sprintf("due=%s", opts.setDue.Format("2006-01-02")))
	}
	if opts.clearDue {
		parts = append(parts, "clear-due")
	}
	return strings.Join(parts, ", ")
}

// anyTagMatch returns true if any of want is present in have (case-insensitive).
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
