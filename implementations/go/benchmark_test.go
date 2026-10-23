package dotreg

import (
	"fmt"
	"strings"
	"testing"
)

var (
	benchmarkEntry  Entry
	benchmarkText   Text
	benchmarkConfig *Config
)

// Generate fixtures outside timed loops. Every document has distinct keys;
// the 512-value fixtures include headings, declarations and untyped values.
func benchmarkSettings(quoted, typed bool) string {
	var out strings.Builder
	out.WriteString("A.\n")
	for group := 0; group < 8; group++ {
		if quoted {
			fmt.Fprintf(&out, " .\"Group\\.%02d\".\n", group)
		} else {
			fmt.Fprintf(&out, " .Group%02d.\n", group)
		}
		for key := 0; key < 64; key++ {
			if quoted {
				fmt.Fprintf(&out, "  .\"Key\\u{2E}%02d\"", key)
			} else {
				fmt.Fprintf(&out, "  .Key%02d", key)
			}
			switch {
			case typed && key%4 == 0:
				out.WriteString("(string): 001\n")
			case typed && key%4 == 1:
				out.WriteString("(int): 42\n")
			case key%4 == 2:
				out.WriteString(": C:\\data\\state.db\n")
			default:
				out.WriteString(": \"hello\\tworld \\u{1F600}\"\n")
			}
		}
	}
	return out.String()
}

func benchmarkBody(escaped bool) string {
	line := strings.Repeat("plain text ", 5) + "\n"
	if escaped {
		line = "hello\\tworld \\u{1F600} \\\"quoted\\\" \\\\ path\n"
	}
	const size = 32 << 10
	return strings.Repeat(line, size/len(line)) + strings.Repeat("x", size%len(line))
}

func benchmarkBlock(body string, form ValueForm) string {
	var out strings.Builder
	if form == TripleQuoted {
		out.WriteString("A: \"\"\"\n")
	} else {
		out.WriteString("A:\n")
	}
	for _, line := range strings.Split(body, "\n") {
		if form == TripleQuoted {
			out.WriteString("  ")
		} else {
			out.WriteString(" |")
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if form == TripleQuoted {
		out.WriteString("  \"\"\"\n")
	}
	return out.String()
}

func benchmarkVisit(entry Entry) error {
	benchmarkEntry = entry
	return nil
}

func BenchmarkParseEach(b *testing.B) {
	fixtures := []struct {
		name, source string
	}{
		{"BareMixed512", benchmarkSettings(false, true)},
		{"QuotedMixed512", benchmarkSettings(true, true)},
		{"LongRaw1MiB", "A: " + strings.Repeat("x", 1<<20) + "\n"},
		{"TriplePlain32KiB", benchmarkBlock(benchmarkBody(false), TripleQuoted)},
		{"TripleEscaped32KiB", benchmarkBlock(benchmarkBody(true), TripleQuoted)},
		{"PipeLiteral32KiB", benchmarkBlock(benchmarkBody(true), PipeBlock)},
	}
	for _, fixture := range fixtures {
		b.Run(fixture.name, func(b *testing.B) {
			if err := ParseEach(strings.NewReader(fixture.source), benchmarkVisit); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(fixture.source)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := ParseEach(strings.NewReader(fixture.source), benchmarkVisit); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkParseDecode512(b *testing.B) {
	source := benchmarkSettings(true, true)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entries, err := Parse(strings.NewReader(source))
		if err != nil {
			b.Fatal(err)
		}
		for _, entry := range entries {
			if entry.Kind == Value {
				benchmarkText, err = DecodeString(entry)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	}
}

func BenchmarkParseBuild512(b *testing.B) {
	source, profile := benchmarkSettings(true, true), DemoProfile()
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entries, err := Parse(strings.NewReader(source))
		if err != nil {
			b.Fatal(err)
		}
		benchmarkConfig, err = Build(entries, profile)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildUntyped512(b *testing.B) {
	source, profile := benchmarkSettings(false, false), DemoProfile()
	entries, err := Parse(strings.NewReader(source))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkConfig, err = Build(entries, profile)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeTriple32KiB(b *testing.B) {
	for _, escaped := range []bool{false, true} {
		name := "Plain"
		if escaped {
			name = "Escaped"
		}
		entry := Entry{Line: 1, Kind: Value, Form: TripleQuoted, Raw: benchmarkBody(escaped)}
		b.Run(name, func(b *testing.B) {
			if _, err := DecodeString(entry); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(entry.Raw)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				benchmarkText, err = DecodeString(entry)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
