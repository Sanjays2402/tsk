package commands

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLogShowsRecentCompletions(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	// Three completions, varying timestamps.
	writeRawTasks(t, dir,
		"- [x] alpha <!-- id:1 prio:medium completed:"+now.Add(-30*time.Minute).Format(time.RFC3339)+" -->",
		"- [x] beta <!-- id:2 prio:medium completed:"+now.Add(-1*time.Hour).Format(time.RFC3339)+" -->",
		"- [x] gamma <!-- id:3 prio:medium completed:"+now.Add(-2*time.Hour).Format(time.RFC3339)+" -->",
		"- [ ] open <!-- id:4 prio:medium -->",
	)
	stdout, _, err := runCmd(t, dir, "log")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in log, got:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "open") {
		t.Fatalf("undone task leaked into log, got:\n%s", stdout)
	}
}

func TestLogOrdersNewestFirst(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeRawTasks(t, dir,
		"- [x] oldest <!-- id:1 prio:medium completed:"+now.Add(-3*time.Hour).Format(time.RFC3339)+" -->",
		"- [x] newest <!-- id:2 prio:medium completed:"+now.Add(-10*time.Minute).Format(time.RFC3339)+" -->",
		"- [x] middle <!-- id:3 prio:medium completed:"+now.Add(-1*time.Hour).Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "log")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	newIdx := strings.Index(stdout, "newest")
	midIdx := strings.Index(stdout, "middle")
	oldIdx := strings.Index(stdout, "oldest")
	if newIdx == -1 || midIdx == -1 || oldIdx == -1 {
		t.Fatalf("missing entries:\n%s", stdout)
	}
	if !(newIdx < midIdx && midIdx < oldIdx) {
		t.Fatalf("expected newest first, got:\n%s", stdout)
	}
}

func TestLogLimitCaps(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	lines := []string{}
	for i := 1; i <= 7; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339)
		lines = append(lines, "- [x] task <!-- id:"+strconv.Itoa(i)+" prio:medium completed:"+ts+" -->")
	}
	writeRawTasks(t, dir, lines...)
	stdout, _, err := runCmd(t, dir, "log", "--limit", "3")
	if err != nil {
		t.Fatalf("log --limit: %v", err)
	}
	if n := strings.Count(stdout, "\n"); n != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", n, stdout)
	}
}

func TestLogSinceTrims(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeRawTasks(t, dir,
		"- [x] recent <!-- id:1 prio:medium completed:"+now.Add(-30*time.Minute).Format(time.RFC3339)+" -->",
		"- [x] ancient <!-- id:2 prio:medium completed:"+now.Add(-48*time.Hour).Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "log", "--since", "1h")
	if err != nil {
		t.Fatalf("log --since: %v", err)
	}
	if !strings.Contains(stdout, "recent") {
		t.Fatalf("expected recent task within 1h, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "ancient") {
		t.Fatalf("2d-old task should be filtered out by --since 1h, got:\n%s", stdout)
	}
}

func TestLogTagFilter(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeRawTasks(t, dir,
		"- [x] work thing <!-- id:1 prio:medium tags:work completed:"+now.Format(time.RFC3339)+" -->",
		"- [x] home thing <!-- id:2 prio:medium tags:home completed:"+now.Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "log", "--tag", "work")
	if err != nil {
		t.Fatalf("log --tag: %v", err)
	}
	if !strings.Contains(stdout, "work thing") || strings.Contains(stdout, "home thing") {
		t.Fatalf("tag filter wrong, got:\n%s", stdout)
	}
}

func TestLogJSONShape(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeRawTasks(t, dir,
		"- [x] one <!-- id:1 prio:medium completed:"+now.Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "log", "--json")
	if err != nil {
		t.Fatalf("log --json: %v", err)
	}
	var tasks []map[string]any
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 1 || tasks[0]["Title"] != "one" {
		t.Fatalf("expected 1 task 'one', got %v", tasks)
	}
}

func TestLogJSONEmptyArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "log", "--json")
	if err != nil {
		t.Fatalf("log --json: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected [] for empty result, got %q", stdout)
	}
}

func TestLogReportsSkippedMissingCompleted(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	// Two done tasks: one with a Completed timestamp, one without.
	writeRawTasks(t, dir,
		"- [x] timestamped <!-- id:1 prio:medium completed:"+now.Format(time.RFC3339)+" -->",
		"- [x] silent <!-- id:2 prio:medium -->",
	)
	stdout, _, err := runCmd(t, dir, "log")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(stdout, "timestamped") {
		t.Fatalf("expected timestamped task, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "silent") {
		t.Fatalf("unstamped task should NOT appear in log entries, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "missing completion timestamp") {
		t.Fatalf("expected footer about skipped tasks, got:\n%s", stdout)
	}
}

func TestLogEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "still open"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "log")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(stdout, "no completed tasks") {
		t.Fatalf("expected empty message, got:\n%s", stdout)
	}
}

func TestLogRejectsBadSince(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "log", "--since", "potato")
	if err == nil {
		t.Fatal("expected error for bogus --since")
	}
}

func TestLogRejectsNegativeLimit(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "log", "--limit", "-1")
	if err == nil {
		t.Fatal("expected error for negative --limit")
	}
}

func TestLogUnlimitedShowsAll(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	lines := []string{}
	for i := 1; i <= 25; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339)
		lines = append(lines, "- [x] task <!-- id:"+strconv.Itoa(i)+" prio:medium completed:"+ts+" -->")
	}
	writeRawTasks(t, dir, lines...)
	stdout, _, err := runCmd(t, dir, "log", "--limit", "0")
	if err != nil {
		t.Fatalf("log --limit 0: %v", err)
	}
	if n := strings.Count(stdout, "\n"); n != 25 {
		t.Fatalf("expected 25 lines from --limit 0, got %d", n)
	}
}

func TestLogAlias(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "recent"); err != nil {
		t.Fatalf("recent alias: %v", err)
	}
}
