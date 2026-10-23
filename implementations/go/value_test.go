package dotreg

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeString(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		unset bool
		want  string
	}{
		{"bare", "001", false, "001"},
		{"source outer spaces", " \tvalue \t", false, "value"},
		{"unset", "", true, ""},
		{"unset spaces", " \t ", true, ""},
		{"explicit empty", `""`, false, ""},
		{"quoted spaces", `"  value  "`, false, "  value  "},
		{"quoted outer spaces", " \t\"  value  \"\t ", false, "  value  "},
		{"bare Windows path", `C:\temp\new`, false, `C:\temp\new`},
		{"quoted Windows path", `"C:\\temp\\new"`, false, `C:\temp\new`},
		{"bare unknown escape is literal", `\q`, false, `\q`},
		{"bare trailing slash is literal", `value\`, false, `value\`},
		{"bare dot escape is literal", `foo\.bar`, false, `foo\.bar`},
		{"quoted dot escape", `"foo\.bar"`, false, `foo.bar`},
		{"quoted quote escape", `"say \"hello\""`, false, `say "hello"`},
		{"quoted slash escape", `"\\"`, false, `\`},
		{"JSON short escapes", `"\"\\\/\b\f\n\r\t"`, false, "\"\\/\b\f\n\r\t"},
		{"bare short escapes remain literal", `\b\f\n\r\t`, false, `\b\f\n\r\t`},
		{"bare Unicode remains literal", `\u{1F600}\uD83D\uDE00`, false, `\u{1F600}\uD83D\uDE00`},
		{"braced Unicode ASCII", `"\u{41}"`, false, "A"},
		{"braced Unicode leading zeros", `"\u{000041}"`, false, "A"},
		{"braced Unicode lowercase hex", `"\u{1f600}"`, false, "😀"},
		{"braced Unicode non-BMP", `"\u{1F98A}"`, false, "🦊"},
		{"fixed Unicode", `"\u0041"`, false, "A"},
		{"fixed Unicode leaves suffix", `"\u0041B"`, false, "AB"},
		{"fixed Unicode BMP boundaries", `"\uD7FF\uE000"`, false, "\ud7ff\ue000"},
		{"fixed Unicode surrogate pair", `"\uD83D\uDE00"`, false, "😀"},
		{"fixed Unicode mixed hex case", `"\ud83D\udE00"`, false, "😀"},
		{"surrogate pair range boundaries", `"\uD800\uDC00\uDBFF\uDFFF"`, false, "\U00010000\U0010ffff"},
		{"Unicode controls are data", `"\u0000\u{A}\u000D\u{9}"`, false, "\x00\n\r\t"},
		{"Unicode maximum scalar", `"\u{10FFFF}"`, false, "\U0010ffff"},
		{"Unicode noncharacters remain data", `"\uFDD0\uFFFF"`, false, "\ufdd0\uffff"},
		{"Unicode unassigned scalar remains data", `"\u{378}"`, false, "\u0378"},
		{"Unicode BOM remains data", `"\uFEFF"`, false, "\ufeff"},
		{"Unicode explicit replacement scalar", `"\uFFFD"`, false, "\ufffd"},
		{"Unicode quote remains data", `"\u0022\u{22}"`, false, "\"\""},
		{"Unicode slash is not decoded twice", `"\u005Cn"`, false, `\n`},
		{"braced slash is not decoded twice", `"\u{5C}u0041"`, false, `\u0041`},
		{"Unicode combining sequence is not normalised", `"\u0065\u{301}"`, false, "e\u0301"},
		{"decode once", `"\\q"`, false, `\q`},
		{"decoded quote remains", `"\""`, false, `"`},
		{"Unicode", `"🦊 日本語"`, false, "🦊 日本語"},
		{"Unicode outer space retained", "\u00a0value\u00a0", false, "\u00a0value\u00a0"},
		{"inline comment literal", `value # literal`, false, `value # literal`},
		{"null text", `null`, false, `null`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DecodeString(Entry{Line: 7, Kind: Value, Raw: test.raw, Unset: test.unset})
			if err != nil || got != (Text{Value: test.want, Unset: test.unset}) {
				t.Fatalf("DecodeString() = %#v, %v; want value %q, unset %v", got, err, test.want, test.unset)
			}
		})
	}
}

func TestDecodeStringRejectsMalformedValues(t *testing.T) {
	for _, raw := range []string{
		`"unterminated`,
		`"trailing\`,
		`"\q"`,
		`"\u"`,
		`"\u0"`,
		`"\u00"`,
		`"\u000"`,
		`"\u00G0"`,
		`"\u+041"`,
		`"\u{}"`,
		`"\u{"`,
		`"\u{41"`,
		`"\u{1234567}"`,
		`"\u{0000000}"`,
		`"\u{110000}"`,
		`"\u{FFFFFF}"`,
		`"\u{D800}"`,
		`"\u{DFFF}"`,
		`"\u{1_F600}"`,
		`"\u{ 41}"`,
		`"\u{41 }"`,
		`"\u{-1}"`,
		`"\u{+41}"`,
		`"\u{0x41}"`,
		`"\u{４１}"`,
		`"\uD800"`,
		`"\uDBFF"`,
		`"\uDC00"`,
		`"\uDFFF"`,
		`"\uD800\uD800"`,
		`"\uDC00\uD800"`,
		`"\uD800\u0041"`,
		`"\uD800A"`,
		`"\uD800\u{DC00}"`,
		`"\u{D800}\uDC00"`,
		`"\uD800\\uDC00"`,
		`"\uD800\uDC0"`,
		`"\U0001F600"`,
		`"\x41"`,
		`"\a"`,
		`"\v"`,
		`"\0"`,
		`"\'"`,
		`"\:"`,
		`"\#"`,
		`"\ "`,
		`"value"tail`,
		`some"value"`,
		`"first" "second"`,
		`\"hello\"`,
		"value\nnext",
		"\"value\rnext\"",
		string([]byte{0xff}),
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := DecodeString(Entry{Line: 9, Kind: Value, Raw: raw})
			assertValueCode(t, err, ErrString)
			if strings.Contains(err.Error(), raw) && len(raw) > 6 {
				t.Fatal("diagnostic contains the raw value")
			}
		})
	}
}

func TestDecodeStringRejectsInconsistentEntries(t *testing.T) {
	for _, entry := range []Entry{
		{Kind: Heading},
		{Kind: Value, Raw: "value", Unset: true},
		{Kind: Value, Raw: ""},
		{Kind: Value, Raw: " \t"},
		{Kind: Value, Form: ValueForm("unknown"), Raw: "value"},
		{Kind: Value, Form: TripleQuoted, Unset: true},
		{Kind: Value, Form: PipeBlock, Unset: true},
		{Kind: Heading, Form: TripleQuoted},
	} {
		_, err := DecodeString(entry)
		assertValueCode(t, err, ErrString)
	}
}

func TestDecodeBlockStrings(t *testing.T) {
	tests := []struct {
		name string
		form ValueForm
		raw  string
		want string
	}{
		{"triple empty is set", TripleQuoted, "", ""},
		{"pipe empty is set", PipeBlock, "", ""},
		{"triple whitespace remains", TripleQuoted, " \t  ", " \t  "},
		{"pipe whitespace remains", PipeBlock, " \t  ", " \t  "},
		{"triple physical line breaks", TripleQuoted, "first\n  second  \n", "first\n  second  \n"},
		{"pipe physical line breaks", PipeBlock, "first\n  second  \n", "first\n  second  \n"},
		{"triple mixed EOL remains", TripleQuoted, "first\r\nsecond\nthird\r\n", "first\r\nsecond\nthird\r\n"},
		{"pipe mixed EOL remains", PipeBlock, "first\r\nsecond\nthird\r\n", "first\r\nsecond\nthird\r\n"},
		{"triple literal quotes", TripleQuoted, `say "hello", then """`, `say "hello", then """`},
		{"pipe literal quotes", PipeBlock, `"unterminated`, `"unterminated`},
		{"pipe backslashes never decode", PipeBlock, `C:\temp\new\q\u{110000}\`, `C:\temp\new\q\u{110000}\`},
		{"triple basic escapes", TripleQuoted, `\.\"\\\/\b\f\n\r\t`, ".\"\\/\b\f\n\r\t"},
		{"triple Unicode and surrogate pair", TripleQuoted, `\u{1F600}\u0041\uD83D\uDE00`, "😀A😀"},
		{"triple escaped controls", TripleQuoted, `\u0000\u{A}\u{D}\u0009`, "\x00\n\r\t"},
		{"triple decoding happens once", TripleQuoted, `\u005Cu{41}\\n`, `\u{41}\n`},
		{"triple decoded quote is data", TripleQuoted, `\u0022\"\"`, `"""`},
		{"triple physical and escaped newlines", TripleQuoted, "A\\n\r\nB", "A\n\r\nB"},
		{"triple decomposed Unicode remains", TripleQuoted, "e\\u{301}", "e\u0301"},
		{"pipe decomposed Unicode remains", PipeBlock, "e\u0301", "e\u0301"},
		{"triple syntax characters are data", TripleQuoted, ".A(T): # |", ".A(T): # |"},
		{"pipe syntax characters are data", PipeBlock, ".A(T): # |", ".A(T): # |"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DecodeString(Entry{Line: 5, Kind: Value, Form: test.form, Raw: test.raw})
			if err != nil || got != (Text{Value: test.want}) {
				t.Fatalf("DecodeString() = %#v, %v; want %q, set", got, err, test.want)
			}
		})
	}
}

func TestDecodeBlockRejectsMalformedSource(t *testing.T) {
	for _, form := range []ValueForm{TripleQuoted, PipeBlock} {
		for _, raw := range []string{string([]byte{0xff}), "body\rline", "body\r"} {
			t.Run(string(form)+"/"+raw, func(t *testing.T) {
				_, err := DecodeString(Entry{Line: 5, Kind: Value, Form: form, Raw: raw})
				assertValueCode(t, err, ErrString)
			})
		}
	}
}

func TestDecodeTripleRejectsMalformedEscapes(t *testing.T) {
	for _, raw := range []string{
		`private\q`, `private\`, `\u`, `\u{}`, `\u{D800}`, `\u{110000}`,
		`\uD83D`, `\uDE00`, `\uD83D\u0041`, `\x41`, `\U0001F600`,
		"private\\\nnext", "private\\\r\nnext", "\\u{4\n1}",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := DecodeString(Entry{Line: 5, Kind: Value, Form: TripleQuoted, Raw: raw})
			assertValueCode(t, err, ErrString)
			if strings.Contains(err.Error(), "private") {
				t.Fatal("diagnostic contains the raw block")
			}
		})
	}
}

func TestDecodeQuotedStopsAtClosingQuote(t *testing.T) {
	got, end, err := decodeQuoted(`"A\.B\"C".Tail`)
	if err != nil || got != `A.B"C` || end != len(`"A\.B\"C"`) {
		t.Fatalf("decodeQuoted() = %q, %d, %v", got, end, err)
	}
}

func assertValueCode(t *testing.T, err error, code Code) {
	t.Helper()
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != code {
		t.Fatalf("error = %v; want code %v", err, code)
	}
}
