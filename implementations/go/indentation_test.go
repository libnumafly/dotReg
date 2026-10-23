package dotreg

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseIndentationTwoSpaceGrid(t *testing.T) {
	const source = "A.B.\n" +
		"  .C.D.\n" +
		"    .E.\n" +
		"      .: third self\n" +
		"      .\"F.G\": \"  value  \"\n" +
		"    .: second self\n" +
		"    .H: child\n" +
		"  .I: sibling\n" +
		"A.B.J: absolute"
	want := []Entry{
		parserHeading(1, "A", "B"),
		parserHeading(2, "A", "B", "C", "D"),
		parserHeading(3, "A", "B", "C", "D", "E"),
		parserValue(4, []string{"A", "B", "C", "D", "E"}, "third self"),
		parserValue(5, []string{"A", "B", "C", "D", "E", "F.G"}, `"  value  "`),
		parserValue(6, []string{"A", "B", "C", "D"}, "second self"),
		parserValue(7, []string{"A", "B", "C", "D", "H"}, "child"),
		parserValue(8, []string{"A", "B", "I"}, "sibling"),
		parserValue(9, []string{"A", "B", "J"}, "absolute"),
	}
	got, err := Parse(strings.NewReader(parserDocument(source)))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("two-space hierarchy = %#v, %v; want %#v", got, err, want)
	}
}

func TestParseIndentationRejectsOddWidthsWithOpenParents(t *testing.T) {
	// Keep enough valid ancestors open that treating space counts as old scope
	// indexes, or rounding an odd width down, would find an apparent parent.
	const parents = "A.\n  .B.\n    .C.\n      .D.\n        .E.\n          .F.\n"
	for _, width := range []int{1, 3, 5, 7, 9} {
		for _, suffix := range []string{".Value: private", ".: private", ".Heading."} {
			source := parents + strings.Repeat(" ", width) + suffix
			t.Run(source, func(t *testing.T) {
				visits := 0
				err := ParseEach(strings.NewReader(parserDocument(source)), func(Entry) error {
					visits++
					return nil
				})
				parserRequireCode(t, err, ErrSyntax, 9)
				if visits != 6 {
					t.Fatalf("notified %d entries, want only the six valid headings", visits)
				}
				got, err := Parse(strings.NewReader(parserDocument(source)))
				parserRequireCode(t, err, ErrSyntax, 9)
				if got != nil {
					t.Fatalf("invalid indentation returned partial entries: %#v", got)
				}
			})
		}
	}
}

func TestParseIndentationRejectsSkippedLevelsAndTabs(t *testing.T) {
	for _, test := range []struct {
		source string
		line   int
	}{
		{"A.\n    .B: skipped", 2},
		{"A.\n  .B.\n      .C: skipped", 3},
		{"A.\n  .B.\n    .C.\n        .D: skipped", 4},
		{"A.\n\t.B: tab", 2},
		{"A.\n \t.B: tab", 2},
		{"A.\n  \t.B: tab", 2},
		{"A.\n  .B.\n    \t.C: tab", 3},
	} {
		t.Run(test.source, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(test.source)))
			parserRequireCode(t, err, ErrSyntax, test.line+2)
			if got != nil {
				t.Fatalf("invalid indentation returned partial entries: %#v", got)
			}
		})
	}
}

func TestParseIndentationDoesNotConstrainBlockSpaces(t *testing.T) {
	for _, test := range []struct {
		name, source string
		want         []Entry
	}{
		{
			name:   "odd triple margin less than structural header",
			source: "A.\n  .B: \"\"\"\n first\n  second\n \"\"\"\n  .C: next",
			want: []Entry{
				parserHeading(1, "A"),
				parserBlockValue(2, []string{"A", "B"}, "first\n second", TripleQuoted),
				parserValue(6, []string{"A", "C"}, "next"),
			},
		},
		{
			name:   "deep header and odd triple margin",
			source: "A.\n  .B.\n    .C: \"\"\"\n   first\n    second\n   \"\"\"\n    .D: next",
			want: []Entry{
				parserHeading(1, "A"),
				parserHeading(2, "A", "B"),
				parserBlockValue(3, []string{"A", "B", "C"}, "first\n second", TripleQuoted),
				parserValue(7, []string{"A", "B", "D"}, "next"),
			},
		},
		{
			name:   "odd pipe indentation relative to header",
			source: "A.\n  .B:\n   |first\n     |  second\n   | .C: literal\n  .C: next",
			want: []Entry{
				parserHeading(1, "A"),
				parserBlockValue(2, []string{"A", "B"}, "first\n  second\n .C: literal", PipeBlock),
				parserValue(6, []string{"A", "C"}, "next"),
			},
		},
		{
			name:   "deep header still requires deeper raw pipe indentation",
			source: "A.\n  .B.\n    .C:\n     |first\n       |second\n    .D: next",
			want: []Entry{
				parserHeading(1, "A"),
				parserHeading(2, "A", "B"),
				parserBlockValue(3, []string{"A", "B", "C"}, "first\nsecond", PipeBlock),
				parserValue(6, []string{"A", "B", "D"}, "next"),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(parserDocument(test.source)))
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("block spaces = %#v, %v; want %#v", got, err, test.want)
			}
		})
	}
}

func TestParseIndentationCallbackCanStopBeforeOddWidthFailure(t *testing.T) {
	for _, source := range []string{
		"A.\n  .B: value\n .C: invalid",
		"A.\n  .B:\n .C: invalid",
		"A.\n  .B:\n   |body\n .C: invalid",
	} {
		t.Run(source, func(t *testing.T) {
			stop := errors.New("stop after the complete value")
			var visited []Entry
			err := ParseEach(strings.NewReader(parserDocument(source)), func(entry Entry) error {
				visited = append(visited, entry)
				if entry.Kind == Value {
					return stop
				}
				return nil
			})
			if err != stop || len(visited) != 2 || visited[1].Line != 4 {
				t.Fatalf("later odd-width failure preempted callback: entries=%#v, error=%v", visited, err)
			}
		})
	}
}
