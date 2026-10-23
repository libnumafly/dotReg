package dotreg

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

// Expected results are hand specified in the shared corpus, not captured from
// this implementation. Keep Go subject to the same checks as every port.
func TestSharedConformance(t *testing.T) {
	data, err := os.ReadFile("../conformance/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Version int `json:"version"`
		Cases   []struct {
			Name     string `json:"name"`
			Input    string `json:"input"`
			InputHex string `json:"input_hex"`
			Entries  []struct {
				Entry
				Decoded string `json:"decoded"`
			} `json:"entries"`
			Error *struct {
				Stage string `json:"stage"`
				Line  int    `json:"line"`
				Code  Code   `json:"code"`
			} `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != 1 || len(corpus.Cases) == 0 {
		t.Fatal("unsupported or empty shared corpus")
	}
	for _, c := range corpus.Cases {
		t.Run(c.Name, func(t *testing.T) {
			input := []byte(c.Input)
			if c.InputHex != "" {
				input, err = hex.DecodeString(c.InputHex)
				if err != nil {
					t.Fatal(err)
				}
			}
			entries, err := Parse(bytes.NewReader(input))
			checkError := func(err error) {
				t.Helper()
				var e *Error
				if !errors.As(err, &e) || e.Line != c.Error.Line || e.Code != c.Error.Code {
					t.Fatalf("error %v; want line %d code %s", err, c.Error.Line, c.Error.Code)
				}
			}
			if c.Error != nil && c.Error.Stage == "parse" {
				checkError(err)
				if entries != nil {
					t.Fatal("partial parse result")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(c.Entries) {
				t.Fatalf("got %d entries; want %d", len(entries), len(c.Entries))
			}
			decodeFailed := false
			for i, entry := range entries {
				if !reflect.DeepEqual(entry, c.Entries[i].Entry) {
					t.Fatalf("entry %d: %#v; want %#v", i, entry, c.Entries[i].Entry)
				}
				if entry.Kind != Value {
					continue
				}
				text, decodeErr := DecodeString(entry)
				if decodeErr != nil {
					if c.Error == nil || c.Error.Stage != "decode" {
						t.Fatal(decodeErr)
					}
					checkError(decodeErr)
					decodeFailed = true
					break
				}
				if c.Error == nil && (text.Value != c.Entries[i].Decoded || text.Unset != entry.Unset) {
					t.Fatalf("entry %d string decoding differs", i)
				}
			}
			if c.Error != nil && !decodeFailed {
				t.Fatal("expected string-decoding failure")
			}
		})
	}
}
