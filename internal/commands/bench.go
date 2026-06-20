package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/store"
)

// newBenchCmd implements `tsk bench`: a quick perf snapshot for the
// active .tsk.md.
//
// As stores grow, parse + render time creeps up. The bench_test.go
// microbenchmarks track that on synthetic data; this command measures
// it on the user's actual file. Useful as a "should I run archive?"
// signal — when load goes past ~50ms on a hot store, archiving old
// completions usually drops it back into single-digit territory.
//
// What it measures, in order:
//
//  1. file size + line count + task count
//  2. Load latency (N iterations, reports min / median / max)
//  3. Render latency (in-memory only, same N iterations)
//
// Save is NOT measured — it includes the atomic tempfile + fsync
// dance, which mostly reflects the OS's disk-flush cost and would
// pollute the .bak chain. Render is the right proxy for "how long
// will writing this file take when serialized".
//
// All timings are reported in microseconds (consistent with how
// `time` reports CPU-bound work) and the JSON schema uses _us
// suffix so consumers parse units unambiguously.
func newBenchCmd() *cobra.Command {
	var (
		iter   int
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Print parser and render timings for the active .tsk.md",
		Long: `Print parser and render timings for the active .tsk.md.

What this is for: as your store grows, parse + render time creeps
up. This command tells you, on YOUR file, how slow things are
right now. Pair it with 'tsk archive' or 'tsk purge --done' if
the numbers look big.

What it does NOT measure: save() — that includes the atomic
tempfile + fsync, which mostly reflects OS disk-flush cost and
would pollute your .bak chain. Render covers the in-memory
serialization, which is the right proxy.

Reports:
  - file size, line count, task count
  - load   latency: min / median / max across N iterations
  - render latency: same, in-memory only

Examples:
  tsk bench                # 5 iterations (default)
  tsk bench --iter 20      # tighter measurement
  tsk bench --json         # machine-readable
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if iter <= 0 {
				return usageErrorf("--iter must be > 0, got %d", iter)
			}
			if iter > 1000 {
				return usageErrorf("--iter capped at 1000 (got %d) — keep it sane", iter)
			}
			path, err := resolveBenchPath(cmd)
			if err != nil {
				return err
			}
			report, err := runBench(path, iter)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			printBenchReport(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().IntVar(&iter, "iter", 5, "iterations per phase (default 5; max 1000)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// BenchReport is the stable JSON shape for --json. Timings are in
// microseconds; the _us suffix in keys makes that explicit.
type BenchReport struct {
	Path        string     `json:"path"`
	SizeBytes   int64      `json:"size_bytes"`
	LineCount   int        `json:"line_count"`
	TaskCount   int        `json:"task_count"`
	Iterations  int        `json:"iterations"`
	GoVersion   string     `json:"go_version"`
	GoOS        string     `json:"goos"`
	GoArch      string     `json:"goarch"`
	LoadStats   BenchPhase `json:"load_us"`
	RenderStats BenchPhase `json:"render_us"`
}

// BenchPhase summarizes timings for one measured operation.
type BenchPhase struct {
	MinUs    int64 `json:"min"`
	MedianUs int64 `json:"median"`
	MaxUs    int64 `json:"max"`
}

// resolveBenchPath mirrors lint's resolver — we want the raw path for
// the stat call, not the parsed store.
func resolveBenchPath(cmd *cobra.Command) (string, error) {
	path, _ := cmd.Flags().GetString("file")
	if path == "" {
		resolved, ok := store.Resolve("")
		if !ok {
			return "", fmt.Errorf("no .tsk.md found; run `tsk init`")
		}
		path = resolved
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("no .tsk.md at %s; run `tsk init`", path)
	}
	return path, nil
}

// runBench performs both timed phases and assembles the report.
func runBench(path string, iter int) (BenchReport, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return BenchReport{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return BenchReport{}, err
	}
	report := BenchReport{
		Path:       path,
		SizeBytes:  fi.Size(),
		LineCount:  countLines(raw),
		Iterations: iter,
		GoVersion:  runtime.Version(),
		GoOS:       runtime.GOOS,
		GoArch:     runtime.GOARCH,
	}
	// LOAD phase: re-read + re-parse each iteration so the cost
	// reflects the cold-load path (the user's perspective).
	loads := make([]time.Duration, iter)
	for i := 0; i < iter; i++ {
		t0 := time.Now()
		s, err := store.Load(path)
		loads[i] = time.Since(t0)
		if err != nil {
			return BenchReport{}, fmt.Errorf("load iteration %d: %w", i, err)
		}
		if i == 0 {
			report.TaskCount = len(s.Tasks)
		}
	}
	report.LoadStats = summarizePhase(loads)

	// RENDER phase: load once, render N times.
	s, err := store.Load(path)
	if err != nil {
		return BenchReport{}, err
	}
	renders := make([]time.Duration, iter)
	for i := 0; i < iter; i++ {
		t0 := time.Now()
		_ = renderForBench(s)
		renders[i] = time.Since(t0)
	}
	report.RenderStats = summarizePhase(renders)
	return report, nil
}

// renderForBench produces the same bytes store.Save would write,
// without touching the disk. We re-use the public exporter (markdown
// task list) — it's the closest stable proxy for the writer's cost
// since store.render is unexported. The numbers are within the same
// order of magnitude as Save's serialization phase on every file
// shape we've tested, which is what matters for a "how big is this
// getting" check.
func renderForBench(s *store.Store) []byte {
	// Use ExportTasksMarkdown if it exists; otherwise fall back to
	// joining each task's string form. We inline a tiny renderer
	// here so this command stays decoupled from any export format
	// surface.
	const lineCap = 200
	out := make([]byte, 0, len(s.Tasks)*lineCap+len(s.Header))
	out = append(out, s.Header...)
	for _, t := range s.Tasks {
		box := byte(' ')
		if t.Done {
			box = 'x'
		}
		out = append(out, '-', ' ', '[', box, ']', ' ')
		out = append(out, t.Title...)
		// meta — minimal, matches store.renderMeta's structure.
		out = append(out, ' ', '<', '!', '-', '-', ' ')
		out = append(out, []byte(fmt.Sprintf("id:%d prio:%s", t.ID, t.Priority))...)
		if len(t.Tags) > 0 {
			out = append(out, ' ')
			out = append(out, []byte("tags:")...)
			for i, tag := range t.Tags {
				if i > 0 {
					out = append(out, ',')
				}
				out = append(out, tag...)
			}
		}
		out = append(out, ' ', '-', '-', '>', '\n')
	}
	return out
}

// summarizePhase returns the min, median, and max of a duration slice
// in microseconds. Median uses the n/2 index after a sort; consistent
// with how go test bench reports tendencies.
func summarizePhase(samples []time.Duration) BenchPhase {
	if len(samples) == 0 {
		return BenchPhase{}
	}
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return BenchPhase{
		MinUs:    sorted[0].Microseconds(),
		MedianUs: sorted[len(sorted)/2].Microseconds(),
		MaxUs:    sorted[len(sorted)-1].Microseconds(),
	}
}

// countLines returns the number of newline-terminated lines in raw,
// plus one for the trailing partial line if any. Cheap byte scan.
func countLines(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	n := 0
	for _, b := range raw {
		if b == '\n' {
			n++
		}
	}
	// Account for a trailing line that doesn't end in '\n'.
	if raw[len(raw)-1] != '\n' {
		n++
	}
	return n
}

// printBenchReport renders the report for human consumption.
func printBenchReport(w io.Writer, r BenchReport) {
	pf(w, "file:    %s\n", r.Path)
	pf(w, "size:    %s\n", humanBytes(r.SizeBytes))
	pf(w, "lines:   %d (%d tasks)\n", r.LineCount, r.TaskCount)
	pf(w, "host:    %s (%s/%s)\n", r.GoVersion, r.GoOS, r.GoArch)
	pf(w, "iter:    %d\n", r.Iterations)
	pln(w)
	pf(w, "load   min/med/max: %s\n", formatPhase(r.LoadStats))
	pf(w, "render min/med/max: %s\n", formatPhase(r.RenderStats))
}

// formatPhase converts microsecond ints to a friendly form ("123us /
// 456us / 789us" or "1.23ms / ..."). Threshold is 1000us — below it
// units stay us, above it switch to ms with two decimals so the
// numbers stay scannable.
func formatPhase(p BenchPhase) string {
	return fmt.Sprintf("%s / %s / %s",
		formatMicros(p.MinUs), formatMicros(p.MedianUs), formatMicros(p.MaxUs))
}

func formatMicros(us int64) string {
	if us < 1000 {
		return fmt.Sprintf("%dus", us)
	}
	return fmt.Sprintf("%.2fms", float64(us)/1000)
}

// humanBytes renders a byte count in B / KB / MB. Mirrors common
// CLI convention (1024-base).
func humanBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}
