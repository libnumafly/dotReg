package dotreg

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

func TestParsePipeAssemblyOutputBufferBoundary(t *testing.T) {
	for _, size := range []int{255, 256, 257} {
		for _, mixed := range []bool{false, true} {
			for _, eof := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d bytes/mixed=%v/eof=%v", size, mixed, eof), func(t *testing.T) {
					firstEnding, secondEnding := "\n", "\n"
					if mixed {
						firstEnding = "\r\n"
					}
					last := strings.Repeat("x", size-len("first")-len(firstEnding)-len(secondEnding))
					source := "A:\n |first" + firstEnding + " |" + secondEnding + " |" + last
					raw := "first" + firstEnding + secondEnding + last
					want := []Entry{parserBlockValue(1, []string{"A"}, raw, PipeBlock)}
					if !eof {
						source += "\r\nB:next"
						want = append(want, parserValue(5, []string{"B"}, "next"))
					}
					got, err := Parse(strings.NewReader(parserDocument(source)))
					if err != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("pipe output boundary changed bytes or lookahead: %v", err)
					}
					decoded, err := DecodeString(got[0])
					if err != nil || decoded.Unset || decoded.Value != raw || len(decoded.Value) != size {
						t.Fatalf("pipe output boundary changed decoded value: %v", err)
					}
				})
			}
		}
	}
}

func TestParsePipeAssemblyOwnedSpillKeepsSmallBodySizing(t *testing.T) {
	fixtures := []struct {
		name string
		rows []string
	}{
		{"eight large rows", strings.Split(strings.Repeat(strings.Repeat("x", 500)+"\n", 7)+strings.Repeat("x", 500), "\n")},
		{"ninth large row", strings.Split(strings.Repeat(strings.Repeat("x", 300)+"\n", 8)+strings.Repeat("x", 300), "\n")},
		{"stack then owned and empty", []string{"first", strings.Repeat("x", 250), "", "last", ""}},
	}
	for _, fixture := range fixtures {
		for _, eof := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/eof=%v", fixture.name, eof), func(t *testing.T) {
				var source, raw strings.Builder
				source.WriteString("A:\n")
				for i, row := range fixture.rows {
					source.WriteString("  |")
					source.WriteString(row)
					if !eof || i < len(fixture.rows)-1 {
						source.WriteByte('\n')
					}
					if i > 0 {
						raw.WriteByte('\n')
					}
					raw.WriteString(row)
				}
				want := []Entry{parserBlockValue(1, []string{"A"}, raw.String(), PipeBlock)}
				if !eof {
					source.WriteString("B:next")
					want = append(want, parserValue(len(fixture.rows)+2, []string{"B"}, "next"))
				}
				got, err := Parse(strings.NewReader(parserDocument(source.String())))
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("owned spill changed bytes or lookahead: %v", err)
				}
			})
		}
	}
}

func TestParsePipeAssemblyKeepsOwnedEntriesAcrossRefills(t *testing.T) {
	var source strings.Builder
	var want []Entry
	physicalLine := 1
	for _, rows := range []int{1, 8, 9, 257} {
		opener := physicalLine
		source.WriteString("A:\r\n")
		physicalLine++
		var expected strings.Builder
		previousEnding := ""
		for i := 0; i < rows; i++ {
			text := "body\\u{1F600}"
			margin := 1
			if i%7 == 1 {
				text = ""
			} else if i%7 == 2 {
				text = strings.Repeat("long ", 2048)
			} else if i%7 == 3 {
				margin = 8193
			}
			ending := "\n"
			if i%2 == 0 {
				ending = "\r\n"
			}
			source.WriteString(strings.Repeat(" ", margin))
			source.WriteByte('|')
			source.WriteString(text)
			source.WriteString(ending)
			if i > 0 {
				expected.WriteString(previousEnding)
			}
			expected.WriteString(text)
			previousEnding = ending
			physicalLine++
		}
		want = append(want, parserBlockValue(opener, []string{"A"}, expected.String(), PipeBlock))
	}
	source.WriteString("B:" + strings.Repeat("next", 8192))
	want = append(want, parserValue(physicalLine, []string{"B"}, strings.Repeat("next", 8192)))
	for _, fragmented := range []bool{false, true} {
		var reader io.Reader = strings.NewReader(parserDocument(source.String()))
		if fragmented {
			reader = iotest.OneByteReader(reader)
		}
		var retained []Entry
		err := ParseEach(reader, func(entry Entry) error {
			retained = append(retained, entry)
			return nil
		})
		if err != nil || !reflect.DeepEqual(retained, want) {
			t.Fatalf("pipe result changed after later buffer refills (fragmented=%v): %v", fragmented, err)
		}
		for _, entry := range retained {
			if _, err := DecodeString(entry); err != nil {
				t.Fatalf("retained entry failed string decoding: %v", err)
			}
		}
	}
}

func TestParsePipeAssemblyDefersBoundaryErrorsAndStopsAtVisitor(t *testing.T) {
	stop := errors.New("visitor stop")
	for _, boundary := range []string{
		"B:\xff\n",
		"B:" + strings.Repeat("private-value", 1024) + "\xff\n",
		"B:" + strings.Repeat("private-value", 1024) + "\rdata\n",
	} {
		for _, stopAtBody := range []bool{false, true} {
			visits := 0
			err := ParseEach(strings.NewReader(parserDocument("A:\n |literal\n"+boundary)), func(entry Entry) error {
				visits++
				if entry.Raw != "literal" || entry.Form != PipeBlock {
					t.Fatalf("boundary replaced completed pipe value: %#v", entry)
				}
				if stopAtBody {
					return stop
				}
				return nil
			})
			if stopAtBody {
				if err != stop {
					t.Fatalf("boundary error preempted visitor stop: %v", err)
				}
			} else {
				parserRequireCode(t, err, ErrSyntax, 5)
			}
			if visits != 1 || strings.Contains(err.Error(), "private") {
				t.Fatalf("unexpected callback count or private text in error: visits=%d error=%v", visits, err)
			}
		}
	}
}

func TestParsePipeAssemblyKeepsBodyReaderFailurePriority(t *testing.T) {
	for _, body := range []string{" |short", " |" + strings.Repeat("private", 2048)} {
		for _, suffix := range []string{"", "\xff", "\rdata"} {
			reader := &refactorTailErrorReader{data: "A:\n" + body + suffix, err: errors.New("private read error")}
			visits := 0
			err := ParseEach(io.MultiReader(strings.NewReader(parserPreamble), reader), func(Entry) error {
				visits++
				return nil
			})
			parserRequireCode(t, err, ErrInput, 4)
			if visits != 0 || strings.Contains(err.Error(), "private") {
				t.Fatalf("body read failure lost priority or leaked text: visits=%d error=%v", visits, err)
			}
		}
	}
}
