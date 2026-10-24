// configcompare generates inspectable fixtures or measures one warmed parser.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	comparison "github.com/libnumafly/dotReg/benchmarks/comparison"
)

var sink comparison.Config

func main() {
	generate := flag.String("generate", "", "write all fixtures and manifest to this directory")
	parser := flag.String("format", "dotreg-direct", "dotreg-direct, dotreg-collected, json, toml, yaml or ini")
	fixture := flag.String("fixture", "", "generated fixture case directory (for example fixtures/large)")
	iterations := flag.Int("iterations", 100, "number of timed parses")
	warmup := flag.Int("warmup", 5, "untimed warmup parses")
	flag.Parse()
	runtime.GOMAXPROCS(1)
	debug.SetGCPercent(100)
	debug.SetMemoryLimit(math.MaxInt64)
	if *generate != "" {
		if err := comparison.WriteFixtures(*generate); err != nil {
			fatal(err)
		}
		return
	}
	if *fixture == "" || *iterations < 1 || *warmup < 0 {
		fatal(fmt.Errorf("provide -fixture, positive -iterations and nonnegative -warmup"))
	}
	c, err := comparison.FindCase(filepath.Base(filepath.Clean(*fixture)))
	if err != nil {
		fatal(err)
	}
	format := comparison.FormatForParser(*parser)
	extension, exists := comparison.Extensions[format]
	if !exists {
		fatal(fmt.Errorf("unknown format %q", *parser))
	}
	data, err := os.ReadFile(filepath.Join(*fixture, "config."+extension))
	if err != nil {
		fatal(err)
	}
	expected := comparison.Expected(c)
	if err := comparison.Validate(*parser, data, expected); err != nil {
		fatal(err)
	}
	for i := 0; i < *warmup; i++ {
		sink, err = comparison.Parse(*parser, data)
		if err != nil {
			fatal(err)
		}
	}
	sink = nil
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	for i := 0; i < *iterations; i++ {
		sink, err = comparison.Parse(*parser, data)
		if err != nil {
			fatal(err)
		}
	}
	elapsed := time.Since(started)
	runtime.ReadMemStats(&after)
	actualFingerprint := comparison.Fingerprint(sink)
	if actualFingerprint != comparison.Fingerprint(expected) {
		fatal(fmt.Errorf("final result fingerprint mismatch"))
	}
	output := map[string]any{
		"parser": *parser, "case": c.Name, "go": runtime.Version(),
		"gomaxprocs": runtime.GOMAXPROCS(0), "gc_percent": 100,
		"go_memory_limit_bytes": debug.SetMemoryLimit(-1),
		"iterations":            *iterations, "warmup": *warmup, "encoded_bytes": len(data),
		"values": c.Sections * c.PerSection, "logical_payload_bytes": comparison.LogicalBytes(expected),
		"semantic_sha256": actualFingerprint, "elapsed_ns": elapsed.Nanoseconds(),
		"ns_per_op":              float64(elapsed.Nanoseconds()) / float64(*iterations),
		"allocated_bytes_per_op": float64(after.TotalAlloc-before.TotalAlloc) / float64(*iterations),
		"allocations_per_op":     float64(after.Mallocs-before.Mallocs) / float64(*iterations),
		"gc_cycles":              after.NumGC - before.NumGC,
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
