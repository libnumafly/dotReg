package dotreg

import (
	"bufio"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	bom         = "\xef\xbb\xbf"
	indentWidth = 2
)

type Kind string

const (
	Value   Kind = "value"
	Heading Kind = "heading"
)

// ValueForm identifies a multiline value. The zero value is single-line syntax.
type ValueForm string

const (
	TripleQuoted ValueForm = "triple_quoted"
	PipeBlock    ValueForm = "pipe"
)

// Entry is a syntax result. Raw preserves undecoded quotes and escapes.
// For multiline forms, Raw excludes markers and their indentation margins,
// retaining the original line endings between body lines. Line is the opener.
// Path holds decoded elements; joining them with dots loses information.
type Entry struct {
	Line    int       `json:"line"`
	Kind    Kind      `json:"kind"`
	Path    []string  `json:"path"`
	Raw     string    `json:"raw"`
	Unset   bool      `json:"unset"`
	Type    string    `json:"type,omitempty"`
	HasType bool      `json:"has_type"`
	Form    ValueForm `json:"form,omitempty"`
}

// Parse validates file properties and collects body entries without partial
// results on failure.
// Call DecodeString or Build before delivering settings to an application.
func Parse(r io.Reader) ([]Entry, error) {
	entries := make([]Entry, 0)
	err := ParseEach(r, func(e Entry) error {
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// ParseEach visits headings and every value occurrence in source order.
// Required file properties and their blank separator are checked first.
// Multiline bodies are consumed before their value is visited. A visitor error
// is returned unchanged, before validating or parsing the next logical entry.
// The reader may buffer bytes ahead. Each call starts with fresh scopes.
func ParseEach(r io.Reader, visit func(Entry) error) error {
	if nilReader(r) || visit == nil {
		return problem(0, ErrInput)
	}
	lines := sourceLines{reader: bufio.NewReader(r), nextLine: 1}
	if eof, err := lines.properties(); err != nil {
		return err
	} else if eof {
		return nil
	}
	var scopes []*pathNode
	for {
		physical := lines.next()
		if err := physical.validate(); err != nil {
			return err
		}
		source, eof, line := physical.text, physical.eof, physical.number
		trimmed := strings.Trim(source, " \t")
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			indent := leadingSpaces(source)
			body := strings.TrimRight(source[indent:], " \t")
			if body[0] == '\t' || indent%indentWidth != 0 {
				return problem(line, ErrSyntax)
			}
			relative := strings.HasPrefix(body, ".")
			if relative != (indent > 0) {
				return problem(line, ErrSyntax)
			}
			e, key, err := splitLine(body, line)
			if err != nil {
				return err
			}
			var parent *pathNode
			if relative {
				depth := indent / indentWidth
				if depth > len(scopes) {
					return problem(line, ErrSyntax)
				}
				if !strings.HasPrefix(key, ".") {
					return problem(line, ErrSyntax)
				}
				// Even a value line closes deeper scopes when it dedents.
				scopes = scopes[:depth]
				parent = scopes[depth-1]
				key = key[1:] // The relative marker, not a key element.
			} else {
				scopes = nil
			}
			var suffix []string
			self := relative && key == "" && e.Kind == Value
			if !self {
				suffix, err = parsePath(key, line)
				if err != nil {
					return err
				}
			}
			node := parent
			if !self {
				node = extendPath(parent, suffix)
			}
			e.Path = node.elements()
			if e.Kind == Heading {
				// Store a persistent path, separate from the visitor's slice.
				scopes = append(scopes, node)
			} else {
				eof, err = lines.valueBody(&e, indent, eof)
				if err != nil {
					return err
				}
			}
			if err := visit(e); err != nil {
				return err
			}
		}
		if eof {
			return nil
		}
	}
}

func nilReader(r io.Reader) bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(r)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// sourceLines keeps one unvalidated lookahead line. A non-pipe line's errors
// belong to its later turn, after the preceding value's visitor has completed.
type sourceLines struct {
	reader     *bufio.Reader
	nextLine   int
	pending    sourceLine
	hasPending bool
}

type sourceLine struct {
	text    string
	ending  string
	number  int
	eof     bool
	readErr error
}

func (lines *sourceLines) next() sourceLine {
	if lines.hasPending {
		line := lines.pending
		lines.pending = sourceLine{}
		lines.hasPending = false
		return line
	}
	line := readPhysicalLine(lines.reader, lines.nextLine)
	lines.nextLine++
	return line
}

func (lines *sourceLines) unread(line sourceLine) {
	lines.pending, lines.hasPending = line, true
}

// readPhysicalLine has no format-specific size limit. Source validation is
// deferred, but the original LF or CRLF is retained separately from the text.
func readPhysicalLine(r *bufio.Reader, number int) sourceLine {
	s, err := r.ReadString('\n')
	line := sourceLine{number: number, eof: err == io.EOF}
	if err != nil && err != io.EOF {
		line.readErr = problem(number, ErrInput)
	}
	if strings.HasSuffix(s, "\n") {
		s, line.ending = s[:len(s)-1], "\n"
		if strings.HasSuffix(s, "\r") {
			s, line.ending = s[:len(s)-1], "\r\n"
		}
	}
	if number == 1 {
		s = strings.TrimPrefix(s, bom)
	}
	line.text = s
	return line
}

func (line sourceLine) validate() error {
	if line.readErr != nil {
		return line.readErr
	}
	if !utf8.ValidString(line.text) || strings.ContainsRune(line.text, '\r') {
		return problem(line.number, ErrSyntax)
	}
	return nil
}

// properties consumes the required declaration region and its blank separator.
// It does not emit entries or inspect the body. Only @dotReg 1 is supported.
func (lines *sourceLines) properties() (bool, error) {
	declared := false
	for {
		line := lines.next()
		if err := line.validate(); err != nil {
			return false, err
		}
		text := strings.TrimRight(line.text, " \t")
		if text == "" {
			// An empty read at EOF is not a physical blank separator line.
			if !declared || (line.text == "" && line.ending == "") {
				return false, problem(line.number, ErrSyntax)
			}
			return line.eof, nil
		}
		separator := strings.IndexAny(text, " \t")
		if separator <= 0 || text[0] != '@' {
			return false, problem(line.number, ErrSyntax)
		}
		name, value := text[:separator], strings.TrimLeft(text[separator:], " \t")
		if value == "" {
			return false, problem(line.number, ErrSyntax)
		}
		if name != "@dotReg" {
			return false, problem(line.number, ErrUnsupported)
		}
		if declared {
			return false, problem(line.number, ErrDuplicate)
		}
		if value != "1" {
			return false, problem(line.number, ErrUnsupported)
		}
		declared = true
		if line.eof {
			return false, problem(line.number+1, ErrSyntax)
		}
	}
}

// valueBody completes a value before notification and returns its EOF state.
// A non-pipe lookahead stays pending; it is parsed only after notification.
func (lines *sourceLines) valueBody(entry *Entry, indent int, eof bool) (bool, error) {
	switch {
	case entry.Raw == `"""`:
		raw, blockEOF, err := lines.tripleBody(entry.Line)
		if err != nil {
			return false, err
		}
		entry.Raw, entry.Form, entry.Unset = raw, TripleQuoted, false
		return blockEOF, nil
	case entry.Unset && !eof:
		raw, blockEOF, found, err := lines.pipeBody(indent)
		if err != nil {
			return false, err
		}
		if found {
			entry.Raw, entry.Form, entry.Unset = raw, PipeBlock, false
			return blockEOF, nil
		}
	}
	return eof, nil
}

func (lines *sourceLines) tripleBody(opening int) (string, bool, error) {
	var body []struct{ text, ending string }
	for {
		line := lines.next()
		if err := line.validate(); err != nil {
			return "", false, err
		}
		margin := leadingSpaces(line.text)
		if strings.TrimRight(line.text[margin:], " \t") == `"""` {
			var raw strings.Builder
			for i, part := range body {
				text := part.text
				if text != "" {
					if leadingSpaces(text) < margin {
						return "", false, problem(opening+i+1, ErrSyntax)
					}
					text = text[margin:]
				}
				if i > 0 {
					raw.WriteString(body[i-1].ending)
				}
				raw.WriteString(text)
			}
			return raw.String(), line.eof, nil
		}
		if line.eof {
			return "", false, problem(opening, ErrSyntax)
		}
		body = append(body, struct{ text, ending string }{line.text, line.ending})
	}
}

func (lines *sourceLines) pipeBody(indent int) (string, bool, bool, error) {
	var raw strings.Builder
	var previousEnding string
	found := false
	for {
		line := lines.next()
		prefix := leadingSpaces(line.text)
		if prefix <= indent || prefix == len(line.text) || line.text[prefix] != '|' {
			lines.unread(line)
			return raw.String(), false, found, nil
		}
		if err := line.validate(); err != nil {
			return "", false, false, err
		}
		if found {
			raw.WriteString(previousEnding)
		}
		raw.WriteString(line.text[prefix+1:])
		found, previousEnding = true, line.ending
		if line.eof {
			return raw.String(), true, true, nil
		}
	}
}

func leadingSpaces(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}

// splitLine stops at the delimiter. It never scans or decodes the right side.
func splitLine(body string, line int) (Entry, string, error) {
	e := Entry{Line: line}
	var key, rhs string
	for i := 0; i < len(body); {
		switch body[i] {
		case '"':
			end, err := scanQuoted(body[i:], nil)
			if err != nil {
				return Entry{}, "", problem(line, ErrSyntax)
			}
			i += end
			continue
		case ':':
			key, rhs = strings.Trim(body[:i], " \t"), body[i+1:]
		case '(':
			key = strings.TrimRight(body[:i], " ")
			if strings.HasSuffix(key, "\t") {
				return Entry{}, "", problem(line, ErrUnsupported)
			}
			close, err := typeEnd(body, i, line)
			if err != nil {
				return Entry{}, "", err
			}
			name := body[i+1 : close]
			if name == "" {
				return Entry{}, "", problem(line, ErrUnsupported)
			}
			tail := strings.TrimLeft(body[close+1:], " ")
			if strings.HasPrefix(tail, "\t") {
				return Entry{}, "", problem(line, ErrUnsupported)
			}
			if tail == "" || tail[0] != ':' {
				return Entry{}, "", problem(line, ErrSyntax)
			}
			e.HasType, e.Type, rhs = true, name, tail[1:]
		default:
			i++
			continue
		}
		e.Kind, e.Raw = Value, strings.Trim(rhs, " \t")
		e.Unset = e.Raw == ""
		return e, key, nil
	}
	if strings.HasSuffix(body, ".") {
		e.Kind = Heading
		return e, body[:len(body)-1], nil
	}
	return Entry{}, "", problem(line, ErrSyntax)
}

// typeEnd locates the outer closing parenthesis without interpreting the type.
// Type syntax other than balanced parentheses and the space prohibition belongs
// to the selected profile. Quotes and escapes in types are not specified yet.
func typeEnd(body string, open, line int) (int, error) {
	depth := 1
	for offset, c := range body[open+1:] {
		switch {
		case unicode.IsSpace(c):
			return 0, problem(line, ErrSyntax)
		case c == '"' || c == '\\':
			return 0, problem(line, ErrUnsupported)
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return open + 1 + offset, nil
			}
		}
	}
	return 0, problem(line, ErrSyntax)
}

func parsePath(key string, line int) ([]string, error) {
	var path []string
	for {
		if key == "" {
			return nil, problem(line, ErrSyntax)
		}
		var element string
		if key[0] == '"' {
			text, end, err := decodeQuoted(key)
			if err != nil {
				return nil, problem(line, ErrSyntax)
			}
			element, key = text, key[end:]
			if key != "" && key[0] != '.' {
				return nil, problem(line, ErrSyntax)
			}
		} else {
			end := strings.IndexByte(key, '.')
			if end < 0 {
				end = len(key)
			}
			element, key = key[:end], key[end:]
			if element == "" {
				return nil, problem(line, ErrSyntax)
			}
			for _, c := range element {
				if unicode.IsSpace(c) || unicode.IsControl(c) || strings.ContainsRune(":()#\"\\", c) {
					return nil, problem(line, ErrSyntax)
				}
			}
		}
		path = append(path, element)
		if key == "" {
			return path, nil
		}
		key = key[1:] // An element separator outside quotes.
	}
}

// Nodes avoid storing a full copy of every ancestor path at every scope.
type pathNode struct {
	parent *pathNode
	suffix []string
	count  int
}

func extendPath(parent *pathNode, suffix []string) *pathNode {
	n := &pathNode{parent: parent, suffix: suffix, count: len(suffix)}
	if parent != nil {
		n.count += parent.count
	}
	return n
}

func (n *pathNode) elements() []string {
	path := make([]string, n.count)
	end := len(path)
	for current := n; current != nil; current = current.parent {
		start := end - len(current.suffix)
		copy(path[start:end], current.suffix)
		end = start
	}
	return path
}

func pathID(path []string) string {
	var b strings.Builder
	size := 2 * len(path) // At least one length digit and a colon per element.
	for _, s := range path {
		size += len(s)
	}
	b.Grow(size)
	for _, s := range path {
		b.WriteString(strconv.Itoa(len(s)))
		b.WriteByte(':')
		b.WriteString(s)
	}
	return b.String()
}
