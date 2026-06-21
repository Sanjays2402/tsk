package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newWhyCmd implements `tsk why <id>`: the chronological provenance
// trail for one task — every timestamp tsk knows about, ordered by
// when it happened.
//
// `tsk show <id>` is the snapshot view (every field labelled). `tsk
// why <id>` is the timeline view (every event in order). They answer
// different questions:
//
//	show: "what does this task look like right now?"
//	why:  "what's the story of this task?"
//
// Events surfaced (in temporal order):
//
//   - created     when the task was first added
//   - started     when start: was stamped (in-progress flag)
//   - waited      when wait: was set (if WaitUntil is in the past,
//     \"expired\"; if future, \"hidden until\")
//   - due         when the deadline is/was (overdue/today/upcoming)
//   - completed   when done: was marked
//
// Output is a small ASCII timeline:
//
//	#3  refactor parser
//
//	  2026-06-15 09:21:00 -0700  created
//	  2026-06-17 14:02:11 -0700  started
//	  2026-06-19 23:59:59 -0700  due (today)
//	  ...
//
// With --json the same events come back as a stable {events: [...]}
// array so consumers can render their own UI without parsing humanized
// strings.
//
// Why not just extend `tsk show`? Because show's contract is "labelled
// fields, stable lines, scriptable with grep" — adding ordered events
// to it would break the field-by-field layout consumers depend on.
func newWhyCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "why <id>",
		Short: "Show the chronological event trail of a single task",
		Long: `Show the timeline of a task — every timestamp tsk knows about, ordered
by when it happened.

Sibling of 'tsk show' (the field snapshot) and 'tsk diff' (the file
delta). Where show answers "what is this task?", why answers "what's
the story of this task?".

Examples:
  tsk why 3                # plain timeline
  tsk why 3 --json         # stable JSON for scripts
  tsk why 3 --json | jq -r '.events[] | "\(.at) \(.kind)"'
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			events := buildTaskTimeline(*t, time.Now())
			if asJSON {
				return emitWhyJSON(cmd.OutOrStdout(), *t, events)
			}
			printWhyTimeline(cmd.OutOrStdout(), *t, events)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit stable JSON {task, events}")
	return cmd
}

// taskEvent is one row in the timeline. Detail is a short human note
// ("today", "expired", "hidden until ...") suitable for the plain
// renderer; consumers using --json can ignore it.
type taskEvent struct {
	At     time.Time `json:"-"`
	AtStr  string    `json:"at"`   // RFC3339-ish display string
	Kind   string    `json:"kind"` // created | started | waited | due | completed
	Detail string    `json:"detail,omitempty"`
}

// whyJSONDoc is the stable schema for --json. Task is the full task
// object (so consumers don't need a second call to get title/tags);
// events is the timeline.
type whyJSONDoc struct {
	Task   model.Task  `json:"task"`
	Events []taskEvent `json:"events"`
}

// buildTaskTimeline emits one event per known timestamp on the task,
// in chronological order. Optional fields (started, completed, etc.)
// are omitted when absent — empty rows would just be noise.
func buildTaskTimeline(t model.Task, now time.Time) []taskEvent {
	events := make([]taskEvent, 0, 5)
	if !t.Created.IsZero() {
		events = append(events, taskEvent{
			At:    t.Created,
			AtStr: t.Created.Format("2006-01-02 15:04:05 -0700"),
			Kind:  "created",
		})
	}
	if t.Started != nil {
		events = append(events, taskEvent{
			At:    *t.Started,
			AtStr: t.Started.Format("2006-01-02 15:04:05 -0700"),
			Kind:  "started",
		})
	}
	if t.WaitUntil != nil {
		detail := "hidden until " + t.WaitUntil.Format(model.DateLayout)
		if !t.WaitUntil.After(now) {
			detail = "expired " + t.WaitUntil.Format(model.DateLayout)
		}
		events = append(events, taskEvent{
			At:     *t.WaitUntil,
			AtStr:  t.WaitUntil.Format(model.DateLayout),
			Kind:   "waited",
			Detail: detail,
		})
	}
	if t.Due != nil {
		detail := dueDetail(t, now)
		events = append(events, taskEvent{
			At:     *t.Due,
			AtStr:  t.Due.Format(model.DateLayout),
			Kind:   "due",
			Detail: detail,
		})
	}
	if t.Completed != nil {
		events = append(events, taskEvent{
			At:    *t.Completed,
			AtStr: t.Completed.Format("2006-01-02 15:04:05 -0700"),
			Kind:  "completed",
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].At.Before(events[j].At)
	})
	return events
}

// dueDetail formats the due-date relative annotation. Done tasks lose
// the overdue framing — "due (overdue)" on a done task is misleading.
func dueDetail(t model.Task, now time.Time) string {
	if t.Done {
		return ""
	}
	switch {
	case t.IsOverdue(now):
		return "overdue"
	case t.IsDueToday(now):
		return "today"
	case t.IsUpcoming(now):
		return "upcoming"
	}
	return ""
}

// printWhyTimeline renders the plain ASCII timeline.
func printWhyTimeline(w io.Writer, t model.Task, events []taskEvent) {
	status := "open"
	if t.Done {
		status = "done"
	}
	pf(w, "#%d  %s  (%s, %s)\n", t.ID, t.Title, status, t.Priority)
	if len(t.Tags) > 0 {
		pf(w, "      tags: #%s\n", strings.Join(t.Tags, " #"))
	}
	pln(w)
	if len(events) == 0 {
		pln(w, "  (no timestamped events — hand-edited task without created:)")
		return
	}
	for _, e := range events {
		line := fmt.Sprintf("  %s  %s", e.AtStr, e.Kind)
		if e.Detail != "" {
			line += "  (" + e.Detail + ")"
		}
		pln(w, line)
	}
}

// emitWhyJSON encodes the stable {task, events} document.
func emitWhyJSON(w io.Writer, t model.Task, events []taskEvent) error {
	doc := whyJSONDoc{Task: t, Events: events}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
