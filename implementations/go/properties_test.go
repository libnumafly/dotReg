package dotreg

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestPropertiesAcceptedDocuments(t *testing.T) {
	for _, test := range []struct {
		name, source string
		want         []Entry
	}{
		{"empty body", "@dotReg 0.1\n\n", []Entry{}},
		{"blank separator without final EOL", "@dotReg 0.1\n \t", []Entry{}},
		{"body", "@dotReg 0.1\n\nA:one", []Entry{
			{Line: 3, Kind: Value, Path: []string{"A"}, Raw: "one"},
		}},
		{"BOM CRLF and alignment", bom + "@dotReg\t 0.1 \t\r\n \t\r\nA:\"two\"", []Entry{
			{Line: 3, Kind: Value, Path: []string{"A"}, Raw: "\"two\""},
		}},
		{"repeated value keys", "@dotReg 0.1\n\nA.Tag:red\nA.Tag:green\nA.Tag:red", []Entry{
			{Line: 3, Kind: Value, Path: []string{"A", "Tag"}, Raw: "red"},
			{Line: 4, Kind: Value, Path: []string{"A", "Tag"}, Raw: "green"},
			{Line: 5, Kind: Value, Path: []string{"A", "Tag"}, Raw: "red"},
		}},
		{"ordinary at-sign key", "@dotReg 0.1\n\n@dotReg:1", []Entry{
			{Line: 3, Kind: Value, Path: []string{"@dotReg"}, Raw: "1"},
		}},
		{"literal value", "@dotReg 0.1\n\nA:@dotReg 0.1", []Entry{
			{Line: 3, Kind: Value, Path: []string{"A"}, Raw: "@dotReg 0.1"},
		}},
		{"pipe data", "@dotReg 0.1\n\nA:\n |@dotReg 0.1", []Entry{
			{Line: 3, Kind: Value, Path: []string{"A"}, Raw: "@dotReg 0.1", Form: PipeBlock},
		}},
		{"triple data", "@dotReg 0.1\n\nA:\"\"\"\n@dotReg 0.1\n\"\"\"", []Entry{
			{Line: 3, Kind: Value, Path: []string{"A"}, Raw: "@dotReg 0.1", Form: TripleQuoted},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(test.source))
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Parse = %#v, %v; want %#v", got, err, test.want)
			}
			visited := make([]Entry, 0)
			err = ParseEach(strings.NewReader(test.source), func(entry Entry) error {
				visited = append(visited, entry)
				return nil
			})
			if err != nil || !reflect.DeepEqual(visited, test.want) {
				t.Fatalf("ParseEach = %#v, %v; want %#v", visited, err, test.want)
			}
		})
	}
}

func TestPropertiesRejectInvalidDocumentsBeforeNotification(t *testing.T) {
	for _, test := range []struct {
		name, source string
		code         Code
		line         int
	}{
		{"empty input", "", ErrSyntax, 1},
		{"BOM only", bom, ErrSyntax, 1},
		{"blank first", "\n@dotReg 0.1\n\n", ErrSyntax, 1},
		{"comment first", "#comment\n@dotReg 0.1\n\n", ErrSyntax, 1},
		{"headerless body", "A:value", ErrSyntax, 1},
		{"indented declaration", " @dotReg 0.1\n\n", ErrSyntax, 1},
		{"tab indented declaration", "\t@dotReg 0.1\n\n", ErrSyntax, 1},
		{"missing separator", "@dotReg1\n\n", ErrSyntax, 1},
		{"missing value", "@dotReg \t\n\n", ErrSyntax, 1},
		{"unknown property", "@private value\n\n", ErrUnsupported, 1},
		{"case sensitive property", "@dotreg 1\n\n", ErrUnsupported, 1},
		{"unsupported version", "@dotReg 2\n\nA:\xff", ErrUnsupported, 1},
		{"version is not a number", "@dotReg 01\n\n", ErrUnsupported, 1},
		{"version is not quoted", "@dotReg \"0.1\"\n\n", ErrUnsupported, 1},
		{"no inline comment", "@dotReg 0.1 #private\n\n", ErrUnsupported, 1},
		{"no unicode separator", "@dotReg\u00a01\n\n", ErrSyntax, 1},
		{"EOF after declaration", "@dotReg 0.1", ErrSyntax, 2},
		{"no physical blank line", "@dotReg 0.1\n", ErrSyntax, 2},
		{"body before blank", "@dotReg 0.1\nA:value", ErrSyntax, 2},
		{"comment inside region", "@dotReg 0.1\n#private\n\n", ErrSyntax, 2},
		{"unknown later property", "@dotReg 0.1\n@private value\n\n", ErrUnsupported, 2},
		{"duplicate declaration", "@dotReg 0.1\n@dotReg 0.1\n\n", ErrDuplicate, 2},
		{"invalid UTF8 in declaration", "@dotReg \xff\n\n", ErrSyntax, 1},
		{"invalid UTF8 in separator", "@dotReg 0.1\n\xff\n", ErrSyntax, 2},
		{"lone CR in separator", "@dotReg 0.1\n \r", ErrSyntax, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			visits := 0
			err := ParseEach(strings.NewReader(test.source), func(Entry) error {
				visits++
				return nil
			})
			requirePropertyError(t, err, test.code, test.line)
			if visits != 0 {
				t.Fatalf("invalid properties notified %d entries", visits)
			}
			got, err := Parse(strings.NewReader(test.source))
			requirePropertyError(t, err, test.code, test.line)
			if got != nil {
				t.Fatalf("invalid properties returned partial results: %#v", got)
			}
		})
	}
}

func TestPropertyDeclarationAfterSeparatorIsOrdinarySyntaxFailure(t *testing.T) {
	var got []Entry
	err := ParseEach(strings.NewReader("@dotReg 0.1\n\nA:one\n@dotReg 0.1"), func(entry Entry) error {
		got = append(got, entry)
		return nil
	})
	requirePropertyError(t, err, ErrSyntax, 4)
	if len(got) != 1 || got[0].Line != 3 || got[0].Raw != "one" {
		t.Fatalf("body notification order changed: %#v", got)
	}
}

type propertiesTailReader struct {
	data  string
	tail  error
	done  bool
	calls int
}

func (reader *propertiesTailReader) Read(buffer []byte) (int, error) {
	reader.calls++
	if reader.done {
		return 0, errors.New("private read after end")
	}
	n := copy(buffer, reader.data)
	reader.data = reader.data[n:]
	if reader.data == "" {
		reader.done = true
		return n, reader.tail
	}
	return n, nil
}

func TestPropertiesPreserveReadErrorsAndStopAtEOF(t *testing.T) {
	private := errors.New("private input failure")
	for _, test := range []struct {
		source string
		tail   error
		code   Code
		line   int
	}{
		{"@dotReg 0.1\n \t", io.EOF, "", 0},
		{"@dotReg 0.1\n\n", io.EOF, "", 0},
		{"@dotReg 0.1", io.EOF, ErrSyntax, 2},
		{"@dotReg 0.1", private, ErrInput, 1},
		{"@dotReg 0.1\n\n", private, ErrInput, 3},
		{"@dotReg 2\n\n", private, ErrUnsupported, 1},
	} {
		reader := &propertiesTailReader{data: test.source, tail: test.tail}
		got, err := Parse(reader)
		if test.code == "" {
			if err != nil || len(got) != 0 {
				t.Fatalf("empty document = %#v, %v", got, err)
			}
		} else {
			requirePropertyError(t, err, test.code, test.line)
			if got != nil {
				t.Fatalf("input failure returned partial results: %#v", got)
			}
		}
		if reader.calls != 1 {
			t.Fatalf("read beyond final data/error: calls=%d, source=%q", reader.calls, test.source)
		}
	}
}

func requirePropertyError(t *testing.T, err error, code Code, line int) {
	t.Helper()
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != code || diagnostic.Line != line {
		t.Fatalf("error = %v; want %s at physical line %d", err, code, line)
	}
	if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "@dotReg") {
		t.Fatalf("property diagnostic exposed source text: %v", err)
	}
}
