package dotreg

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

const parserPreamble = "@dotReg 0.1\n\n"

func parserDocument(source string) string {
	if strings.HasPrefix(source, "\ufeff") {
		return "\ufeff" + parserPreamble + strings.TrimPrefix(source, "\ufeff")
	}
	return parserPreamble + source
}

func parserValue(line int, path []string, raw string) Entry {
	return Entry{Line: line + 2, Kind: Value, Path: path, Raw: raw, Unset: raw == ""}
}

func parserHeading(line int, path ...string) Entry {
	return Entry{Line: line + 2, Kind: Heading, Path: path}
}

func parserRequireCode(t *testing.T, err error, code Code, line int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %v, got success", code)
	}
	var diagnostic *Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if diagnostic.Code != code {
		t.Errorf("error code = %v, want %v: %v", diagnostic.Code, code, err)
	}
	if line > 0 && diagnostic.Line != line {
		t.Errorf("error line = %d, want %d", diagnostic.Line, line)
	}
}

func TestParseAccepted(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []Entry
	}{
		{
			name:   "one element and leading zero",
			source: "A:001",
			want:   []Entry{parserValue(1, []string{"A"}, "001")},
		},
		{
			name:   "flat keys and descendants coexist",
			source: "A: root\nA.B: parent\nA.B.C: child\n",
			want: []Entry{
				parserValue(1, []string{"A"}, "root"),
				parserValue(2, []string{"A", "B"}, "parent"),
				parserValue(3, []string{"A", "B", "C"}, "child"),
			},
		},
		{
			name:   "unicode numeric case and punctuation",
			source: "1: a\n01: b\n1A.2: c\nA.B: d\nA.b: e\n店舗.名称: 本店\n端末-1.有効: true\nA_B.Code: 001\n",
			want: []Entry{
				parserValue(1, []string{"1"}, "a"),
				parserValue(2, []string{"01"}, "b"),
				parserValue(3, []string{"1A", "2"}, "c"),
				parserValue(4, []string{"A", "B"}, "d"),
				parserValue(5, []string{"A", "b"}, "e"),
				parserValue(6, []string{"店舗", "名称"}, "本店"),
				parserValue(7, []string{"端末-1", "有効"}, "true"),
				parserValue(8, []string{"A_B", "Code"}, "001"),
			},
		},
		{
			name:   "raw rhs is trimmed only at ASCII edges",
			source: "A \t: \thello  world \t\nB:\u3000value\u00a0\nC: first:second # literal\nD: #literal\n",
			want: []Entry{
				parserValue(1, []string{"A"}, "hello  world"),
				parserValue(2, []string{"B"}, "\u3000value\u00a0"),
				parserValue(3, []string{"C"}, "first:second # literal"),
				parserValue(4, []string{"D"}, "#literal"),
			},
		},
		{
			name:   "unset and explicit values stay distinct",
			source: "A:\nB: \t \nC: \"\"\nD: null\nE: value:\n",
			want: []Entry{
				parserValue(1, []string{"A"}, ""),
				parserValue(2, []string{"B"}, ""),
				parserValue(3, []string{"C"}, "\"\""),
				parserValue(4, []string{"D"}, "null"),
				parserValue(5, []string{"E"}, "value:"),
			},
		},
		{
			name:   "CRLF and EOF",
			source: "\r\n# comment\r\nA: first\r\nB: second",
			want: []Entry{
				parserValue(3, []string{"A"}, "first"),
				parserValue(4, []string{"B"}, "second"),
			},
		},
		{
			name:   "one initial BOM only",
			source: "\ufeff\ufeffA: \ufeffvalue\nB\ufeff: x\n",
			want: []Entry{
				parserValue(1, []string{"\ufeffA"}, "\ufeffvalue"),
				parserValue(2, []string{"B\ufeff"}, "x"),
			},
		},
		{
			name:   "quoted elements preserve boundaries",
			source: "\"A.B\": first\nA.B: second\nA.\"B.C\": third\nA.B.C: fourth\n",
			want: []Entry{
				parserValue(1, []string{"A.B"}, "first"),
				parserValue(2, []string{"A", "B"}, "second"),
				parserValue(3, []string{"A", "B.C"}, "third"),
				parserValue(4, []string{"A", "B", "C"}, "fourth"),
			},
		},
		{
			name:   "empty names at every position",
			source: "\"\": x\n\"\".A: y\nA.\"\": z\nA.\"\".B: q\n\"\".\"\": r\n",
			want: []Entry{
				parserValue(1, []string{""}, "x"),
				parserValue(2, []string{"", "A"}, "y"),
				parserValue(3, []string{"A", ""}, "z"),
				parserValue(4, []string{"A", "", "B"}, "q"),
				parserValue(5, []string{"", ""}, "r"),
			},
		},
		{
			name:   "quoted key escapes decode once",
			source: "\"A\\.B\": x\n\"C\\\\.D\": y\n\"E\\\"F\": z\n",
			want: []Entry{
				parserValue(1, []string{"A.B"}, "x"),
				parserValue(2, []string{"C\\.D"}, "y"),
				parserValue(3, []string{"E\"F"}, "z"),
			},
		},
		{
			name:   "comments preserve scope",
			source: "A.\n  .B.\n\n\t  # quoted \"\\q and tab indentation are comment text\n    .C: value\n",
			want: []Entry{
				parserHeading(1, "A"),
				parserHeading(2, "A", "B"),
				parserValue(5, []string{"A", "B", "C"}, "value"),
			},
		},
		{
			name:   "heading declaration depth differs from element count",
			source: "A.B.C.\n  .D.E.\n    .F: first\n  .G: second\n",
			want: []Entry{
				parserHeading(1, "A", "B", "C"),
				parserHeading(2, "A", "B", "C", "D", "E"),
				parserValue(3, []string{"A", "B", "C", "D", "E", "F"}, "first"),
				parserValue(4, []string{"A", "B", "C", "G"}, "second"),
			},
		},
		{
			name:   "self empty child and literal dot child",
			source: "A.\n  .:\n  .\"\":\n  .\".\": dot\n",
			want: []Entry{
				parserHeading(1, "A"),
				parserValue(2, []string{"A"}, ""),
				parserValue(3, []string{"A", ""}, ""),
				parserValue(4, []string{"A", "."}, "dot"),
			},
		},
		{
			name:   "empty heading self",
			source: "\"\".\n  .: value\n",
			want: []Entry{
				parserHeading(1, ""),
				parserValue(2, []string{""}, "value"),
			},
		},
		{
			name:   "heading redeclaration and absolute reset",
			source: "A.\n  .B: first\nA.\n  .C: second\nX.\n  .D: third\n",
			want: []Entry{
				parserHeading(1, "A"),
				parserValue(2, []string{"A", "B"}, "first"),
				parserHeading(3, "A"),
				parserValue(4, []string{"A", "C"}, "second"),
				parserHeading(5, "X"),
				parserValue(6, []string{"X", "D"}, "third"),
			},
		},
		{
			name:   "canonically equivalent sequences remain distinct keys",
			source: "\u00e9: first\ne\u0301: second\n",
			want: []Entry{
				parserValue(1, []string{"\u00e9"}, "first"),
				parserValue(2, []string{"e\u0301"}, "second"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(test.source)))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("entries = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestParseEmpty(t *testing.T) {
	for _, source := range []string{"", "\ufeff", "\n\r\n", " \t\n\t# comment\n"} {
		got, err := Parse(strings.NewReader(parserDocument(source)))
		if err != nil || len(got) != 0 {
			t.Errorf("Parse(%q) = %#v, %v; want zero entries and success", source, got, err)
		}
	}
}

func TestParseTypeAnnotationPlacement(t *testing.T) {
	for _, source := range []string{
		"A.Code(string):001",
		"A.Code (string): 001",
		"A.Code     (string):     001",
		"A.Code     (string)     :     001",
	} {
		got, err := Parse(strings.NewReader(parserDocument(source)))
		if err != nil {
			t.Fatalf("Parse(%q): %v", source, err)
		}
		want := parserValue(1, []string{"A", "Code"}, "001")
		want.Type, want.HasType = "string", true
		if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
			t.Errorf("Parse(%q) = %#v, want %#v", source, got, want)
		}
	}
	got, err := Parse(strings.NewReader(parserDocument("A.B.\n  .(Custom_1):\n  .\"C.D\"(Other): value\nX: (string) 001\n")))
	if err != nil {
		t.Fatal(err)
	}
	wantSelf := parserValue(2, []string{"A", "B"}, "")
	wantSelf.Type, wantSelf.HasType = "Custom_1", true
	wantChild := parserValue(3, []string{"A", "B", "C.D"}, "value")
	wantChild.Type, wantChild.HasType = "Other", true
	want := []Entry{
		parserHeading(1, "A", "B"),
		wantSelf,
		wantChild,
		parserValue(4, []string{"X"}, "(string) 001"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entries = %#v, want %#v", got, want)
	}
	got, err = Parse(strings.NewReader(parserDocument("A.\n  .(int):")))
	if err != nil {
		t.Fatalf("typed self assignment: %v", err)
	}
	wantTypedSelf := parserValue(2, []string{"A"}, "")
	wantTypedSelf.Type, wantTypedSelf.HasType = "int", true
	if len(got) != 2 || !reflect.DeepEqual(got[1], wantTypedSelf) {
		t.Errorf("typed self assignment = %#v, want %#v", got, wantTypedSelf)
	}
}

func TestParseOpaqueTypeExpressions(t *testing.T) {
	for _, expression := range []string{
		"list(int)", "map(string,list(int))", "A|B", "名前", "[]",
		"T:U", "map(T:U,list(V))", "List[string]", "1", "_",
	} {
		t.Run(expression, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument("A(" + expression + "): first:second # literal")))
			if err != nil {
				t.Fatal(err)
			}
			want := parserValue(1, []string{"A"}, "first:second # literal")
			want.HasType, want.Type = true, expression
			if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
				t.Errorf("opaque type = %#v, want %#v", got, want)
			}
		})
	}
}

func TestParseQuotedKeyAllowsWhitespaceSyntaxAndControls(t *testing.T) {
	for _, name := range []string{
		" A B ", "A\tB", "A:B", "A(B)", "A#B", "A\x00B", "\x01\x7f",
		"\u00a0A\u3000", "\u0085", "#", "()", ":",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument("\"" + name + "\": value")))
			if err != nil {
				t.Fatal(err)
			}
			want := parserValue(1, []string{name}, "value")
			if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
				t.Errorf("quoted key = %#v, want %#v", got, want)
			}
		})
	}
}

func TestParseControlCharactersDoNotCollapseKeyBoundaries(t *testing.T) {
	// A NUL-delimited key representation would incorrectly merge these paths.
	source := "\"A\x00\".B: first\nA.\"\x00B\": second\n"
	got, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		parserValue(1, []string{"A\x00", "B"}, "first"),
		parserValue(2, []string{"A", "\x00B"}, "second"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("control-character key paths = %#v, want %#v", got, want)
	}
}

func TestParseEscapedKeyCharactersAreNotReinterpretedAsSyntax(t *testing.T) {
	const source = "\"\\u0041\": first\n" +
		"\"\\u{2e}\\u003a\\u0028\\u0029\\u0023\": second\n" +
		"\"line\\n\\r\\t\\b\\f\\/\": third\n" +
		"\"\\u{1F600}.A\": fourth\n" +
		"\"\\uD83D\\uDE00\".A: fifth\n"
	got, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		parserValue(1, []string{"A"}, "first"),
		parserValue(2, []string{".:()#"}, "second"),
		parserValue(3, []string{"line\n\r\t\b\f/"}, "third"),
		parserValue(4, []string{"😀.A"}, "fourth"),
		parserValue(5, []string{"😀", "A"}, "fifth"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("escaped key paths = %#v, want %#v", got, want)
	}
}

func TestParseUnicodeEscapeAliasesPreserveRepeatedKeys(t *testing.T) {
	for _, source := range []string{
		"\"\\u0041\": first\nA: second",
		"\"\\u{1F600}\": first\n\"\\uD83D\\uDE00\": second",
		"\"\\u{1F600}\": first\n😀: second",
		"\"\\u002e\": first\n\".\": second",
		"\"line\\nkey\": first\n\"line\\u000akey\": second",
	} {
		t.Run(source, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(source)))
			if err != nil || len(got) != 2 {
				t.Fatalf("repeated escaped key: entries=%#v, error=%v", got, err)
			}
			if !reflect.DeepEqual(got[0].Path, got[1].Path) ||
				got[0].Line != 3 || got[1].Line != 4 ||
				got[0].Raw != "first" || got[1].Raw != "second" {
				t.Errorf("escaped aliases lost identity or order: %#v", got)
			}
		})
	}
}

func TestParseEscapedNewlineDoesNotCreatePhysicalLines(t *testing.T) {
	got, err := Parse(strings.NewReader(parserDocument("\"line\\nkey\": value\nB: second")))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Line != 3 || got[0].Path[0] != "line\nkey" || got[1].Line != 4 {
		t.Errorf("escaped newline changed physical line counting: %#v", got)
	}
	got, err = Parse(strings.NewReader(parserDocument("\"line\nkey\": value")))
	parserRequireCode(t, err, ErrSyntax, 3)
	if got != nil {
		t.Error("physical newline inside key returned partial results")
	}
}

func parserBlockValue(line int, path []string, raw string, form ValueForm) Entry {
	return Entry{Line: line + 2, Kind: Value, Path: path, Raw: raw, Form: form}
}

func TestParseMultilineValues(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []Entry
	}{
		{
			name:   "triple margin and next line",
			source: "A: \"\"\"\n  first\n  second\n  \"\"\"\nB: next",
			want: []Entry{
				parserBlockValue(1, []string{"A"}, "first\nsecond", TripleQuoted),
				parserValue(5, []string{"B"}, "next"),
			},
		},
		{
			name:   "triple preserves extra indentation and trailing empty line",
			source: "A: \"\"\"\n    first \t\n  second\n\n  \"\"\" \t\n",
			want:   []Entry{parserBlockValue(1, []string{"A"}, "  first \t\nsecond\n", TripleQuoted)},
		},
		{
			name:   "triple zero margin and literal quotes",
			source: "A: \"\"\"\nsay \"hello\" and \\n\n\"\"\"",
			want:   []Entry{parserBlockValue(1, []string{"A"}, "say \"hello\" and \\n", TripleQuoted)},
		},
		{
			name:   "triple empty",
			source: "A: \"\"\"\n\"\"\"",
			want:   []Entry{parserBlockValue(1, []string{"A"}, "", TripleQuoted)},
		},
		{
			name:   "triple one empty body line",
			source: "A: \"\"\"\n\n\"\"\"",
			want:   []Entry{parserBlockValue(1, []string{"A"}, "", TripleQuoted)},
		},
		{
			name:   "triple mixed source line endings",
			source: "A: \"\"\"\r\n  first\r\n  second\n  \"\"\"\r\nB: next",
			want: []Entry{
				parserBlockValue(1, []string{"A"}, "first\r\nsecond", TripleQuoted),
				parserValue(5, []string{"B"}, "next"),
			},
		},
		{
			name:   "pipe literal whitespace and next line",
			source: "A:\n |first\n   | second \t\nB: next",
			want: []Entry{
				parserBlockValue(1, []string{"A"}, "first\n second \t", PipeBlock),
				parserValue(4, []string{"B"}, "next"),
			},
		},
		{
			name:   "pipe literal syntax and escapes",
			source: "A:\n |\"quotes\" \\n # comment : value\n ||: second\n",
			want:   []Entry{parserBlockValue(1, []string{"A"}, "\"quotes\" \\n # comment : value\n|: second", PipeBlock)},
		},
		{
			name:   "pipe explicit empty at EOF",
			source: "A:\n |",
			want:   []Entry{parserBlockValue(1, []string{"A"}, "", PipeBlock)},
		},
		{
			name:   "pipe trailing newline",
			source: "A:\n |first\n |\n",
			want:   []Entry{parserBlockValue(1, []string{"A"}, "first\n", PipeBlock)},
		},
		{
			name:   "pipe mixed source line endings",
			source: "A:\r\n |first\r\n |second\nB: next",
			want: []Entry{
				parserBlockValue(1, []string{"A"}, "first\r\nsecond", PipeBlock),
				parserValue(4, []string{"B"}, "next"),
			},
		},
		{
			name:   "pipe terminates at blank and replays line",
			source: "A:\n |first\n\nB: next",
			want: []Entry{
				parserBlockValue(1, []string{"A"}, "first", PipeBlock),
				parserValue(4, []string{"B"}, "next"),
			},
		},
		{
			name:   "comment immediately after header does not start pipe",
			source: "A:\n# comment\nB: next",
			want: []Entry{
				parserValue(1, []string{"A"}, ""),
				parserValue(3, []string{"B"}, "next"),
			},
		},
		{
			name:   "blank immediately after header does not start pipe",
			source: "A:\n \t\nB: next",
			want: []Entry{
				parserValue(1, []string{"A"}, ""),
				parserValue(3, []string{"B"}, "next"),
			},
		},
		{
			name:   "unindented pipe is a key",
			source: "A:\n|: ordinary key",
			want: []Entry{
				parserValue(1, []string{"A"}, ""),
				parserValue(2, []string{"|"}, "ordinary key"),
			},
		},
		{
			name:   "shallow pipe is an absolute key",
			source: "A.\n  .B:\n|: ordinary key",
			want: []Entry{
				parserHeading(1, "A"),
				parserValue(2, []string{"A", "B"}, ""),
				parserValue(3, []string{"|"}, "ordinary key"),
			},
		},
		{
			name:   "nonempty RHS does not start pipe",
			source: "A:|",
			want:   []Entry{parserValue(1, []string{"A"}, "|")},
		},
		{
			name:   "triple opener must be exact",
			source: "A: \"\"\"suffix",
			want:   []Entry{parserValue(1, []string{"A"}, "\"\"\"suffix")},
		},
		{
			name:   "pipe leaves heading scope unchanged",
			source: "A.\n  .B:\n   |first\n  .C: second",
			want: []Entry{
				parserHeading(1, "A"),
				parserBlockValue(2, []string{"A", "B"}, "first", PipeBlock),
				parserValue(4, []string{"A", "C"}, "second"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(test.source)))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("multiline entries = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestParseTypedMultilineSelfAndChild(t *testing.T) {
	got, err := Parse(strings.NewReader(parserDocument("A.\n  .(string): \"\"\"\n   first\n   \"\"\"\n  .B(string):\n   |second")))
	if err != nil {
		t.Fatal(err)
	}
	self := parserBlockValue(2, []string{"A"}, "first", TripleQuoted)
	self.Type, self.HasType = "string", true
	child := parserBlockValue(5, []string{"A", "B"}, "second", PipeBlock)
	child.Type, child.HasType = "string", true
	want := []Entry{parserHeading(1, "A"), self, child}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("typed multiline values = %#v, want %#v", got, want)
	}
}

func TestParseRejectsInvalidMultilineStructure(t *testing.T) {
	for _, source := range []string{
		"A: \"\"\"",
		"A: \"\"\"\nbody",
		"A: \"\"\"\n body\n  \"\"\"",
		"A: \"\"\"\n \n  \"\"\"",
		"A: \"\"\"\n\tbody\n  \"\"\"",
		"A: \"\"\"\nbody\n\"\"\"suffix",
		"A: \"\"\"\n\xff\n\"\"\"",
		"A:\n\n |not immediate",
		"A:\n# comment\n |not immediate",
		"A.\n  .B:\n   |value\n    .C: values do not create scope",
	} {
		t.Run(source, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(source)))
			parserRequireCode(t, err, ErrSyntax, 0)
			if got != nil {
				t.Error("invalid multiline structure returned partial results")
			}
		})
	}
}

func TestParseMultilineRepeatedValuesAreNotifiedInOrder(t *testing.T) {
	for _, source := range []string{
		"A: \"\"\"\nfirst\n\"\"\"\nA: second",
		"A:\n |first\nA: second",
	} {
		var got []Entry
		err := ParseEach(strings.NewReader(parserDocument(source)), func(entry Entry) error {
			got = append(got, entry)
			return nil
		})
		if err != nil || len(got) != 2 {
			t.Fatalf("repeated multiline values: entries=%#v, error=%v", got, err)
		}
		if got[0].Form == "" || got[1].Form != "" ||
			got[0].Raw != "first" || got[1].Raw != "second" ||
			!reflect.DeepEqual(got[0].Path, got[1].Path) {
			t.Errorf("repeated multiline values lost form, identity or order: %#v", got)
		}
	}
}

func TestParseRepeatedValuesPreserveAllOccurrences(t *testing.T) {
	const source = "A: red\nB: blue\nA: green\nA: red\nA:\nA: \"\"\nA:\n"
	want := []Entry{
		parserValue(1, []string{"A"}, "red"),
		parserValue(2, []string{"B"}, "blue"),
		parserValue(3, []string{"A"}, "green"),
		parserValue(4, []string{"A"}, "red"),
		parserValue(5, []string{"A"}, ""),
		parserValue(6, []string{"A"}, `""`),
		parserValue(7, []string{"A"}, ""),
	}
	got, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("collected occurrences=%#v, error=%v; want %#v", got, err, want)
	}
	var visited []Entry
	err = ParseEach(strings.NewReader(parserDocument(source)), func(entry Entry) error {
		visited = append(visited, entry)
		return nil
	})
	if err != nil || !reflect.DeepEqual(visited, want) {
		t.Errorf("visited occurrences=%#v, error=%v; want %#v", visited, err, want)
	}
}

func TestParseRepeatedValuesRetainTheirFormsAndTypes(t *testing.T) {
	const source = "A(T): \"\"\"\none\n\"\"\"\nA(U):\n |two\nA: \"\"\nA:\nA: \"\"\"\n\"\"\"\nA:\n |"
	triple := parserBlockValue(1, []string{"A"}, "one", TripleQuoted)
	triple.Type, triple.HasType = "T", true
	pipe := parserBlockValue(4, []string{"A"}, "two", PipeBlock)
	pipe.Type, pipe.HasType = "U", true
	want := []Entry{
		triple,
		pipe,
		parserValue(6, []string{"A"}, `""`),
		parserValue(7, []string{"A"}, ""),
		parserBlockValue(8, []string{"A"}, "", TripleQuoted),
		parserBlockValue(10, []string{"A"}, "", PipeBlock),
	}
	got, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("repeated forms/types=%#v, error=%v; want %#v", got, err, want)
	}
}

func TestParseRepeatedValueCallbackCanStopBeforeLaterErrors(t *testing.T) {
	for _, source := range []string{
		"A: red\nA: green\nA: \"\"\"\nunclosed",
		"A: red\nA:\n |green\nA: \xff",
	} {
		t.Run(source, func(t *testing.T) {
			stop := errors.New("stop after the second occurrence")
			var got []Entry
			err := ParseEach(strings.NewReader(parserDocument(source)), func(entry Entry) error {
				got = append(got, entry)
				if len(got) == 2 {
					return stop
				}
				return nil
			})
			if err != stop || len(got) != 2 || got[0].Raw != "red" || got[1].Raw != "green" {
				t.Errorf("repeated callback stop: entries=%#v, error=%v", got, err)
			}
		})
	}
}

func TestParseLookaheadDoesNotPreemptCurrentCallback(t *testing.T) {
	for _, source := range []string{"A:\nB: \xff", "A:\n |body\nB: \xff"} {
		t.Run(source, func(t *testing.T) {
			sentinel := errors.New("callback wins over peeked invalid UTF-8")
			count := 0
			err := ParseEach(strings.NewReader(parserDocument(source)), func(Entry) error {
				count++
				return sentinel
			})
			if err != sentinel || count != 1 {
				t.Errorf("peek preempted callback: error=%v count=%d", err, count)
			}
			count = 0
			err = ParseEach(strings.NewReader(parserDocument(source)), func(Entry) error {
				count++
				return nil
			})
			parserRequireCode(t, err, ErrSyntax, 0)
			if count != 1 {
				t.Errorf("invalid next line changed current callback count: %d", count)
			}
		})
	}
	t.Run("reader error is deferred until unset callback", func(t *testing.T) {
		sentinel := errors.New("callback wins over peeked input error")
		reader := &parserFailReader{data: []byte("A:\n"), err: errors.New("private read failure")}
		count := 0
		err := ParseEach(io.MultiReader(strings.NewReader(parserPreamble), reader), func(entry Entry) error {
			count++
			if !entry.Unset {
				t.Errorf("expected current entry to remain unset: %#v", entry)
			}
			return sentinel
		})
		if err != sentinel || count != 1 {
			t.Errorf("input-error peek preempted callback: error=%v count=%d", err, count)
		}
		reader = &parserFailReader{data: []byte("A:\n"), err: errors.New("private read failure")}
		count = 0
		err = ParseEach(io.MultiReader(strings.NewReader(parserPreamble), reader), func(Entry) error { count++; return nil })
		parserRequireCode(t, err, ErrInput, 0)
		if count != 1 {
			t.Errorf("input error discarded current unset callback: %d", count)
		}
	})
}

func TestParsePipeLookaheadReplaysSameIndentLine(t *testing.T) {
	var got []Entry
	err := ParseEach(strings.NewReader(parserDocument("A.\n  .B:\n  |:x")), func(entry Entry) error {
		got = append(got, entry)
		return nil
	})
	parserRequireCode(t, err, ErrSyntax, 5)
	// The pipe is not deeper than the header. It must be replayed as an ordinary
	// line, whose indentation is invalid without the relative leading dot.
	want := []Entry{
		parserHeading(1, "A"),
		parserValue(2, []string{"A", "B"}, ""),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("same-indent lookahead callbacks = %#v, want %#v", got, want)
	}
}

func TestParsePreservesRawBeforeStringDecoding(t *testing.T) {
	for _, raw := range []string{
		"\"  value  \"", "\"\"", "\"unterminated", "\"\\q\"", "\"value\"tail",
		"some\"value\"", "\"first\" \"second\"", "\\\"hello\\\"",
		"some\"unterminated", "prefix\"\"\"suffix", "prefix\"\"\"",
		"{\"name\":\"Alice\",\"text\":\"line\\nnext\"}",
		"C:\\temp\\new", "\\q", "value\\", "value: # literal", "\"\\",
		"internal\tspace", "\x00value\x7f",
		"\"\\uD800\"", "\"\\u{110000}\"", "\\u{1F600}", "\\n",
	} {
		got, err := Parse(strings.NewReader(parserDocument("A: \t" + raw + " \t")))
		if err != nil {
			t.Fatalf("raw %q: %v", raw, err)
		}
		if len(got) != 1 || got[0].Raw != raw || got[0].Unset {
			t.Errorf("raw %q: got %#v", raw, got)
		}
	}
}

func TestParseRightHandQuotesDoNotAffectQuotedKeyOrType(t *testing.T) {
	const raw = "{\"A:B\":\"value\", \"tail\":\"\\q"
	const expression = "map(T:U,list(V))"
	for _, typed := range []bool{false, true} {
		header := "\"A:B\""
		want := parserValue(1, []string{"A:B"}, raw)
		if typed {
			header += "(" + expression + ")"
			want.Type, want.HasType = expression, true
		}
		got, err := Parse(strings.NewReader(parserDocument(header + ": \t" + raw + " \t")))
		if err != nil || !reflect.DeepEqual(got, []Entry{want}) {
			t.Errorf("RHS quote leaked into header parsing: entries=%#v, error=%v; want %#v", got, err, want)
		}
	}
}

func TestParseRejectsMalformedSyntax(t *testing.T) {
	tests := []struct {
		name   string
		source string
		line   int
	}{
		{"missing colon", "A", 1},
		{"missing key", ": value", 1},
		{"missing middle element", "A..B: value", 1},
		{"missing final element", "A.: value", 1},
		{"missing heading element", "A..", 1},
		{"relative at column zero", ".A: value", 1},
		{"relative self without parent", "  .: value", 1},
		{"indent without relative dot", "A.\n  B: value", 2},
		{"relative without parent", "  .A: value", 1},
		{"relative heading without child", "A.\n  .", 2},
		{"relative heading with omitted element", "A.\n  ..", 2},
		{"indent jump", "A.\n    .B: value", 2},
		{"tab indent", "A.\n\t.B: value", 2},
		{"mixed tab indent", "A.\n \t.B: value", 2},
		{"relative value is not parent", "A.\n  .B: value\n    .C: invalid", 3},
		{"closed heading does not revive", "A.\n  .B.\n    .C: value\n  .D: value\n    .E: invalid", 5},
		{"absolute value closes scope", "A.\n  .B: value\nX.Y: value\n  .C: invalid", 4},
		{"bare backslash key", "A\\.B: value", 1},
		{"bare terminal backslash", "A\\: value", 1},
		{"bare escape heading", "A\\.B.", 1},
		{"partial quoted key", "A\"B\": value", 1},
		{"quoted key suffix", "\"A\"B: value", 1},
		{"unclosed key", "\"A: value", 1},
		{"unknown quoted key escape", "\"A\\q\": value", 1},
		{"isolated surrogate key escape", "\"\\uD800\": value", 1},
		{"reversed surrogate key escape", "\"\\uDE00\\uD83D\": value", 1},
		{"out of range key escape", "\"\\u{110000}\": value", 1},
		{"unsupported uppercase unicode escape", "\"\\U0001F600\": value", 1},
		{"incomplete quoted key escape", "\"A\\", 1},
		{"comment marker in key", "A#B: value", 1},
		{"space in bare key", "A B: value", 1},
		{"unicode whitespace in bare key", "A\u3000B: value", 1},
		{"control in bare key", "A\x00B: value", 1},
		{"physical LF in quoted key", "\"A\nB\": value", 1},
		{"physical CRLF in quoted key", "\"A\r\nB\": value", 1},
		{"bare CR in quoted key", "\"A\rB\": value", 1},
		{"bad utf8 in quoted key", "\"A\xffB\": value", 1},
		{"space inside type", "A( string ): value", 1},
		{"tab inside type", "A(list(\tint)): value", 1},
		{"NBSP inside type", "A(T\u00a0U): value", 1},
		{"full-width space inside type", "A(T\u3000U): value", 1},
		{"nested type whitespace", "A(map(string, list(int))): value", 1},
		{"unclosed nested type", "A(map(string,list(int)): value", 1},
		{"extra type closing parenthesis", "A(T)): value", 1},
		{"typed heading", "A(T).", 1},
		{"relative typed heading", "A.\n  .B(list(int)).", 2},
		{"bare CR", "A: x\rB: y", 1},
		{"bare CR at EOF", "A: x\r", 1},
		{"bare CR in comment", "# comment\r", 1},
		{"bad utf8 in value", "A: \xff", 1},
		{"bad utf8 in comment", "# \xff", 1},
		{"bad utf8 after valid line", "A: x\nB: \xff", 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(test.source)))
			parserRequireCode(t, err, ErrSyntax, test.line+2)
			if got != nil {
				t.Errorf("failure returned partial entries: %#v", got)
			}
		})
	}
}

func TestParseReportsUndecidedGrammarAsUnsupported(t *testing.T) {
	// These restrictions describe this sample's supported subset, not additional
	// decisions about the format's still-undecided grammar.
	for _, source := range []string{
		"A(): x", "A(T\\U): x", "A(\"T\"): x",
		"A\t(T): x", "A(T)\t: x",
	} {
		t.Run(source, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(source)))
			parserRequireCode(t, err, ErrUnsupported, 3)
			if got != nil {
				t.Errorf("failure returned entries: %#v", got)
			}
		})
	}
}

func TestParseRepeatedKeyIdentity(t *testing.T) {
	for _, source := range []string{
		"A.B: first\nA.\n  .B: second",
		"A: first\nA.\n  .: second",
		"A.B: first\nA.\"B\": second",
		"\"A.B\": first\n\"A\\.B\": second",
		"A.\"B.C\": first\nA.\"B\\.C\": second",
		"\"\": first\n\"\": second",
		"A: first\nA(T): second",
		"A(T): first\nA(U): second",
		"\"A\x00B\": first\n\"A\x00B\": second",
	} {
		t.Run(source, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(source)))
			if err != nil {
				t.Fatal(err)
			}
			var values []Entry
			for _, entry := range got {
				if entry.Kind == Value {
					values = append(values, entry)
				}
			}
			if len(values) != 2 {
				t.Fatalf("repeated key lost an occurrence: %#v", got)
			}
			if !reflect.DeepEqual(values[0].Path, values[1].Path) ||
				values[0].Raw != "first" || values[1].Raw != "second" ||
				values[0].Line != 3 || values[1].Line != strings.Count(source, "\n")+3 {
				t.Errorf("repeated key lost identity or source order: %#v", got)
			}
		})
	}
}

func TestParseEachCallbackContracts(t *testing.T) {
	t.Run("source order includes headings", func(t *testing.T) {
		source := "\n# comment\nA.\n  .B: first\n  .C.\n    .: second\n"
		var got []Entry
		err := ParseEach(strings.NewReader(parserDocument(source)), func(entry Entry) error {
			got = append(got, entry)
			return nil
		})
		want := []Entry{
			parserHeading(3, "A"),
			parserValue(4, []string{"A", "B"}, "first"),
			parserHeading(5, "A", "C"),
			parserValue(6, []string{"A", "C"}, "second"),
		}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("ParseEach = %#v, %v; want %#v", got, err, want)
		}
	})
	t.Run("callback error identity and immediate stop", func(t *testing.T) {
		sentinel := errors.New("callback sentinel")
		count := 0
		err := ParseEach(strings.NewReader(parserDocument("A: first\nthis is invalid\n")), func(Entry) error {
			count++
			return sentinel
		})
		if err != sentinel || count != 1 {
			t.Errorf("error = %v, count = %d; want identical sentinel and one callback", err, count)
		}
	})
	t.Run("repeated values are delivered in source order", func(t *testing.T) {
		var got []Entry
		err := ParseEach(strings.NewReader(parserDocument("A: first\nA: second\nB: third\n")), func(entry Entry) error {
			got = append(got, entry)
			return nil
		})
		want := []Entry{
			parserValue(1, []string{"A"}, "first"),
			parserValue(2, []string{"A"}, "second"),
			parserValue(3, []string{"B"}, "third"),
		}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("callbacks=%#v, error=%v; want %#v", got, err, want)
		}
	})
	t.Run("callback cannot mutate scope or later repeated paths", func(t *testing.T) {
		var paths [][]string
		err := ParseEach(strings.NewReader(parserDocument("A.\n  .B: first\nA.B: second\n")), func(entry Entry) error {
			paths = append(paths, append([]string(nil), entry.Path...))
			entry.Path[0] = "changed"
			return nil
		})
		want := [][]string{{"A"}, {"A", "B"}, {"A", "B"}}
		if err != nil || !reflect.DeepEqual(paths, want) {
			t.Errorf("callback paths=%#v, error=%v; want %#v", paths, err, want)
		}
	})
}

type parserFailReader struct {
	data []byte
	err  error
}

func (reader *parserFailReader) Read(buffer []byte) (int, error) {
	if len(reader.data) > 0 {
		n := copy(buffer, reader.data)
		reader.data = reader.data[n:]
		return n, nil
	}
	return 0, reader.err
}

func TestParseInputFailureAndDiagnosticPrivacy(t *testing.T) {
	t.Run("missing reader", func(t *testing.T) {
		got, err := Parse(nil)
		parserRequireCode(t, err, ErrInput, 0)
		if got != nil {
			t.Errorf("nil input returned entries: %#v", got)
		}
	})
	t.Run("missing callback", func(t *testing.T) {
		err := ParseEach(strings.NewReader(parserDocument("A: x")), nil)
		parserRequireCode(t, err, ErrInput, 0)
	})
	t.Run("reader error is sanitized and results discarded", func(t *testing.T) {
		secret := "private input error containing token"
		reader := &parserFailReader{data: []byte("A: value\n"), err: errors.New(secret)}
		got, err := Parse(io.MultiReader(strings.NewReader(parserPreamble), reader))
		parserRequireCode(t, err, ErrInput, 0)
		if got != nil {
			t.Errorf("read failure returned partial entries: %#v", got)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("diagnostic leaked underlying reader error: %v", err)
		}
	})
	t.Run("syntax error does not contain rhs", func(t *testing.T) {
		secret := "private RHS token"
		_, err := Parse(strings.NewReader(parserDocument("A..B: " + secret)))
		parserRequireCode(t, err, ErrSyntax, 3)
		if strings.Contains(err.Error(), secret) {
			t.Errorf("diagnostic leaked rhs: %v", err)
		}
	})
	t.Run("reader data with EOF is consumed", func(t *testing.T) {
		got, err := Parse(io.MultiReader(strings.NewReader(parserPreamble), &parserDataEOFReader{}))
		if err != nil || len(got) != 1 || got[0].Raw != "value" {
			t.Errorf("data+EOF parse = %#v, %v", got, err)
		}
	})
}

type parserDataEOFReader struct{ done bool }

func (reader *parserDataEOFReader) Read(buffer []byte) (int, error) {
	if reader.done {
		return 0, io.EOF
	}
	reader.done = true
	return copy(buffer, "A: value"), io.EOF
}

func TestParseLongPhysicalLines(t *testing.T) {
	// Exercise several large physical-line lengths. These are test samples, not
	// maximum lengths imposed by the format or parser.
	const sampleBytes = 160 * 1024
	tests := []struct {
		name   string
		source string
	}{
		{"value", "A:" + strings.Repeat("x", sampleBytes)},
		{"comment", "#" + strings.Repeat("x", sampleBytes)},
		{"blank", strings.Repeat(" ", sampleBytes)},
		{"trailing spaces", "A:x" + strings.Repeat(" ", sampleBytes)},
		{"type annotation", "A(map(T:U,list(V))):" + strings.Repeat("x", sampleBytes)},
		{"multibyte utf8", "A:" + strings.Repeat("あ", sampleBytes/3)},
		{"quoted key", "\"" + strings.Repeat("K", sampleBytes) + "\":value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, ending := range []string{"", "\n", "\r\n"} {
				for _, prefix := range []string{"", "\ufeff"} {
					got, err := Parse(strings.NewReader(parserDocument(prefix + test.source + ending)))
					if err != nil {
						t.Fatalf("long line, prefix=%q ending=%q: %v", prefix, ending, err)
					}
					if test.name == "value" && (len(got) != 1 || len(got[0].Raw) != sampleBytes) {
						t.Error("long raw value was not preserved")
					}
					if test.name == "quoted key" && (len(got) != 1 || len(got[0].Path[0]) != sampleBytes) {
						t.Error("long quoted key was not preserved")
					}
				}
			}
		})
	}
	source := "\ufeffA.\r\n  .B:" + strings.Repeat("x", sampleBytes) + "\r\n"
	if _, err := Parse(strings.NewReader(parserDocument(source))); err != nil {
		t.Fatalf("long relative value line: %v", err)
	}
}

func TestParseLongExpandedPaths(t *testing.T) {
	parent := strings.Repeat("A", 90000)
	child := strings.Repeat("B", 100000)
	for _, heading := range []bool{false, true} {
		suffix := ": value"
		if heading {
			suffix = "."
		}
		source := "\ufeff" + parent + ".\r\n  .\"" + child + "\"" + suffix + "\r\n"
		got, err := Parse(strings.NewReader(parserDocument(source)))
		if err != nil {
			t.Fatalf("long expanded path, heading=%v: %v", heading, err)
		}
		path := got[len(got)-1].Path
		if len(path) != 2 || path[0] != parent || path[1] != child {
			t.Errorf("long expanded path did not preserve the two segments")
		}
	}
}

func TestParseHasNoIndependentDepthLimit(t *testing.T) {
	// Deep compound paths are parsed without a format-defined depth cap.
	source := strings.Repeat("A.", 4095) + "A: value"
	got, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Path) != 4096 {
		t.Errorf("expected a 4096-element path")
	}
}

func TestParseHasNoWholeFileByteLimit(t *testing.T) {
	// File size is independent of the sizes of its individual physical lines.
	source := "A:" + strings.Repeat("x", 40000) + "\nB:" + strings.Repeat("y", 40000)
	got, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil || len(got) != 2 {
		t.Errorf("large file returned %d entries, %v; want two entries and success", len(got), err)
	}
}

func TestParseStateIsPerCall(t *testing.T) {
	for range 2 {
		if _, err := Parse(strings.NewReader(parserDocument("A.\n  .B: value"))); err != nil {
			t.Fatalf("repeated parse failed: %v", err)
		}
	}
	_, err := Parse(strings.NewReader(parserDocument("  .C: value")))
	parserRequireCode(t, err, ErrSyntax, 3)
}

func FuzzSampleParserNoPanic(f *testing.F) {
	for _, source := range []string{
		"", "A:001", "A.\n  .", "A.\n  ..", "A.\n  .(int):",
		"\"A\\.B\": \"say \\\"hello\\\"\"", "\"\":", "\ufeffA: value\r\n",
		"A: \"\\q\"", "A: value\\", "A..B: value", "A:\xff",
		"\"A\x00 B:#()\": value", "A(map(T:U,list(V))): first:second",
		"\"\\u{1F600}\": \"\\uD83D\\uDE00\\n\"",
		"\"\\uD800\": value", "\"line\\nkey\": \"\\u{0}\"",
		"A: \"\"\"\n  first\\nsecond\n  \"\"\"\nB: value",
		"A:\n |literal \\q\n |\nB: value",
		"A:red\nA:green\nA:red\nA:\nA:\"\"",
		"A: \"\"\"\n body\n  \"\"\"", "A:\n |body\nB: \xff",
	} {
		f.Add(parserDocument(source))
	}
	for _, source := range []string{
		"", "A:001", "@dotReg", "@dotReg 0.1", "@dotReg 0.1\n",
		"@dotReg 2\n\n", "@dotReg 0.1\n@dotReg 0.1\n\n", "@unknown value\n\n",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		// This harness-only bound keeps fuzzing focused on syntax. It is not a
		// limit in the format or parser.
		const fuzzInputBytes = 128 * 1024
		if len(source) > fuzzInputBytes {
			t.Skip()
		}
		entries, err := Parse(strings.NewReader(source))
		if err != nil && entries != nil {
			t.Fatal("failed Parse returned partial results")
		}
		for _, entry := range entries {
			if entry.Kind == Value {
				_, _ = DecodeString(entry)
			}
		}
		// Exercise the string layer even when the input was not valid syntax.
		raw := strings.Trim(source, " \t")
		_, _ = DecodeString(Entry{Kind: Value, Raw: raw, Unset: raw == ""})
	})
}
