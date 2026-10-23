package dotreg

import (
	"errors"
	"strings"
	"testing"
)

func TestUnicodeSequencesArePreservedThroughLookup(t *testing.T) {
	const composed = "\u304c"
	const decomposed = "\u304b\u3099"
	entries, err := Parse(strings.NewReader(
		composed + ": " + decomposed + "\n" +
			decomposed + ": " + composed + "\n",
	))
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
	entries, err := Parse(strings.NewReader(source))
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
		" .(string): \"\"\"\n" +
		"  say \"hello\"\\n\\u{1F600}\n" +
		"  \"\"\"\n" +
		" .Literal:\n" +
		"  |say \"hello\"\\n\\u{1F600}\n" +
		"EmptyTriple: \"\"\"\n\"\"\"\n" +
		"EmptyPipe:\n |\n" +
		"Unset:\n"
	entries, err := Parse(strings.NewReader(source))
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
		entries, err := Parse(strings.NewReader(source))
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
	entries, err := Parse(strings.NewReader("Good: first\nA(int):\n |00\\u0031"))
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
	const source = "A.\n .B: \"\"\"\n  \\\"\"\"\n  \"\"\"\n .C: next"
	entries, err := Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[1].Form != TripleQuoted || entries[1].Raw != "\\\"\"\"" || entries[1].Line != 2 || entries[2].Line != 5 {
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
	entries, err := Parse(strings.NewReader(
		"\"A:B (C)#\t\"(" + typeName + "):\" 001 \"\n",
	))
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
	entries, err := Parse(strings.NewReader(
		"\"A\x00B\": one\n\"A\".\"B\": two\n\"A\x00\".B: three\n",
	))
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
	_, err = Build([]Entry{
		{Kind: Value, Path: []string{"A.B"}, Raw: "first"},
		{Kind: Value, Path: []string{"A.B"}, Raw: "second"},
	}, DemoProfile())
	assertValueCode(t, err, ErrDuplicate)
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
