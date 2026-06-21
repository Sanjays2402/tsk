package commands

import (
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newDailyCmd implements `tsk daily`: a synthesized morning briefing.
//
// One screen, three sections, no filler:
//
//	OVERDUE (N)
//	  #3 [H] fix release pipeline   due:2026-06-18  (2 days late)
//	TODAY (N)
//	  #5 [U] ship daily command     due:2026-06-20
//	UPCOMING (N)
//	  #7 [H] open-source draft      due:2026-06-21
//	  #9 [M] tweet about it         due:2026-06-23
//
// Each section is sorted by the same canonical tie-break used by
// `tsk top`/`tsk next` (priority desc, dated-first, earliest-due, lower
// id) so the most important thing in each bucket is always on top.
//
// Defaults:
//   - --upcoming N    cap upcoming section to N rows (default 3)
//   - --no-tags       suppress tag display (tighter for narrow terms)
//   - --json          stable schema { overdue, today, upcoming } so the
//     three buckets don't get flattened into one array
//
// Empty sections are still rendered (with a "(none)" placeholder in
// plain mode) so the user always sees the three slots and notices a
// missing one immediately.
func newDailyCmd() *cobra.Command {
	var (
		upcomingN int
		tag       string
		noTags    bool
		asJSON    bool
		format    string
	)
	cmd := &cobra.Command{
		Use:     "daily",
		Aliases: []string{"morning", "today-plan"},
		Short:   "Morning briefing: overdue + due today + top upcoming, in one screen",
		Long: `Synthesized morning briefing in one screen.

Three sections:
  OVERDUE   undone tasks past their due date (most important first)
  TODAY     undone tasks due today (most important first)
  UPCOMING  next N undone tasks due in the future (default 3)

All sections sort by priority desc, then earliest-due, then ID — so
the top of each bucket is always the most important thing in it.

  --upcoming N    cap upcoming to N rows (default 3; 0 to hide section)
  --tag T         restrict every section to tasks tagged T
  --no-tags       suppress tag display (narrower output)
  --json          emit a stable schema {overdue, today, upcoming}

Examples:
  tsk daily
  tsk daily --upcoming 5
  tsk daily --tag work
  tsk daily --json | jq '.overdue | length'
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if upcomingN < 0 {
				return usageErrorf("--upcoming must be >= 0, got %d", upcomingN)
			}
			outFormat, err := resolveLsFormat(format, asJSON)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			plan := buildDailyPlan(s.Tasks, time.Now(), tag, upcomingN)
			return emitDailyPlan(cmd.OutOrStdout(), plan, noTags, outFormat)
		},
	}
	cmd.Flags().IntVar(&upcomingN, "upcoming", 3, "cap upcoming section to N rows (0 to hide)")
	cmd.Flags().StringVar(&tag, "tag", "", "restrict every section to tasks tagged T")
	cmd.Flags().BoolVar(&noTags, "no-tags", false, "suppress tag display in plain output")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().StringVar(&format, "format", "", "output format: plain, table, or json")
	return cmd
}

// DailyPlan is the JSON-stable struct emitted by --json. Fields are
// arrays (never nil) so jq pipelines can always count them.
type DailyPlan struct {
	Overdue  []model.Task `json:"overdue"`
	Today    []model.Task `json:"today"`
	Upcoming []model.Task `json:"upcoming"`
}

// buildDailyPlan groups the undone tasks into the three buckets,
// sorts each by the canonical tie-break, and caps upcoming to N.
//
// Filtering: only undone tasks are considered (overdue + today + a
// briefing is for "what's next", not history). Optional --tag narrows
// every bucket consistently.
func buildDailyPlan(tasks []model.Task, now time.Time, tag string, upcomingLimit int) DailyPlan {
	tag = strings.TrimSpace(tag)
	plan := DailyPlan{
		Overdue:  []model.Task{},
		Today:    []model.Task{},
		Upcoming: []model.Task{},
	}
	for _, t := range tasks {
		if t.Done {
			continue
		}
		if tag != "" && !t.HasTag(tag) {
			continue
		}
		switch {
		case t.IsDueToday(now):
			// Check today BEFORE overdue: a task literally dated today
			// should always land in TODAY, even if its stored UTC midnight
			// would compare as "before local startOfDay" (the markdown
			// store keeps dates as UTC midnight; the current TZ may offset
			// it back across the boundary).
			plan.Today = append(plan.Today, t)
		case t.IsOverdue(now):
			plan.Overdue = append(plan.Overdue, t)
		case t.IsUpcoming(now):
			plan.Upcoming = append(plan.Upcoming, t)
		}
	}
	sortDailyBucket(plan.Overdue)
	sortDailyBucket(plan.Today)
	sortDailyBucket(plan.Upcoming)
	// --upcoming 0 hides the section entirely (truncate to zero). The
	// docstring promises this; users who want "no limit" should pick a
	// big number — daily is intentionally one-screen.
	if upcomingLimit >= 0 && len(plan.Upcoming) > upcomingLimit {
		plan.Upcoming = plan.Upcoming[:upcomingLimit]
	}
	return plan
}

// sortDailyBucket applies the canonical tie-break: higher priority
// first, then dated-first, then earliest-due, then lower ID. Same
// ordering as `tsk top` / `tsk next` so the top item agrees.
func sortDailyBucket(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		switch {
		case a.Due != nil && b.Due == nil:
			return true
		case a.Due == nil && b.Due != nil:
			return false
		case a.Due != nil && b.Due != nil:
			if !a.Due.Equal(*b.Due) {
				return a.Due.Before(*b.Due)
			}
		}
		return a.ID < b.ID
	})
}

// emitDailyPlan dispatches per format.
func emitDailyPlan(w io.Writer, plan DailyPlan, noTags bool, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	case "table":
		return printDailyPlanTable(w, plan)
	default:
		return printDailyPlanPlain(w, plan, noTags)
	}
}

// printDailyPlanPlain renders the three-section briefing.
func printDailyPlanPlain(w io.Writer, plan DailyPlan, noTags bool) error {
	now := time.Now()
	loc := ResolveTZ()
	todayStart := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	printDailySection(w, "OVERDUE", plan.Overdue, noTags, todayStart, true)
	printDailySection(w, "TODAY", plan.Today, noTags, todayStart, false)
	printDailySection(w, "UPCOMING", plan.Upcoming, noTags, todayStart, false)
	return nil
}

// printDailySection writes one labelled bucket. If showOverdueLag is
// set, an "(N day(s) late)" suffix is appended per row.
func printDailySection(w io.Writer, label string, rows []model.Task, noTags bool, todayStart time.Time, showOverdueLag bool) {
	pf(w, "%s (%d)\n", label, len(rows))
	if len(rows) == 0 {
		pln(w, "  (none)")
		return
	}
	for _, t := range rows {
		line := "  #" + strconv.Itoa(t.ID) + " [" + t.Priority.Short() + "] " + t.Title
		if t.Due != nil {
			line += "  due:" + t.Due.Format(model.DateLayout)
			if showOverdueLag {
				days := daysLate(*t.Due, todayStart)
				if days > 0 {
					if days == 1 {
						line += "  (1 day late)"
					} else {
						line += "  (" + strconv.Itoa(days) + " days late)"
					}
				}
			}
		}
		if !noTags && len(t.Tags) > 0 {
			line += "  #" + strings.Join(t.Tags, " #")
		}
		pln(w, line)
	}
}

// daysLate returns how many whole days have elapsed since `due` until
// the start of today. Returns 0 if the due date is today or future.
func daysLate(due, todayStart time.Time) int {
	// Normalize due to the start of its day so partial-day diffs don't
	// off-by-one the count.
	dueStart := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, todayStart.Location())
	diff := todayStart.Sub(dueStart)
	if diff <= 0 {
		return 0
	}
	return int(diff / (24 * time.Hour))
}

// printDailyPlanTable renders all three buckets as separate small tables
// for terminals that prefer fixed columns. Each section is printed with
// its label as a header, blank line between.
func printDailyPlanTable(w io.Writer, plan DailyPlan) error {
	sections := []struct {
		label string
		rows  []model.Task
	}{
		{"OVERDUE", plan.Overdue},
		{"TODAY", plan.Today},
		{"UPCOMING", plan.Upcoming},
	}
	for i, s := range sections {
		if i > 0 {
			pln(w, "")
		}
		pf(w, "%s (%d)\n", s.label, len(s.rows))
		if len(s.rows) == 0 {
			pln(w, "  (none)")
			continue
		}
		if err := printTasksTable(w, s.rows); err != nil {
			return err
		}
	}
	return nil
}
