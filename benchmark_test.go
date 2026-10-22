package gocloc

import (
	"flag"
	"os"
	"testing"
)

var (
	benchmarkPath    = flag.String("gocloc.bench-path", "", "directory to analyze in BenchmarkProcessorAnalyze")
	benchmarkWorkers = flag.Int("gocloc.bench-workers", 0, "number of analysis workers in BenchmarkProcessorAnalyze")
	benchmarkDedup   = flag.Bool("gocloc.bench-dedup", false, "enable duplicate-file detection in BenchmarkProcessorAnalyze")
)

func newBenchmarkOptions(workers int, dedup bool) *Options {
	return &Options{Workers: workers, Dedup: dedup}
}

// BenchmarkProcessorAnalyze profiles real-directory scanning, duplicate detection,
// and line analysis. Supply -gocloc.bench-path to run it.
func BenchmarkProcessorAnalyze(b *testing.B) {
	b.StopTimer()
	if *benchmarkPath == "" {
		b.Skip("set -gocloc.bench-path to the directory to analyze")
	}
	info, err := os.Stat(*benchmarkPath)
	if err != nil {
		b.Fatalf("stat benchmark path: %v", err)
	}
	if !info.IsDir() {
		b.Fatalf("benchmark path %q is not a directory", *benchmarkPath)
	}

	paths := []string{*benchmarkPath}
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
	for range b.N {
		opts := newBenchmarkOptions(*benchmarkWorkers, *benchmarkDedup)
		if _, err := Analyze(paths, opts); err != nil {
			b.Fatal(err)
		}
	}
}
