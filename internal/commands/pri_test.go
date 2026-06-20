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
