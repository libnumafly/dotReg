package dotreg

import (
	"reflect"
	"strings"
	"testing"
)

// A heading's structural level can represent several decoded elements. Every
// callback still owns its path, including assignments to that heading itself.
func TestParseMultiElementScopePathsRemainIndependent(t *testing.T) {
	const source = "\"A.B\".\"\".\n" +
		"  .C.D.\n" +
		"    .\"E.F\".\"\": first\n" +
		"    .: self\n" +
		"    .G.H.I: several\n" +
		"  .J.K: dedented\n" +
		"  .L: sibling\n" +
		"M.N.O: absolute\n"
	want := [][]string{
		{"A.B", ""},
		{"A.B", "", "C", "D"},
		{"A.B", "", "C", "D", "E.F", ""},
		{"A.B", "", "C", "D"},
		{"A.B", "", "C", "D", "G", "H", "I"},
		{"A.B", "", "J", "K"},
		{"A.B", "", "L"},
		{"M", "N", "O"},
	}
	var got [][]string
	err := ParseEach(strings.NewReader(parserDocument(source)), func(entry Entry) error {
		got = append(got, append([]string(nil), entry.Path...))
		for i := range entry.Path {
			entry.Path[i] = "changed by the visitor"
		}
		return nil
	})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("paths after visitor mutation = %#v, %v; want %#v", got, err, want)
	}
}

func TestParseBareKeyUnicodeAndASCIIClassification(t *testing.T) {
	// Unicode format characters are neither Unicode whitespace nor control
	// characters. An ASCII fast path must preserve the existing distinction.
	for _, name := range []string{
		"!$%&'*+,-/0123456789;<=>?@[]^_`{|}~",
		"café", "東京", "😀", "L\u00adR", "L\u200bR", "L\ufeffR",
	} {
		t.Run("accepted/"+name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(name + ":value\n")))
			want := []Entry{parserValue(1, []string{name}, "value")}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("key %q = %#v, %v; want %#v", name, got, err, want)
			}
		})
	}
	for _, char := range []rune{
		0, 1, '\t', '\v', '\f', 0x1f, ' ', 0x7f, 0x85, 0x9f,
		0xa0, 0x1680, 0x2000, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000,
	} {
		name := "L" + string(char) + "R"
		t.Run("rejected/"+name, func(t *testing.T) {
			entries, err := Parse(strings.NewReader(parserDocument(name + ":value\n")))
			parserRequireCode(t, err, ErrSyntax, 3)
			if entries != nil {
				t.Fatal("invalid bare key returned partial entries")
			}
		})
	}
}

func TestParsePipeBodyBufferBoundaries(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		rows   []string
		margin int
		mixed  bool
	}{
		{"empty body line", []string{""}, 1, false},
		{"eight mixed rows", []string{"", "one", "", "two", "three", "", "four", ""}, 3, true},
		{"ninth row", []string{"", "one", "", "two", "three", "", "four", "", "last"}, 3, true},
		// With LF endings each original line is exactly 2,048 bytes.
		{"4096 original bytes", []string{strings.Repeat("a", 2045), strings.Repeat("b", 2045)}, 1, false},
		{"over 4096 original bytes", []string{strings.Repeat("a", 2046), strings.Repeat("b", 2046)}, 1, false},
		{"long discarded margin", []string{"first", "", "last"}, 4097, true},
		{"long body line", []string{strings.Repeat("long body ", 16384), "last"}, 1, true},
	} {
		for _, finish := range []struct {
			name string
			eof  bool
		}{{"at EOF", true}, {"before next entry", false}} {
			t.Run(fixture.name+"/"+finish.name, func(t *testing.T) {
				var source, expected strings.Builder
				source.WriteString("A:\n")
				var previousEnding string
				for i, row := range fixture.rows {
					ending := "\n"
					if fixture.mixed && i%2 == 0 {
						ending = "\r\n"
					}
					if finish.eof && i == len(fixture.rows)-1 {
						ending = ""
					}
					source.WriteString(strings.Repeat(" ", fixture.margin))
					source.WriteByte('|')
					source.WriteString(row)
					source.WriteString(ending)
					if i > 0 {
						expected.WriteString(previousEnding)
					}
					expected.WriteString(row)
					previousEnding = ending
				}
				want := []Entry{parserBlockValue(1, []string{"A"}, expected.String(), PipeBlock)}
				if !finish.eof {
					source.WriteString("B:next")
					want = append(want, parserValue(len(fixture.rows)+2, []string{"B"}, "next"))
				}
				got, err := Parse(strings.NewReader(parserDocument(source.String())))
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("pipe buffer boundary changed body or lookahead: %v", err)
				}
			})
		}
	}
}

func TestParsePipeBodyBufferDefersInvalidLookahead(t *testing.T) {
	for _, rows := range []int{8, 9} {
		body := strings.Repeat(" |body\r\n", rows)
		visits := 0
		err := ParseEach(strings.NewReader(parserDocument("A:\n"+body+"B:\xff\n")), func(entry Entry) error {
			visits++
			if entry.Kind != Value || entry.Form != PipeBlock || entry.Line != 3 {
				t.Fatalf("invalid lookahead replaced completed pipe body: %#v", entry)
			}
			return nil
		})
		parserRequireCode(t, err, ErrSyntax, rows+4)
		if visits != 1 {
			t.Fatalf("%d pipe rows produced %d callbacks; want one before invalid lookahead", rows, visits)
		}
	}
}
