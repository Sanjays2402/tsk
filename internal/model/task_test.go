package model

import (
	"testing"
	"time"
)

func TestParsePriority(t *testing.T) {
	tests := []struct {
		in   string
		want Priority
		err  bool
	}{
		{"low", PriorityLow, false},
		{"LOW", PriorityLow, false},
		{"medium", PriorityMedium, false},
		{"", PriorityMedium, false},
		{"m", PriorityMedium, false},
		{"high", PriorityHigh, false},
		{"urgent", PriorityUrgent, false},
		{"critical", PriorityUrgent, false},
		{"wat", PriorityMedium, true},
	}
	for _, tt := range tests {
		got, err := ParsePriority(tt.in)
		if (err != nil) != tt.err {
			t.Errorf("ParsePriority(%q) err=%v want err=%v", tt.in, err, tt.err)
		}
		if !tt.err && got != tt.want {
			t.Errorf("ParsePriority(%q) = %v want %v", tt.in, got, tt.want)
		}
	}
}

func TestTaskDueBuckets(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	overdue := Task{Due: &yesterday}
	today := Task{Due: &now}
	upcoming := Task{Due: &tomorrow}
	none := Task{}

	if !overdue.IsOverdue(now) {
		t.Error("expected overdue")
	}
	if today.IsOverdue(now) {
		t.Error("today should not be overdue")
	}
	if !today.IsDueToday(now) {
		t.Error("expected today")
	}
	if !upcoming.IsUpcoming(now) {
		t.Error("expected upcoming")
	}
	if none.IsOverdue(now) || none.IsDueToday(now) || none.IsUpcoming(now) {
		t.Error("no-due task should not match any bucket")
	}

	done := Task{Done: true, Due: &yesterday}
	if done.IsOverdue(now) {
		t.Error("done task cannot be overdue")
	}
}

func TestNormalizeTags(t *testing.T) {
	task := Task{Tags: []string{"Home", "home", " WORK ", "", "errand"}}
	task.NormalizeTags()
	want := []string{"errand", "home", "work"}
	if len(task.Tags) != len(want) {
		t.Fatalf("got %v want %v", task.Tags, want)
	}
	for i, v := range want {
		if task.Tags[i] != v {
			t.Fatalf("got %v want %v", task.Tags, want)
		}
	}
}

func TestPriorityStringAndShort(t *testing.T) {
	cases := []struct {
		p     Priority
		long  string
		short string
	}{
		{PriorityLow, "low", "L"},
		{PriorityMedium, "medium", "M"},
		{PriorityHigh, "high", "H"},
		{PriorityUrgent, "urgent", "U"},
		{Priority(99), "medium", "M"},
	}
	for _, c := range cases {
		if c.p.String() != c.long {
			t.Errorf("%d.String()=%q want %q", c.p, c.p.String(), c.long)
		}
		if c.p.Short() != c.short {
			t.Errorf("%d.Short()=%q want %q", c.p, c.p.Short(), c.short)
		}
	}
}

func TestHasTagAndDue(t *testing.T) {
	tm := time.Now()
	task := Task{Tags: []string{"home", "work"}, Due: &tm}
	if !task.HasTag("HOME") {
		t.Error("HasTag failed")
	}
	if task.HasTag("nope") {
		t.Error("HasTag false positive")
	}
	if !task.HasDue() {
		t.Error("HasDue")
	}
	empty := Task{}
	if empty.HasDue() {
		t.Error("empty HasDue")
	}
}

func TestSortBy(t *testing.T) {
	a := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
	b := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	tasks := []Task{
		{ID: 1, Priority: PriorityLow, Due: &b},
		{ID: 2, Priority: PriorityUrgent, Due: nil},
		{ID: 3, Priority: PriorityHigh, Due: &a},
	}
	cp := append([]Task(nil), tasks...)
	SortBy(cp, "priority")
	if cp[0].ID != 2 || cp[1].ID != 3 || cp[2].ID != 1 {
		t.Errorf("priority sort wrong: %v", cp)
	}
	cp = append([]Task(nil), tasks...)
	SortBy(cp, "due")
	if cp[0].ID != 3 || cp[1].ID != 1 || cp[2].ID != 2 {
		t.Errorf("due sort wrong: %v", cp)
	}
	cp = append([]Task(nil), tasks...)
	SortBy(cp, "id")
	if cp[0].ID != 1 || cp[2].ID != 3 {
		t.Errorf("id sort wrong: %v", cp)
	}
}
