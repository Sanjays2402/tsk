package commands

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDueSetsExplicitDate(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "due", "1", "2099-12-31")
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	if !strings.Contains(stdout, "#1 due - -> 2099-12-31") {
		t.Fatalf("expected transition line, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "due:2099-12-31") {
		t.Fatalf("expected due on disk, got:\n%s", content)
	}
}

func TestDueAcceptsNaturalLanguage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "due", "1", "tomorrow"); err != nil {
		t.Fatalf("due tomorrow: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	want := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	if !strings.Contains(content, "due:"+want) {
		t.Fatalf("expected due:%s in content, got:\n%s", want, content)
	}
}

func TestDueClearRemovesDate(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-d", "2099-12-31"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "due", "1", "--clear")
	if err != nil {
		t.Fatalf("due --clear: %v", err)
	}
	if !strings.Contains(stdout, "#1 due 2099-12-31 -> cleared") {
		t.Fatalf("expected clear transition, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(content, "due:") {
		t.Fatalf("expected no due:, got:\n%s", content)
	}
}

func TestDueClearOnAlreadyClearIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "due", "1", "--clear")
	if err != nil {
		t.Fatalf("due --clear noop: %v", err)
	}
	if !strings.Contains(stdout, "already has no due date") {
		t.Fatalf("expected noop message, got: %q", stdout)
	}
}

func TestDueSetSameDateIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-d", "2099-12-31"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "due", "1", "2099-12-31")
	if err != nil {
		t.Fatalf("due same: %v", err)
	}
	if !strings.Contains(stdout, "already due 2099-12-31") {
		t.Fatalf("expected noop message, got: %q", stdout)
	}
}

func TestDueRejectsBadDate(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "due", "1", "not-a-date")
	if err == nil {
		t.Fatal("expected error for bad date")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

func TestDueRejectsClearWithDate(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "due", "1", "tomorrow", "--clear")
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
}

func TestDueRequiresDateOrClear(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "due", "1")
	if err == nil {
		t.Fatal("expected error when neither date nor --clear given")
	}
}

func TestDueUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "due", "999", "tomorrow")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}
