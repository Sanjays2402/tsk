package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTopDefaultsToFive(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 10; i++ {
		if _, _, err := runCmd(t, dir, "add", "task", "-p", "medium"); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "top")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	if got := strings.Count(stdout, "\n"); got != 5 {
		t.Fatalf("expected 5 lines, got %d:\n%s", got, stdout)
	}
}

func TestTopHonoursExplicitN(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 6; i++ {
		if _, _, err := runCmd(t, dir, "add", "task"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "top", "3")
	if err != nil {
		t.Fatalf("top 3: %v", err)
	}
	if got := strings.Count(stdout, "\n"); got != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", got, stdout)
	}
}

func TestTopAllShowsEveryMatch(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 8; i++ {
		if _, _, err := runCmd(t, dir, "add", "task"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "top", "all")
	if err != nil {
		t.Fatalf("top all: %v", err)
	}
	if got := strings.Count(stdout, "\n"); got != 8 {
		t.Fatalf("top all should show every undone task (8), got %d:\n%s", got, stdout)
	}
}

func TestTopOrderingMatchesNext(t *testing.T) {
	dir := t.TempDir()
	// Add tasks at varying priorities; the top of `top` must match `next`.
	for _, p := range []string{"low", "high", "medium", "urgent", "low"} {
		if _, _, err := runCmd(t, dir, "add", "task "+p, "-p", p); err != nil {
			t.Fatalf("add %s: %v", p, err)
		}
	}
	nextOut, _, err := runCmd(t, dir, "next")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	topOut, _, err := runCmd(t, dir, "top", "5")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	// next prints "#N [prio] title …"; top prints "1. [ ] #N [P] title …".
	// Both must reference task "urgent" as the top.
	if !strings.Contains(nextOut, "task urgent") {
		t.Fatalf("next should surface urgent task, got: %s", nextOut)
	}
	firstLine := strings.SplitN(topOut, "\n", 2)[0]
	if !strings.Contains(firstLine, "task urgent") {
		t.Fatalf("top[0] should be urgent task, got: %s", firstLine)
	}
}

func TestTopDueDateBreaksTies(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "no due", "-p", "high"); err != nil {
		t.Fatalf("add no-due: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "with due", "-p", "high", "-d", "2099-01-01"); err != nil {
		t.Fatalf("add with-due: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "top", "2")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	lines := strings.SplitN(stdout, "\n", 3)
	if !strings.Contains(lines[0], "with due") {
		t.Fatalf("dued task should rank first at equal priority, got:\n%s", stdout)
	}
}

func TestTopFiltersByTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work thing", "-t", "work"); err != nil {
		t.Fatalf("add work: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home thing", "-t", "home"); err != nil {
		t.Fatalf("add home: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "top", "--tag", "work")
	if err != nil {
		t.Fatalf("top --tag: %v", err)
	}
	if !strings.Contains(stdout, "work thing") || strings.Contains(stdout, "home thing") {
		t.Fatalf("tag filter wrong, got:\n%s", stdout)
	}
}

func TestTopFiltersByPriority(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "high thing", "-p", "high"); err != nil {
		t.Fatalf("add high: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "low thing", "-p", "low"); err != nil {
		t.Fatalf("add low: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "top", "--priority", "high")
	if err != nil {
		t.Fatalf("top --priority: %v", err)
	}
	if !strings.Contains(stdout, "high thing") || strings.Contains(stdout, "low thing") {
		t.Fatalf("priority filter wrong, got:\n%s", stdout)
	}
}

func TestTopAllIncludesDone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "two"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Without --all, only #2 should appear.
	stdout, _, err := runCmd(t, dir, "top")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	if strings.Contains(stdout, "one") {
		t.Fatalf("undone-only top should hide done #1, got:\n%s", stdout)
	}
	// With --all, #1 should appear too.
	stdout, _, err = runCmd(t, dir, "top", "--all")
	if err != nil {
		t.Fatalf("top --all: %v", err)
	}
	if !strings.Contains(stdout, "one") || !strings.Contains(stdout, "two") {
		t.Fatalf("--all should show both, got:\n%s", stdout)
	}
}

func TestTopJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "top", "--json")
	if err != nil {
		t.Fatalf("top --json: %v", err)
	}
	var tasks []map[string]any
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0]["Title"] != "alpha" {
		t.Fatalf("expected alpha first (higher prio), got %v", tasks[0]["Title"])
	}
}

func TestTopEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "top")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	if !strings.Contains(stdout, "no tasks") {
		t.Fatalf("expected 'no tasks', got %q", stdout)
	}
}

func TestTopRejectsNegativeN(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "top", "-3")
	if err == nil {
		t.Fatal("expected error for negative N")
	}
}

func TestTopRejectsBadN(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "top", "banana")
	if err == nil {
		t.Fatal("expected error for non-int non-'all' N")
	}
}

func TestParseTopLimitTable(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    int
		wantErr bool
	}{
		{"empty", nil, 5, false},
		{"explicit", []string{"7"}, 7, false},
		{"zero", []string{"0"}, 0, false},
		{"all", []string{"all"}, 0, false},
		{"ALL-uppercase", []string{"ALL"}, 0, false},
		{"whitespace", []string{"  "}, 5, false},
		{"negative", []string{"-1"}, 0, true},
		{"text", []string{"five"}, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTopLimit(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %d, got %d", tc.want, got)
			}
		})
	}
}
