package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDailyGroupsByBucket(t *testing.T) {
	dir := t.TempDir()
	// Use dueDate offsets — far enough from the day boundary that
	// the store's TZ-flattening-on-persist (markdown stores due dates
	// as UTC midnight) doesn't push a same-day item across into the
	// wrong bucket. ls_due_test.go uses the same +/-2d trick.
	yesterday := dueDate(-2)
	today := dueDate(0)
	tomorrow := dueDate(2)
	mustAdd(t, dir, "late thing", "-p", "high", "-d", yesterday)
	mustAdd(t, dir, "today thing", "-p", "urgent", "-d", today)
	mustAdd(t, dir, "later thing", "-p", "medium", "-d", tomorrow)
	mustAdd(t, dir, "no due thing", "-p", "high")
	mustAdd(t, dir, "done thing", "-p", "high", "-d", today)
	if _, _, err := runCmd(t, dir, "done", "5"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "daily")
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	for _, want := range []string{"OVERDUE (1)", "TODAY (1)", "UPCOMING (1)", "late thing", "today thing", "later thing"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q, got:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "done thing") {
		t.Fatalf("done tasks should not appear in daily, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "no due thing") {
		t.Fatalf("tasks without due dates should not appear in daily, got:\n%s", stdout)
	}
}

func TestDailyOverdueLagSuffix(t *testing.T) {
	dir := t.TempDir()
	threeDaysAgo := dueDate(-3)
	mustAdd(t, dir, "late", "-p", "high", "-d", threeDaysAgo)
	stdout, _, err := runCmd(t, dir, "daily")
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	if !strings.Contains(stdout, "3 days late") {
		t.Fatalf("expected lag suffix in overdue, got:\n%s", stdout)
	}
}

func TestDailyOverdueLagSingularDay(t *testing.T) {
	dir := t.TempDir()
	oneDayAgo := dueDate(-1)
	mustAdd(t, dir, "late", "-p", "high", "-d", oneDayAgo)
	stdout, _, err := runCmd(t, dir, "daily")
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	if !strings.Contains(stdout, "1 day late") || strings.Contains(stdout, "1 days late") {
		t.Fatalf("expected singular '1 day late', got:\n%s", stdout)
	}
}

func TestDailyUpcomingLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 2; i <= 8; i++ {
		// Start at +2d so we're safely past the today/upcoming boundary.
		mustAdd(t, dir, "up"+string(rune('A'+i-2)), "-p", "high", "-d", dueDate(i))
	}
	stdout, _, err := runCmd(t, dir, "daily", "--upcoming", "2")
	if err != nil {
		t.Fatalf("daily --upcoming: %v", err)
	}
	if !strings.Contains(stdout, "UPCOMING (2)") {
		t.Fatalf("expected UPCOMING (2), got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "upA") || !strings.Contains(stdout, "upB") {
		t.Fatalf("expected first two upcoming, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "upC") {
		t.Fatalf("limit should drop later items, got:\n%s", stdout)
	}
}

func TestDailyUpcomingZeroHides(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "thing", "-p", "medium", "-d", dueDate(2))
	stdout, _, err := runCmd(t, dir, "daily", "--upcoming", "0")
	if err != nil {
		t.Fatalf("daily --upcoming 0: %v", err)
	}
	if !strings.Contains(stdout, "UPCOMING (0)") {
		t.Fatalf("expected UPCOMING (0), got:\n%s", stdout)
	}
}

func TestDailyJSONSchema(t *testing.T) {
	dir := t.TempDir()
	yesterday := dueDate(-2)
	today := dueDate(0)
	tomorrow := dueDate(2)
	mustAdd(t, dir, "late", "-p", "high", "-d", yesterday)
	mustAdd(t, dir, "today", "-p", "urgent", "-d", today)
	mustAdd(t, dir, "later", "-p", "medium", "-d", tomorrow)
	stdout, _, err := runCmd(t, dir, "daily", "--json")
	if err != nil {
		t.Fatalf("daily --json: %v", err)
	}
	var plan DailyPlan
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if len(plan.Overdue) != 1 || len(plan.Today) != 1 || len(plan.Upcoming) != 1 {
		t.Fatalf("expected 1/1/1 buckets, got: %+v", plan)
	}
	if plan.Overdue[0].ID != 1 || plan.Today[0].ID != 2 || plan.Upcoming[0].ID != 3 {
		t.Fatalf("wrong IDs in buckets: %+v", plan)
	}
}

func TestDailyJSONEmptySectionsAreArrays(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "no due", "-p", "medium")
	stdout, _, err := runCmd(t, dir, "daily", "--json")
	if err != nil {
		t.Fatalf("daily --json: %v", err)
	}
	if strings.Contains(stdout, "null") {
		t.Fatalf("buckets must be [] not null, got:\n%s", stdout)
	}
}

func TestDailyTagFilter(t *testing.T) {
	dir := t.TempDir()
	today := dueDate(0)
	mustAdd(t, dir, "work item", "-p", "urgent", "-d", today, "-t", "work")
	mustAdd(t, dir, "home item", "-p", "urgent", "-d", today, "-t", "home")
	stdout, _, err := runCmd(t, dir, "daily", "--tag", "work")
	if err != nil {
		t.Fatalf("daily --tag: %v", err)
	}
	if !strings.Contains(stdout, "work item") || strings.Contains(stdout, "home item") {
		t.Fatalf("tag filter failed, got:\n%s", stdout)
	}
}

func TestDailyOverdueOrderedByPriority(t *testing.T) {
	dir := t.TempDir()
	yesterday := dueDate(-2)
	mustAdd(t, dir, "lowprio", "-p", "low", "-d", yesterday)
	mustAdd(t, dir, "urgentprio", "-p", "urgent", "-d", yesterday)
	mustAdd(t, dir, "mediumprio", "-p", "medium", "-d", yesterday)
	stdout, _, err := runCmd(t, dir, "daily")
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	uIdx := strings.Index(stdout, "urgentprio")
	mIdx := strings.Index(stdout, "mediumprio")
	lIdx := strings.Index(stdout, "lowprio")
	if !(uIdx < mIdx && mIdx < lIdx) {
		t.Fatalf("expected priority-desc order in OVERDUE, got:\n%s", stdout)
	}
}

func TestDailyNegativeUpcomingRejected(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "x", "-p", "medium")
	_, _, err := runCmd(t, dir, "daily", "--upcoming", "-1")
	if err == nil {
		t.Fatal("expected error for --upcoming -1")
	}
}

func TestDailyEmptyShowsAllZeros(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "no due", "-p", "medium")
	stdout, _, err := runCmd(t, dir, "daily")
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	for _, want := range []string{"OVERDUE (0)", "TODAY (0)", "UPCOMING (0)", "(none)"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in empty briefing, got:\n%s", want, stdout)
		}
	}
}

// (mustAdd helper lives in search_test.go — shared across test files
// because it's the convenient pattern for setting up scratch state.)
