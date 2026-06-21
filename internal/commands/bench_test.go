package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBenchSmokeRuns(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if _, _, err := runCmd(t, dir, "add", "task", "-p", "high"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "bench")
	if err != nil {
		t.Fatalf("bench: %v", err)
	}
	for _, want := range []string{"file:", "lines:", "load   min/med/max:", "render min/med/max:"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in output:\n%s", want, stdout)
		}
	}
	if !strings.Contains(stdout, "5 tasks") {
		t.Fatalf("expected task count, got:\n%s", stdout)
	}
}

func TestBenchJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "bench", "--json", "--iter", "3")
	if err != nil {
		t.Fatalf("bench --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	required := []string{
		"path", "size_bytes", "line_count", "task_count",
		"iterations", "go_version", "goos", "goarch",
		"load_us", "render_us",
	}
	for _, k := range required {
		if _, ok := doc[k]; !ok {
			t.Fatalf("missing key %q in JSON:\n%s", k, stdout)
		}
	}
	loadUs, ok := doc["load_us"].(map[string]any)
	if !ok {
		t.Fatalf("load_us should be object, got %T", doc["load_us"])
	}
	for _, k := range []string{"min", "median", "max"} {
		if _, ok := loadUs[k].(float64); !ok {
			t.Fatalf("load_us.%s should be number, got %T", k, loadUs[k])
		}
	}
	if it, _ := doc["iterations"].(float64); int(it) != 3 {
		t.Fatalf("iterations should be 3, got %v", doc["iterations"])
	}
}

func TestBenchRejectsBadIter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "bench", "--iter", "0"); err == nil {
		t.Fatal("expected error for --iter 0")
	}
	if _, _, err := runCmd(t, dir, "bench", "--iter", "-3"); err == nil {
		t.Fatal("expected error for --iter -3")
	}
	if _, _, err := runCmd(t, dir, "bench", "--iter", "10000"); err == nil {
		t.Fatal("expected error for --iter > 1000 cap")
	}
}

func TestFormatMicrosBoundary(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0us"},
		{999, "999us"},
		{1000, "1.00ms"},
		{1234, "1.23ms"},
		{50000, "50.00ms"},
	}
	for _, c := range cases {
		if got := formatMicros(c.in); got != c.want {
			t.Fatalf("formatMicros(%d) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestHumanBytesUnits(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{2048, "2.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{int64(1.5 * 1024 * 1024), "1.5 MB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Fatalf("humanBytes(%d) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestSummarizePhaseSorts(t *testing.T) {
	samples := []time.Duration{
		5 * time.Millisecond,
		2 * time.Millisecond,
		10 * time.Millisecond,
		3 * time.Millisecond,
		8 * time.Millisecond,
	}
	got := summarizePhase(samples)
	if got.MinUs != 2000 {
		t.Fatalf("min should be 2000us, got %d", got.MinUs)
	}
	if got.MaxUs != 10000 {
		t.Fatalf("max should be 10000us, got %d", got.MaxUs)
	}
	// Sorted [2,3,5,8,10] -> index 2 = 5ms = 5000us.
	if got.MedianUs != 5000 {
		t.Fatalf("median should be 5000us, got %d", got.MedianUs)
	}
}

func TestSummarizePhaseEmpty(t *testing.T) {
	got := summarizePhase(nil)
	if got.MinUs != 0 || got.MaxUs != 0 || got.MedianUs != 0 {
		t.Fatalf("empty input should be zero phase, got %+v", got)
	}
}

func TestCountLinesEdges(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"\n", 1},
		{"hello\n", 1},
		{"hello\nworld\n", 2},
		{"hello", 1},        // no trailing newline still counts
		{"hello\nworld", 2}, // ditto
		{"\n\n\n", 3},
	}
	for _, c := range cases {
		if got := countLines([]byte(c.in)); got != c.want {
			t.Fatalf("countLines(%q) = %d want %d", c.in, got, c.want)
		}
	}
}
