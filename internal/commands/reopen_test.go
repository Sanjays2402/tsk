package commands

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReopenIDsActsLikeUndo(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "reopen", "1")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !strings.Contains(stdout, "reopened 1 task(s): #1") {
		t.Fatalf("expected reopened line, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] thing") {
		t.Fatalf("expected task back to open, got:\n%s", content)
	}
	if strings.Contains(content, "completed:") {
		t.Fatalf("expected completed: cleared, got:\n%s", content)
	}
}

func TestReopenAcceptsMultipleIDs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2", "3"); err != nil {
		t.Fatalf("done all: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "reopen", "1", "3")
	if err != nil {
		t.Fatalf("reopen multi: %v", err)
	}
	if !strings.Contains(stdout, "reopened 2 task(s): #1, #3") {
		t.Fatalf("expected #1, #3, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	// 1 and 3 should be open, 2 still done.
	if !strings.Contains(content, "- [ ] a") || !strings.Contains(content, "- [ ] c") {
		t.Fatalf("expected a and c reopened, got:\n%s", content)
	}
	if !strings.Contains(content, "- [x] b") {
		t.Fatalf("expected b still done, got:\n%s", content)
	}
}

func TestReopenLastPicksMostRecent(t *testing.T) {
	dir := t.TempDir()
	// Three tasks completed at distinct times by writing raw lines.
	now := time.Now()
	t0 := now.Add(-30 * time.Minute).Format(time.RFC3339)
	t1 := now.Add(-10 * time.Minute).Format(time.RFC3339)
	t2 := now.Add(-1 * time.Minute).Format(time.RFC3339)
	writeRawTasks(t, dir,
		"- [x] oldest <!-- id:1 prio:medium completed:"+t0+" -->",
		"- [x] middle <!-- id:2 prio:medium completed:"+t1+" -->",
		"- [x] newest <!-- id:3 prio:medium completed:"+t2+" -->",
	)
	stdout, _, err := runCmd(t, dir, "reopen", "--last")
	if err != nil {
		t.Fatalf("reopen --last: %v", err)
	}
	if !strings.Contains(stdout, "reopened 1 task(s): #3") {
		t.Fatalf("expected #3 picked as last, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] newest") {
		t.Fatalf("expected newest now open, got:\n%s", content)
	}
	// Older two still done.
	if !strings.Contains(content, "- [x] oldest") || !strings.Contains(content, "- [x] middle") {
		t.Fatalf("expected older two still done, got:\n%s", content)
	}
}

func TestReopenLastEmptyStoreIsFriendly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Nothing is done — --last should report no-op (not error).
	stdout, _, err := runCmd(t, dir, "reopen", "--last")
	if err != nil {
		t.Fatalf("reopen --last with no completions: %v", err)
	}
	if !strings.Contains(stdout, "no tasks to reopen") {
		t.Fatalf("expected friendly no-op, got: %q", stdout)
	}
}

func TestReopenSinceWindow(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := now.Add(-2 * time.Hour).Format(time.RFC3339)
	recent1 := now.Add(-20 * time.Minute).Format(time.RFC3339)
	recent2 := now.Add(-5 * time.Minute).Format(time.RFC3339)
	writeRawTasks(t, dir,
		"- [x] old <!-- id:1 prio:medium completed:"+old+" -->",
		"- [x] recent-1 <!-- id:2 prio:medium completed:"+recent1+" -->",
		"- [x] recent-2 <!-- id:3 prio:medium completed:"+recent2+" -->",
		"- [ ] never-done <!-- id:4 prio:medium -->",
	)
	stdout, _, err := runCmd(t, dir, "reopen", "--since", "30m")
	if err != nil {
		t.Fatalf("reopen --since 30m: %v", err)
	}
	// The two recents (id 2 & 3) should be reopened; the 2h-old one (id 1)
	// should NOT be touched.
	if !strings.Contains(stdout, "reopened 2 task(s): #2, #3") {
		t.Fatalf("expected #2 and #3, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [x] old") {
		t.Fatalf("old task should still be done, got:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] recent-1") || !strings.Contains(content, "- [ ] recent-2") {
		t.Fatalf("recent tasks should be open, got:\n%s", content)
	}
}

func TestReopenSinceMinutesNotMonths(t *testing.T) {
	// Guard against accidentally importing store.ParseDuration, which
	// treats "m" as months. For reopen, --since 5m must mean 5 minutes.
	dir := t.TempDir()
	now := time.Now()
	// One task completed 6 minutes ago — outside a 5m window, inside a 10m window.
	t6m := now.Add(-6 * time.Minute).Format(time.RFC3339)
	writeRawTasks(t, dir,
		"- [x] six-min <!-- id:1 prio:medium completed:"+t6m+" -->",
	)
	// 5m window: nothing should match.
	stdout, _, err := runCmd(t, dir, "reopen", "--since", "5m")
	if err != nil {
		t.Fatalf("reopen --since 5m: %v", err)
	}
	if !strings.Contains(stdout, "no tasks to reopen") {
		t.Fatalf("expected nothing in 5m window (6m old task), got: %q", stdout)
	}
	// 10m window: it should match.
	stdout, _, err = runCmd(t, dir, "reopen", "--since", "10m")
	if err != nil {
		t.Fatalf("reopen --since 10m: %v", err)
	}
	if !strings.Contains(stdout, "reopened 1 task(s): #1") {
		t.Fatalf("expected #1 in 10m window, got: %q", stdout)
	}
}

func TestReopenRequiresMode(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "reopen")
	if err == nil {
		t.Fatal("expected error when no mode given")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

func TestReopenRejectsCombinedModes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	cases := [][]string{
		{"reopen", "1", "--last"},
		{"reopen", "1", "--since", "1h"},
		{"reopen", "--last", "--since", "1h"},
	}
	for _, args := range cases {
		_, _, err := runCmd(t, dir, args...)
		if err == nil {
			t.Fatalf("expected mutual-exclusion error for %v", args)
		}
	}
}

func TestReopenSinceRejectsBadDuration(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, bad := range []string{"banana", "0", "-1h", "abc5m"} {
		_, _, err := runCmd(t, dir, "reopen", "--since", bad)
		if err == nil {
			t.Fatalf("expected error for --since %q", bad)
		}
	}
}

func TestParseReopenDurationTable(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{"30m", 30 * time.Minute, false},
		{"1h", time.Hour, false},
		{"2h30m", 2*time.Hour + 30*time.Minute, false}, // Go native form
		{"2d", 48 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"45s", 45 * time.Second, false},
		{"60", 60 * time.Second, false}, // bare int -> seconds
		{"", 0, true},
		{"0", 0, true},
		{"-1h", 0, true},
		{"banana", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseReopenDuration(tc.in)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestReopenUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "reopen", "999")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}
