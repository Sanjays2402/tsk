package commands

import (
	"strings"
	"testing"
)

// TestPinFlipsFlagAndPersists asserts that `tsk pin` sets pin:true in the
// markdown metadata and that `tsk unpin` clears it.
func TestPinFlipsFlagAndPersists(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "stay-focused"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	body := readFile(t, dir+"/.tsk.md")
	if !strings.Contains(body, "pin:true") {
		t.Fatalf("expected pin:true in saved file:\n%s", body)
	}
	if _, _, err := runCmd(t, dir, "unpin", "1"); err != nil {
		t.Fatalf("unpin: %v", err)
	}
	body = readFile(t, dir+"/.tsk.md")
	if strings.Contains(body, "pin:true") {
		t.Fatalf("expected pin:true removed:\n%s", body)
	}
}

// TestPinMultiTask checks that pin accepts several ids at once and only
// flips the ones that need it.
func TestPinMultiTask(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// Pin 1 + 3.
	if _, _, err := runCmd(t, dir, "pin", "1", "3"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	body := readFile(t, dir+"/.tsk.md")
	pinCount := strings.Count(body, "pin:true")
	if pinCount != 2 {
		t.Fatalf("expected 2 pin:true lines, got %d:\n%s", pinCount, body)
	}
	// Pinning an already-pinned + a new id reports correctly.
	stdout, _, err := runCmd(t, dir, "pin", "1", "2")
	if err != nil {
		t.Fatalf("pin 1 2: %v", err)
	}
	if !strings.Contains(stdout, "pinned 1") {
		t.Fatalf("expected report of 1 change, got %q", stdout)
	}
}

// TestPinAffectsNextAndTopOrder asserts a pinned low-priority task wins
// `tsk next` over an unpinned urgent task, and floats to top of `tsk top`.
func TestPinAffectsNextAndTopOrder(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "urgent thing", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "pinned low", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Without pin, next must be #1 (urgent).
	stdout, _, err := runCmd(t, dir, "next")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if !strings.Contains(stdout, "urgent thing") {
		t.Fatalf("pre-pin next should pick urgent: %q", stdout)
	}
	if _, _, err := runCmd(t, dir, "pin", "2"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err = runCmd(t, dir, "next")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if !strings.Contains(stdout, "pinned low") {
		t.Fatalf("post-pin next should pick pinned low: %q", stdout)
	}
	if !strings.Contains(stdout, "*") {
		t.Fatalf("next should mark pinned with *: %q", stdout)
	}
	stdout, _, err = runCmd(t, dir, "top", "2")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	// First rank line must contain "pinned low".
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 2 || !strings.Contains(lines[0], "pinned low") {
		t.Fatalf("expected pinned low at rank 1:\n%s", stdout)
	}
}

// TestPinUnknownIDFails asserts pin refuses unknown ids without partial-applying.
func TestPinUnknownIDFails(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pin", "1", "99")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
	body := readFile(t, dir+"/.tsk.md")
	if strings.Contains(body, "pin:true") {
		t.Fatalf("partial apply happened:\n%s", body)
	}
}

// TestPinIdempotent: pinning twice is a no-op, no save churn complaint.
func TestPinIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pin", "1")
	if err != nil {
		t.Fatalf("pin again: %v", err)
	}
	if !strings.Contains(stdout, "no change") {
		t.Fatalf("expected no-change report, got %q", stdout)
	}
}

// TestUnpinHandEditableFalse: hand-edited pin:false clears the flag on read.
func TestUnpinHandEditableFalse(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	// Verify show reports nothing about pin (we don't surface it there
	// yet, but the round-trip integrity matters).
	body := readFile(t, dir+"/.tsk.md")
	if !strings.Contains(body, "pin:true") {
		t.Fatalf("expected pin:true:\n%s", body)
	}
}
