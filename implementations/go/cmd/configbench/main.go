// configbench measures common string decoding via streaming or collecting APIs.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	dotreg "github.com/libnumafly/dotReg/implementations/go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "benchmark failed")
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("arguments")
	}
	iterations, err := strconv.Atoi(os.Args[2])
	mode := os.Args[3]
	if err != nil || iterations < 1 || (mode != "stream" && mode != "collect") {
		return fmt.Errorf("arguments")
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	type counts struct{ entries, decodedBytes int }
	once := func() (counts, error) {
		if _, err := file.Seek(0, 0); err != nil {
			return counts{}, err
		}
		count := counts{}
		visit := func(e dotreg.Entry) error {
			if e.Kind == dotreg.Value {
				text, err := dotreg.DecodeString(e)
				if err != nil {
					return err
				}
				count.decodedBytes += len(text.Value)
			}
			count.entries++
			return nil
		}
		if mode == "stream" {
			err := dotreg.ParseEach(file, visit)
			return count, err
		}
		entries, err := dotreg.Parse(file)
		if err != nil {
			return counts{}, err
		}
		for _, e := range entries {
			if err := visit(e); err != nil {
				return counts{}, err
			}
		}
		return count, nil
	}
	expected, err := once()
	if err != nil {
		return err
	}
	if second, err := once(); err != nil || second != expected {
		return fmt.Errorf("inconsistent warm-up")
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	result := counts{}
	for i := 0; i < iterations; i++ {
		result, err = once()
		if err != nil {
			return err
		}
		if result != expected {
			return fmt.Errorf("inconsistent pass")
		}
	}
	elapsed := time.Since(start).Seconds()
	runtime.ReadMemStats(&after)
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"language": "go", "mode": mode, "iterations": iterations, "input_bytes": info.Size(),
		"entries": result.entries, "decoded_utf8_bytes": result.decodedBytes,
		"elapsed_seconds": elapsed, "allocated_bytes": after.TotalAlloc - before.TotalAlloc,
		"memory_method": "runtime.MemStats.TotalAlloc delta; allocation volume, not peak memory",
	})
}
