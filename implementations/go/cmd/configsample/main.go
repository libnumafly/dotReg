// configsample prints syntax, decoded strings, or sample-profile values as JSON.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/libnumafly/dotReg/implementations/go"
)

type output struct {
	dotreg.Entry
	Decoded *string `json:"decoded,omitempty"`
	Value   any     `json:"value,omitempty"`
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("configsample", flag.ContinueOnError)
	flags.SetOutput(stderr)
	mode := flags.String("mode", "strings", "output mode: raw, strings, or demo")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: configsample [-mode raw|strings|demo] [file|-]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return fmt.Errorf("invalid command arguments")
	}
	if flags.NArg() > 1 || (*mode != "raw" && *mode != "strings" && *mode != "demo") {
		flags.Usage()
		return fmt.Errorf("invalid command arguments")
	}
	reader := stdin
	if flags.NArg() == 1 && flags.Arg(0) != "-" {
		file, err := os.Open(flags.Arg(0))
		if err != nil {
			return fmt.Errorf("input file could not be opened")
		}
		defer file.Close()
		reader = file
	}
	entries, err := dotreg.Parse(reader)
	if err != nil {
		return err
	}
	var config *dotreg.Config
	var occurrences map[string]int
	if *mode == "demo" {
		config, err = dotreg.Build(entries, dotreg.DemoProfile())
		if err != nil {
			return err
		}
		occurrences = make(map[string]int)
	}
	results := make([]output, 0, len(entries))
	for _, entry := range entries {
		result := output{Entry: entry}
		if entry.Kind == dotreg.Value {
			switch *mode {
			case "strings":
				text, err := dotreg.DecodeString(entry)
				if err != nil {
					return err
				}
				result.Decoded = &text.Value
			case "demo":
				want := entry.Type
				if !entry.HasType {
					want = "string"
				}
				// JSON preserves element boundaries, empty names and controls.
				// Headings do not consume a value occurrence.
				encodedPath, _ := json.Marshal(entry.Path)
				key := string(encodedPath)
				index := occurrences[key]
				result.Value, err = config.GetAt(entry.Path, index, want)
				if err != nil {
					return err
				}
				occurrences[key] = index + 1
			}
		}
		results = append(results, result)
	}
	// Build and validate everything before emitting output.
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON output could not be encoded")
	}
	data = append(data, '\n')
	if _, err := stdout.Write(data); err != nil {
		return fmt.Errorf("output write failed")
	}
	return nil
}
