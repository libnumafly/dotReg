package dotreg

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestUnicodeSequencesArePreservedThroughLookup(t *testing.T) {
	const composed = "\u304c"
	const decomposed = "\u304b\u3099"
	entries, err := Parse(strings.NewReader(parserDocument(
		composed + ": " + decomposed + "\n" +
			decomposed + ": " + composed + "\n",
	)))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ key, want string }{
		{composed, decomposed},
		{decomposed, composed},
	} {
		got, err := cfg.Get([]string{test.key}, "string")
		if err != nil || got != test.want {
			t.Errorf("Get(%q) = %#v, %v; want %q", test.key, got, err, test.want)
		}
	}
	// A lookup must not silently rewrite a caller's key to an existing one.
	one, err := Build(entries[:1], DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	_, err = one.Get([]string{decomposed}, "string")
	assertValueCode(t, err, ErrMissing)
}

func TestDecodedUnicodeKeysAndValuesReachProfileWithoutNormalization(t *testing.T) {
	const source = "\"\\u304c\": \"\\u{1F600}\\n\\uD83D\\uDE00\"\n" +
		"\"\\u304b\\u3099\": decomposed\n" +
		"\"A\\u002eB\": dot in one element\n" +
		"\"line\\nkey\": \"\\b\\f\\r\\t\\/\\u{0}\"\n" +
		"\"\\u{1F600}\": \\uD83D\\uDE00\n"
	entries, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path []string
		want string
	}{
		{[]string{"が"}, "😀\n😀"},
		{[]string{"か\u3099"}, "decomposed"},
		{[]string{"A.B"}, "dot in one element"},
		{[]string{"line\nkey"}, "\b\f\r\t/\x00"},
		{[]string{"😀"}, "\\uD83D\\uDE00"},
	} {
		got, err := cfg.Get(test.path, "string")
		if err != nil || got != test.want {
			t.Errorf("Get(%q) = %#v, %v; want %q", test.path, got, err, test.want)
		}
	}
	_, err = cfg.Get([]string{"A", "B"}, "string")
	assertValueCode(t, err, ErrMissing)
	// The escaped spelling must not cause composed/decomposed names to merge.
	composedOnly, err := Build(entries[:1], DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	_, err = composedOnly.Get([]string{"か\u3099"}, "string")
	assertValueCode(t, err, ErrMissing)
}

func TestMultilineFormsReachProfileWithTheirOwnDecodingRules(t *testing.T) {
	const source = "A.\n" +
		"  .(string): \"\"\"\n" +
		"  say \"hello\"\\n\\u{1F600}\n" +
		"  \"\"\"\n" +
		"  .Literal:\n" +
		"   |say \"hello\"\\n\\u{1F600}\n" +
		"EmptyTriple: \"\"\"\n\"\"\"\n" +
		"EmptyPipe:\n |\n" +
		"Unset:\n"
	entries, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path  []string
		want  string
		unset bool
	}{
		{[]string{"A"}, "say \"hello\"\n😀", false},
		{[]string{"A", "Literal"}, "say \"hello\"\\n\\u{1F600}", false},
		{[]string{"EmptyTriple"}, "", false},
		{[]string{"EmptyPipe"}, "", false},
		{[]string{"Unset"}, "", true},
	} {
		value, err := cfg.Get(test.path, "string")
		if err != nil || value != test.want {
			t.Errorf("Get(%q) = %#v, %v; want %q", test.path, value, err, test.want)
		}
		unset, err := cfg.IsUnset(test.path)
		if err != nil || unset != test.unset {
			t.Errorf("IsUnset(%q) = %v, %v; want %v", test.path, unset, err, test.unset)
		}
	}
}

func TestMultilineDecodingPrecedesDeclaredTypeConversion(t *testing.T) {
	for _, source := range []string{
		"A(int): \"\"\"\n00\\u0031\n\"\"\"",
		"A(int):\n |001",
	} {
		entries, err := Parse(strings.NewReader(parserDocument(source)))
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := Build(entries, DemoProfile())
		if err != nil {
			t.Fatal(err)
		}
		value, err := cfg.Get([]string{"A"}, "int")
		if err != nil || value != int64(1) {
			t.Errorf("multiline declared int = %#v, %v", value, err)
		}
	}
	entries, err := Parse(strings.NewReader(parserDocument("Good: first\nA(int):\n |00\\u0031")))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	assertValueCode(t, err, ErrTypeConversion)
	if cfg != nil {
		t.Error("literal pipe conversion failure returned a partial config")
	}
}

func TestEscapedTripleClosingMarkerIsLiteralThroughLookup(t *testing.T) {
	const source = "A.\n  .B: \"\"\"\n  \\\"\"\"\n  \"\"\"\n  .C: next"
	entries, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[1].Form != TripleQuoted || entries[1].Raw != "\\\"\"\"" || entries[1].Line != 4 || entries[2].Line != 7 {
		t.Fatalf("escaped closing marker was treated as structure: %#v", entries)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path []string
		want string
	}{
		{[]string{"A", "B"}, "\"\"\""},
		{[]string{"A", "C"}, "next"},
	} {
		got, err := cfg.Get(test.path, "string")
		if err != nil || got != test.want {
			t.Errorf("Get(%q) = %#v, %v; want %q", test.path, got, err, test.want)
		}
	}
}

func TestParsedOpaqueTypeIsInterpretedOnlyByProfile(t *testing.T) {
	const typeName = "map(string,list(int))"
	entries, err := Parse(strings.NewReader(parserDocument(
		"\"A:B (C)#\t\"(" + typeName + "):\" 001 \"\n",
	)))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	profile := Profile{StringType: "string", Types: map[string]Converter{
		typeName: func(text Text) (any, error) {
			calls++
			if text.Unset || text.Value != " 001 " {
				t.Fatalf("converter input = %#v", text)
			}
			return "custom result", nil
		},
	}}
	cfg, err := Build(entries, profile)
	if err != nil {
		t.Fatal(err)
	}
	path := []string{"A:B (C)#\t"}
	got, err := cfg.Get(path, typeName)
	if err != nil || got != "custom result" || calls != 1 {
		t.Fatalf("Get() = %#v, %v; conversions = %d", got, err, calls)
	}
	_, err = cfg.Get(path, "string")
	assertValueCode(t, err, ErrTypeMismatch)
	// The example does not turn compound expressions into built-in types.
	_, err = Build(entries, DemoProfile())
	assertValueCode(t, err, ErrTypeConversion)
}

func TestQuotedControlCharactersRetainKeyBoundariesThroughLookup(t *testing.T) {
	entries, err := Parse(strings.NewReader(parserDocument(
		"\"A\x00B\": one\n\"A\".\"B\": two\n\"A\x00\".B: three\n",
	)))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path []string
		want string
	}{
		{[]string{"A\x00B"}, "one"},
		{[]string{"A", "B"}, "two"},
		{[]string{"A\x00", "B"}, "three"},
	} {
		got, err := cfg.Get(test.path, "string")
		if err != nil || got != test.want {
			t.Errorf("Get(%q) = %#v, %v; want %q", test.path, got, err, test.want)
		}
	}
}

func TestUntypedValuesConvertOnDemand(t *testing.T) {
	cfg, err := Build([]Entry{
		{Kind: Value, Path: []string{"Code"}, Raw: "001"},
		{Kind: Value, Path: []string{"Enabled"}, Raw: "true"},
		{Kind: Value, Path: []string{"Text"}, Raw: `"001"`},
	}, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		key  string
		want string
		got  any
	}{
		{"Code", "string", "001"},
		{"Code", "int", int64(1)},
		{"Code", "string", "001"}, // A conversion does not alter the stored string.
		{"Enabled", "bool", true},
		{"Enabled", "string", "true"},
		{"Text", "int", int64(1)}, // Quotes do not declare a string type.
	}
	for _, check := range checks {
		got, err := cfg.Get([]string{check.key}, check.want)
		if err != nil || got != check.got {
			t.Fatalf("Get(%q, %q) = %#v, %v; want %#v", check.key, check.want, got, err, check.got)
		}
	}
	for _, want := range []string{"bool", "unknown", ""} {
		_, err := cfg.Get([]string{"Code"}, want)
		assertValueCode(t, err, ErrTypeMismatch)
	}
}

func TestDeclaredTypeConvertsOnlyOnceAndRequiresExactMatch(t *testing.T) {
	calls := 0
	profile := Profile{StringType: "text", Types: map[string]Converter{
		"number": func(text Text) (any, error) {
			calls++
			return "converted:" + text.Value, nil
		},
	}}
	cfg, err := Build([]Entry{{Line: 3, Kind: Value, Path: []string{"A"}, Raw: "001", Type: "number", HasType: true}}, profile)
	if err != nil || calls != 1 {
		t.Fatalf("Build() calls = %d, error = %v", calls, err)
	}
	for range 2 {
		got, err := cfg.Get([]string{"A"}, "number")
		if err != nil || got != "converted:001" {
			t.Fatalf("Get() = %v, %v", got, err)
		}
	}
	for _, want := range []string{"text", "Number", "unknown"} {
		_, err := cfg.Get([]string{"A"}, want)
		assertValueCode(t, err, ErrTypeMismatch)
	}
	if calls != 1 {
		t.Fatalf("declared value reconverted: calls = %d", calls)
	}
}

func TestUnsetAndQuotedEmptyRemainDistinct(t *testing.T) {
	cfg, err := Build([]Entry{
		{Kind: Value, Path: []string{"Unset"}, Unset: true},
		{Kind: Value, Path: []string{"Empty"}, Raw: `""`},
		{Kind: Value, Path: []string{"Declared"}, Unset: true, Type: "string", HasType: true},
	}, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"Unset", "Empty", "Declared"} {
		value, err := cfg.Get([]string{key}, "string")
		if err != nil || value != "" {
			t.Fatalf("Get(%q) = %#v, %v", key, value, err)
		}
		unset, err := cfg.IsUnset([]string{key})
		if err != nil || unset != (key != "Empty") {
			t.Fatalf("IsUnset(%q) = %v, %v", key, unset, err)
		}
		_, err = cfg.Get([]string{key}, "int")
		assertValueCode(t, err, ErrTypeMismatch)
	}
}

func TestMismatchPrecedesUnsetStringInterpretation(t *testing.T) {
	profile := Profile{StringType: "string", Types: map[string]Converter{
		"optional": func(text Text) (any, error) { return text.Unset, nil },
	}}
	cfg, err := Build([]Entry{{Kind: Value, Path: []string{"A"}, Unset: true, Type: "optional", HasType: true}}, profile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cfg.Get([]string{"A"}, "string")
	assertValueCode(t, err, ErrTypeMismatch)
	got, err := cfg.Get([]string{"A"}, "optional")
	if err != nil || got != true {
		t.Fatalf("Get(optional) = %v, %v", got, err)
	}
}

func TestBuildFailureReturnsNoPartialConfig(t *testing.T) {
	for _, entry := range []Entry{
		{Kind: Value, Path: []string{"Bad"}, Raw: "not-a-number", Type: "int", HasType: true},
		{Kind: Value, Path: []string{"Bad"}, Raw: "1", Type: "unknown", HasType: true},
		{Kind: Value, Path: []string{"Bad"}, Unset: true, Type: "int", HasType: true},
	} {
		cfg, err := Build([]Entry{{Kind: Value, Path: []string{"Good"}, Raw: "ok"}, entry}, DemoProfile())
		if cfg != nil {
			t.Fatal("Build returned a partial config")
		}
		assertValueCode(t, err, ErrTypeConversion)
	}
}

func TestConfigPreservesElementBoundariesAndEmptyNames(t *testing.T) {
	cfg, err := Build([]Entry{
		{Kind: Heading, Path: []string{"Group"}},
		{Kind: Value, Path: []string{"A.B"}, Raw: "one"},
		{Kind: Value, Path: []string{"A", "B"}, Raw: "two"},
		{Kind: Value, Path: []string{""}, Raw: "empty name"},
		{Kind: Value, Path: []string{"", ""}, Raw: "two empty names"},
	}, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		path []string
		want string
	}{
		{[]string{"A.B"}, "one"},
		{[]string{"A", "B"}, "two"},
		{[]string{""}, "empty name"},
		{[]string{"", ""}, "two empty names"},
	} {
		got, err := cfg.Get(check.path, "string")
		if err != nil || got != check.want {
			t.Fatalf("Get(%q) = %v, %v", check.path, got, err)
		}
	}
	for _, path := range [][]string{nil, {}, {"Group"}, {"missing"}} {
		_, err := cfg.Get(path, "string")
		assertValueCode(t, err, ErrMissing)
		_, err = cfg.IsUnset(path)
		assertValueCode(t, err, ErrMissing)
	}
	cfg, err = Build([]Entry{
		{Line: 1, Kind: Value, Path: []string{"A.B"}, Raw: "first"},
		{Line: 4, Kind: Value, Path: []string{"A.B"}, Raw: "second"},
	}, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	values, err := cfg.GetAll([]string{"A.B"}, "string")
	if err != nil || !reflect.DeepEqual(values, []any{"first", "second"}) {
		t.Errorf("repeated one-element key = %#v, %v", values, err)
	}
	_, err = cfg.Get([]string{"A.B"}, "string")
	assertProfileErrorLine(t, err, ErrMultipleValues, 4)
}

func assertProfileErrorLine(t *testing.T, err error, code Code, line int) {
	t.Helper()
	assertValueCode(t, err, code)
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Line != line {
		t.Errorf("error = %v, want header line %d", err, line)
	}
}

func TestRepeatedSettingsPreserveOrderEqualValuesAndUnsetStates(t *testing.T) {
	cfg, err := Build([]Entry{
		{Line: 1, Kind: Heading, Path: []string{"A"}},
		{Line: 2, Kind: Value, Path: []string{"A"}, Unset: true},
		{Line: 4, Kind: Value, Path: []string{"B"}, Raw: "unrelated"},
		{Line: 5, Kind: Value, Path: []string{"A"}, Raw: "\"\""},
		{Line: 8, Kind: Value, Path: []string{"A"}, Form: PipeBlock},
		{Line: 10, Kind: Value, Path: []string{"A"}, Raw: "same"},
		{Line: 11, Kind: Value, Path: []string{"A"}, Raw: "same"},
	}, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	path := []string{"A"}
	wantValues := []any{"", "", "", "same", "same"}
	values, err := cfg.GetAll(path, "string")
	if err != nil || !reflect.DeepEqual(values, wantValues) {
		t.Fatalf("GetAll = %#v, %v; want %#v", values, err, wantValues)
	}
	wantStates := []bool{true, false, false, false, false}
	states, err := cfg.IsUnsetAll(path)
	if err != nil || !reflect.DeepEqual(states, wantStates) {
		t.Fatalf("IsUnsetAll = %#v, %v; want %#v", states, err, wantStates)
	}
	// Results must not expose the Config's slices or another call's slices.
	values[0], states[0] = "caller change", false
	values, err = cfg.GetAll(path, "string")
	if err != nil || !reflect.DeepEqual(values, wantValues) {
		t.Errorf("GetAll was changed by caller: %#v, %v", values, err)
	}
	states, err = cfg.IsUnsetAll(path)
	if err != nil || !reflect.DeepEqual(states, wantStates) {
		t.Errorf("IsUnsetAll was changed by caller: %#v, %v", states, err)
	}
	_, err = cfg.Get(path, "unknown")
	assertProfileErrorLine(t, err, ErrMultipleValues, 5)
	_, err = cfg.IsUnset(path)
	assertProfileErrorLine(t, err, ErrMultipleValues, 5)
	for i, want := range wantValues {
		value, err := cfg.GetAt(path, i, "string")
		if err != nil || value != want {
			t.Errorf("GetAt(%d) = %#v, %v; want %#v", i, value, err, want)
		}
	}
	single, err := cfg.GetAll([]string{"B"}, "string")
	if err != nil || !reflect.DeepEqual(single, []any{"unrelated"}) {
		t.Errorf("single-occurrence GetAll = %#v, %v", single, err)
	}
}

func TestRepeatedParsedAliasesRetainEachOccurrenceAndKeyBoundary(t *testing.T) {
	entries, err := Parse(strings.NewReader(parserDocument(
		"\"A.B\": first\n\"\\u0041.B\": second\nA.B: separate\nA.\n  .B: relative",
	)))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path []string
		want []any
	}{
		{[]string{"A.B"}, []any{"first", "second"}},
		{[]string{"A", "B"}, []any{"separate", "relative"}},
	} {
		got, err := cfg.GetAll(test.path, "string")
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("GetAll(%q) = %#v, %v; want %#v", test.path, got, err, test.want)
		}
	}
}

func TestBareInternalQuotesRemainLiteralThroughTypedRepeatedRetrieval(t *testing.T) {
	want := []any{
		"say \"hello\"",
		"\\\"hello\\\"",
		"raw \"unterminated",
		"{\"quote\":\"value\",\"path\":\"C:\\\\temp\"}",
		"\u00a0\"\\q\"",
		"value\"\\",
		"prefix \"\"\"",
	}
	var source strings.Builder
	for _, value := range want {
		source.WriteString("A(string): ")
		source.WriteString(value.(string))
		source.WriteByte('\n')
	}
	entries, err := Parse(strings.NewReader(parserDocument(source.String())))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Form != "" {
			t.Errorf("internal quotes selected a block form: %#v", entry)
		}
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	values, err := cfg.GetAll([]string{"A"}, "string")
	if err != nil || !reflect.DeepEqual(values, want) {
		t.Errorf("literal typed GetAll = %#v, %v; want %#v", values, err, want)
	}
	for index, expected := range want {
		value, err := cfg.GetAt([]string{"A"}, index, "string")
		if err != nil || value != expected {
			t.Errorf("literal typed GetAt(%d) = %#v, %v; want %#v", index, value, err, expected)
		}
	}
	states, err := cfg.IsUnsetAll([]string{"A"})
	if err != nil || !reflect.DeepEqual(states, make([]bool, len(want))) {
		t.Errorf("literal values changed unset state: %#v, %v", states, err)
	}
}

func TestBareInternalQuoteFailsOnlyWhenRequestedConversionCannotInterpretIt(t *testing.T) {
	entries, err := Parse(strings.NewReader(parserDocument("A:001\nA: raw\"001")))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatalf("literal quote failed string decoding: %v", err)
	}
	values, err := cfg.GetAll([]string{"A"}, "int")
	assertProfileErrorLine(t, err, ErrTypeMismatch, 4)
	if values != nil {
		t.Errorf("failed retrieval returned partial values: %#v", values)
	}
	values, err = cfg.GetAll([]string{"A"}, "string")
	if err != nil || !reflect.DeepEqual(values, []any{"001", "raw\"001"}) {
		t.Errorf("failed numeric retrieval changed stored strings: %#v, %v", values, err)
	}
}

func TestRepeatedRetrievalConversionIsPerOccurrenceAndPerCall(t *testing.T) {
	calls := 0
	profile := Profile{StringType: "text", Types: map[string]Converter{
		"number": func(text Text) (any, error) {
			calls++
			return "number:" + text.Value, nil
		},
	}}
	cfg, err := Build([]Entry{
		{Line: 3, Kind: Value, Path: []string{"A"}, Raw: "001", Type: "number", HasType: true},
		{Line: 6, Kind: Value, Path: []string{"A"}, Raw: "002"},
		{Line: 9, Kind: Value, Path: []string{"A"}, Raw: "003", Type: "number", HasType: true},
	}, profile)
	if err != nil || calls != 2 {
		t.Fatalf("Build error = %v, converter calls = %d; want 2", err, calls)
	}
	path := []string{"A"}
	for range 2 {
		value, err := cfg.GetAt(path, 0, "number")
		if err != nil || value != "number:001" {
			t.Errorf("declared GetAt = %#v, %v", value, err)
		}
	}
	if calls != 2 {
		t.Errorf("declared GetAt converted again: calls=%d", calls)
	}
	for iteration := 0; iteration < 2; iteration++ {
		values, err := cfg.GetAll(path, "number")
		want := []any{"number:001", "number:002", "number:003"}
		if err != nil || !reflect.DeepEqual(values, want) || calls != 3+iteration {
			t.Errorf("GetAll = %#v, %v; calls=%d", values, err, calls)
		}
	}
	for iteration := 0; iteration < 2; iteration++ {
		value, err := cfg.GetAt(path, 1, "number")
		if err != nil || value != "number:002" || calls != 5+iteration {
			t.Errorf("untyped GetAt = %#v, %v; calls=%d", value, err, calls)
		}
	}
	value, err := cfg.GetAt(path, 1, "text")
	if err != nil || value != "002" || calls != 6 {
		t.Errorf("conversion changed original untyped string: %#v, %v; calls=%d", value, err, calls)
	}
	values, err := cfg.GetAll(path, "text")
	assertProfileErrorLine(t, err, ErrTypeMismatch, 3)
	if values != nil || calls != 6 {
		t.Errorf("declared mismatch converted or returned partial values: %#v calls=%d", values, calls)
	}
}

func TestRepeatedMixedDeclarationsAreIndependent(t *testing.T) {
	cfg, err := Build([]Entry{
		{Line: 2, Kind: Value, Path: []string{"A"}, Raw: "1", Type: "int", HasType: true},
		{Line: 7, Kind: Value, Path: []string{"A"}, Raw: "2", Type: "string", HasType: true},
	}, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	first, err := cfg.GetAt([]string{"A"}, 0, "int")
	if err != nil || first != int64(1) {
		t.Errorf("first mixed declaration = %#v, %v", first, err)
	}
	second, err := cfg.GetAt([]string{"A"}, 1, "string")
	if err != nil || second != "2" {
		t.Errorf("second mixed declaration = %#v, %v", second, err)
	}
	values, err := cfg.GetAll([]string{"A"}, "int")
	assertProfileErrorLine(t, err, ErrTypeMismatch, 7)
	if values != nil {
		t.Errorf("mixed mismatch returned partial values: %#v", values)
	}
	states, err := cfg.IsUnsetAll([]string{"A"})
	if err != nil || !reflect.DeepEqual(states, []bool{false, false}) {
		t.Errorf("mixed unset query = %#v, %v", states, err)
	}
	_, err = cfg.Get([]string{"A"}, "bool")
	assertProfileErrorLine(t, err, ErrMultipleValues, 7)
}

func TestRepeatedRetrievalFailureReturnsNoPartialValuesOrPrivateError(t *testing.T) {
	calls := 0
	profile := Profile{StringType: "string", Types: map[string]Converter{
		"number": func(text Text) (any, error) {
			calls++
			if strings.Contains(text.Value, "private") {
				return nil, errors.New("converter input: " + text.Value)
			}
			return text.Value, nil
		},
	}}
	cfg, err := Build([]Entry{
		{Line: 2, Kind: Value, Path: []string{"A"}, Raw: "first"},
		{Line: 9, Kind: Value, Path: []string{"A"}, Raw: "private-bad-value"},
		{Line: 12, Kind: Value, Path: []string{"A"}, Raw: "third"},
	}, profile)
	if err != nil || calls != 0 {
		t.Fatalf("untyped Build = %v; calls=%d", err, calls)
	}
	for iteration := 1; iteration <= 2; iteration++ {
		values, err := cfg.GetAll([]string{"A"}, "number")
		assertProfileErrorLine(t, err, ErrTypeMismatch, 9)
		if values != nil || calls != iteration*2 || strings.Contains(err.Error(), "private") {
			t.Errorf("failed GetAll = %#v, %v; calls=%d", values, err, calls)
		}
	}
	_, err = cfg.GetAt([]string{"A"}, 1, "number")
	assertProfileErrorLine(t, err, ErrTypeMismatch, 9)
	if calls != 5 || strings.Contains(err.Error(), "private") {
		t.Errorf("failed GetAt = %v; calls=%d", err, calls)
	}
}

func TestRepeatedBuildFailureDoesNotBecomeDuplicateOrPartialConfig(t *testing.T) {
	cfg, err := Build([]Entry{
		{Line: 1, Kind: Value, Path: []string{"A"}, Raw: "1", Type: "int", HasType: true},
		{Line: 4, Kind: Value, Path: []string{"A"}, Raw: "private-bad-value", Type: "int", HasType: true},
	}, DemoProfile())
	assertProfileErrorLine(t, err, ErrTypeConversion, 4)
	if cfg != nil || strings.Contains(err.Error(), "private") {
		t.Errorf("failed repeated Build = %#v, %v", cfg, err)
	}
}

func TestRepeatedRetrievalMissingAndIndexBounds(t *testing.T) {
	cfg, err := Build([]Entry{{Kind: Value, Path: []string{"A"}, Raw: "value"}}, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, config := range []*Config{nil, cfg} {
		for _, path := range [][]string{nil, {}, {"missing"}} {
			values, err := config.GetAll(path, "string")
			assertProfileErrorLine(t, err, ErrMissing, 0)
			if values != nil {
				t.Errorf("missing GetAll returned %#v", values)
			}
			states, err := config.IsUnsetAll(path)
			assertProfileErrorLine(t, err, ErrMissing, 0)
			if states != nil {
				t.Errorf("missing IsUnsetAll returned %#v", states)
			}
			_, err = config.GetAt(path, 0, "string")
			assertProfileErrorLine(t, err, ErrMissing, 0)
		}
	}
	for _, index := range []int{-1, 1, 100} {
		_, err := cfg.GetAt([]string{"A"}, index, "string")
		assertProfileErrorLine(t, err, ErrMissing, 0)
	}
	states, err := cfg.IsUnsetAll([]string{"A"})
	if err != nil || !reflect.DeepEqual(states, []bool{false}) {
		t.Errorf("single IsUnsetAll = %#v, %v", states, err)
	}
}

func TestProfileConversionDiagnosticsDoNotLeakValues(t *testing.T) {
	const secret = "secret-value-271828"
	profile := Profile{StringType: "string", Types: map[string]Converter{
		"fails": func(text Text) (any, error) { return nil, errors.New("bad input: " + text.Value) },
	}}
	_, err := Build([]Entry{{Kind: Value, Path: []string{"A"}, Raw: secret, Type: "fails", HasType: true}}, profile)
	assertValueCode(t, err, ErrTypeConversion)
	if strings.Contains(err.Error(), secret) {
		t.Fatal("declared conversion leaked the value")
	}
	cfg, err := Build([]Entry{{Kind: Value, Path: []string{"A"}, Raw: secret}}, profile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cfg.Get([]string{"A"}, "fails")
	assertValueCode(t, err, ErrTypeMismatch)
	if strings.Contains(err.Error(), secret) {
		t.Fatal("retrieval conversion leaked the value")
	}
}

func TestDemoProfileRejectsImplicitGoLiteralForms(t *testing.T) {
	for _, raw := range []string{"0x10", "1_000", "1.0", "1e2", "+", " 1 ", "9223372036854775808"} {
		converter := DemoProfile().Types["int"]
		if _, err := converter(Text{Value: raw}); err == nil {
			t.Fatalf("demo int accepted %q", raw)
		}
	}
	for _, raw := range []string{"TRUE", "False", "1", "0", "yes", ""} {
		converter := DemoProfile().Types["bool"]
		if _, err := converter(Text{Value: raw}); err == nil {
			t.Fatalf("demo bool accepted %q", raw)
		}
	}
}

func TestProfileIsCopiedAndValidated(t *testing.T) {
	for _, profile := range []Profile{
		{},
		{StringType: "string", Types: map[string]Converter{"string": func(Text) (any, error) { return nil, nil }}},
		{StringType: "string", Types: map[string]Converter{"int": nil}},
		{StringType: "string", Types: map[string]Converter{"": func(Text) (any, error) { return nil, nil }}},
	} {
		_, err := Build(nil, profile)
		assertValueCode(t, err, ErrInput)
	}
	profile := DemoProfile()
	cfg, err := Build([]Entry{{Kind: Value, Path: []string{"A"}, Raw: "001"}}, profile)
	if err != nil {
		t.Fatal(err)
	}
	delete(profile.Types, "int")
	got, err := cfg.Get([]string{"A"}, "int")
	if err != nil || got != int64(1) {
		t.Fatalf("caller changed config profile: Get() = %v, %v", got, err)
	}
}
