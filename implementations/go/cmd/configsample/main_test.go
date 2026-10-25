package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libnumafly/dotReg/implementations/go"
)

func cliDocument(source string) string {
	if strings.HasPrefix(source, "\ufeff") {
		return "\ufeff@dotReg 0.1\n\n" + strings.TrimPrefix(source, "\ufeff")
	}
	return "@dotReg 0.1\n\n" + source
}

func decodeCLIOutput(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var entries []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&entries); err != nil {
		t.Fatalf("invalid CLI JSON: %v; output=%q", err, data)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("unexpected trailing CLI output: %v", err)
	}
	return entries
}

func requireCLIErrorCode(t *testing.T, err error, code dotreg.Code) {
	t.Helper()
	var diagnostic *dotreg.Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != code {
		t.Fatalf("error = %v; want configuration code %q", err, code)
	}
}

func TestRunRequiredPropertiesHeaderIsCheckedInEveryMode(t *testing.T) {
	tests := []struct {
		name   string
		source string
		empty  bool
	}{
		{"LF", "@dotReg 0.1\n\nA: value", false},
		{"CRLF with BOM", "\ufeff@dotReg 0.1\r\n\r\nA: value", false},
		{"properties only", "@dotReg 0.1\n\n", true},
	}
	for _, test := range tests {
		for _, mode := range []string{"raw", "strings", "demo"} {
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				// Pass the complete document through the production run path.
				err := run([]string{"-mode", mode}, strings.NewReader(test.source), &stdout, &stderr)
				if err != nil || stderr.Len() != 0 {
					t.Fatalf("properties header: %v; stderr=%q", err, stderr.String())
				}
				got := decodeCLIOutput(t, stdout.Bytes())
				if test.empty {
					if len(got) != 0 {
						t.Errorf("properties generated entries: %#v", got)
					}
					return
				}
				if len(got) != 1 || got[0]["line"] != json.Number("3") || got[0]["raw"] != "value" {
					t.Fatalf("properties changed entries or physical lines: %#v", got)
				}
				path, ok := got[0]["path"].([]any)
				if !ok || len(path) != 1 || path[0] != "A" {
					t.Errorf("header was exposed as a key: %#v", got)
				}
			})
		}
	}
}

func TestRunRejectsInvalidPropertiesWithoutPartialJSON(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		code   dotreg.Code
	}{
		{"empty input", "", dotreg.ErrSyntax},
		{"missing declaration", "A: value", dotreg.ErrSyntax},
		{"initial blank without declaration", "\nA: value", dotreg.ErrSyntax},
		{"declaration without blank", "@dotReg 0.1", dotreg.ErrSyntax},
		{"line ending is not separator blank", "@dotReg 0.1\n", dotreg.ErrSyntax},
		{"body before separator blank", "@dotReg 0.1\nA: value", dotreg.ErrSyntax},
		{"comment before separator blank", "@dotReg 0.1\n# private-property-value\n\nA: value", dotreg.ErrSyntax},
		{"unknown property", "@Unknown private-property-value\n@dotReg 0.1\n\nA: value", dotreg.ErrUnsupported},
		{"property names are case sensitive", "@dotreg 1\n\nA: value", dotreg.ErrUnsupported},
		{"unsupported version", "@dotReg private-property-value\n\nA: value", dotreg.ErrUnsupported},
		{"duplicate declaration", "@dotReg 0.1\n@dotReg 0.1\n\nA: value", dotreg.ErrDuplicate},
		{"declaration has no value", "@dotReg\n\nA: value", dotreg.ErrSyntax},
		{"declaration in body", "@dotReg 0.1\n\nA: first\n@dotReg 0.1", dotreg.ErrSyntax},
	} {
		for _, mode := range []string{"raw", "strings", "demo"} {
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				err := run([]string{"-mode", mode}, strings.NewReader(test.source), &stdout, &stderr)
				requireCLIErrorCode(t, err, test.code)
				if stdout.Len() != 0 {
					t.Errorf("invalid properties emitted partial output: %q", stdout.String())
				}
				if strings.Contains(err.Error()+stderr.String(), "private-property-value") {
					t.Errorf("property diagnostic exposed its value: %v; stderr=%q", err, stderr.String())
				}
			})
		}
	}
}

func TestRunPropertiesNotationInBodyIsOrdinaryData(t *testing.T) {
	const source = "@dotReg 0.1\n\n@Name:value\nLiteral: @dotReg 0.1\n" +
		"Triple: \"\"\"\n@dotReg 0.1\n\"\"\"\nPipe:\n |@dotReg 0.1"
	wantNames := []string{"@Name", "Literal", "Triple", "Pipe"}
	wantLines := []json.Number{"3", "4", "5", "8"}
	wantValues := []string{"value", "@dotReg 0.1", "@dotReg 0.1", "@dotReg 0.1"}
	for _, mode := range []string{"raw", "strings", "demo"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run([]string{"-mode", mode}, strings.NewReader(source), &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			got := decodeCLIOutput(t, stdout.Bytes())
			if len(got) != len(wantNames) {
				t.Fatalf("properties-like body entry count = %d, want %d", len(got), len(wantNames))
			}
			for i, entry := range got {
				path, ok := entry["path"].([]any)
				if !ok || len(path) != 1 || path[0] != wantNames[i] || entry["line"] != wantLines[i] || entry["raw"] != wantValues[i] {
					t.Errorf("body entry %d was reinterpreted as a property: %#v", i, entry)
				}
				if mode == "strings" && entry["decoded"] != wantValues[i] {
					t.Errorf("body text changed during decoding: %#v", entry)
				}
				if mode == "demo" && entry["value"] != wantValues[i] {
					t.Errorf("body text changed during conversion: %#v", entry)
				}
			}
		})
	}
}

func TestRunOutputLayers(t *testing.T) {
	const source = "A:001\nB(int):001\nC: \"  spaced  \"\n"
	for _, mode := range []string{"raw", "strings", "demo"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{"-mode", mode}, strings.NewReader(cliDocument(source)), &stdout, &stderr)
			if err != nil || stderr.Len() != 0 {
				t.Fatalf("run: %v; stderr=%q", err, stderr.String())
			}
			got := decodeCLIOutput(t, stdout.Bytes())
			if len(got) != 3 {
				t.Fatalf("entry count = %d, want 3", len(got))
			}
			if got[0]["raw"] != "001" || got[1]["raw"] != "001" || got[2]["raw"] != "\"  spaced  \"" {
				t.Errorf("raw values were changed: %#v", got)
			}
			switch mode {
			case "raw":
				for _, entry := range got {
					if _, ok := entry["decoded"]; ok {
						t.Error("raw mode unexpectedly emitted decoded text")
					}
					if _, ok := entry["value"]; ok {
						t.Error("raw mode unexpectedly emitted a converted value")
					}
				}
			case "strings":
				if got[0]["decoded"] != "001" || got[1]["decoded"] != "001" || got[2]["decoded"] != "  spaced  " {
					t.Errorf("string layer output = %#v", got)
				}
				if _, ok := got[1]["value"]; ok {
					t.Error("strings mode applied profile conversion")
				}
			case "demo":
				if got[0]["value"] != "001" || got[1]["value"] != json.Number("1") || got[2]["value"] != "  spaced  " {
					t.Errorf("demo profile output = %#v", got)
				}
				if _, ok := got[1]["decoded"]; ok {
					t.Error("demo output unexpectedly emitted the strings-only field")
				}
			}
		})
	}
}

func TestRunOpaqueTypeAndControlKeyRemainIndependentOfProfile(t *testing.T) {
	const key = "A:#\t\x00"
	const expression = "map(T:U,list(V))"
	const source = "\"" + key + "\" (" + expression + "):001"
	for _, mode := range []string{"raw", "strings", "demo"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{"-mode", mode}, strings.NewReader(cliDocument(source)), &stdout, &stderr)
			if mode == "demo" {
				requireCLIErrorCode(t, err, dotreg.ErrTypeConversion)
				if stdout.Len() != 0 {
					t.Errorf("profile rejection emitted partial output: %q", stdout.String())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := decodeCLIOutput(t, stdout.Bytes())
			if len(got) != 1 || got[0]["type"] != expression || got[0]["raw"] != "001" {
				t.Fatalf("opaque type output = %#v", got)
			}
			path, ok := got[0]["path"].([]any)
			if !ok || len(path) != 1 || path[0] != key {
				t.Errorf("quoted control key output = %#v", got[0]["path"])
			}
			if mode == "strings" && got[0]["decoded"] != "001" {
				t.Errorf("string layer changed the unconverted value: %#v", got[0])
			}
		})
	}
}

func TestRunRawRetainsInvalidQuotedValue(t *testing.T) {
	for _, raw := range []string{
		"\"unterminated", "\"value\"tail", "\"first\" \"second\"",
		"\"\\q\"", "\"\"\"suffix", "\"trailing\\",
	} {
		for _, mode := range []string{"raw", "strings", "demo"} {
			t.Run(mode+"/"+raw, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				source := "A: first\nA: \t" + raw + " \t"
				err := run([]string{"-mode", mode}, strings.NewReader(cliDocument(source)), &stdout, &stderr)
				if mode == "raw" {
					if err != nil {
						t.Fatal(err)
					}
					got := decodeCLIOutput(t, stdout.Bytes())
					if len(got) != 2 || got[0]["raw"] != "first" || got[1]["raw"] != raw {
						t.Errorf("raw output = %#v", got)
					}
					return
				}
				requireCLIErrorCode(t, err, dotreg.ErrString)
				var diagnostic *dotreg.Error
				if !errors.As(err, &diagnostic) || diagnostic.Line != 4 {
					t.Errorf("quoted-value failure has the wrong line: %v", err)
				}
				if stdout.Len() != 0 {
					t.Errorf("failed %s mode emitted partial output: %q", mode, stdout.String())
				}
			})
		}
	}
}

func TestRunNonLeadingQuotesAndBackslashesRemainLiteral(t *testing.T) {
	rawValues := []string{
		"{\"name\":\"Alice\",\"text\":\"line\\nnext\",\"path\":\"C:\\\\temp\"}",
		"some\"value\"",
		"some\"unterminated",
		"before \"a:b\" # after (T)",
		"\\\"hello\\\"",
		"C:\\temp\\\"name\"\\q\\",
		"prefix\"\"\"suffix",
		"some\"value\"",
		"prefix\"\"\"",
	}
	lines := make([]string, len(rawValues))
	for i, raw := range rawValues {
		lines[i] = "A: \t" + raw + " \t"
	}
	// No terminal newline: a triple quote in the final literal is not a block.
	source := strings.Join(lines, "\n")
	for _, mode := range []string{"raw", "strings", "demo"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run([]string{"-mode", mode}, strings.NewReader(cliDocument(source)), &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			got := decodeCLIOutput(t, stdout.Bytes())
			if len(got) != len(rawValues) || stderr.Len() != 0 {
				t.Fatalf("literal occurrences=%#v, stderr=%q", got, stderr.String())
			}
			for i, entry := range got {
				path, ok := entry["path"].([]any)
				if !ok || len(path) != 1 || path[0] != "A" ||
					entry["kind"] != string(dotreg.Value) || entry["has_type"] != false ||
					entry["raw"] != rawValues[i] || entry["unset"] != false {
					t.Errorf("literal occurrence %d changed syntax/state: %#v", i, entry)
				}
				if _, exists := entry["form"]; exists {
					t.Errorf("literal occurrence %d became a block: %#v", i, entry)
				}
				if mode == "raw" {
					if _, exists := entry["decoded"]; exists {
						t.Error("raw mode unexpectedly decoded a literal")
					}
					if _, exists := entry["value"]; exists {
						t.Error("raw mode unexpectedly converted a literal")
					}
					continue
				}
				field := "decoded"
				if mode == "demo" {
					field = "value"
				}
				if entry[field] != rawValues[i] {
					t.Errorf("literal occurrence %d decoded to %#v; want %q", i, entry[field], rawValues[i])
				}
			}
		})
	}
}

func TestRunUnicodeAndControlEscapesPreserveRawAndDecodeForJSON(t *testing.T) {
	const raw = "\"\\u{1F600}\\n\\t\\u0000\""
	const source = "\"line\\nkey\": " + raw + "\nBare: \\u{1F600}\n"
	for _, mode := range []string{"raw", "strings", "demo"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{"-mode", mode}, strings.NewReader(cliDocument(source)), &stdout, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			got := decodeCLIOutput(t, stdout.Bytes())
			if len(got) != 2 || got[0]["raw"] != raw || got[1]["raw"] != "\\u{1F600}" {
				t.Fatalf("raw escape spelling was changed: %#v", got)
			}
			path, ok := got[0]["path"].([]any)
			if !ok || len(path) != 1 || path[0] != "line\nkey" {
				t.Errorf("decoded newline key = %#v", got[0]["path"])
			}
			if mode == "raw" {
				return
			}
			field := "decoded"
			if mode == "demo" {
				field = "value"
			}
			if got[0][field] != "😀\n\t\x00" || got[1][field] != "\\u{1F600}" {
				t.Errorf("decoded output = %#v", got)
			}
			// JSON must represent decoded control characters using escapes.
			if bytes.Contains(stdout.Bytes(), []byte{0}) || !strings.Contains(stdout.String(), "\\n\\t\\u0000") {
				t.Errorf("decoded control characters were not JSON escaped: %q", stdout.String())
			}
		})
	}
}

func TestRunMultilineFormsKeepRawAndDecodeByForm(t *testing.T) {
	const source = "Triple: \"\"\"\n  first\\nsecond\n  \"\"\"\n" +
		"Pipe:\n |first\\nsecond\n" +
		"Empty:\n |\n"
	for _, mode := range []string{"raw", "strings", "demo"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{"-mode", mode}, strings.NewReader(cliDocument(source)), &stdout, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			got := decodeCLIOutput(t, stdout.Bytes())
			if len(got) != 3 {
				t.Fatalf("multiline CLI count = %d, want 3", len(got))
			}
			if got[0]["form"] != "triple_quoted" || got[1]["form"] != "pipe" || got[2]["form"] != "pipe" {
				t.Errorf("multiline forms were not retained: %#v", got)
			}
			if got[0]["raw"] != "first\\nsecond" || got[1]["raw"] != "first\\nsecond" || got[2]["raw"] != "" || got[2]["unset"] != false {
				t.Errorf("multiline raw/state output = %#v", got)
			}
			if mode == "raw" {
				if _, ok := got[0]["decoded"]; ok {
					t.Error("raw multiline output unexpectedly contained decoded text")
				}
				return
			}
			field := "decoded"
			if mode == "demo" {
				field = "value"
			}
			if got[0][field] != "first\nsecond" || got[1][field] != "first\\nsecond" || got[2][field] != "" {
				t.Errorf("multiline decoded output = %#v", got)
			}
		})
	}
}

func TestRunInvalidSurrogateDoesNotEmitPartialJSON(t *testing.T) {
	for _, raw := range []string{"\"private\\uD800\"", "\"private\\uD83D\\u0041\""} {
		for _, mode := range []string{"raw", "strings", "demo"} {
			t.Run(mode+raw, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				err := run([]string{"-mode", mode}, strings.NewReader(cliDocument("A: first\nB: "+raw)), &stdout, &stderr)
				if mode == "raw" {
					if err != nil {
						t.Fatal(err)
					}
					got := decodeCLIOutput(t, stdout.Bytes())
					if len(got) != 2 || got[1]["raw"] != raw {
						t.Errorf("raw malformed escape was not preserved: %#v", got)
					}
					return
				}
				requireCLIErrorCode(t, err, dotreg.ErrString)
				if stdout.Len() != 0 || strings.Contains(err.Error()+stderr.String(), "private") {
					t.Errorf("surrogate failure exposed partial JSON or RHS: stdout=%q stderr=%q error=%v", stdout.String(), stderr.String(), err)
				}
			})
		}
	}
}

func TestRunUnsetAndExplicitEmptyRemainDistinct(t *testing.T) {
	for _, mode := range []string{"raw", "strings", "demo"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{"-mode", mode}, strings.NewReader(cliDocument("A:\nB: \"\"\n")), &stdout, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			got := decodeCLIOutput(t, stdout.Bytes())
			if len(got) != 2 {
				t.Fatalf("entry count = %d, want 2", len(got))
			}
			if got[0]["unset"] != true || got[1]["unset"] != false || got[0]["raw"] != "" || got[1]["raw"] != "\"\"" {
				t.Errorf("unset/explicit-empty states = %#v", got)
			}
			field := ""
			if mode == "strings" {
				field = "decoded"
			} else if mode == "demo" {
				field = "value"
			}
			if field != "" && (got[0][field] != "" || got[1][field] != "") {
				t.Errorf("empty strings must both be present in %q: %#v", field, got)
			}
		})
	}
}

func TestRunRepeatedValuesKeepEachOccurrence(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		raw     []string
		decoded []string
		demo    []any
		unset   []bool
	}{
		{
			name:    "repeated identical values",
			source:  "A.Tag: red\nA.Tag: green\nA.Tag: red\n",
			raw:     []string{"red", "green", "red"},
			decoded: []string{"red", "green", "red"},
			demo:    []any{"red", "green", "red"},
			unset:   []bool{false, false, false},
		},
		{
			name:    "independent type declarations",
			source:  "A(string):001\nA(int):2\nA:003\nA(bool):false\n",
			raw:     []string{"001", "2", "003", "false"},
			decoded: []string{"001", "2", "003", "false"},
			demo:    []any{"001", json.Number("2"), "003", false},
			unset:   []bool{false, false, false, false},
		},
		{
			name:    "unset and empty forms",
			source:  "A:\nA:\"\"\nA:\n |\nA:\"\"\"\n\"\"\"\n",
			raw:     []string{"", "\"\"", "", ""},
			decoded: []string{"", "", "", ""},
			demo:    []any{"", "", "", ""},
			unset:   []bool{true, false, false, false},
		},
	}
	for _, test := range tests {
		for _, mode := range []string{"raw", "strings", "demo"} {
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				if err := run([]string{"-mode", mode}, strings.NewReader(cliDocument(test.source)), &stdout, &stderr); err != nil {
					t.Fatal(err)
				}
				got := decodeCLIOutput(t, stdout.Bytes())
				if len(got) != len(test.raw) {
					t.Fatalf("entry count = %d, want %d", len(got), len(test.raw))
				}
				for i, entry := range got {
					if entry["raw"] != test.raw[i] || entry["unset"] != test.unset[i] {
						t.Errorf("occurrence %d source/state changed: %#v", i, entry)
					}
					if mode == "strings" && entry["decoded"] != test.decoded[i] {
						t.Errorf("occurrence %d decoded = %#v, want %q", i, entry["decoded"], test.decoded[i])
					}
					if mode == "demo" && entry["value"] != test.demo[i] {
						t.Errorf("occurrence %d value = %#v, want %#v", i, entry["value"], test.demo[i])
					}
				}
			})
		}
	}
}

func TestRunDemoCountsRepeatedDecodedPathsWithoutLosingBoundaries(t *testing.T) {
	const source = "A.B: first\nA.\n  .B: second\nA.\n  .\"\\u0042\": third\n" +
		"\"A.B\": literal-first\n\"A.B\": literal-second\n" +
		"\"\": empty-first\n\"\": empty-second\n" +
		"\"A\\u0000B\": nul-first\nA.\"\": child-first\n" +
		"\"A\\u0000B\": nul-second\nA.\"\": child-second\n"
	want := []string{"first", "second", "third", "literal-first", "literal-second", "empty-first", "empty-second", "nul-first", "child-first", "nul-second", "child-second"}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"-mode", "demo"}, strings.NewReader(cliDocument(source)), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	got := decodeCLIOutput(t, stdout.Bytes())
	i := 0
	for _, entry := range got {
		if entry["kind"] == string(dotreg.Heading) {
			continue
		}
		if i >= len(want) || entry["value"] != want[i] {
			t.Fatalf("occurrence %d = %#v; expected values %v", i, entry, want)
		}
		i++
	}
	if i != len(want) {
		t.Fatalf("value count = %d, want %d", i, len(want))
	}
}

func TestRunRejectsOddHierarchyIndentationInEveryMode(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"one-space child", "A.\n .B: private-RHS"},
		{"one-space self", "A.\n .: private-RHS"},
		{"three-space grandchild", "A.\n  .B.\n   .C: private-RHS"},
	} {
		for _, mode := range []string{"raw", "strings", "demo"} {
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				err := run([]string{"-mode", mode}, strings.NewReader(cliDocument(test.source)), &stdout, &stderr)
				requireCLIErrorCode(t, err, dotreg.ErrSyntax)
				if stdout.Len() != 0 {
					t.Errorf("invalid indentation emitted partial output: %q", stdout.String())
				}
				if strings.Contains(err.Error()+stderr.String(), "private-RHS") {
					t.Errorf("indentation failure leaked the RHS: %v; stderr=%q", err, stderr.String())
				}
			})
		}
	}
}

func TestRunFailureProducesNoPartialJSON(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		source string
		code   dotreg.Code
	}{
		{"late syntax failure", "raw", "A: first\nB..C: private-RHS", dotreg.ErrSyntax},
		{"late repeated string failure", "strings", "A: first\nA: \"private-RHS\"tail", dotreg.ErrString},
		{"late string failure", "strings", "A: first\nB: \"private-RHS\"tail", dotreg.ErrString},
		{"late profile failure", "demo", "A: first\nB(int): private-RHS", dotreg.ErrTypeConversion},
		{"unclosed triple block", "raw", "A: first\nB: \"\"\"\nprivate-RHS", dotreg.ErrSyntax},
		{"triple margin failure", "raw", "A: first\nB: \"\"\"\n private-RHS\n  \"\"\"", dotreg.ErrSyntax},
		{"triple escape failure", "strings", "A: first\nB: \"\"\"\nprivate-RHS\\q\n\"\"\"", dotreg.ErrString},
		{"pipe profile failure", "demo", "A: first\nB(int):\n |private-RHS", dotreg.ErrTypeConversion},
		{"repeated multiline escape failure", "strings", "A:\n |first\nA: \"\"\"\nprivate-RHS\\q\n\"\"\"", dotreg.ErrString},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{"-mode", test.mode}, strings.NewReader(cliDocument(test.source)), &stdout, &stderr)
			requireCLIErrorCode(t, err, test.code)
			if stdout.Len() != 0 {
				t.Errorf("failure emitted partial output: %q", stdout.String())
			}
			if strings.Contains(err.Error()+stderr.String(), "private-RHS") {
				t.Errorf("failure leaked the RHS: %v; stderr=%q", err, stderr.String())
			}
		})
	}
}

type cliErrorReader struct {
	data string
}

func (reader *cliErrorReader) Read(buffer []byte) (int, error) {
	if reader.data != "" {
		n := copy(buffer, reader.data)
		reader.data = reader.data[n:]
		return n, nil
	}
	return 0, errors.New("private-reader-error")
}

func TestRunReaderErrorIsSanitized(t *testing.T) {
	var stdout, stderr bytes.Buffer
	reader := io.MultiReader(strings.NewReader("@dotReg 0.1\n\n"), &cliErrorReader{data: "A: first\n"})
	err := run(nil, reader, &stdout, &stderr)
	requireCLIErrorCode(t, err, dotreg.ErrInput)
	if stdout.Len() != 0 || strings.Contains(err.Error()+stderr.String(), "private-reader-error") {
		t.Errorf("read failure exposed input or partial output: stdout=%q stderr=%q error=%v", stdout.String(), stderr.String(), err)
	}
}

func TestRunArgumentsAndFiles(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.conf")
	for _, test := range []struct {
		name    string
		args    []string
		wantErr bool
		usage   bool
	}{
		{"help", []string{"-h"}, false, true},
		{"unknown mode", []string{"-mode", "other"}, true, true},
		{"too many files", []string{"one", "two"}, true, true},
		{"unknown flag", []string{"-unknown"}, true, true},
		{"missing file", []string{missing}, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(test.args, strings.NewReader("invalid input"), &stdout, &stderr)
			if (err != nil) != test.wantErr {
				t.Errorf("error = %v; wantErr=%v", err, test.wantErr)
			}
			if stdout.Len() != 0 {
				t.Errorf("argument handling emitted configuration output: %q", stdout.String())
			}
			if test.usage && !strings.Contains(stderr.String(), "Usage:") {
				t.Errorf("missing usage text: %q", stderr.String())
			}
			if test.name == "missing file" && strings.Contains(err.Error(), missing) {
				t.Errorf("file-open error exposed the underlying error: %v", err)
			}
		})
	}
	t.Run("explicit stdin and default strings mode", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if err := run([]string{"-"}, strings.NewReader(cliDocument("A:001")), &stdout, &stderr); err != nil {
			t.Fatal(err)
		}
		got := decodeCLIOutput(t, stdout.Bytes())
		if len(got) != 1 || got[0]["decoded"] != "001" {
			t.Errorf("default stdin output = %#v", got)
		}
	})
	t.Run("file input replaces stdin", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sample.conf")
		if err := os.WriteFile(path, []byte(cliDocument("A: file-value")), 0600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if err := run([]string{path}, strings.NewReader("invalid stdin"), &stdout, &stderr); err != nil {
			t.Fatal(err)
		}
		got := decodeCLIOutput(t, stdout.Bytes())
		if len(got) != 1 || got[0]["decoded"] != "file-value" {
			t.Errorf("file output = %#v", got)
		}
	})
}
