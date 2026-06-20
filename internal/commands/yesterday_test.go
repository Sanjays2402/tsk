package commands

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestYesterdayShowsOnlyYesterdaysCompletions(t *testing.T) {
	dir := t.TempDir()
	loc := ResolveTZ()
	now := time.Now().In(loc)
	todayMid := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc)
	yesterdayMid := todayMid.AddDate(0, 0, -1)
	twoDaysAgoMid := todayMid.AddDate(0, 0, -2)
	writeRawTasks(t, dir,
		"- [x] today task <!-- id:1 prio:medium completed:"+todayMid.Format(time.RFC3339)+" -->",
		"- [x] yesterday morning <!-- id:2 prio:medium completed:"+yesterdayMid.Add(-3*time.Hour).Format(time.RFC3339)+" -->",
		"- [x] yesterday evening <!-- id:3 prio:medium completed:"+yesterdayMid.Add(5*time.Hour).Format(time.RFC3339)+" -->",
		"- [x] two days ago <!-- id:4 prio:medium completed:"+twoDaysAgoMid.Format(time.RFC3339)+" -->",
		"- [ ] open <!-- id:5 prio:medium -->",
	)
	stdout, _, err := runCmd(t, dir, "yesterday")
	if err != nil {
		t.Fatalf("yesterday: %v", err)
	}
	for _, want := range []string{"yesterday morning", "yesterday evening"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q, got:\n%s", want, stdout)
		}
	}
	for _, dontWant := range []string{"today task", "two days ago", "open"} {
		if strings.Contains(stdout, dontWant) {
			t.Fatalf("did NOT expect %q in yesterday output, got:\n%s", dontWant, stdout)
		}
	}
	if !strings.Contains(stdout, "2 task(s) completed") {
		t.Fatalf("expected summary header, got:\n%s", stdout)
	}
}

func TestYesterdayBoundaryEdges(t *testing.T) {
	dir := t.TempDir()
	loc := ResolveTZ()
	now := time.Now().In(loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	writeRawTasks(t, dir,
		// Exactly at 00:00 today — must be excluded (boundary belongs to today).
		"- [x] today midnight <!-- id:1 prio:medium completed:"+todayStart.Format(time.RFC3339)+" -->",
		// Exactly at 00:00 yesterday — must be included (boundary belongs to yesterday).
		"- [x] yesterday midnight <!-- id:2 prio:medium completed:"+yesterdayStart.Format(time.RFC3339)+" -->",
		// 23:59:59 yesterday — must be included.
		"- [x] yesterday lastsec <!-- id:3 prio:medium completed:"+yesterdayStart.Add(24*time.Hour-time.Second).Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "yesterday")
	if err != nil {
		t.Fatalf("yesterday: %v", err)
	}
	if strings.Contains(stdout, "today midnight") {
		t.Fatalf("00:00 today must be excluded, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "yesterday midnight") || !strings.Contains(stdout, "yesterday lastsec") {
		t.Fatalf("boundary cases dropped, got:\n%s", stdout)
	}
}

func TestYesterdayEmpty(t *testing.T) {
	dir := t.TempDir()
	// Only today's completion.
	loc := ResolveTZ()
	now := time.Now().In(loc)
	writeRawTasks(t, dir,
		"- [x] today only <!-- id:1 prio:medium completed:"+now.Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "yesterday")
	if err != nil {
		t.Fatalf("yesterday: %v", err)
	}
	if !strings.Contains(stdout, "nothing completed") {
		t.Fatalf("expected empty message, got:\n%s", stdout)
	}
}

func TestYesterdayTagFilter(t *testing.T) {
	dir := t.TempDir()
	loc := ResolveTZ()
	now := time.Now().In(loc)
	yesterdayMid := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc).AddDate(0, 0, -1)
	writeRawTasks(t, dir,
		"- [x] work item <!-- id:1 prio:medium tags:work completed:"+yesterdayMid.Format(time.RFC3339)+" -->",
		"- [x] home item <!-- id:2 prio:medium tags:home completed:"+yesterdayMid.Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "yesterday", "--tag", "work")
	if err != nil {
		t.Fatalf("yesterday --tag: %v", err)
	}
	if !strings.Contains(stdout, "work item") || strings.Contains(stdout, "home item") {
		t.Fatalf("tag filter failed, got:\n%s", stdout)
	}
}

func TestYesterdayJSON(t *testing.T) {
	dir := t.TempDir()
	loc := ResolveTZ()
	now := time.Now().In(loc)
	yesterdayMid := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc).AddDate(0, 0, -1)
	writeRawTasks(t, dir,
		"- [x] yesterday "+strconv.Itoa(int(yesterdayMid.Unix()))+" <!-- id:1 prio:medium completed:"+yesterdayMid.Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "yesterday", "--json")
	if err != nil {
		t.Fatalf("yesterday --json: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "[") {
		t.Fatalf("expected JSON array, got:\n%s", stdout)
	}
}

func TestYesterdayJSONEmptyIsArray(t *testing.T) {
	dir := t.TempDir()
	writeRawTasks(t, dir,
		"- [ ] only undone <!-- id:1 prio:medium -->",
	)
	stdout, _, err := runCmd(t, dir, "yesterday", "--json")
	if err != nil {
		t.Fatalf("yesterday --json: %v", err)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed != "[]" {
		t.Fatalf("empty JSON must be [] not null, got: %q", trimmed)
	}
}

func TestYesterdayNewestFirst(t *testing.T) {
	dir := t.TempDir()
	loc := ResolveTZ()
	now := time.Now().In(loc)
	yesterdayMid := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc).AddDate(0, 0, -1)
	writeRawTasks(t, dir,
		"- [x] morning <!-- id:1 prio:medium completed:"+yesterdayMid.Add(-3*time.Hour).Format(time.RFC3339)+" -->",
		"- [x] evening <!-- id:2 prio:medium completed:"+yesterdayMid.Add(5*time.Hour).Format(time.RFC3339)+" -->",
		"- [x] noon <!-- id:3 prio:medium completed:"+yesterdayMid.Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "yesterday")
	if err != nil {
		t.Fatalf("yesterday: %v", err)
	}
	eveIdx := strings.Index(stdout, "evening")
	noonIdx := strings.Index(stdout, "noon")
	mornIdx := strings.Index(stdout, "morning")
	if eveIdx == -1 || noonIdx == -1 || mornIdx == -1 {
		t.Fatalf("missing rows:\n%s", stdout)
	}
	if !(eveIdx < noonIdx && noonIdx < mornIdx) {
		t.Fatalf("expected newest-first, got:\n%s", stdout)
	}
}
