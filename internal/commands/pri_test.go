package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPriUpdatesPriority(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pri", "1", "high")
	if err != nil {
		t.Fatalf("pri: %v", err)
	}
	if !strings.Contains(stdout, "#1 priority low -> high") {
		t.Fatalf("expected transition line, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "prio:high") {
		t.Fatalf("expected prio:high on disk, got:\n%s", content)
	}
}

func TestPriAliasPriority(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "priority", "1", "urgent"); err != nil {
		t.Fatalf("priority alias: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "prio:urgent") {
		t.Fatalf("alias should work, got:\n%s", content)
	}
}

func TestPriShortFormsParse(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, in := range []string{"l", "m", "h", "u", "LOW", "HIGH"} {
		if _, _, err := runCmd(t, dir, "pri", "1", in); err != nil {
			t.Fatalf("pri %s: %v", in, err)
		}
	}
}

func TestPriUnchangedIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pri", "1", "high")
	if err != nil {
		t.Fatalf("pri unchanged: %v", err)
	}
	if !strings.Contains(stdout, "already at priority high") {
		t.Fatalf("expected no-op notice, got: %q", stdout)
	}
}

func TestPriRejectsBadPriority(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pri", "1", "banana")
	if err == nil {
		t.Fatal("expected error for bad priority")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

func TestPriUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pri", "999", "high")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestPriUpDownCycleBumps(t *testing.T) {
	dir := t.TempDir()
	// Add with low; --up to medium; --up to high; --up to urgent; --up no-op.
	if _, _, err := runCmd(t, dir, "add", "x", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for i, want := range []string{"low -> medium", "medium -> high", "high -> urgent"} {
		stdout, _, err := runCmd(t, dir, "pri", "1", "--up")
		if err != nil {
			t.Fatalf("--up #%d: %v", i, err)
		}
		if !strings.Contains(stdout, want) {
			t.Fatalf("--up #%d expected %q, got %q", i, want, stdout)
		}
	}
	// At urgent, --up is a no-op.
	stdout, _, err := runCmd(t, dir, "pri", "1", "--up")
	if err != nil {
		t.Fatalf("--up at urgent: %v", err)
	}
	if !strings.Contains(stdout, "already at priority urgent") {
		t.Fatalf("expected urgent no-op, got %q", stdout)
	}
	// --down: urgent -> high -> medium -> low; low is then no-op.
	for i, want := range []string{"urgent -> high", "high -> medium", "medium -> low"} {
		stdout, _, err := runCmd(t, dir, "pri", "1", "--down")
		if err != nil {
			t.Fatalf("--down #%d: %v", i, err)
		}
		if !strings.Contains(stdout, want) {
			t.Fatalf("--down #%d expected %q, got %q", i, want, stdout)
		}
	}
	stdout, _, err = runCmd(t, dir, "pri", "1", "--down")
	if err != nil {
		t.Fatalf("--down at low: %v", err)
	}
	if !strings.Contains(stdout, "already at priority low") {
		t.Fatalf("expected low no-op, got %q", stdout)
	}
}

func TestPriCycleWrapsAtUrgent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pri", "1", "--cycle")
	if err != nil {
		t.Fatalf("--cycle from urgent: %v", err)
	}
	if !strings.Contains(stdout, "urgent -> low") {
		t.Fatalf("--cycle from urgent should wrap to low, got %q", stdout)
	}
	// And from medium, --cycle still steps one up.
	if _, _, err := runCmd(t, dir, "pri", "1", "medium"); err != nil {
		t.Fatalf("reset to medium: %v", err)
	}
	stdout, _, err = runCmd(t, dir, "pri", "1", "--cycle")
	if err != nil {
		t.Fatalf("--cycle from medium: %v", err)
	}
	if !strings.Contains(stdout, "medium -> high") {
		t.Fatalf("--cycle from medium should -> high, got %q", stdout)
	}
}

func TestPriRejectsConflictingFlags(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	cases := [][]string{
		{"pri", "1", "--up", "--down"},
		{"pri", "1", "--up", "--cycle"},
		{"pri", "1", "--down", "--cycle"},
		{"pri", "1", "high", "--up"},
		{"pri", "1", "low", "--cycle"},
	}
	for _, args := range cases {
		_, _, err := runCmd(t, dir, args...)
		if err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}

func TestPriRequiresPriorityOrFlag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pri", "1")
	if err == nil {
		t.Fatal("expected error when neither <priority> nor --up/--down/--cycle is set")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected ExitCode 2, got %v", err)
	}
}
