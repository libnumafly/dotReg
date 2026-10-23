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
	once := func() (int, error) {
		if _, err := file.Seek(0, 0); err != nil {
			return 0, err
		}
		count := 0
		visit := func(e dotreg.Entry) error {
			if e.Kind == dotreg.Value {
				if _, err := dotreg.DecodeString(e); err != nil {
					return err
				}
			}
			count++
			return nil
		}
		if mode == "stream" {
			err := dotreg.ParseEach(file, visit)
			return count, err
		}
		entries, err := dotreg.Parse(file)
		if err != nil {
			return 0, err
		}
		for _, e := range entries {
			if err := visit(e); err != nil {
				return 0, err
			}
		}
		return count, nil
	}
	for i := 0; i < 2; i++ {
		if _, err := once(); err != nil {
			return err
		}
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	entries := 0
	for i := 0; i < iterations; i++ {
		entries, err = once()
		if err != nil {
			return err
		}
	}
	elapsed := time.Since(start).Seconds()
	runtime.ReadMemStats(&after)
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"language": "go", "mode": mode, "iterations": iterations, "input_bytes": info.Size(),
		"entries": entries, "elapsed_seconds": elapsed, "allocated_bytes": after.TotalAlloc - before.TotalAlloc,
		"memory_method": "runtime.MemStats.TotalAlloc delta; allocation volume, not peak memory",
	})
}
