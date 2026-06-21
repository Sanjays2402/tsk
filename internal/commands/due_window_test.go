package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTodayShowsDueToday(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().Format("2006-01-02")
	if _, _, err := runCmd(t, dir, "add", "due today", "-d", today); err != nil {
		t.Fatalf("add today: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "due never"); err != nil {
		t.Fatalf("add never: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "today")
	if err != nil {
		t.Fatalf("today: %v", err)
	}
	if !strings.Contains(stdout, "due today") {
		t.Fatalf("expected today's task, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "due never") {
		t.Fatalf("undated task leaked into today, got:\n%s", stdout)
	}
}

func TestTodayJSON(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().Format("2006-01-02")
	if _, _, err := runCmd(t, dir, "add", "today task", "-d", today); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "today", "--json")
	if err != nil {
		t.Fatalf("today --json: %v", err)
	}
	var tasks []map[string]any
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0]["Title"] != "today task" {
		t.Fatalf("expected 1 task 'today task', got %v", tasks)
	}
}

func TestTodayWithTag(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().Format("2006-01-02")
	if _, _, err := runCmd(t, dir, "add", "work today", "-d", today, "-t", "work"); err != nil {
		t.Fatalf("add work: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home today", "-d", today, "-t", "home"); err != nil {
		t.Fatalf("add home: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "today", "--tag", "work")
	if err != nil {
		t.Fatalf("today --tag: %v", err)
	}
	if !strings.Contains(stdout, "work today") || strings.Contains(stdout, "home today") {
		t.Fatalf("tag filter mis-applied, got:\n%s", stdout)
	}
}

func TestTodayEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "later", "-d", "2099-12-31"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "today")
	if err != nil {
		t.Fatalf("today: %v", err)
	}
	if !strings.Contains(stdout, "no tasks") {
		t.Fatalf("expected 'no tasks' when nothing due today, got:\n%s", stdout)
	}
}

func TestOverdueShowsPastDue(t *testing.T) {
	dir := t.TempDir()
	past := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	if _, _, err := runCmd(t, dir, "add", "late thing", "-d", past); err != nil {
		t.Fatalf("add past: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "future thing", "-d", "2099-12-31"); err != nil {
		t.Fatalf("add future: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(stdout, "late thing") {
		t.Fatalf("overdue should show past-due task, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "future thing") {
		t.Fatalf("future task leaked into overdue, got:\n%s", stdout)
	}
}

func TestOverdueHidesDoneEvenWithAll(t *testing.T) {
	dir := t.TempDir()
	past := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	if _, _, err := runCmd(t, dir, "add", "done past", "-d", past); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "overdue", "--all")
	if err != nil {
		t.Fatalf("overdue --all: %v", err)
	}
	// IsOverdue() requires !Done, so done past-due tasks should never appear.
	if strings.Contains(stdout, "done past") {
		t.Fatalf("done task should never appear in overdue (even with --all), got:\n%s", stdout)
	}
}

func TestOverdueJSON(t *testing.T) {
	dir := t.TempDir()
	past := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if _, _, err := runCmd(t, dir, "add", "yesterday's mess", "-d", past); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "overdue", "--json")
	if err != nil {
		t.Fatalf("overdue --json: %v", err)
	}
	var tasks []map[string]any
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 overdue task, got %d", len(tasks))
	}
}

func TestOverdueEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "later", "-d", "2099-12-31"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(stdout, "no tasks") {
		t.Fatalf("expected 'no tasks' for empty overdue, got:\n%s", stdout)
	}
}

func TestOverdueRejectsBadPriority(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-d", "2020-01-01"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "overdue", "--priority", "bogus")
	if err == nil {
		t.Fatal("expected error for bogus --priority")
	}
}

func TestTodayTableFormat(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().Format("2006-01-02")
	if _, _, err := runCmd(t, dir, "add", "alpha", "-d", today); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "today", "--format", "table")
	if err != nil {
		t.Fatalf("today --format table: %v", err)
	}
	if !strings.Contains(stdout, "ID") || !strings.Contains(stdout, "TITLE") {
		t.Fatalf("expected table header, got:\n%s", stdout)
	}
}
