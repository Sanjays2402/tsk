package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// seedStore writes a .tsk.md with n tasks into dir. Called once per bench
// iteration only when the file doesn't already exist; keeps the bench
// focused on Load cost, not write cost.
func seedStore(tb testing.TB, dir string, n int) string {
	tb.Helper()
	path := filepath.Join(dir, ".tsk.md")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	s, err := Load(path)
	if err != nil {
		tb.Fatalf("seed load: %v", err)
	}
	base := time.Now()
	for i := 0; i < n; i++ {
		due := base.AddDate(0, 0, i%30)
		s.Add(model.Task{
			Title:    fmt.Sprintf("task %d — with some realistic text padding", i),
			Priority: model.Priority(i % 4),
			Due:      &due,
			Tags:     []string{"work", "benchmark", fmt.Sprintf("bucket%d", i%7)},
			Notes:    strings.Repeat("note line\n", 2),
			Done:     i%5 == 0,
		})
	}
	if err := s.Save(); err != nil {
		tb.Fatalf("seed save: %v", err)
	}
	return path
}

// BenchmarkLoad10K — should stay well under 50ms on Apple Silicon.
// If this regresses to >100ms somebody has accidentally introduced an
// O(n^2) step in the parser.
func BenchmarkLoad10K(b *testing.B) {
	dir := b.TempDir()
	path := seedStore(b, dir, 10000)
	fi, _ := os.Stat(path)
	b.ReportMetric(float64(fi.Size())/1024, "KB")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, err := Load(path)
		if err != nil {
			b.Fatalf("load: %v", err)
		}
		if len(s.Tasks) != 10000 {
			b.Fatalf("expected 10000 tasks, got %d", len(s.Tasks))
		}
	}
}

func BenchmarkLoad1K(b *testing.B) {
	dir := b.TempDir()
	path := seedStore(b, dir, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Load(path); err != nil {
			b.Fatalf("load: %v", err)
		}
	}
}

func BenchmarkRender10K(b *testing.B) {
	dir := b.TempDir()
	path := seedStore(b, dir, 10000)
	s, _ := Load(path)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.render()
	}
}

// TestLoadScales10K — correctness guard paired with the bench. Makes sure
// 10k tasks survive the round trip (catches silent field drops that a
// microbenchmark wouldn't notice).
func TestLoadScales10K(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 10k scale test in -short mode")
	}
	dir := t.TempDir()
	path := seedStore(t, dir, 10000)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s.Tasks) != 10000 {
		t.Fatalf("expected 10000 tasks, got %d", len(s.Tasks))
	}
	// Spot-check: every task keeps its priority across the round trip.
	for i, task := range s.Tasks {
		if int(task.Priority) != i%4 {
			t.Fatalf("task %d priority drift: got %v, want %v", i, task.Priority, model.Priority(i%4))
		}
	}
}
