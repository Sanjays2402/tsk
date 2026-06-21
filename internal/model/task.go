// Package model defines the core task data types used across tsk.
package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Priority represents task urgency. Zero value is PriorityMedium.
type Priority int

// Priority levels in ascending order of urgency.
const (
	PriorityLow Priority = iota
	PriorityMedium
	PriorityHigh
	PriorityUrgent
)

// DateLayout is the canonical date format used for due dates in storage and CLI input.
const DateLayout = "2006-01-02"

// String returns the lowercase name of the priority.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityUrgent:
		return "urgent"
	default:
		return "medium"
	}
}

// Short returns a single-character priority marker.
func (p Priority) Short() string {
	switch p {
	case PriorityLow:
		return "L"
	case PriorityMedium:
		return "M"
	case PriorityHigh:
		return "H"
	case PriorityUrgent:
		return "U"
	default:
		return "M"
	}
}

// ParsePriority resolves a string (case-insensitive, short or long form) to a Priority.
// Returns an error if the value is not recognized.
func ParsePriority(s string) (Priority, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "medium", "med", "m":
		return PriorityMedium, nil
	case "low", "l":
		return PriorityLow, nil
	case "high", "h":
		return PriorityHigh, nil
	case "urgent", "u", "critical":
		return PriorityUrgent, nil
	}
	return PriorityMedium, fmt.Errorf("unknown priority %q", s)
}

// Task is a single todo item parsed from (or written to) a markdown store.
type Task struct {
	ID        int
	Title     string
	Done      bool
	Pinned    bool
	Priority  Priority
	Due       *time.Time
	WaitUntil *time.Time
	Tags      []string
	Notes     string
	Created   time.Time
	Started   *time.Time
	Completed *time.Time
	// DependsOn is the set of task IDs this task is blocked by. A
	// task can only be marked done when every id in DependsOn refers
	// to a done task in the same store. Persisted as a comma list in
	// the meta block under the `depends:` key. Strictly additive:
	// old files round-trip unchanged when the slice is empty.
	DependsOn []int
}

// HasDependencies reports whether this task is blocked on at least
// one other task by id (regardless of whether those tasks are still
// open or done).
func (t *Task) HasDependencies() bool { return len(t.DependsOn) > 0 }

// HasDue reports whether the task has a due date set.
func (t *Task) HasDue() bool { return t.Due != nil }

// IsOverdue reports whether the task is undone and the due date is before today.
func (t *Task) IsOverdue(now time.Time) bool {
	if t.Done || t.Due == nil {
		return false
	}
	return t.Due.Before(startOfDay(now))
}

// IsDueToday reports whether the due date falls on now's calendar day.
func (t *Task) IsDueToday(now time.Time) bool {
	if t.Due == nil {
		return false
	}
	return sameDay(*t.Due, now)
}

// IsUpcoming reports whether the due date is strictly after today.
func (t *Task) IsUpcoming(now time.Time) bool {
	if t.Due == nil {
		return false
	}
	return t.Due.After(endOfDay(now))
}

// HasTag reports whether the task contains the given tag (case-insensitive).
func (t *Task) HasTag(tag string) bool {
	tag = strings.ToLower(strings.TrimSpace(tag))
	for _, x := range t.Tags {
		if strings.EqualFold(x, tag) {
			return true
		}
	}
	return false
}

// NormalizeTags trims, lowercases and de-duplicates tags in place.
func (t *Task) NormalizeTags() {
	seen := make(map[string]struct{}, len(t.Tags))
	out := t.Tags[:0]
	for _, tag := range t.Tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	sort.Strings(out)
	t.Tags = out
}

// IsWaiting reports whether the task has a wait-until date in the future
// (relative to now). Tasks waiting in the future are hidden from default
// views — they only surface explicitly via `tsk ls --all` / `tsk wait
// --list` / direct `tsk show <id>` lookup, or once the wait-until date
// has passed.
func (t *Task) IsWaiting(now time.Time) bool {
	if t.WaitUntil == nil {
		return false
	}
	return t.WaitUntil.After(now)
}

// IsInProgress reports whether the task is currently being worked on:
// has a Started timestamp, is not yet Done. Cleared by `tsk done` (the
// task moved past in-progress) and by `tsk stop` (the user actively
// paused). Stays true if you stop and then `tsk start` it again — the
// timestamp resets to the newer start.
func (t *Task) IsInProgress() bool {
	return t.Started != nil && !t.Done
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// SortBy sorts tasks in place according to the named strategy.
// Recognized values: "priority", "due", "created", "id". Unknown values default to "id".
func SortBy(tasks []Task, strategy string) {
	switch strings.ToLower(strategy) {
	case "priority":
		sort.SliceStable(tasks, func(i, j int) bool {
			if tasks[i].Priority != tasks[j].Priority {
				return tasks[i].Priority > tasks[j].Priority
			}
			return tasks[i].ID < tasks[j].ID
		})
	case "due":
		sort.SliceStable(tasks, func(i, j int) bool {
			ai, aj := tasks[i].Due, tasks[j].Due
			switch {
			case ai == nil && aj == nil:
				return tasks[i].ID < tasks[j].ID
			case ai == nil:
				return false
			case aj == nil:
				return true
			default:
				return ai.Before(*aj)
			}
		})
	case "created":
		sort.SliceStable(tasks, func(i, j int) bool {
			return tasks[i].Created.Before(tasks[j].Created)
		})
	default:
		sort.SliceStable(tasks, func(i, j int) bool {
			return tasks[i].ID < tasks[j].ID
		})
	}
}
