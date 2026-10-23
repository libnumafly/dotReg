package dotreg

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

// Header classification precedes path and scope checks. Keep the observable
// error category and line stable even when more than one condition is wrong.
func TestRefactorHeaderErrorPrecedence(t *testing.T) {
	tests := []struct {
		name, source string
		code         Code
		line, visits int
	}{
		{"bare path before empty type", "A..B():private", ErrUnsupported, 1, 0},
		{"quoted boundary before empty type", "\"A\"tail():private", ErrUnsupported, 1, 0},
		{"invalid escape before empty type", "\"A\\q\"():private", ErrSyntax, 1, 0},
		{"missing scope and empty type", " .A..B():private", ErrUnsupported, 1, 0},
		{"dedented scope and empty type", "A.\n .B.\n .C:ok\n  .D():private", ErrUnsupported, 4, 3},
		{"dedented scope and valid type", "A.\n .B.\n .C:ok\n  .D(T):private", ErrSyntax, 4, 3},
		{"absolute closed scope and empty type", "A.\nB:ok\n .C():private", ErrUnsupported, 3, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			visits := 0
			err := ParseEach(strings.NewReader(parserDocument(test.source)), func(Entry) error {
				visits++
				return nil
			})
			parserRequireCode(t, err, test.code, test.line+2)
			if visits != test.visits || strings.Contains(err.Error(), "private") {
				t.Fatalf("visits=%d, error=%v; want %d visits without value text", visits, err, test.visits)
			}
			if entries, err := Parse(strings.NewReader(parserDocument(test.source))); entries != nil || err == nil {
				t.Fatal("failed collection returned a partial result")
			}
		})
	}
}

func TestRefactorRepeatedKeysPreserveLaterFailurePriority(t *testing.T) {
	for _, test := range []struct {
		source string
		code   Code
		line   int
		visits int
	}{
		{"A:red\nA():private", ErrUnsupported, 2, 1},
		{"A:red\nA:\"\"\"\nunclosed", ErrSyntax, 2, 1},
		{"A:red\nA:\n |\xff", ErrSyntax, 3, 1},
		{"A:red\nA:green\nA:\xff", ErrSyntax, 3, 2},
	} {
		t.Run(test.source, func(t *testing.T) {
			var got []Entry
			err := ParseEach(strings.NewReader(parserDocument(test.source)), func(entry Entry) error {
				got = append(got, entry)
				return nil
			})
			parserRequireCode(t, err, test.code, test.line+2)
			if len(got) != test.visits || got[0].Raw != "red" {
				t.Errorf("valid occurrences before failure=%#v", got)
			}
			if entries, err := Parse(strings.NewReader(parserDocument(test.source))); err == nil || entries != nil {
				t.Fatalf("failed collection returned partial occurrences: %#v, %v", entries, err)
			}
		})
	}
}

type refactorTailErrorReader struct {
	data string
	err  error
}

func (reader *refactorTailErrorReader) Read(p []byte) (int, error) {
	n := copy(p, reader.data)
	reader.data = reader.data[n:]
	if reader.data == "" {
		return n, reader.err // Data and its error may arrive in the same Read.
	}
	return n, nil
}

func TestRefactorPipeLookaheadDefersLongPartialReadFailure(t *testing.T) {
	const private = "private reader failure"
	source := "A:\n |literal\nB:" + strings.Repeat("private-value", 10000)
	stop := errors.New("stop before replay")
	for _, stopAtCurrent := range []bool{true, false} {
		reader := &refactorTailErrorReader{data: source, err: errors.New(private)}
		visits := 0
		err := ParseEach(io.MultiReader(strings.NewReader(parserPreamble), reader), func(entry Entry) error {
			visits++
			if entry.Line != 3 || entry.Form != PipeBlock || entry.Raw != "literal" || entry.Unset {
				t.Fatalf("lookahead altered the complete pipe entry: %#v", entry)
			}
			if stopAtCurrent {
				return stop
			}
			return nil
		})
		if stopAtCurrent {
			if err != stop {
				t.Fatalf("peeked failure preempted callback: %v", err)
			}
		} else {
			parserRequireCode(t, err, ErrInput, 5)
			if strings.Contains(err.Error(), private) || strings.Contains(err.Error(), "private-value") {
				t.Fatal("read diagnostic leaked source or underlying error")
			}
		}
		if visits != 1 {
			t.Fatalf("received %d callbacks, want one complete pipe entry", visits)
		}
	}
	entries, err := Parse(io.MultiReader(strings.NewReader(parserPreamble), &refactorTailErrorReader{data: source, err: errors.New(private)}))
	parserRequireCode(t, err, ErrInput, 5)
	if entries != nil {
		t.Fatal("long partial read failure returned partial entries")
	}
}

func TestRefactorTripleQuoteClosingCandidates(t *testing.T) {
	tests := []struct{ raw, decoded string }{
		{`"""tail`, `"""tail`},
		{`prefix """`, `prefix """`},
		{`""""`, `""""`},
		{`\"""`, `"""`},
		{`\"\""`, `"""`},
		{`\"\"\"`, `"""`},
		{`\u0022""`, `"""`},
	}
	for _, test := range tests {
		// A tab is allowed after the actual closing marker, never as margin.
		source := "A: \"\"\"\r\n  " + test.raw + "\r\n  \"\"\"\t \nB:next"
		entries, err := Parse(strings.NewReader(parserDocument(source)))
		if err != nil || len(entries) != 2 {
			t.Fatalf("raw %q: entries=%#v, error=%v", test.raw, entries, err)
		}
		if entries[0].Raw != test.raw || entries[0].Form != TripleQuoted || entries[1].Line != 6 {
			t.Fatalf("closing-candidate structure changed: %#v", entries)
		}
		decoded, err := DecodeString(entries[0])
		if err != nil || decoded != (Text{Value: test.decoded}) {
			t.Fatalf("raw %q: decoded=%#v, error=%v; want %q", test.raw, decoded, err, test.decoded)
		}
	}
}

func TestRefactorVisitorEntriesSurviveLaterInput(t *testing.T) {
	source := "\"A\\u{2E}B\".\n .\"C\\\"D\": \" value \"\n"
	for i := 0; i < 8; i++ {
		source += " ." + strings.Repeat("K", i+1) + ":" + strings.Repeat(string(rune('a'+i)), 5000) + "\n"
	}
	var retained, snapshots []Entry
	err := ParseEach(strings.NewReader(parserDocument(source)), func(entry Entry) error {
		retained = append(retained, entry)
		snapshot := entry
		snapshot.Raw = strings.Clone(entry.Raw)
		snapshot.Path = make([]string, len(entry.Path))
		for i, element := range entry.Path {
			snapshot.Path[i] = strings.Clone(element)
		}
		snapshots = append(snapshots, snapshot)
		return nil
	})
	if err != nil || !reflect.DeepEqual(retained, snapshots) {
		t.Fatalf("callback-owned entries changed after later reads: %v", err)
	}
}

func TestRefactorManualEntriesStayValidatedAndUnmodified(t *testing.T) {
	long := strings.Repeat("plain ", 8192)
	entries := []Entry{
		{Line: 7, Kind: Value, Path: []string{"A", ""}, Form: PipeBlock, Raw: " \t" + long + "\r\n  "},
		{Line: 11, Kind: Value, Path: []string{"B"}, Form: TripleQuoted, Raw: long + "\\u{1F600}"},
	}
	want := append([]Entry(nil), entries...)
	for i := range want {
		want[i].Path = append([]string(nil), entries[i].Path...)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := cfg.Get([]string{"A", ""}, "string")
	if err != nil || pipe != entries[0].Raw {
		t.Fatal("manual literal block was trimmed or reinterpreted")
	}
	triple, err := cfg.Get([]string{"B"}, "string")
	if err != nil || triple != long+"😀" {
		t.Fatal("escape after a long plain prefix was not decoded")
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatal("Build modified caller-owned entries")
	}
	for _, invalid := range []Entry{
		{Line: 19, Kind: Value, Path: []string{"C"}, Form: ValueForm("unknown"), Raw: long},
		{Line: 19, Kind: Value, Path: []string{"C"}, Form: TripleQuoted, Raw: long + "\\q"},
		{Line: 19, Kind: Value, Path: []string{"C"}, Form: PipeBlock, Raw: long + "\r"},
		{Line: 19, Kind: Value, Path: []string{"C"}, Form: TripleQuoted, Raw: long + "\xff"},
	} {
		_, err := DecodeString(invalid)
		parserRequireCode(t, err, ErrString, 19)
		input := append(append([]Entry(nil), entries...), invalid)
		cfg, err := Build(input, DemoProfile())
		parserRequireCode(t, err, ErrString, 19)
		if cfg != nil || !reflect.DeepEqual(input[len(input)-1], invalid) {
			t.Fatal("failed Build returned partial configuration or modified its input")
		}
	}
}
