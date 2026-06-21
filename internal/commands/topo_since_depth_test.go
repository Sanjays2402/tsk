package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestTopoSinceDepthForwardLimitsLayer1: chain 4 -> 3 -> 2 -> 1,
// `topo --since 1 --depth 1` keeps anchor #1 plus its direct
// dependent #2 only — drops #3 and #4 which are 2+ hops out.
func TestTopoSinceDepthForwardLimitsLayer1(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 4 -> 3 -> 2 -> 1 (each higher id depends on the previous)
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "1", "--depth", "1")
	if err != nil {
		t.Fatalf("topo --since 1 --depth 1: %v", err)
	}
	got := strings.TrimSpace(stdout)
	// Anchor #1 + its direct dependent #2 only.
	if got != "1,2" {
		t.Fatalf("expected '1,2', got %q", got)
	}
}

// TestTopoSinceDepthForwardLimitsLayer2: same chain, --depth 2
// keeps #1, #2, #3 (anchor + 2 layers) but drops #4.
func TestTopoSinceDepthForwardLimitsLayer2(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "1", "--depth", "2")
	if err != nil {
		t.Fatalf("topo --since 1 --depth 2: %v", err)
	}
	got := strings.TrimSpace(stdout)
	if got != "1,2,3" {
		t.Fatalf("expected '1,2,3', got %q", got)
	}
}

// TestTopoSinceDepthReverseLimitsPrereqLayers: chain 4 -> 3 -> 2 -> 1.
// `topo --since 4 --reverse --depth 1` keeps just the immediate
// prereq #3 (one layer back from the anchor).
func TestTopoSinceDepthReverseLimitsPrereqLayers(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "4", "--reverse", "--depth", "1")
	if err != nil {
		t.Fatalf("topo --since 4 --reverse --depth 1: %v", err)
	}
	got := strings.TrimSpace(stdout)
	// Only #3 (one prereq layer back). Anchor #4 is excluded by
	// --reverse; #2 and #1 are too far back.
	if got != "3" {
		t.Fatalf("expected '3', got %q", got)
	}
}

// TestTopoSinceDepthReverseTwoLayers: same chain, --depth 2 keeps
// #3 (layer 1) and #2 (layer 2). #1 is at layer 3 → excluded.
func TestTopoSinceDepthReverseTwoLayers(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "4", "--reverse", "--depth", "2")
	if err != nil {
		t.Fatalf("topo --since 4 --reverse --depth 2: %v", err)
	}
	got := strings.TrimSpace(stdout)
	// Topological order preserved: #2 before #3.
	if got != "2,3" {
		t.Fatalf("expected '2,3', got %q", got)
	}
}

// TestTopoSinceDepthRequiresSince: --depth without --since is a
// usage error.
func TestTopoSinceDepthRequiresSince(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "topo", "--depth", "2")
	if err == nil {
		t.Fatal("expected usage error: --depth without --since")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "--since") {
		t.Fatalf("error should mention --since, got: %v", err)
	}
}

// TestTopoSinceDepthRejectsNegative: --depth -1 errors out.
func TestTopoSinceDepthRejectsNegative(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "topo", "--since", "1", "--depth", "-1")
	if err == nil {
		t.Fatal("expected error for negative --depth")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestTopoSinceDepthBranching: branching graph — anchor #1 has
// two direct dependents #2 and #3, plus #4 that depends on #3.
// --depth 1 keeps #1, #2, #3 but not #4.
func TestTopoSinceDepthBranching(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "branch-a", "branch-b", "leaf"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #2 and #3 both depend on #1. #4 depends on #3.
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "1", "--depth", "1")
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	got := strings.TrimSpace(stdout)
	if !strings.Contains(got, "1") || !strings.Contains(got, "2") || !strings.Contains(got, "3") {
		t.Fatalf("expected #1, #2, #3 in output, got %q", got)
	}
	if strings.Contains(got, "4") {
		t.Fatalf("#4 is 2 hops out and should be excluded at --depth 1, got %q", got)
	}
}

// TestTopoSinceDepthZeroNoLimit: --depth 0 is the default; topo
// emits the full forward slice from the anchor as if --depth was
// not supplied at all.
func TestTopoSinceDepthZeroNoLimit(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	withDepth, _, err := runCmd(t, dir, "topo", "--ids", "--since", "1", "--depth", "0")
	if err != nil {
		t.Fatalf("topo --depth 0: %v", err)
	}
	withoutDepth, _, err := runCmd(t, dir, "topo", "--ids", "--since", "1")
	if err != nil {
		t.Fatalf("topo (no depth): %v", err)
	}
	if withDepth != withoutDepth {
		t.Fatalf("--depth 0 should match no --depth flag\nWITH: %q\nWITHOUT: %q", withDepth, withoutDepth)
	}
}

// TestLimitTopoDepthIsolated: direct unit test on the helper. Builds
// a small store and checks layer math without going through cobra
// plumbing.
func TestLimitTopoDepthIsolated(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	s, err := store.Load(filepath.Join(dir, ".tsk.md"))
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	// Construct an `ordered` slice for the full chain [1, 2, 3, 4].
	ordered := []topoTask{
		{Task: *s.ByID(1)},
		{Task: *s.ByID(2)},
		{Task: *s.ByID(3)},
		{Task: *s.ByID(4)},
	}
	// Forward depth=2 from #1 keeps #1, #2, #3.
	got := limitTopoDepth(s, ordered, 1, 2, false)
	ids := idsOf(got)
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Fatalf("forward depth=2 from #1 should be [1,2,3], got %v", ids)
	}
	// Reverse depth=1 with input being sliceTopoBefore output
	// (prereqs of #4 = [1, 2, 3]) keeps only #3.
	prereqs := []topoTask{
		{Task: *s.ByID(1)},
		{Task: *s.ByID(2)},
		{Task: *s.ByID(3)},
	}
	got = limitTopoDepth(s, prereqs, 4, 1, true)
	ids = idsOf(got)
	if len(ids) != 1 || ids[0] != 3 {
		t.Fatalf("reverse depth=1 from #4 should be [3], got %v", ids)
	}
}
