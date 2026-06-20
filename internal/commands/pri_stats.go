package commands

import (
	"encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newPriStatsCmd implements `tsk pri-stats`: priority distribution.
//
// The complement of `tsk stats` (totals/streak/top-tags) and `tsk tags`
// (per-tag counts): a focused breakdown of where the workload sits on
// the priority axis. Surfaces obvious imbalances ("everything is high",
// "no one is doing the low pile") that the headline summary hides.
//
// Default scope is UNDONE-ONLY — this is a workload view, not a
// historical one. --done flips to done-only; --all is union.
//
// Default rendering is a labelled count + percentage:
//
//	urgent      4   (22%)
//	high        7   (39%)
//	medium      5   (27%)
//	low         2   (11%)
//	-----
//	total       18
//
// Pass --bar for a tiny inline bar chart instead of just percentages.
// Pass --by-tag to break the same distribution down per tag (each tag
// gets its own block; tags are ordered by descending undone count).
// Pass --json for a stable schema.
//
// Tag-less tasks under --by-tag bucket into "(untagged)" so they're
// surfaced rather than silently omitted — easy to miss otherwise.
func newPriStatsCmd() *cobra.Command {
	var (
		doneOnly bool
		showAll  bool
		byTag    bool
		bar      bool
		asJSON   bool
	)
	cmd := &cobra.Command{
		Use:     "pri-stats",
		Aliases: []string{"pristats", "prio-stats"},
		Short:   "Priority distribution (low/medium/high/urgent) — optionally per tag",
		Long: `Show how tasks are distributed across the four priorities.

Default scope: UNDONE only (this is a workload view — see ` + "`tsk stats`" + `
for whole-store completion metrics). Use --done for done-only or
--all for the union.

  --by-tag    additionally break the distribution down per tag
  --bar       render an inline bar alongside each count
  --json      emit a stable JSON document

Examples:
  tsk pri-stats                  # default — undone, plain
  tsk pri-stats --bar            # with inline sparkline-style bars
  tsk pri-stats --by-tag --json  # per-tag breakdown, scriptable
  tsk pri-stats --all            # whole store
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if doneOnly && showAll {
				return usageErrorf("--done and --all are mutually exclusive")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			scope := priStatsScope{doneOnly: doneOnly, all: showAll}
			report := computePriStats(s.Tasks, scope, byTag)
			out := cmd.OutOrStdout()
			if asJSON {
				return emitPriStatsJSON(out, report)
			}
			printPriStats(out, report, bar)
			return nil
		},
	}
	cmd.Flags().BoolVar(&doneOnly, "done", false, "count done tasks only (default: undone only)")
	cmd.Flags().BoolVar(&showAll, "all", false, "count done + undone")
	cmd.Flags().BoolVar(&byTag, "by-tag", false, "additionally break down per tag")
	cmd.Flags().BoolVar(&bar, "bar", false, "render an inline bar chart")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit stable JSON")
	return cmd
}

// priStatsScope holds the done-state arbitration for filtering.
type priStatsScope struct {
	doneOnly bool
	all      bool
}

// allows mirrors the same arbitration used by `ls`/`grep` for done state.
func (s priStatsScope) allows(t model.Task) bool {
	switch {
	case s.all:
		return true
	case s.doneOnly:
		return t.Done
	default:
		return !t.Done
	}
}

// priCount is one row of the report: a priority and how many tasks
// fell into it (within whatever scope/tag-group we're aggregating).
type priCount struct {
	Priority model.Priority
	Label    string
	Count    int
}

// priReport is the full result. Total = sum of Buckets.Count. When
// ByTag is non-nil, it holds the per-tag breakdowns; the top-level
// Buckets still reflect the overall distribution so the JSON consumer
// always has the headline numbers in one place.
type priReport struct {
	Total   int
	Buckets []priCount
	ByTag   []priTagGroup // empty when --by-tag wasn't passed
}

// priTagGroup is one tag's distribution within --by-tag mode.
type priTagGroup struct {
	Tag     string
	Total   int
	Buckets []priCount
}

// orderedPriorities is the canonical render order (descending urgency).
// We always emit all four buckets even if some are zero so the user can
// see WHERE the zeros are.
var orderedPriorities = []model.Priority{
	model.PriorityUrgent,
	model.PriorityHigh,
	model.PriorityMedium,
	model.PriorityLow,
}

// computePriStats counts per-priority occurrences within scope, and
// optionally also per-tag. Tasks with no tags under --by-tag land in
// the special "(untagged)" bucket so they're not silently lost.
func computePriStats(tasks []model.Task, scope priStatsScope, byTag bool) priReport {
	rep := priReport{Buckets: emptyPriBuckets()}
	tagCounts := map[string]map[model.Priority]int{}
	tagTotals := map[string]int{}
	const untaggedKey = "(untagged)"
	for _, t := range tasks {
		if !scope.allows(t) {
			continue
		}
		rep.Total++
		bumpPriBucket(rep.Buckets, t.Priority)
		if !byTag {
			continue
		}
		keys := t.Tags
		if len(keys) == 0 {
			keys = []string{untaggedKey}
		}
		for _, tg := range keys {
			if tagCounts[tg] == nil {
				tagCounts[tg] = map[model.Priority]int{}
			}
			tagCounts[tg][t.Priority]++
			tagTotals[tg]++
		}
	}
	if byTag {
		rep.ByTag = assemblePriTagGroups(tagCounts, tagTotals)
	}
	return rep
}

// emptyPriBuckets pre-seeds zero counts for every priority so the
// bucket order is stable even when the store is empty.
func emptyPriBuckets() []priCount {
	out := make([]priCount, 0, len(orderedPriorities))
	for _, p := range orderedPriorities {
		out = append(out, priCount{Priority: p, Label: p.String()})
	}
	return out
}

// bumpPriBucket adds 1 to the matching priority's count.
func bumpPriBucket(buckets []priCount, p model.Priority) {
	for i := range buckets {
		if buckets[i].Priority == p {
			buckets[i].Count++
			return
		}
	}
}

// assemblePriTagGroups turns the per-tag maps into a stable slice
// sorted by total desc (alpha tiebreak), so the most-loaded tags
// appear first.
func assemblePriTagGroups(tagCounts map[string]map[model.Priority]int, tagTotals map[string]int) []priTagGroup {
	groups := make([]priTagGroup, 0, len(tagCounts))
	for tag, counts := range tagCounts {
		group := priTagGroup{Tag: tag, Total: tagTotals[tag], Buckets: emptyPriBuckets()}
		for _, p := range orderedPriorities {
			for i := range group.Buckets {
				if group.Buckets[i].Priority == p {
					group.Buckets[i].Count = counts[p]
				}
			}
		}
		groups = append(groups, group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Total != groups[j].Total {
			return groups[i].Total > groups[j].Total
		}
		return groups[i].Tag < groups[j].Tag
	})
	return groups
}

// printPriStats renders the human view: per-bucket count + percentage,
// optional inline bar, then the per-tag block when --by-tag was set.
func printPriStats(w io.Writer, r priReport, bar bool) {
	printPriBuckets(w, r.Buckets, r.Total, bar)
	pf(w, "-----\n")
	pf(w, "total       %d\n", r.Total)
	if len(r.ByTag) == 0 {
		return
	}
	pln(w)
	pln(w, "by tag:")
	for _, g := range r.ByTag {
		pln(w)
		pf(w, "  #%s (total %d)\n", g.Tag, g.Total)
		printPriBuckets(indent2(w), g.Buckets, g.Total, bar)
	}
}

// printPriBuckets does the actual per-row rendering used by both the
// headline and the per-tag blocks. Empty (total=0) groups emit "(none)"
// to keep parsing trivial.
func printPriBuckets(w io.Writer, buckets []priCount, total int, bar bool) {
	if total == 0 {
		pln(w, "(none)")
		return
	}
	for _, b := range buckets {
		pct := 0.0
		if total > 0 {
			pct = float64(b.Count) / float64(total) * 100
		}
		if bar {
			pf(w, "%-10s %3d   (%3.0f%%)  %s\n", b.Label, b.Count, pct, makeBar(b.Count, total, 20))
		} else {
			pf(w, "%-10s %3d   (%3.0f%%)\n", b.Label, b.Count, pct)
		}
	}
}

// makeBar renders a fixed-width '█'-bar for the count over the total,
// using runes only (NO_COLOR safe; same alphabet conventions as the
// stats sparkline). Empty count = empty bar (length-0 string).
func makeBar(count, total, width int) string {
	if count <= 0 || total <= 0 || width <= 0 {
		return ""
	}
	filled := count * width / total
	if filled == 0 && count > 0 {
		filled = 1
	}
	return strings.Repeat("█", filled)
}

// emitPriStatsJSON encodes the report in a stable schema. The shape is
// deliberately verbose (label + count + percent per bucket) so a
// consumer doesn't have to recompute anything.
func emitPriStatsJSON(w io.Writer, r priReport) error {
	doc := priStatsJSON{
		Total:   r.Total,
		Buckets: bucketsToJSON(r.Buckets, r.Total),
		ByTag:   make([]priTagGroupJSON, 0, len(r.ByTag)),
	}
	for _, g := range r.ByTag {
		doc.ByTag = append(doc.ByTag, priTagGroupJSON{
			Tag:     g.Tag,
			Total:   g.Total,
			Buckets: bucketsToJSON(g.Buckets, g.Total),
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func bucketsToJSON(buckets []priCount, total int) []priBucketJSON {
	out := make([]priBucketJSON, 0, len(buckets))
	for _, b := range buckets {
		pct := 0.0
		if total > 0 {
			pct = float64(b.Count) / float64(total) * 100
		}
		out = append(out, priBucketJSON{
			Priority: b.Label,
			Count:    b.Count,
			Percent:  roundPct(pct),
		})
	}
	return out
}

// roundPct rounds to one decimal place so consumers don't get long
// float tails in their JSON.
func roundPct(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

type priStatsJSON struct {
	Total   int               `json:"total"`
	Buckets []priBucketJSON   `json:"buckets"`
	ByTag   []priTagGroupJSON `json:"by_tag"`
}

type priBucketJSON struct {
	Priority string  `json:"priority"`
	Count    int     `json:"count"`
	Percent  float64 `json:"percent"`
}

type priTagGroupJSON struct {
	Tag     string          `json:"tag"`
	Total   int             `json:"total"`
	Buckets []priBucketJSON `json:"buckets"`
}

// indent2 returns a writer that prefixes every line with two spaces.
// Used to nest the per-bucket block under "  #<tag>" in --by-tag mode.
type indentedWriter struct {
	w      io.Writer
	prefix []byte
	atBOL  bool
}

func indent2(w io.Writer) io.Writer {
	return &indentedWriter{w: w, prefix: []byte("  "), atBOL: true}
}

func (iw *indentedWriter) Write(p []byte) (int, error) {
	// We honour the byte contract by returning len(p) on success even
	// though we may write more bytes underneath (the prefixes).
	total := 0
	for _, b := range p {
		if iw.atBOL {
			if _, err := iw.w.Write(iw.prefix); err != nil {
				return total, err
			}
			iw.atBOL = false
		}
		n, err := iw.w.Write([]byte{b})
		total += n
		if err != nil {
			return total, err
		}
		if b == '\n' {
			iw.atBOL = true
		}
	}
	return len(p), nil
}

// indent2 returns a writer that prefixes every line with two spaces.
// Used to nest the per-bucket block under "  #<tag>" in --by-tag mode.
