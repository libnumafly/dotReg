package dotreg

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"unicode/utf8"
)

// bodyLine may borrow bufio's storage until the next read. Only pipe assembly
// consumes it directly; a lookahead line becomes owned before it is replayed.
type bodyLine struct {
	text      []byte
	ownedText string
	ending    string
	number    int
	eof       bool
	readErr   error
}

func (lines *sourceLines) nextBodyLine() bodyLine {
	if lines.hasPending {
		line := lines.next()
		return bodyLine{ownedText: line.text, ending: line.ending, number: line.number, eof: line.eof, readErr: line.readErr}
	}
	line := readBodyLine(lines.reader, lines.nextLine)
	lines.nextLine++
	return line
}

// Like readPhysicalLine, this has no format-specific line limit. Only lines
// exceeding bufio's buffer need spill storage. Copy full fragments first, then
// combine once, so an arbitrarily long line does not repeatedly copy a growing
// prefix. The usual short line remains borrowed throughout pipe assembly.
func readBodyLine(reader *bufio.Reader, number int) bodyLine {
	text, err := reader.ReadSlice('\n')
	var owned string
	if err == bufio.ErrBufferFull {
		var fragments [][]byte
		size := 0
		for err == bufio.ErrBufferFull {
			fragment := append([]byte(nil), text...)
			fragments = append(fragments, fragment)
			size += len(fragment)
			text, err = reader.ReadSlice('\n')
		}
		var complete strings.Builder
		complete.Grow(size + len(text))
		for _, fragment := range fragments {
			complete.Write(fragment)
		}
		complete.Write(text)
		owned, text = complete.String(), nil
	}
	line := bodyLine{number: number, eof: err == io.EOF}
	if err != nil && err != io.EOF {
		line.readErr = problem(number, ErrInput)
	}
	if owned != "" {
		if owned[len(owned)-1] == '\n' {
			owned, line.ending = owned[:len(owned)-1], "\n"
			if len(owned) > 0 && owned[len(owned)-1] == '\r' {
				owned, line.ending = owned[:len(owned)-1], "\r\n"
			}
		}
		if number == 1 {
			owned = strings.TrimPrefix(owned, bom)
		}
	} else if len(text) > 0 && text[len(text)-1] == '\n' {
		text, line.ending = text[:len(text)-1], "\n"
		if len(text) > 0 && text[len(text)-1] == '\r' {
			text, line.ending = text[:len(text)-1], "\r\n"
		}
	}
	if number == 1 {
		text = bytes.TrimPrefix(text, []byte(bom))
	}
	line.text, line.ownedText = text, owned
	return line
}

func (line bodyLine) owned() sourceLine {
	text := line.ownedText
	if text == "" {
		text = string(line.text)
	}
	return sourceLine{text, line.ending, line.number, line.eof, line.readErr}
}

func (line bodyLine) validate() error {
	if line.readErr != nil {
		return line.readErr
	}
	valid := utf8.Valid(line.text) && bytes.IndexByte(line.text, '\r') < 0
	if line.ownedText != "" {
		valid = utf8.ValidString(line.ownedText) && strings.IndexByte(line.ownedText, '\r') < 0
	}
	if !valid {
		return problem(line.number, ErrSyntax)
	}
	return nil
}

func (line bodyLine) size() int { return len(line.text) + len(line.ownedText) }

func (line bodyLine) pipePrefix(indent int) (int, bool) {
	if line.ownedText != "" {
		prefix := leadingSpaces(line.ownedText)
		return prefix, prefix > indent && prefix < len(line.ownedText) && line.ownedText[prefix] == '|'
	}
	prefix := 0
	for prefix < len(line.text) && line.text[prefix] == ' ' {
		prefix++
	}
	return prefix, prefix > indent && prefix < len(line.text) && line.text[prefix] == '|'
}

func (line bodyLine) copyTail(destination []byte, start int) int {
	if line.ownedText != "" {
		return copy(destination, line.ownedText[start:])
	}
	return copy(destination, line.text[start:])
}

func (line bodyLine) ownTail(start int) string {
	if line.ownedText != "" {
		return strings.Clone(line.ownedText[start:])
	}
	return string(line.text[start:])
}

func (line bodyLine) writeTail(destination *strings.Builder, start int) {
	if line.ownedText != "" {
		destination.WriteString(line.ownedText[start:])
		return
	}
	destination.Write(line.text[start:])
}
