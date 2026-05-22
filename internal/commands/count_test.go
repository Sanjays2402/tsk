package commands

import (
	"strings"
	"testing"
)

func TestCountDefaultIsUndone(t *testing.T) {
	tmp := t.TempDir()
	if _, _, err := runCmd(t, tmp, "init"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if _, _, err := runCmd(t, tmp, "add", "first"); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, _, err := runCmd(t, tmp, "add", "second"); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, _, err := runCmd(t, tmp, "add", "third"); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, _, err := runCmd(t, tmp, "done", "1"); err != nil {
		t.Fatalf("done failed: %v", err)
	}

	stdout, _, err := runCmd(t, tmp, "count")
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "2" {
		t.Fatalf("count default: want 2 undone, got %q", got)
	}
}

func TestCountAllAndDone(t *testing.T) {
	tmp := t.TempDir()
	if _, _, err := runCmd(t, tmp, "init"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, tmp, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, tmp, "done", "2"); err != nil {
		t.Fatalf("done failed: %v", err)
	}

	stdout, _, err := runCmd(t, tmp, "count", "--all")
	if err != nil {
		t.Fatalf("count --all failed: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "3" {
		t.Fatalf("count --all: want 3, got %q", got)
	}

	stdout, _, err = runCmd(t, tmp, "count", "--done")
	if err != nil {
		t.Fatalf("count --done failed: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "1" {
		t.Fatalf("count --done: want 1, got %q", got)
	}
}

func TestCountTagFilter(t *testing.T) {
	tmp := t.TempDir()
	if _, _, err := runCmd(t, tmp, "init"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if _, _, err := runCmd(t, tmp, "add", "tagged", "-t", "work"); err != nil {
		t.Fatalf("add tagged failed: %v", err)
	}
	if _, _, err := runCmd(t, tmp, "add", "untagged"); err != nil {
		t.Fatalf("add untagged failed: %v", err)
	}

	stdout, _, err := runCmd(t, tmp, "count", "--tag", "work")
	if err != nil {
		t.Fatalf("count --tag failed: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "1" {
		t.Fatalf("count --tag work: want 1, got %q", got)
	}

	stdout, _, err = runCmd(t, tmp, "count", "--tag", "nonexistent")
	if err != nil {
		t.Fatalf("count --tag nonexistent failed: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "0" {
		t.Fatalf("count --tag nonexistent: want 0, got %q", got)
	}
}
