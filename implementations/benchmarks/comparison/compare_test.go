package comparison

import (
	"bytes"
	"math/rand"
	"os"
	"strconv"
	"testing"
)

var resultSink Config

func TestAllScenarios(t *testing.T) {
	for _, c := range Cases {
		t.Run(c.Name, func(t *testing.T) {
			expected := Expected(c)
			for _, parser := range Parsers {
				t.Run(parser, func(t *testing.T) {
					data, err := Encode(FormatForParser(parser), expected)
					if err != nil {
						t.Fatal(err)
					}
					if err := Validate(parser, data, expected); err != nil {
						t.Fatal(err)
					}
					again, err := Encode(FormatForParser(parser), expected)
					if err != nil || !bytes.Equal(data, again) {
						t.Fatal("fixture serialisation is not deterministic", err)
					}
				})
			}
		})
	}
}

func TestStringContracts(t *testing.T) {
	expected := Config{"Case": {
		"leadingZero": "001", "booleanLooking": "true", "nullLooking": "null",
		"empty": "", "quote": `a "quoted" value`, "backslash": `C:\path\`,
		"unicode": "が/が/東京/😀", "multiline": "one\n  two\nthree",
		"space": " surrounded ", "tab": "one\ttwo", "interpolation": "%(name)s",
		"key": "lower", "Key": "upper", "markers": "#comment; x=y:literal",
	}}
	for _, parser := range Parsers {
		t.Run(parser, func(t *testing.T) {
			data, err := Encode(FormatForParser(parser), expected)
			if err != nil {
				t.Fatal(err)
			}
			if err := Validate(parser, data, expected); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func BenchmarkConfig(b *testing.B) {
	type operation struct {
		c      Case
		parser string
	}
	var operations []operation
	for _, c := range Cases {
		for _, parser := range Parsers {
			operations = append(operations, operation{c, parser})
		}
	}
	seed := int64(1)
	if raw := os.Getenv("DOTREG_BENCH_SEED"); raw != "" {
		var err error
		seed, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			b.Fatal("invalid DOTREG_BENCH_SEED", err)
		}
	}
	rand.New(rand.NewSource(seed)).Shuffle(len(operations), func(i, j int) {
		operations[i], operations[j] = operations[j], operations[i]
	})
	for _, op := range operations {
		c, parser := op.c, op.parser
		expected := Expected(c)
		data, err := Encode(FormatForParser(parser), expected)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(c.Name+"/"+parser, func(b *testing.B) {
			b.StopTimer()
			if err := Validate(parser, data, expected); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			b.StartTimer()
			for i := 0; i < b.N; i++ {
				result, err := Parse(parser, data)
				if err != nil {
					b.Fatal(err)
				}
				resultSink = result
			}
			b.StopTimer()
			b.ReportMetric(float64(c.Sections*c.PerSection), "values/op")
			b.ReportMetric(float64(LogicalBytes(expected)), "logical-B/op")
			b.ReportMetric(float64(c.Sections*c.PerSection)*float64(b.N)/b.Elapsed().Seconds(), "values/s")
			b.ReportMetric(float64(LogicalBytes(expected))*float64(b.N)/b.Elapsed().Seconds(), "logical-B/s")
		})
	}
}
