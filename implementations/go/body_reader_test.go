package dotreg

import (
	"bufio"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

// The pipe-only reader must preserve every property of ordinary physical-line
// reads, including fragment boundaries and errors returned with the last data.
func TestBodyReaderMatchesPhysicalLines(t *testing.T) {
	for _, source := range []string{
		"", "\ufeffA\n\r\nB", "\ufeff", "A\rB\n\xff\n",
		strings.Repeat("x", 15) + "\r\nnext\n",
		strings.Repeat("x", 16) + "\r\nnext\n",
		strings.Repeat("x", 17) + "\r\nnext\n",
		strings.Repeat("long ", 2048) + "\r\n" + strings.Repeat(" ", 8193),
		strings.Repeat("x", 4095) + "\r\nlast",
		strings.Repeat("x", 4096) + "\nlast\r\n",
		strings.Repeat("x", 4097) + "\nlast",
	} {
		for _, fragmented := range []bool{false, true} {
			for _, failed := range []bool{false, true} {
				makeReader := func() io.Reader {
					var reader io.Reader = strings.NewReader(source)
					if failed {
						reader = &refactorTailErrorReader{data: source, err: errors.New("private failure")}
					}
					if fragmented {
						reader = iotest.OneByteReader(reader)
					}
					return reader
				}
				ordinary := bufio.NewReaderSize(makeReader(), 16)
				borrowed := bufio.NewReaderSize(makeReader(), 16)
				for number := 1; ; number++ {
					want := readPhysicalLine(ordinary, number)
					line := readBodyLine(borrowed, number)
					got := line.owned()
					if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(line.validate(), want.validate()) {
						t.Fatalf("physical line differs at %d (fragmented=%v failed=%v)", number, fragmented, failed)
					}
					if want.eof || want.readErr != nil {
						break
					}
				}
			}
		}
	}
}

func TestBodyReaderReplaysPendingLineBeforeReading(t *testing.T) {
	lines := sourceLines{reader: bufio.NewReader(strings.NewReader("next\n")), nextLine: 11}
	want := sourceLine{text: "pending", ending: "\r\n", number: 10, readErr: problem(10, ErrInput)}
	lines.unread(want)
	if got := lines.nextBodyLine().owned(); !reflect.DeepEqual(got, want) || lines.nextLine != 11 {
		t.Fatal("pending line changed or advanced the physical reader")
	}
	if got := lines.nextBodyLine().owned(); got.text != "next" || got.number != 11 || lines.nextLine != 12 {
		t.Fatal("physical numbering changed after replay")
	}
}
