package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// sectionKind identifies one of the five collapsible task groupings rendered by the TUI.
type sectionKind int

// Section kinds in render order.
const (
	sectionOverdue sectionKind = iota
	sectionToday
	sectionUpcoming
	sectionNoDue
	sectionDone
)

func (k sectionKind) label() string {
	switch k {
	case sectionOverdue:
		return "Overdue"
	case sectionToday:
		return "Today"
	case sectionUpcoming:
		return "Upcoming"
	case sectionNoDue:
		return "No Due"
	case sectionDone:
		return "Done"
	}
	return "?"
}

// groupedTasks assigns tasks to their display section.
func groupedTasks(tasks []model.Task, now time.Time) map[sectionKind][]model.Task {
	out := map[sectionKind][]model.Task{
		sectionOverdue:  {},
		sectionToday:    {},
		sectionUpcoming: {},
		sectionNoDue:    {},
		sectionDone:     {},
	}
	for _, t := range tasks {
		switch {
		case t.Done:
			out[sectionDone] = append(out[sectionDone], t)
		case t.IsOverdue(now):
			out[sectionOverdue] = append(out[sectionOverdue], t)
		case t.IsDueToday(now):
			out[sectionToday] = append(out[sectionToday], t)
		case t.IsUpcoming(now):
			out[sectionUpcoming] = append(out[sectionUpcoming], t)
		default:
			out[sectionNoDue] = append(out[sectionNoDue], t)
		}
	}
	for k := range out {
		model.SortBy(out[k], "priority")
	}
	return out
}

func progressBar(done, total, width int) string {
	if total == 0 {
		return fmt.Sprintf("[%s] 0%% · 0/0 done", strings.Repeat("░", width))
	}
	pct := float64(done) / float64(total)
	fill := int(pct * float64(width))
	if fill > width {
		fill = width
	}
	return fmt.Sprintf("[%s%s] %.0f%% · %d/%d done",
		strings.Repeat("█", fill),
		strings.Repeat("░", width-fill),
		pct*100, done, total)
}

func priorityLabel(p model.Priority, pal Palette) string {
	switch p {
	case model.PriorityUrgent:
		return pal.Urgent.Render("●U")
	case model.PriorityHigh:
		return pal.High.Render("●H")
	case model.PriorityMedium:
		return pal.Medium.Render("●M")
	case model.PriorityLow:
		return pal.Low.Render("●L")
	}
	return "●M"
}

func checkbox(done bool) string {
	if done {
		return "[x]"
	}
	return "[ ]"
}
