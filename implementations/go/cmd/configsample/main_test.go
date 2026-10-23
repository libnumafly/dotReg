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

func TestRunOutputLayers(t *testing.T) {
	const source = "A:001\nB(int):001\nC: \"  spaced  \"\n"
	for _, mode := range []string{"raw", "strings", "demo"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{"-mode", mode}, strings.NewReader(source), &stdout, &stderr)
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
			err := run([]string{"-mode", mode}, strings.NewReader(source), &stdout, &stderr)
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
	const source = "A: \"unterminated"
	for _, mode := range []string{"raw", "strings", "demo"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{"-mode", mode}, strings.NewReader(source), &stdout, &stderr)
			if mode == "raw" {
				if err != nil {
					t.Fatal(err)
				}
				got := decodeCLIOutput(t, stdout.Bytes())
				if len(got) != 1 || got[0]["raw"] != "\"unterminated" {
					t.Errorf("raw output = %#v", got)
				}
				return
			}
			requireCLIErrorCode(t, err, dotreg.ErrString)
			if stdout.Len() != 0 {
				t.Errorf("failed %s mode emitted partial output: %q", mode, stdout.String())
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
			err := run([]string{"-mode", mode}, strings.NewReader(source), &stdout, &stderr)
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
			err := run([]string{"-mode", mode}, strings.NewReader(source), &stdout, &stderr)
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
				err := run([]string{"-mode", mode}, strings.NewReader("A: first\nB: "+raw), &stdout, &stderr)
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
			err := run([]string{"-mode", mode}, strings.NewReader("A:\nB: \"\"\n"), &stdout, &stderr)
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

func TestRunFailureProducesNoPartialJSON(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		source string
		code   dotreg.Code
	}{
		{"late syntax failure", "raw", "A: first\nB..C: private-RHS", dotreg.ErrSyntax},
		{"late duplicate", "strings", "A: first\nA: private-RHS", dotreg.ErrDuplicate},
		{"late string failure", "strings", "A: first\nB: \"private-RHS\"tail", dotreg.ErrString},
		{"late profile failure", "demo", "A: first\nB(int): private-RHS", dotreg.ErrTypeConversion},
		{"unclosed triple block", "raw", "A: first\nB: \"\"\"\nprivate-RHS", dotreg.ErrSyntax},
		{"triple margin failure", "raw", "A: first\nB: \"\"\"\n private-RHS\n  \"\"\"", dotreg.ErrSyntax},
		{"triple escape failure", "strings", "A: first\nB: \"\"\"\nprivate-RHS\\q\n\"\"\"", dotreg.ErrString},
		{"pipe profile failure", "demo", "A: first\nB(int):\n |private-RHS", dotreg.ErrTypeConversion},
		{"multiline duplicate", "strings", "A:\n |first\nA: private-RHS", dotreg.ErrDuplicate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{"-mode", test.mode}, strings.NewReader(test.source), &stdout, &stderr)
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
	err := run(nil, &cliErrorReader{data: "A: first\n"}, &stdout, &stderr)
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
		if err := run([]string{"-"}, strings.NewReader("A:001"), &stdout, &stderr); err != nil {
			t.Fatal(err)
		}
		got := decodeCLIOutput(t, stdout.Bytes())
		if len(got) != 1 || got[0]["decoded"] != "001" {
			t.Errorf("default stdin output = %#v", got)
		}
	})
	t.Run("file input replaces stdin", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sample.conf")
		if err := os.WriteFile(path, []byte("A: file-value"), 0600); err != nil {
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
