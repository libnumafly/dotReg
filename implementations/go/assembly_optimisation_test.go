package dotreg

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestParseTripleAssemblyMarginsAndLineBoundaries(t *testing.T) {
	fixtures := []struct {
		name   string
		rows   []string
		want   []string
		margin int
	}{
		{"no rows", nil, nil, 7},
		{"true empty row", []string{""}, []string{""}, 3},
		{"space-only row", []string{"   "}, []string{""}, 3},
		{"extra whitespace and empty rows", []string{"    first \t", "", "  ", "  last"}, []string{"  first \t", "", "", "last"}, 2},
		{"long discarded margin", []string{strings.Repeat(" ", 8193) + "first", "", strings.Repeat(" ", 8193) + "last"}, []string{"first", "", "last"}, 8193},
		{"long content", []string{" " + strings.Repeat("body ", 32768), " last"}, []string{strings.Repeat("body ", 32768), "last"}, 1},
	}
	for _, count := range []int{8, 9, 1025} {
		rows, want := make([]string, count), make([]string, count)
		for i := range rows {
			rows[i], want[i] = "   text\\u{1F600}", " text\\u{1F600}"
			if i%3 == 1 {
				rows[i], want[i] = "", ""
			} else if i%3 == 2 {
				rows[i], want[i] = "  ", ""
			}
		}
		name := "eight rows"
		if count == 9 {
			name = "ninth row"
		} else if count > 9 {
			name = "many rows"
		}
		fixtures = append(fixtures, struct {
			name   string
			rows   []string
			want   []string
			margin int
		}{name, rows, want, 2})
	}
	for _, fixture := range fixtures {
		for _, eof := range []bool{false, true} {
			finish := "before next entry"
			if eof {
				finish = "closer at EOF"
			}
			t.Run(fixture.name+"/"+finish, func(t *testing.T) {
				var source, expected strings.Builder
				source.WriteString("A: \"\"\"\r\n")
				var previousEnding string
				for i, row := range fixture.rows {
					ending := "\n"
					if i%2 == 0 {
						ending = "\r\n"
					}
					source.WriteString(row)
					source.WriteString(ending)
					if i > 0 {
						expected.WriteString(previousEnding)
					}
					expected.WriteString(fixture.want[i])
					previousEnding = ending
				}
				source.WriteString(strings.Repeat(" ", fixture.margin))
				source.WriteString("\"\"\"\t ")
				want := []Entry{parserBlockValue(1, []string{"A"}, expected.String(), TripleQuoted)}
				if !eof {
					source.WriteString("\r\nB:next")
					want = append(want, parserValue(len(fixture.rows)+3, []string{"B"}, "next"))
				}
				got, err := Parse(strings.NewReader(parserDocument(source.String())))
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("triple assembly changed content, margin or following entry: %v", err)
				}
			})
		}
	}
}

func TestParseTripleAssemblyPreservesValidationPriority(t *testing.T) {
	for _, fixture := range []struct {
		name, source string
		line         int
	}{
		{"first short margin", "A:\"\"\"\n  good\n private\nprivate\n  \"\"\"", 5},
		{"space-only short margin", "A:\"\"\"\n\n \nprivate\n  \"\"\"", 5},
		{"later UTF-8 error wins", "A:\"\"\"\n private\n\xff\n  \"\"\"", 5},
		{"later CR error wins", "A:\"\"\"\n private\nprivate\rbody\n  \"\"\"", 5},
		{"missing closer wins", "A:\"\"\"\n private\n  body", 3},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			visits := 0
			err := ParseEach(strings.NewReader(parserDocument(fixture.source)), func(Entry) error {
				visits++
				return nil
			})
			parserRequireCode(t, err, ErrSyntax, fixture.line)
			if visits != 0 || strings.Contains(err.Error(), "private") {
				t.Fatalf("invalid body visited or source text leaked: visits=%d, error=%v", visits, err)
			}
			got, err := Parse(strings.NewReader(parserDocument(fixture.source)))
			if err == nil || got != nil {
				t.Fatal("invalid triple body returned partial entries")
			}
		})
	}
	for _, source := range []string{
		"A:\"\"\"\n private\n  body",
		"A:\"\"\"\n private\n  \"\"\"",
	} {
		reader := &refactorTailErrorReader{data: source, err: errors.New("private reader error")}
		visits := 0
		err := ParseEach(io.MultiReader(strings.NewReader(parserPreamble), reader), func(Entry) error {
			visits++
			return nil
		})
		parserRequireCode(t, err, ErrInput, 5)
		if visits != 0 || strings.Contains(err.Error(), "private") {
			t.Fatalf("reader failure lost priority or leaked private text: visits=%d, error=%v", visits, err)
		}
	}
}
