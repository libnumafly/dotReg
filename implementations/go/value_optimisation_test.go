package dotreg

import (
	"strings"
	"testing"
)

func TestDecodeStringEscapedSpanSizes(t *testing.T) {
	prefix := strings.Repeat("東京abc", 257)
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"plain Unicode", prefix, prefix},
		{"long literal spans", prefix + `\n` + prefix, prefix + "\n" + prefix},
		{"dense Unicode contraction", strings.Repeat(`\u{000041}`, 1024), strings.Repeat("A", 1024)},
		{"surrogate pair contraction", strings.Repeat(`\uD83D\uDE00`, 257), strings.Repeat("😀", 257)},
		{"mixed scalar widths", `\u0000\u007F\u0080\u07FF\u0800\uFFFF\u{10000}\u{10FFFF}`, "\x00\x7f\u0080\u07ff\u0800\uffff\U00010000\U0010ffff"},
		{"escaped source is not reinterpreted", prefix + `\u005Cn\"\\\u{22}`, prefix + `\n"\"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, form := range []ValueForm{"", TripleQuoted} {
				raw := test.raw
				if form == "" {
					raw = `"` + raw + `"`
				}
				got, err := DecodeString(Entry{Line: 17, Kind: Value, Form: form, Raw: raw})
				if err != nil || got != (Text{Value: test.want}) {
					t.Fatalf("form %q: DecodeString() = %#v, %v; want %q", form, got, err, test.want)
				}
			}
		})
	}
}

func TestDecodeStringRejectsLateInvalidEscapes(t *testing.T) {
	prefix := strings.Repeat(`private\u{41}`, 1024)
	for _, tail := range []string{`\`, `\q`, `\uD800`, `\u{110000}`, `\u{0000000}`} {
		for _, form := range []ValueForm{"", TripleQuoted} {
			raw := prefix + tail
			if form == "" {
				raw = `"` + raw + `"`
			}
			_, err := DecodeString(Entry{Line: 29, Kind: Value, Form: form, Raw: raw})
			assertValueCode(t, err, ErrString)
			if strings.Contains(err.Error(), "private") {
				t.Fatal("diagnostic contains source text")
			}
		}
	}
}
