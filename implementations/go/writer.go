package dotreg

import (
	"bytes"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// WriteValue is a decoded string value to append to a configuration document.
// HasType and Type retain an optional, opaque type expression; this package
// does not convert the value while writing it.
type WriteValue struct {
	Text    Text
	Type    string
	HasType bool
}

// AppendValue appends one value occurrence to a valid dotReg document while
// preserving all existing bytes. It adds the value at the end of the last
// matching heading section when one exists, using that heading's two-space
// indentation. Otherwise it appends a fully qualified key at end of file. Text
// is written as a quoted single-line value with escapes, or as an empty
// right-hand side when unset. This is a sample insertion policy, not a
// canonical serializer.
func AppendValue(source []byte, path []string, value WriteValue) ([]byte, error) {
	entries, err := Parse(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	return appendValue(source, entries, path, value)
}

// SetText replaces a key's only occurrence or appends an untyped value when
// the key is absent. Repeated keys are ambiguous and return ErrMultipleValues.
// An existing type annotation and key spelling are preserved when replacing.
func SetText(source []byte, path []string, text Text) ([]byte, error) {
	entries, err := Parse(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	matches := matchingValues(entries, path)
	switch len(matches) {
	case 0:
		return appendValue(source, entries, path, WriteValue{Text: text})
	case 1:
		return replaceText(source, matches[0], text)
	default:
		return nil, problem(matches[1].Line, ErrMultipleValues)
	}
}

// SetTextAt replaces one zero-based occurrence of a key and preserves its
// key spelling and type annotation. A missing key or occurrence returns
// ErrMissing.
func SetTextAt(source []byte, path []string, occurrence int, text Text) ([]byte, error) {
	entries, err := Parse(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	if len(path) == 0 || occurrence < 0 {
		return nil, problem(0, ErrMissing)
	}
	matches := matchingValues(entries, path)
	if occurrence >= len(matches) {
		return nil, problem(0, ErrMissing)
	}
	return replaceText(source, matches[occurrence], text)
}

func appendValue(source []byte, entries []Entry, path []string, value WriteValue) ([]byte, error) {
	if len(path) == 0 || (value.HasType && value.Type == "") || (!value.HasType && value.Type != "") || (value.Text.Unset && value.Text.Value != "") || !utf8.ValidString(value.Text.Value) {
		return nil, problem(0, ErrInput)
	}

	key, err := encodeKeyPath(path)
	if err != nil {
		return nil, err
	}
	line, err := encodeWriteLine(key, value)
	if err != nil {
		return nil, err
	}
	indent := 0
	insertAt := len(source)
	if heading, ok := deepestHeadingAncestor(entries, path); ok {
		lines := physicalSourceLines(source)
		headingLine := lines[heading.Line-1]
		indent = leadingSpaces(headingLine.text) + indentWidth
		remainder := line[len(key):]
		if len(path) == len(heading.Path) {
			line = "." + remainder
		} else {
			suffix, encodeErr := encodeKeyPath(path[len(heading.Path):])
			if encodeErr != nil {
				return nil, encodeErr
			}
			line = "." + suffix + remainder
		}
		insertAt = headingSectionEnd(entries, lines, heading)
	}
	if indent != 0 {
		line = strings.Repeat(" ", indent) + line
	}

	ending := lineEndingAt(source, insertAt)
	updated := insertSourceLine(source, insertAt, []byte(line), ending)
	if _, err := Parse(bytes.NewReader(updated)); err != nil {
		return nil, err
	}
	return updated, nil
}

func matchingValues(entries []Entry, path []string) []Entry {
	matches := make([]Entry, 0, 1)
	for _, entry := range entries {
		if entry.Kind == Value && samePathElements(entry.Path, path) {
			matches = append(matches, entry)
		}
	}
	return matches
}

func replaceText(source []byte, entry Entry, text Text) ([]byte, error) {
	if text.Unset && text.Value != "" || !utf8.ValidString(text.Value) {
		return nil, problem(0, ErrInput)
	}
	lines := physicalSourceLines(source)
	startLine := entry.Line - 1
	if startLine < 0 || startLine >= len(lines) {
		return nil, problem(entry.Line, ErrSyntax)
	}
	endLine, err := entryEndLine(lines, entry)
	if err != nil {
		return nil, err
	}
	separator := valueSeparator(lines[startLine].text)
	if separator < 0 {
		return nil, problem(entry.Line, ErrSyntax)
	}
	rhs := ""
	if !text.Unset {
		quoted, err := encodeQuotedValue(text.Value)
		if err != nil {
			return nil, err
		}
		rhs = " " + quoted
	}

	first := lines[startLine]
	last := lines[endLine]
	replacement := make([]byte, 0, separator+1+len(rhs)+last.end-last.contentEnd)
	replacement = append(replacement, source[first.start:first.start+separator+1]...)
	replacement = append(replacement, rhs...)
	replacement = append(replacement, source[last.contentEnd:last.end]...)
	updated := make([]byte, 0, len(source)-(last.end-first.start)+len(replacement))
	updated = append(updated, source[:first.start]...)
	updated = append(updated, replacement...)
	updated = append(updated, source[last.end:]...)
	if _, err := Parse(bytes.NewReader(updated)); err != nil {
		return nil, err
	}
	return updated, nil
}

func entryEndLine(lines []sourceLineSpan, entry Entry) (int, error) {
	startLine := entry.Line - 1
	if startLine < 0 || startLine >= len(lines) {
		return 0, problem(entry.Line, ErrSyntax)
	}
	endLine := startLine
	switch entry.Form {
	case "":
	case TripleQuoted:
		for i := startLine + 1; i < len(lines); i++ {
			if isTripleClosingLine(lines[i].text) {
				return i, nil
			}
		}
		return 0, problem(entry.Line, ErrSyntax)
	case PipeBlock:
		parentIndent := leadingSpaces(lines[startLine].text)
		for i := startLine + 1; i < len(lines); i++ {
			indent := leadingSpaces(lines[i].text)
			if indent <= parentIndent || indent == len(lines[i].text) || lines[i].text[indent] != '|' {
				break
			}
			endLine = i
		}
		if endLine == startLine {
			return 0, problem(entry.Line, ErrSyntax)
		}
	default:
		return 0, problem(entry.Line, ErrSyntax)
	}
	return endLine, nil
}

func samePathElements(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func valueSeparator(line string) int {
	quoted := false
	depth := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			if quoted {
				i++
			}
		case '"':
			quoted = !quoted
		case '(':
			if !quoted {
				depth++
			}
		case ')':
			if !quoted && depth > 0 {
				depth--
			}
		case ':':
			if !quoted && depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isTripleClosingLine(line string) bool {
	indent := leadingSpaces(line)
	return trimRightSpaceTab(line[indent:]) == `"""`
}

type sourceLineSpan struct {
	text       string
	start      int
	contentEnd int
	end        int
}

func physicalSourceLines(source []byte) []sourceLineSpan {
	lines := make([]sourceLineSpan, 0)
	for start := 0; start < len(source); {
		relativeEnd := bytes.IndexByte(source[start:], '\n')
		end := len(source)
		if relativeEnd >= 0 {
			end = start + relativeEnd + 1
		}
		contentEnd := end
		if end > start && source[end-1] == '\n' {
			contentEnd--
			if contentEnd > start && source[contentEnd-1] == '\r' {
				contentEnd--
			}
		}
		lines = append(lines, sourceLineSpan{
			text: string(source[start:contentEnd]), start: start,
			contentEnd: contentEnd, end: end,
		})
		start = end
	}
	return lines
}

func encodeWriteLine(key string, value WriteValue) (string, error) {
	if value.HasType {
		key += " (" + value.Type + ")"
	}
	line := key + ":"
	if value.Text.Unset {
		return line, nil
	}
	quoted, err := encodeQuotedValue(value.Text.Value)
	if err != nil {
		return "", err
	}
	return line + " " + quoted, nil
}

func encodeKeyPath(path []string) (string, error) {
	if len(path) == 0 {
		return "", problem(0, ErrInput)
	}
	var result strings.Builder
	for i, element := range path {
		if i > 0 {
			result.WriteByte('.')
		}
		encoded, err := encodeKeyElement(element)
		if err != nil {
			return "", err
		}
		result.WriteString(encoded)
	}
	return result.String(), nil
}

func encodeKeyElement(element string) (string, error) {
	if !utf8.ValidString(element) {
		return "", problem(0, ErrInput)
	}
	plain := element != ""
	for _, r := range element {
		if r == '.' || strings.ContainsRune(":()#\"\\", r) || unicode.IsSpace(r) || unicode.IsControl(r) {
			plain = false
			break
		}
	}
	if plain {
		return element, nil
	}

	var result strings.Builder
	result.WriteByte('"')
	for _, r := range element {
		writeQuotedRune(&result, r)
	}
	result.WriteByte('"')
	return result.String(), nil
}

func encodeQuotedValue(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", problem(0, ErrInput)
	}
	var result strings.Builder
	result.WriteByte('"')
	for _, r := range value {
		writeQuotedRune(&result, r)
	}
	result.WriteByte('"')
	return result.String(), nil
}

func writeQuotedRune(result *strings.Builder, r rune) {
	switch r {
	case '"':
		result.WriteString(`\"`)
	case '\\':
		result.WriteString(`\\`)
	case '\b':
		result.WriteString(`\b`)
	case '\f':
		result.WriteString(`\f`)
	case '\n':
		result.WriteString(`\n`)
	case '\r':
		result.WriteString(`\r`)
	case '\t':
		result.WriteString(`\t`)
	default:
		if unicode.IsControl(r) {
			result.WriteString(`\u{`)
			result.WriteString(strconv.FormatInt(int64(r), 16))
			result.WriteByte('}')
		} else {
			result.WriteRune(r)
		}
	}
}

func deepestHeadingAncestor(entries []Entry, path []string) (Entry, bool) {
	var best Entry
	found := false
	for _, entry := range entries {
		if entry.Kind != Heading || !pathHasPrefix(path, entry.Path) {
			continue
		}
		if !found || len(entry.Path) >= len(best.Path) {
			best, found = entry, true
		}
	}
	return best, found
}

func headingSectionEnd(entries []Entry, lines []sourceLineSpan, heading Entry) int {
	headingLine := heading.Line - 1
	if headingLine < 0 || headingLine >= len(lines) {
		return lines[len(lines)-1].end
	}
	indent := leadingSpaces(lines[headingLine].text)
	insertAt := lines[headingLine].end
	seen := false
	for _, entry := range entries {
		if !seen {
			if entry.Kind == Heading && entry.Line == heading.Line {
				seen = true
			}
			continue
		}
		lineIndex := entry.Line - 1
		if lineIndex < 0 || lineIndex >= len(lines) {
			break
		}
		if leadingSpaces(lines[lineIndex].text) <= indent {
			break
		}
		endLine, err := entryEndLine(lines, entry)
		if err != nil {
			break
		}
		insertAt = lines[endLine].end
	}
	return insertAt
}

func pathHasPrefix(path, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

func lineEndingAt(source []byte, offset int) string {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	for i := offset; i < len(source); i++ {
		if source[i] == '\n' {
			if i > 0 && source[i-1] == '\r' {
				return "\r\n"
			}
			return "\n"
		}
	}
	for i := offset - 1; i >= 0; i-- {
		if source[i] == '\n' {
			if i > 0 && source[i-1] == '\r' {
				return "\r\n"
			}
			return "\n"
		}
	}
	return "\n"
}

func insertSourceLine(source []byte, offset int, line []byte, ending string) []byte {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	hasPreviousLineBreak := offset == 0 || source[offset-1] == '\n'
	hasFollowingContent := offset < len(source)
	endsWithLineBreak := len(source) > 0 && source[len(source)-1] == '\n'
	needsFollowingLineBreak := hasFollowingContent || (offset == len(source) && endsWithLineBreak)
	capacity := len(source) + len(line) + 2*len(ending)
	result := make([]byte, 0, capacity)
	result = append(result, source[:offset]...)
	if !hasPreviousLineBreak {
		result = append(result, ending...)
	}
	result = append(result, line...)
	if needsFollowingLineBreak {
		result = append(result, ending...)
	}
	result = append(result, source[offset:]...)
	return result
}
