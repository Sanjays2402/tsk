package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// runDependPending implements the `tsk depend --pending` view: the
// "now-unblocked notification queue".
//
// Semantics — what counts as "pending"?
//
//	A task is pending if ALL of these hold:
//	  1. It is open (not done).
//	  2. It has at least one DependsOn entry (otherwise it was always
//	     actionable; not interesting for this view).
//	  3. None of its DependsOn entries are still open — every prereq
//	     is either done or dangling (per unmetBlockers' policy).
//	  4. AT LEAST ONE of its done prereqs was completed within the
//	     --since window (default 24h). Long-since-unblocked tasks are
//	     boring noise here; we want the FRESH unblocks.
//
// The user story this serves: "I just closed a batch of prereqs.
// Which tasks just became actionable?" Or, morning standup: "what
// got unblocked overnight while I was asleep?" — a much better
// answer than "every task that happens to be unblocked right now"
// (which includes work that's been waiting on you for weeks).
//
// Filtering policy:
//   - Waiting tasks (wait:<future date>) are excluded — they're
//     hidden in default views; surfacing them in a "newly actionable"
//     review would suggest work the user explicitly deferred.
//   - Done prereqs without a Completed timestamp (hand-edited) are
//     IGNORED for the recency check but still count as "satisfied".
//     They don't prove RECENT unblocking, so a task with only those
//     as deps won't appear unless some OTHER dep was recently closed.
//     This is conservative: better to under-report than incorrectly
//     flag stale unblocks as new.
//
// Sort order: most-recent unblocking completion FIRST. That mirrors
// `tsk log`'s newest-first ordering — the freshest unblocks at the
// top is what the user wants when scanning.
//
// Each row annotates which prereq's completion was the unblocking
// trigger (the most-recent done dep's id + when), so the user
// understands the "why now?" without a follow-up.
func runDependPending(w io.Writer, s *store.Store, sinceRaw string, asJSON bool) error {
	sinceDur, err := parsePendingSince(sinceRaw)
	if err != nil {
		return err
	}
	now := time.Now()
	rows := collectPendingRows(s, now, sinceDur)
	if asJSON {
		return emitPendingJSON(w, rows)
	}
	return emitPendingPlain(w, rows, sinceDur)
}

// parsePendingSince validates and parses the --since flag value.
// Reuses parseDurationLocal (the same parser `tsk log`/`tsk stats`
// use) so 7d, 2w, 1h30m etc all work consistently across the CLI.
// Zero/negative durations are a usage error — the whole point of
// --pending is the recency window, so an empty window is invalid.
func parsePendingSince(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		// Empty means "default" — same as 24h. Treat empty as 0
		// here and fall back; cmd default already sets "24h" so
		// this branch is defensive.
		return 24 * time.Hour, nil
	}
	d, err := parseDurationLocal(raw)
	if err != nil {
		return 0, usageErrorf("invalid --since %q: %v", raw, err)
	}
	if d <= 0 {
		return 0, usageErrorf("--since must be a positive duration, got %q", raw)
	}
	return d, nil
}

// pendingRow describes one task that just became actionable, plus
// the prereq whose completion triggered the unblocking.
//
// TriggerID is the id of the most-recently-completed prereq —
// "this is why the task is on this list". TriggerCompleted is its
// completion time. Title is the task's own title; the trigger's
// title is not included in the structured row to keep the schema
// tight (callers can `tsk show <trigger>` if needed).
type pendingRow struct {
	ID               int       `json:"id"`
	Title            string    `json:"title"`
	Priority         string    `json:"priority"`
	TriggerID        int       `json:"trigger_id"`
	TriggerCompleted time.Time `json:"trigger_completed"`
}

// collectPendingRows scans the store and returns every task that
// matches the pending criteria, sorted newest trigger first.
func collectPendingRows(s *store.Store, now time.Time, since time.Duration) []pendingRow {
	cutoff := now.Add(-since)
	out := make([]pendingRow, 0)
	for _, t := range s.Tasks {
		if t.Done {
			continue
		}
		if !t.HasDependencies() {
			continue
		}
		if t.IsWaiting(now) {
			continue
		}
		t := t
		if len(unmetBlockers(s, &t, nil)) > 0 {
			continue
		}
		// Find the most-recently-completed prereq. We only care
		// about prereqs WITH a Completed timestamp (timestamp-less
		// done deps don't prove RECENT unblocking).
		var triggerID int
		var triggerAt time.Time
		for _, dep := range t.DependsOn {
			bt := s.ByID(dep)
			if bt == nil {
				continue
			}
			if !bt.Done || bt.Completed == nil {
				continue
			}
			if bt.Completed.After(triggerAt) {
				triggerAt = *bt.Completed
				triggerID = bt.ID
			}
		}
		// No timestamped done prereq → can't prove recent unblock.
		// Skip rather than guess.
		if triggerID == 0 {
			continue
		}
		// Trigger must be inside the --since window.
		if triggerAt.Before(cutoff) {
			continue
		}
		out = append(out, pendingRow{
			ID:               t.ID,
			Title:            t.Title,
			Priority:         t.Priority.String(),
			TriggerID:        triggerID,
			TriggerCompleted: triggerAt,
		})
	}
	// Newest trigger first; stable tie-break by id so output is
	// deterministic when two tasks were unblocked by the same close.
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].TriggerCompleted.Equal(out[j].TriggerCompleted) {
			return out[i].TriggerCompleted.After(out[j].TriggerCompleted)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// emitPendingPlain renders rows as a readable list. The header
// summarises the window (so the user understands why they got these
// N tasks and not more); each row annotates which prereq's
// completion triggered the unblock + how long ago.
//
// Empty result gets an explicit message — silent output would be
// ambiguous for a "what's new?" query.
func emitPendingPlain(w io.Writer, rows []pendingRow, since time.Duration) error {
	if len(rows) == 0 {
		pf(w, "no tasks freshly unblocked in the last %s\n", humanizeDuration(since))
		return nil
	}
	loc := PacificLoc()
	pf(w, "freshly unblocked (last %s): %d task(s)\n", humanizeDuration(since), len(rows))
	for _, r := range rows {
		when := r.TriggerCompleted.In(loc).Format("2006-01-02 15:04")
		pf(w, "  #%d  %s  (unblocked by #%d at %s)\n", r.ID, r.Title, r.TriggerID, when)
	}
	return nil
}

// emitPendingJSON renders the rows array verbatim. Empty stays `[]`
// (not null) so consumers iterating with `.length` work cleanly.
func emitPendingJSON(w io.Writer, rows []pendingRow) error {
	if rows == nil {
		rows = []pendingRow{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// humanizeDuration formats a duration as a short human-readable
// string. Coarse buckets only — the window is for a human-facing
// header, not log analytics. Goes for "24h", "7d", "1h30m" shapes
// matching what the user would TYPE for --since.
func humanizeDuration(d time.Duration) string {
	if d >= 24*time.Hour && d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	}
	if d >= time.Hour && d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	if d >= time.Minute && d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	// Fall back to Go's String(); covers compound forms like 1h30m.
	return d.String()
}
