package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestWaitSetHidesFromDefaultLs verifies that setting a future wait
// date hides the task from `tsk ls` defaults but leaves it visible
// through --all and --include-waiting.
func TestWaitSetHidesFromDefaultLs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "stay-visible"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "deferred-thing"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "2", "2099-12-31"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	// Default ls excludes the waiting task.
	stdout, _, err := runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if strings.Contains(stdout, "deferred-thing") {
		t.Fatalf("waiting task should be hidden:\n%s", stdout)
	}
	if !strings.Contains(stdout, "stay-visible") {
		t.Fatalf("non-waiting task should still appear:\n%s", stdout)
	}
	// --all surfaces it.
	stdout, _, err = runCmd(t, dir, "ls", "--all")
	if err != nil {
		t.Fatalf("ls --all: %v", err)
	}
	if !strings.Contains(stdout, "deferred-thing") {
		t.Fatalf("--all should surface waiting tasks:\n%s", stdout)
	}
	// --include-waiting also surfaces it.
	stdout, _, err = runCmd(t, dir, "ls", "--include-waiting")
	if err != nil {
		t.Fatalf("ls --include-waiting: %v", err)
	}
	if !strings.Contains(stdout, "deferred-thing") {
		t.Fatalf("--include-waiting should surface waiting tasks:\n%s", stdout)
	}
}

// TestWaitPersistedInMarkdown asserts the wait:<date> meta key lands in
// the .tsk.md file and round-trips through reload.
func TestWaitPersistedInMarkdown(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "1", "2099-06-15"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	body := readFile(t, dir+"/.tsk.md")
	if !strings.Contains(body, "wait:2099-06-15") {
		t.Fatalf("expected wait:2099-06-15 in file:\n%s", body)
	}
	// Show surfaces it.
	stdout, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(stdout, "wait:      2099-06-15") {
		t.Fatalf("expected wait line in show:\n%s", stdout)
	}
}

// TestWaitClearReappears: clearing wait makes the task visible again.
func TestWaitClearReappears(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "deferred"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "1", "2099-01-01"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	// Hidden.
	stdout, _, err := runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if strings.Contains(stdout, "deferred") {
		t.Fatalf("should be hidden:\n%s", stdout)
	}
	// Clear.
	if _, _, err := runCmd(t, dir, "wait", "1", "--clear"); err != nil {
		t.Fatalf("wait --clear: %v", err)
	}
	stdout, _, err = runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls after clear: %v", err)
	}
	if !strings.Contains(stdout, "deferred") {
		t.Fatalf("should be visible after clear:\n%s", stdout)
	}
}

// TestWaitListShowsAllWaiting: --list mode dumps the waiting queue.
func TestWaitListShowsAllWaiting(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "wait", "1", "2099-01-15"); err != nil {
		t.Fatalf("wait 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "3", "2099-01-05"); err != nil {
		t.Fatalf("wait 3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wait", "--list")
	if err != nil {
		t.Fatalf("wait --list: %v", err)
	}
	if !strings.Contains(stdout, "alpha") || !strings.Contains(stdout, "gamma") {
		t.Fatalf("list missing waiting tasks:\n%s", stdout)
	}
	if strings.Contains(stdout, "beta") {
		t.Fatalf("non-waiting task should not appear:\n%s", stdout)
	}
	// Order: earliest wait-until first (gamma 2099-01-05 before alpha 2099-01-15).
	gammaIdx := strings.Index(stdout, "gamma")
	alphaIdx := strings.Index(stdout, "alpha")
	if gammaIdx == -1 || alphaIdx == -1 || gammaIdx > alphaIdx {
		t.Fatalf("expected gamma (earlier wait) before alpha:\n%s", stdout)
	}
}

// TestWaitListJSONShape: --list --json emits an array, empty case [].
func TestWaitListJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wait", "--list", "--json")
	if err != nil {
		t.Fatalf("wait --list --json (empty): %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected [], got %q", stdout)
	}
	// With one entry.
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "1", "2099-01-01"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	stdout, _, err = runCmd(t, dir, "wait", "--list", "--json")
	if err != nil {
		t.Fatalf("wait --list --json: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	if len(arr) != 1 || arr[0]["Title"] != "x" {
		t.Fatalf("unexpected JSON: %s", stdout)
	}
}

// TestWaitPastDateImmediatelyVisible: a wait-until in the PAST means
// the task is visible right now (the wait already expired).
func TestWaitPastDateImmediatelyVisible(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "yesterdays-deferred"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "1", "2020-01-01"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(stdout, "yesterdays-deferred") {
		t.Fatalf("expired-wait task should be visible:\n%s", stdout)
	}
}

// TestWaitHidesFromTopAndNext: waiting tasks don't show up in top/next
// since those are "what should I do now?" queries.
func TestWaitHidesFromTopAndNext(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "deferred", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "active", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "1", "2099-01-01"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "next")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if strings.Contains(stdout, "deferred") {
		t.Fatalf("waiting task should not win next:\n%s", stdout)
	}
	if !strings.Contains(stdout, "active") {
		t.Fatalf("expected 'active' to win:\n%s", stdout)
	}
	stdout, _, err = runCmd(t, dir, "top")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	if strings.Contains(stdout, "deferred") {
		t.Fatalf("waiting task should not appear in top:\n%s", stdout)
	}
}

// TestWaitListAndIDConflict: --list cannot be combined with an id.
func TestWaitListAndIDConflict(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "wait", "1", "--list")
	if err == nil {
		t.Fatal("expected error when --list combined with id")
	}
}

// TestIsWaitingFunction is a tiny direct unit test that the model
// method behaves correctly for all three cases.
func TestIsWaitingFunction(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)
	cases := []struct {
		name string
		t    model.Task
		want bool
	}{
		{"no wait", model.Task{}, false},
		{"future wait", model.Task{WaitUntil: &future}, true},
		{"past wait", model.Task{WaitUntil: &past}, false},
	}
	for _, tc := range cases {
		if got := tc.t.IsWaiting(now); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
