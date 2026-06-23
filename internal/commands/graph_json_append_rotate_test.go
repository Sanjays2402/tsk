package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGraphJSONAppendRotateTrimsOldest: with --rotate N, after the
// (N+1)th append the file holds exactly N records, with the OLDEST
// (head-of-file) dropped. FIFO eviction confirms by checking root
// ids: appends 1..5 with --rotate 3 produces a file with roots 3,4,5
// (oldest two evicted).
func TestGraphJSONAppendRotateTrimsOldest(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"t1", "t2", "t3", "t4", "t5"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "history.jsonl")
	for _, root := range []string{"1", "2", "3", "4", "5"} {
		if _, _, err := runCmd(t, dir, "graph", "--reachable", root, "--json", "--output", outPath, "--append", "--rotate", "3"); err != nil {
			t.Fatalf("append --rotate root=%s: %v", root, err)
		}
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines after 5 appends with --rotate 3, got %d:\n%s", len(lines), body)
	}
	wantRoots := []int{3, 4, 5}
	for i, line := range lines {
		var doc subgraphDoc
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("parse line %d: %v\nline: %s", i, err, line)
		}
		if doc.RootID != wantRoots[i] {
			t.Errorf("line %d: expected root_id=%d (FIFO eviction kept newest), got %d", i, wantRoots[i], doc.RootID)
		}
	}
}

// TestGraphJSONAppendRotateReportsDroppedCount: when rotation
// actually trims lines, the status message includes the dropped
// count and the kept-newest target. Silent rotation would surprise
// users; the explicit report keeps the feature observable.
func TestGraphJSONAppendRotateReportsDroppedCount(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "trim.jsonl")
	// First two appends fit under the cap (no rotation message).
	for _, root := range []string{"1", "2"} {
		stdout, _, err := runCmd(t, dir, "graph", "--reachable", root, "--json", "--output", outPath, "--append", "--rotate", "2")
		if err != nil {
			t.Fatalf("append %s: %v", root, err)
		}
		if strings.Contains(stdout, "rotated") {
			t.Errorf("did not expect rotation message under cap, got: %s", stdout)
		}
	}
	// Third append trips the cap (dropped 1, kept 2).
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "3", "--json", "--output", outPath, "--append", "--rotate", "2")
	if err != nil {
		t.Fatalf("append 3: %v", err)
	}
	if !strings.Contains(stdout, "rotated") || !strings.Contains(stdout, "dropped 1 oldest line(s)") || !strings.Contains(stdout, "kept newest 2") {
		t.Errorf("expected rotation report on cap breach, got: %s", stdout)
	}
}

// TestGraphJSONAppendRotateNoTrimUnderCap: rotation under the cap
// is a no-op — no lines dropped, no rotation message, file
// retains every record.
func TestGraphJSONAppendRotateNoTrimUnderCap(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "under.jsonl")
	// 2 appends under a cap of 10 stay in the file.
	for i := 0; i < 2; i++ {
		stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append", "--rotate", "10")
		if err != nil {
			t.Fatalf("append: %v", err)
		}
		if strings.Contains(stdout, "rotated") {
			t.Errorf("did not expect rotation message under cap, got: %s", stdout)
		}
	}
	body, _ := os.ReadFile(outPath)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 retained lines, got %d:\n%s", len(lines), body)
	}
}

// TestGraphJSONAppendRotateRequiresAppend: --rotate without --append
// is a usage error. Rotation only applies to the streaming JSONL
// path; passing it on the overwriting --output path would be
// vacuous.
func TestGraphJSONAppendRotateRequiresAppend(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "x.json")
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--rotate", "5")
	if err == nil {
		t.Fatal("expected error for --rotate without --append")
	}
	if !strings.Contains(err.Error(), "--rotate requires --append") {
		t.Fatalf("expected rotate-requires-append error, got: %v", err)
	}
}

// TestGraphJSONAppendRotateRejectsNegative: a negative --rotate is
// a usage error (the help docs the valid range as >= 0).
func TestGraphJSONAppendRotateRejectsNegative(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "x.jsonl")
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append", "--rotate", "-1")
	if err == nil {
		t.Fatal("expected error for negative --rotate")
	}
	if !strings.Contains(err.Error(), "--rotate must be >= 0") {
		t.Fatalf("expected rotate-must-be-nonnegative error, got: %v", err)
	}
}

// TestGraphJSONAppendRotateZeroIsDisable: --rotate 0 explicitly
// disables rotation (matches the default). Useful as a script-side
// override toggle.
func TestGraphJSONAppendRotateZeroIsDisable(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "disabled.jsonl")
	for i := 0; i < 5; i++ {
		if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append", "--rotate", "0"); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	body, _ := os.ReadFile(outPath)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 retained lines with --rotate 0 (disabled), got %d", len(lines))
	}
}

// TestGraphJSONAppendRotateAtomicOnRenameSuccess: after rotation the
// .tmp helper file is gone (rename consumed it). Regression guard
// that we don't leave orphan .tmp files lying around in long-running
// snapshot loops.
func TestGraphJSONAppendRotateCleansUpTmp(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "clean.jsonl")
	for _, root := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "graph", "--reachable", root, "--json", "--output", outPath, "--append", "--rotate", "2"); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	// The .rotate.tmp helper must not survive.
	if _, err := os.Stat(outPath + ".rotate.tmp"); err == nil {
		t.Errorf("expected NO orphan .rotate.tmp file, but it exists")
	}
}

// TestGraphJSONAppendRotateOneKeepsOnlyLast: --rotate 1 produces a
// rolling 1-record file (always the newest). Edge case worth
// pinning since it exercises the keepN=1 path.
func TestGraphJSONAppendRotateOneKeepsOnlyLast(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	outPath := filepath.Join(dir, "single.jsonl")
	for _, root := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "graph", "--reachable", root, "--json", "--output", outPath, "--append", "--rotate", "1"); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	body, _ := os.ReadFile(outPath)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line with --rotate 1, got %d:\n%s", len(lines), body)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(lines[0]), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.RootID != 3 {
		t.Errorf("expected only the most-recent record (root_id=3), got %d", doc.RootID)
	}
}

// TestRotateJSONLFileHelperEdgeCases: direct unit-test on the helper.
// Covers the four interesting branches: missing file (no-op),
// under-cap (no-op), at-cap (no-op), over-cap (trim).
func TestRotateJSONLFileHelperEdgeCases(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.jsonl")
	if dropped, err := rotateJSONLFile(missing, 5); err != nil {
		t.Errorf("missing-file rotation should be a no-op error-free, got err=%v", err)
	} else if dropped != 0 {
		t.Errorf("missing-file: expected 0 dropped, got %d", dropped)
	}

	underPath := filepath.Join(dir, "under.jsonl")
	if err := os.WriteFile(underPath, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if dropped, err := rotateJSONLFile(underPath, 5); err != nil {
		t.Errorf("under-cap rotation: %v", err)
	} else if dropped != 0 {
		t.Errorf("under-cap: expected 0 dropped, got %d", dropped)
	}

	atPath := filepath.Join(dir, "at.jsonl")
	if err := os.WriteFile(atPath, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if dropped, err := rotateJSONLFile(atPath, 3); err != nil {
		t.Errorf("at-cap rotation: %v", err)
	} else if dropped != 0 {
		t.Errorf("at-cap: expected 0 dropped, got %d", dropped)
	}

	overPath := filepath.Join(dir, "over.jsonl")
	if err := os.WriteFile(overPath, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	dropped, err := rotateJSONLFile(overPath, 2)
	if err != nil {
		t.Fatalf("over-cap rotation: %v", err)
	}
	if dropped != 3 {
		t.Errorf("over-cap: expected 3 dropped, got %d", dropped)
	}
	out, _ := os.ReadFile(overPath)
	if string(out) != "d\ne\n" {
		t.Errorf("expected tail-preserved content 'd\\ne\\n', got %q", out)
	}
}

// TestGraphJSONAppendRotateZeroNoOpForOversizeFile: edge case —
// --rotate 0 leaves an oversize file alone (disabled means
// disabled; we don't trim opportunistically). Compare with N>0
// behavior to confirm.
func TestGraphJSONAppendRotateZeroLeavesOversizeFileAlone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "over.jsonl")
	for i := 0; i < 5; i++ {
		if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append"); err != nil {
			t.Fatalf("seed append: %v", err)
		}
	}
	// Now one more append with --rotate 0 (explicitly disabled).
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append", "--rotate", "0"); err != nil {
		t.Fatalf("disabled append: %v", err)
	}
	body, _ := os.ReadFile(outPath)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 retained lines with --rotate 0, got %d", len(lines))
	}
}
