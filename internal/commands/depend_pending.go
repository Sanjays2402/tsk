package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
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
//   - --tag narrows the pending queue to tasks carrying the named
//     tag (case-insensitive, single tag — same shape as
//     `tsk ls --tag`). Useful for "what unblocked overnight on my
//     work projects?" without seeing personal tasks in the same
//     feed. Empty value = no tag filter.
//   - --priority narrows the queue to a single priority level
//     (low/medium/high/urgent — same parser ls/top/add use).
//     Composes with --tag as an INTERSECTION: tasks must match BOTH.
//     Useful for "what's freshly unblocked AND high-priority?"
//     — the most-actionable subset of the freshly-unblocked feed.
//     Empty value = no priority filter (mirroring --tag's
//     defensive shell-var-typo policy).
//
// Sort order: most-recent unblocking completion FIRST. That mirrors
// `tsk log`'s newest-first ordering — the freshest unblocks at the
// top is what the user wants when scanning.
//
// Each row annotates which prereq's completion was the unblocking
// trigger (the most-recent done dep's id + when), so the user
// understands the "why now?" without a follow-up.
func runDependPending(w io.Writer, s *store.Store, sinceRaw, tag, priorityRaw string, asJSON bool) error {
	sinceDur, err := parsePendingSince(sinceRaw)
	if err != nil {
		return err
	}
	prio, prioActive, err := parsePendingPriority(priorityRaw)
	if err != nil {
		return err
	}
	now := time.Now()
	rows := collectPendingRows(s, now, sinceDur, tag, prio, prioActive)
	if asJSON {
		return emitPendingJSON(w, rows)
	}
	return emitPendingPlain(w, rows, sinceDur, tag, priorityRaw, prioActive)
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

// parsePendingPriority resolves the --priority flag string to a
// model.Priority value, returning prioActive=false on empty input
// so the caller can skip the filter cleanly. Mirrors --tag's
// "empty value behaves like no filter" stance: defensive against
// an unset shell variable that leaves --priority="".
//
// Invalid values are surfaced as a usage error (exit-2) with the
// list of valid names, so a typo doesn't silently degrade to "no
// filter" (which would be confusing — the user clearly meant
// something).
func parsePendingPriority(raw string) (model.Priority, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false, nil
	}
	p, err := model.ParsePriority(raw)
	if err != nil {
		return 0, false, usageErrorf("invalid --priority %q: %v", raw, err)
	}
	return p, true, nil
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
// matches the pending criteria, sorted newest trigger first. When
// tag is non-empty, results are restricted to tasks carrying that
// tag (case-insensitive via Task.HasTag). When prioActive is true,
// results are further narrowed to tasks at exactly prio (the
// canonical exact-match on priority — same semantics ls/top use).
func collectPendingRows(s *store.Store, now time.Time, since time.Duration, tag string, prio model.Priority, prioActive bool) []pendingRow {
	cutoff := now.Add(-since)
	tag = strings.TrimSpace(tag)
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
		if tag != "" && !t.HasTag(tag) {
			continue
		}
		if prioActive && t.Priority != prio {
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
//
// Active filters are reflected in the header AND in the empty
// message so the user understands WHY they got a narrower or
// empty result — silent filter-induced emptiness is hostile.
func emitPendingPlain(w io.Writer, rows []pendingRow, since time.Duration, tag, priorityRaw string, prioActive bool) error {
	tag = strings.TrimSpace(tag)
	filters := buildPendingFilterSummary(tag, priorityRaw, prioActive)
	if len(rows) == 0 {
		if filters != "" {
			pf(w, "no tasks freshly unblocked in the last %s (%s)\n", humanizeDuration(since), filters)
		} else {
			pf(w, "no tasks freshly unblocked in the last %s\n", humanizeDuration(since))
		}
		return nil
	}
	loc := PacificLoc()
	if filters != "" {
		pf(w, "freshly unblocked (last %s, %s): %d task(s)\n", humanizeDuration(since), filters, len(rows))
	} else {
		pf(w, "freshly unblocked (last %s): %d task(s)\n", humanizeDuration(since), len(rows))
	}
	for _, r := range rows {
		when := r.TriggerCompleted.In(loc).Format("2006-01-02 15:04")
		pf(w, "  #%d  %s  (unblocked by #%d at %s)\n", r.ID, r.Title, r.TriggerID, when)
	}
	return nil
}

// buildPendingFilterSummary produces the "tag=X, priority=Y" trailer
// that appears in headers and empty-state messages. Order is
// deterministic (tag, then priority) so output is reproducible
// across invocations. Empty → empty string (no trailing comma).
//
// Lowercases the priority for display so output is consistent
// regardless of how the user typed it on the command line.
func buildPendingFilterSummary(tag, priorityRaw string, prioActive bool) string {
	parts := make([]string, 0, 2)
	if tag != "" {
		parts = append(parts, "tag="+tag)
	}
	if prioActive {
		parts = append(parts, "priority="+strings.ToLower(strings.TrimSpace(priorityRaw)))
	}
	return strings.Join(parts, ", ")
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
