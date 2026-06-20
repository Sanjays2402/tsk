package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSwapExchangesPositions(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "swap", "1", "3"); err != nil {
		t.Fatalf("swap: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	// After swap, file order should be gamma, beta, alpha (by title).
	// IDs survive: gamma keeps id:3, alpha keeps id:1.
	g := strings.Index(content, "gamma")
	b := strings.Index(content, "beta")
	a := strings.Index(content, "alpha")
	if g < 0 || b < 0 || a < 0 {
		t.Fatalf("missing tasks in file:\n%s", content)
	}
	if !(g < b && b < a) {
		t.Fatalf("expected file order gamma,beta,alpha; got positions g=%d b=%d a=%d\n%s", g, b, a, content)
	}
	// IDs unchanged.
	if !strings.Contains(content, "gamma <!-- id:3") {
		t.Fatalf("gamma should keep id:3 after swap, got:\n%s", content)
	}
	if !strings.Contains(content, "alpha <!-- id:1") {
		t.Fatalf("alpha should keep id:1 after swap, got:\n%s", content)
	}
}

func TestSwapIsRoundTrippable(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	before := readFile(t, filepath.Join(dir, ".tsk.md"))
	if _, _, err := runCmd(t, dir, "swap", "1", "2"); err != nil {
		t.Fatalf("swap: %v", err)
	}
	if _, _, err := runCmd(t, dir, "swap", "2", "1"); err != nil {
		t.Fatalf("swap back: %v", err)
	}
	after := readFile(t, filepath.Join(dir, ".tsk.md"))
	if before != after {
		t.Fatalf("swap twice should be identity, diff:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
	}
}

func TestSwapRejectsSelfSwap(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "swap", "1", "1")
	if err == nil {
		t.Fatal("expected error for self-swap")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected ExitCode 2, got %v", err)
	}
}

func TestSwapRejectsUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "swap", "1", "99")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestSwapRejectsBadID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "swap", "abc", "1")
	if err == nil {
		t.Fatal("expected error for non-numeric id")
	}
}

func TestSwapPreservesFields(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "first", "-p", "high", "-t", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "second", "-p", "low", "-t", "later"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "swap", "1", "2"); err != nil {
		t.Fatalf("swap: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	// Both tasks still carry their priority + tag metadata.
	if !strings.Contains(content, "prio:high") || !strings.Contains(content, "prio:low") {
		t.Fatalf("priorities should survive swap, got:\n%s", content)
	}
	if !strings.Contains(content, "tags:urgent") || !strings.Contains(content, "tags:later") {
		t.Fatalf("tags should survive swap, got:\n%s", content)
	}
}

func TestSwapNeedsExactlyTwoArgs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "swap", "1"); err == nil {
		t.Fatal("expected error for single arg")
	}
	if _, _, err := runCmd(t, dir, "swap", "1", "2", "3"); err == nil {
		t.Fatal("expected error for three args")
	}
}
