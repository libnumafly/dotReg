// Package assembly benchmarks public loading APIs with independently specified
// syntax and decoded results. Both parser variants use this identical module.
package assembly

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	dotreg "github.com/libnumafly/dotReg/implementations/go"
)

type fixture struct {
	name, source string
	want         []dotreg.Entry
	decoded      []string
	collect      bool
	build        bool
}

var (
	entrySink  []dotreg.Entry
	textSink   dotreg.Text
	configSink *dotreg.Config
)

func blockFixture(name string, form dotreg.ValueForm, ending string, body []string, margin int) fixture {
	var source strings.Builder
	source.WriteString("@dotReg 1" + ending + ending + "Payload:")
	if form == dotreg.TripleQuoted {
		source.WriteString(`"""`)
	}
	source.WriteString(ending)
	prefix := strings.Repeat(" ", margin)
	for _, line := range body {
		source.WriteString(prefix)
		if form == dotreg.PipeBlock {
			source.WriteByte('|')
		}
		source.WriteString(line)
		source.WriteString(ending)
	}
	if form == dotreg.TripleQuoted {
		source.WriteString(prefix + `"""` + ending)
	}
	raw := strings.Join(body, ending)
	return fixture{name: name, source: source.String(),
		want:    []dotreg.Entry{{Line: 3, Kind: dotreg.Value, Path: []string{"Payload"}, Raw: raw, Form: form}},
		decoded: []string{raw}}
}

func ordinaryFixture(name string, quoted, build, repeated bool) fixture {
	var source strings.Builder
	source.WriteString("@dotReg 1\n\nRoot.\n")
	result := fixture{name: name, collect: true, build: build,
		want:    []dotreg.Entry{{Line: 3, Kind: dotreg.Heading, Path: []string{"Root"}}},
		decoded: []string{""}}
	for i := 0; i < 512; i++ {
		keyIndex := i
		if repeated {
			keyIndex %= 16
		}
		key := fmt.Sprintf("Key%04d", keyIndex)
		value := fmt.Sprintf("value-%04d", i)
		raw := value
		if quoted {
			key = fmt.Sprintf("Key.%04d", keyIndex)
			raw = fmt.Sprintf(`"value\t%04d"`, i)
			value = fmt.Sprintf("value\t%04d", i)
			fmt.Fprintf(&source, `  ."Key\u{2E}%04d":%s`+"\n", keyIndex, raw)
		} else {
			fmt.Fprintf(&source, "  .%s:%s\n", key, raw)
		}
		result.want = append(result.want, dotreg.Entry{Line: i + 4, Kind: dotreg.Value,
			Path: []string{"Root", key}, Raw: raw})
		result.decoded = append(result.decoded, value)
	}
	result.source = source.String()
	return result
}

func pipeBoundaryFixture() fixture {
	result := blockFixture("PipeBoundary1MiB", dotreg.PipeBlock, "\n", []string{"first", "second", "third"}, 2)
	value := strings.Repeat("x", 1<<20)
	result.source += "Boundary:" + value + "\n"
	result.want = append(result.want, dotreg.Entry{Line: 7, Kind: dotreg.Value, Path: []string{"Boundary"}, Raw: value})
	result.decoded = append(result.decoded, value)
	return result
}

func fixtures() []fixture {
	short := []string{"first", "  second", "third"}
	// 2,048 independently known short ASCII lines produce 32,767 LF body bytes.
	// CRLF changes physical bytes only; its retained raw separators are explicit.
	many := make([]string, 2048)
	for i := range many {
		many[i] = "abcdefghijklmno"
	}
	long := []string{strings.Repeat("x", 1<<20)}
	margin := []string{"tiny", "short", "end"}
	buffered := make([]string, 8)
	for i := range buffered {
		buffered[i] = strings.Repeat("x", 500)
	}
	return []fixture{
		blockFixture("PipeShort3LF", dotreg.PipeBlock, "\n", short, 2),
		blockFixture("TripleShort3LF", dotreg.TripleQuoted, "\n", short, 2),
		blockFixture("PipeManyShortLF", dotreg.PipeBlock, "\n", many, 2),
		blockFixture("PipeManyShortCRLF", dotreg.PipeBlock, "\r\n", many, 2),
		blockFixture("TripleManyShortLF", dotreg.TripleQuoted, "\n", many, 2),
		blockFixture("TripleManyShortCRLF", dotreg.TripleQuoted, "\r\n", many, 2),
		blockFixture("PipeLong1MiB", dotreg.PipeBlock, "\n", long, 2),
		blockFixture("TripleLong1MiB", dotreg.TripleQuoted, "\n", long, 2),
		blockFixture("PipeDiscardedMargin", dotreg.PipeBlock, "\n", margin, 16<<10),
		blockFixture("TripleDiscardedMargin", dotreg.TripleQuoted, "\n", margin, 16<<10),
		pipeBoundaryFixture(),
		blockFixture("PipeBuffered8Rows", dotreg.PipeBlock, "\n", buffered, 2),
		ordinaryFixture("Bare512CollectDecode", false, false, false),
		ordinaryFixture("Quoted512CollectDecode", true, false, false),
		ordinaryFixture("ParseBuildDistinct512", false, true, false),
		ordinaryFixture("ParseBuildRepeated512", false, true, true),
	}
}

func validate(t testing.TB, f fixture) {
	t.Helper()
	entries, err := dotreg.Parse(strings.NewReader(f.source))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(entries, f.want) {
		t.Fatal("syntax entries differ from independent paths, raw text, forms or source lines")
	}
	var visited []dotreg.Entry
	if err := dotreg.ParseEach(strings.NewReader(f.source), func(entry dotreg.Entry) error {
		visited = append(visited, entry)
		return nil
	}); err != nil || !reflect.DeepEqual(visited, f.want) {
		t.Fatalf("streamed entries differ from the same independent oracle: %v", err)
	}
	for i, entry := range entries {
		if entry.Kind != dotreg.Value {
			continue
		}
		text, err := dotreg.DecodeString(entry)
		if err != nil || text.Unset || text.Value != f.decoded[i] {
			t.Fatalf("decoded value %d differs from its independent string: %v", i, err)
		}
	}
	if f.build {
		cfg, err := dotreg.Build(entries, dotreg.DemoProfile())
		if err != nil {
			t.Fatal(err)
		}
		selected, err := cfg.SelectPrefix(nil)
		want := make([]dotreg.SelectedValue, 0, len(f.want)-1)
		for i, entry := range f.want {
			if entry.Kind == dotreg.Value {
				want = append(want, dotreg.SelectedValue{Line: entry.Line,
					Path: entry.Path, RelativePath: entry.Path, Value: f.decoded[i]})
			}
		}
		if err != nil || !reflect.DeepEqual(selected, want) {
			t.Fatalf("built values differ from independently specified occurrence order: %v", err)
		}
	}
}

func TestFixtures(t *testing.T) {
	for _, f := range fixtures() {
		t.Run(f.name, func(t *testing.T) { validate(t, f) })
	}
}

func decodeVisit(entry dotreg.Entry) error {
	if entry.Kind == dotreg.Value {
		var err error
		textSink, err = dotreg.DecodeString(entry)
		return err
	}
	return nil
}

// Block cases time ParseEach followed by DecodeString per value. Ordinary
// collection cases time Parse then DecodeString. Build cases time Parse then
// Build with a profile prepared outside the timer. Validation and fixture
// construction never contribute to the reported operation time or allocation.
func BenchmarkAssembly(b *testing.B) {
	for _, f := range fixtures() {
		b.Run(f.name, func(b *testing.B) {
			validate(b, f)
			profile := dotreg.DemoProfile()
			b.ReportAllocs()
			b.SetBytes(int64(len(f.source)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if !f.collect {
					if err := dotreg.ParseEach(strings.NewReader(f.source), decodeVisit); err != nil {
						b.Fatal(err)
					}
					continue
				}
				var err error
				entrySink, err = dotreg.Parse(strings.NewReader(f.source))
				if err != nil {
					b.Fatal(err)
				}
				if f.build {
					configSink, err = dotreg.Build(entrySink, profile)
					if err != nil {
						b.Fatal(err)
					}
				} else {
					for _, entry := range entrySink {
						if err := decodeVisit(entry); err != nil {
							b.Fatal(err)
						}
					}
				}
			}
		})
	}
}
