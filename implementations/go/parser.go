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

const bom = "\xef\xbb\xbf"

type Kind string

const (
	Value   Kind = "value"
	Heading Kind = "heading"
)

// Entry is a syntax result. Raw still contains any quotes and escapes.
// Path holds decoded elements; joining them with dots loses information.
type Entry struct {
	Line    int      `json:"line"`
	Kind    Kind     `json:"kind"`
	Path    []string `json:"path"`
	Raw     string   `json:"raw"`
	Unset   bool     `json:"unset"`
	Type    string   `json:"type,omitempty"`
	HasType bool     `json:"has_type"`
}

// Parse collects syntax results. It returns no partial result on failure.
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

// ParseEach visits headings and values synchronously in source order.
// A visitor error is returned unchanged, before parsing another line.
// The reader may buffer bytes ahead. Each call starts with fresh scopes/keys.
func ParseEach(r io.Reader, visit func(Entry) error) error {
	if nilReader(r) || visit == nil {
		return problem(0, ErrInput)
	}
	reader := bufio.NewReader(r)
	var scopes []*pathNode
	seen := make(map[string]struct{})
	for line := 1; ; line++ {
		source, eof, err := physicalLine(reader, line)
		if err != nil {
			return err
		}
		trimmed := strings.Trim(source, " \t")
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			indent := len(source) - len(strings.TrimLeft(source, " "))
			body := strings.TrimRight(source[indent:], " \t")
			if body[0] == '\t' {
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
				if indent > len(scopes) {
					return problem(line, ErrSyntax)
				}
				if !strings.HasPrefix(key, ".") {
					return problem(line, ErrSyntax)
				}
				// Even a value line closes deeper scopes when it dedents.
				scopes = scopes[:indent]
				parent = scopes[indent-1]
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
				id := pathID(e.Path)
				if _, exists := seen[id]; exists {
					return problem(line, ErrDuplicate)
				}
				seen[id] = struct{}{}
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

// physicalLine reads one whole line without a format-specific size limit.
// It removes CRLF and the one initial BOM before validating the source.
func physicalLine(r *bufio.Reader, line int) (string, bool, error) {
	var data []byte
	eof := false
	for {
		part, err := r.ReadSlice('\n')
		data = append(data, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && err != io.EOF {
			return "", false, problem(line, ErrInput)
		}
		eof = err == io.EOF
		break
	}
	s := string(data)
	if strings.HasSuffix(s, "\n") {
		s = strings.TrimSuffix(s, "\n")
		s = strings.TrimSuffix(s, "\r")
	}
	if line == 1 {
		s = strings.TrimPrefix(s, bom)
	}
	if !utf8.ValidString(s) || strings.ContainsRune(s, '\r') {
		return "", false, problem(line, ErrSyntax)
	}
	return s, eof, nil
}

// splitLine stops at the delimiter. It never scans or decodes the right side.
func splitLine(body string, line int) (Entry, string, error) {
	e := Entry{Line: line}
	for i := 0; i < len(body); {
		switch body[i] {
		case '"':
			_, end, err := decodeQuoted(body[i:])
			if err != nil {
				return Entry{}, "", problem(line, ErrSyntax)
			}
			i += end
		case ':':
			e.Kind = Value
			e.Raw = strings.Trim(body[i+1:], " \t")
			e.Unset = e.Raw == ""
			return e, strings.Trim(body[:i], " \t"), nil
		case '(':
			key := strings.TrimRight(body[:i], " ")
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
			if tail == "." {
				return Entry{}, "", problem(line, ErrSyntax)
			}
			if strings.HasPrefix(tail, "\t") {
				return Entry{}, "", problem(line, ErrUnsupported)
			}
			if tail == "" || tail[0] != ':' {
				return Entry{}, "", problem(line, ErrSyntax)
			}
			e.Kind, e.HasType, e.Type = Value, true, name
			e.Raw = strings.Trim(tail[1:], " \t")
			e.Unset = e.Raw == ""
			return e, key, nil
		default:
			i++
		}
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
		if unicode.IsSpace(c) {
			return 0, problem(line, ErrSyntax)
		}
		switch c {
		case '"', '\\':
			return 0, problem(line, ErrUnsupported)
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return open + 1 + offset, nil
			}
		}
	}
	return 0, problem(line, ErrSyntax)
}

func parsePath(key string, line int) ([]string, error) {
	if key == "" {
		return nil, problem(line, ErrSyntax)
	}
	var path []string
	for {
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
		if key == "" {
			return nil, problem(line, ErrSyntax)
		}
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
	for _, s := range path {
		b.WriteString(strconv.Itoa(len(s)))
		b.WriteByte(':')
		b.WriteString(s)
	}
	return b.String()
}
