package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestPriStatsDefaultCounts: undone-only scope, all four buckets in
// canonical urgent→low order, total line present.
func TestPriStatsDefaultCounts(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"urgent", "urgent", "high", "medium", "low"} {
		if _, _, err := runCmd(t, dir, "add", p+" task", "-p", p); err != nil {
			t.Fatalf("add %s: %v", p, err)
		}
	}
	// One done task that must be EXCLUDED from the default view.
	if _, _, err := runCmd(t, dir, "add", "finished", "-p", "high"); err != nil {
		t.Fatalf("add finished: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "6"); err != nil {
		t.Fatalf("done: %v", err)
	}
	out, _, err := runCmd(t, dir, "pri-stats")
	if err != nil {
		t.Fatalf("pri-stats: %v", err)
	}
	// Bucket presence + counts.
	for _, want := range []string{
		"urgent       2", "high         1",
		"medium       1", "low          1", "total       5",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in:\n%s", want, out)
		}
	}
	// Order: urgent must appear before low.
	if strings.Index(out, "urgent") > strings.Index(out, "low") {
		t.Fatalf("expected urgent before low, got:\n%s", out)
	}
}

// TestPriStatsAll: --all includes done tasks too.
func TestPriStatsAll(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "u", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "h", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	out, _, err := runCmd(t, dir, "pri-stats", "--all")
	if err != nil {
		t.Fatalf("pri-stats --all: %v", err)
	}
	if !strings.Contains(out, "total       2") {
		t.Fatalf("expected total=2 with --all, got:\n%s", out)
	}
}

// TestPriStatsDoneAllMutex: --done and --all reject combined use.
func TestPriStatsDoneAllMutex(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pri-stats", "--done", "--all")
	if err == nil {
		t.Fatal("expected error for --done + --all combination")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected ExitCode 2, got %v", err)
	}
}

// TestPriStatsEmptyStoreRendersNone: a fresh store renders "(none)" so
// the user knows it's actually empty (vs. crashed or filtered out).
func TestPriStatsEmptyStoreRendersNone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	out, _, err := runCmd(t, dir, "pri-stats")
	if err != nil {
		t.Fatalf("pri-stats: %v", err)
	}
	if !strings.Contains(out, "(none)") || !strings.Contains(out, "total       0") {
		t.Fatalf("expected (none) + total=0, got:\n%s", out)
	}
}

// TestPriStatsBarRenders: --bar produces inline █ characters; the
// urgent bucket (the most common in this fixture) must have the
// longest bar.
func TestPriStatsBarRenders(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if _, _, err := runCmd(t, dir, "add", "u", "-p", "urgent"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "add", "l", "-p", "low"); err != nil {
		t.Fatalf("add low: %v", err)
	}
	out, _, err := runCmd(t, dir, "pri-stats", "--bar")
	if err != nil {
		t.Fatalf("pri-stats --bar: %v", err)
	}
	if !strings.Contains(out, "█") {
		t.Fatalf("expected '█' in --bar output, got:\n%s", out)
	}
	// Pluck the urgent and low rows; urgent's bar must be longer.
	urgent, low := "", ""
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "urgent"):
			urgent = line
		case strings.HasPrefix(line, "low"):
			low = line
		}
	}
	if strings.Count(urgent, "█") <= strings.Count(low, "█") {
		t.Fatalf("expected urgent bar > low bar, urgent=%q low=%q", urgent, low)
	}
}

// TestPriStatsByTag: --by-tag breaks down per tag, untagged tasks land
// in "(untagged)" bucket, tags are ordered by descending total.
func TestPriStatsByTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "urgent", "-t", "work"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-p", "high", "-t", "work"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "c", "-p", "medium", "-t", "home"); err != nil {
		t.Fatalf("add c: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "d", "-p", "low"); err != nil {
		t.Fatalf("add d (untagged): %v", err)
	}
	out, _, err := runCmd(t, dir, "pri-stats", "--by-tag")
	if err != nil {
		t.Fatalf("pri-stats --by-tag: %v", err)
	}
	for _, want := range []string{"#work", "#home", "(untagged)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in --by-tag output, got:\n%s", want, out)
		}
	}
	// work has 2 tasks, home and (untagged) have 1 each → work first.
	posWork := strings.Index(out, "#work")
	posHome := strings.Index(out, "#home")
	if posWork == -1 || posHome == -1 || posWork > posHome {
		t.Fatalf("expected #work before #home (higher total), got:\n%s", out)
	}
}

// TestPriStatsJSONShape: --json carries total, buckets array (4 entries,
// canonical priority order), by_tag array.
func TestPriStatsJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "high", "-t", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pri-stats", "--by-tag", "--json")
	if err != nil {
		t.Fatalf("pri-stats --json: %v", err)
	}
	var doc struct {
		Total   int `json:"total"`
		Buckets []struct {
			Priority string  `json:"priority"`
			Count    int     `json:"count"`
			Percent  float64 `json:"percent"`
		} `json:"buckets"`
		ByTag []struct {
			Tag     string `json:"tag"`
			Total   int    `json:"total"`
			Buckets []struct {
				Priority string `json:"priority"`
				Count    int    `json:"count"`
			} `json:"buckets"`
		} `json:"by_tag"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if doc.Total != 1 {
		t.Fatalf("total=%d want 1", doc.Total)
	}
	if len(doc.Buckets) != 4 {
		t.Fatalf("buckets len=%d want 4", len(doc.Buckets))
	}
	wantOrder := []string{"urgent", "high", "medium", "low"}
	for i, want := range wantOrder {
		if doc.Buckets[i].Priority != want {
			t.Fatalf("bucket[%d]=%s want %s", i, doc.Buckets[i].Priority, want)
		}
	}
	if len(doc.ByTag) != 1 || doc.ByTag[0].Tag != "x" {
		t.Fatalf("by_tag wrong: %+v", doc.ByTag)
	}
}

// TestPriStatsByTagUntaggedSeparate: tasks without tags go into
// "(untagged)" — they're not silently dropped.
func TestPriStatsByTagUntaggedSeparate(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "tagged", "-t", "real"); err != nil {
		t.Fatalf("add tagged: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "plain"); err != nil {
		t.Fatalf("add plain: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pri-stats", "--by-tag", "--json")
	if err != nil {
		t.Fatalf("pri-stats: %v", err)
	}
	var doc struct {
		ByTag []struct {
			Tag   string `json:"tag"`
			Total int    `json:"total"`
		} `json:"by_tag"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	sawUntagged := false
	for _, g := range doc.ByTag {
		if g.Tag == "(untagged)" && g.Total == 1 {
			sawUntagged = true
		}
	}
	if !sawUntagged {
		t.Fatalf("expected (untagged) group with total=1, got %+v", doc.ByTag)
	}
}

// TestComputePriStatsSyntheticDirect: exercise the aggregator without
// the CLI so we get a sharper signal when the math is wrong.
func TestComputePriStatsSyntheticDirect(t *testing.T) {
	tasks := []model.Task{
		{ID: 1, Priority: model.PriorityUrgent},
		{ID: 2, Priority: model.PriorityUrgent},
		{ID: 3, Priority: model.PriorityHigh},
		{ID: 4, Priority: model.PriorityMedium, Done: true},
		{ID: 5, Priority: model.PriorityLow},
	}
	r := computePriStats(tasks, priStatsScope{}, false)
	if r.Total != 4 { // done excluded by default
		t.Fatalf("total=%d want 4", r.Total)
	}
	want := map[model.Priority]int{
		model.PriorityUrgent: 2,
		model.PriorityHigh:   1,
		model.PriorityMedium: 0,
		model.PriorityLow:    1,
	}
	for _, b := range r.Buckets {
		if b.Count != want[b.Priority] {
			t.Fatalf("bucket %s count=%d want %d", b.Label, b.Count, want[b.Priority])
		}
	}
}

// TestComputePriStatsAllScopeUnion: --all union includes done tasks.
func TestComputePriStatsAllScopeUnion(t *testing.T) {
	tasks := []model.Task{
		{ID: 1, Priority: model.PriorityHigh},
		{ID: 2, Priority: model.PriorityMedium, Done: true},
	}
	r := computePriStats(tasks, priStatsScope{all: true}, false)
	if r.Total != 2 {
		t.Fatalf("total=%d want 2", r.Total)
	}
}

// TestComputePriStatsByTagMultiTag: a single task with multiple tags
// counts in EACH tag's bucket — the natural interpretation for "how
// is the work spread across tags?". Total stays singular.
func TestComputePriStatsByTagMultiTag(t *testing.T) {
	tasks := []model.Task{
		{ID: 1, Priority: model.PriorityHigh, Tags: []string{"a", "b"}},
	}
	r := computePriStats(tasks, priStatsScope{}, true)
	if r.Total != 1 {
		t.Fatalf("total=%d want 1", r.Total)
	}
	if len(r.ByTag) != 2 {
		t.Fatalf("by_tag len=%d want 2", len(r.ByTag))
	}
	for _, g := range r.ByTag {
		if g.Total != 1 {
			t.Fatalf("tag %s total=%d want 1", g.Tag, g.Total)
		}
	}
}

// TestMakeBarMonotonic: bar length should increase with count for a
// fixed total — guard against a refactor that breaks the proportion.
func TestMakeBarMonotonic(t *testing.T) {
	prev := -1
	for c := 0; c <= 20; c++ {
		got := len([]rune(makeBar(c, 20, 20)))
		if got < prev {
			t.Fatalf("bar should be monotonic, got %d after %d at c=%d", got, prev, c)
		}
		prev = got
	}
}

// TestMakeBarMinFloor: a non-zero count must produce at least one bar
// even if rounding would otherwise drop it to zero — visible signal
// beats mathematical precision.
func TestMakeBarMinFloor(t *testing.T) {
	if got := makeBar(1, 100, 20); got != "█" {
		t.Fatalf("expected exactly one █ for count=1/total=100, got %q", got)
	}
}
