package commands

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnoozeForwardSucceeds(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-d", "2099-12-30"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "snooze", "1", "2099-12-31")
	if err != nil {
		t.Fatalf("snooze: %v", err)
	}
	if !strings.Contains(stdout, "#1 due 2099-12-30 -> 2099-12-31") {
		t.Fatalf("expected forward transition, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "due:2099-12-31") {
		t.Fatalf("expected new due on disk, got:\n%s", content)
	}
}

func TestSnoozeBackwardRefused(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-d", "2099-12-31"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "snooze", "1", "2099-12-01")
	if err == nil {
		t.Fatal("expected refusal for backward move")
	}
	if !strings.Contains(err.Error(), "refusing to move") {
		t.Fatalf("expected explanatory error, got: %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 for refusal, got %v", err)
	}
	// File should be unchanged.
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "due:2099-12-31") {
		t.Fatalf("expected original due preserved, got:\n%s", content)
	}
}

func TestSnoozeBackwardWithForce(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-d", "2099-12-31"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "snooze", "1", "2099-12-01", "--force"); err != nil {
		t.Fatalf("snooze --force: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "due:2099-12-01") {
		t.Fatalf("expected --force to apply backward move, got:\n%s", content)
	}
}

func TestSnoozeInitialSetOnUndatedTask(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "snooze", "1", "tomorrow")
	if err != nil {
		t.Fatalf("snooze: %v", err)
	}
	if !strings.Contains(stdout, "(initial)") {
		t.Fatalf("expected (initial) marker on first-ever set, got: %q", stdout)
	}
	want := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "due:"+want) {
		t.Fatalf("expected due:%s on disk, got:\n%s", want, content)
	}
}

func TestSnoozeSameDateIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-d", "2099-12-31"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "snooze", "1", "2099-12-31")
	if err != nil {
		t.Fatalf("snooze same: %v", err)
	}
	if !strings.Contains(stdout, "no snooze applied") {
		t.Fatalf("expected no-op notice, got: %q", stdout)
	}
}

func TestSnoozeAcceptsNaturalLanguage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "snooze", "1", "tomorrow"); err != nil {
		t.Fatalf("snooze tomorrow: %v", err)
	}
	if _, _, err := runCmd(t, dir, "snooze", "1", "in 7 days"); err != nil {
		t.Fatalf("snooze in 7 days: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	want := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	if !strings.Contains(content, "due:"+want) {
		t.Fatalf("expected due:%s after second snooze, got:\n%s", want, content)
	}
}

func TestSnoozeRejectsBadDate(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "snooze", "1", "not-a-date")
	if err == nil {
		t.Fatal("expected error for bad date")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

func TestSnoozeRejectsEmptyDate(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "snooze", "1", "   ")
	if err == nil {
		t.Fatal("expected error for empty date")
	}
}

func TestSnoozeUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "snooze", "999", "tomorrow")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestSnoozeRequiresExactArgs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "snooze", "1"); err == nil {
		t.Fatal("expected error for missing date arg")
	}
	if _, _, err := runCmd(t, dir, "snooze"); err == nil {
		t.Fatal("expected error for no args")
	}
}
