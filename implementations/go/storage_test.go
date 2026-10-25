package dotreg

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// Interleaving forces repeated keys to remain separate from source order. The
// expected values come from input literals, without inspecting Config storage.
func TestStoredOccurrencesKeepInterleavedKeysAndOwnedResults(t *testing.T) {
	paths := [][]string{{"Root", ""}, {"Root", "A.B"}, {"Root", "A", "B"}, {"Else", "尾"}, {"Root"}}
	var entries []Entry
	want := make([]SelectedValue, 0, 257)
	wantByKey := make([][]any, len(paths))
	wantUnset := make([][]bool, len(paths))
	secondLines := make([]int, len(paths))
	var converted, expectedConversions []string
	profile := Profile{StringType: "text", Types: map[string]Converter{
		"custom": func(text Text) (any, error) {
			converted = append(converted, text.Value)
			return "converted:" + text.Value, nil
		},
	}}
	for i := 0; i < 257; i++ {
		if i%9 == 0 {
			entries = append(entries, Entry{Line: i*2 + 1, Kind: Heading, Path: []string{"Ignored"}})
		}
		key := i % len(paths)
		path := append([]string(nil), paths[key]...)
		text := fmt.Sprintf("value-%03d", i)
		unset := i%11 == 0
		if unset {
			text = ""
		}
		entry := Entry{Line: i*2 + 2, Kind: Value, Path: path, Raw: text, Unset: unset}
		var value any = text
		if i%7 == 0 {
			entry.Type, entry.HasType = "custom", true
			value = "converted:" + text
			expectedConversions = append(expectedConversions, text)
		} else if i%3 == 0 {
			entry.Type, entry.HasType = "text", true
		}
		entries = append(entries, entry)
		want = append(want, SelectedValue{
			Path: append([]string(nil), path...), RelativePath: append([]string(nil), path...),
			Value: value, Type: entry.Type, HasType: entry.HasType, Unset: unset, Line: entry.Line,
		})
		wantByKey[key] = append(wantByKey[key], value)
		wantUnset[key] = append(wantUnset[key], unset)
		if len(wantByKey[key]) == 2 {
			secondLines[key] = entry.Line
		}
	}
	cfg, err := Build(entries, profile)
	if err != nil || !reflect.DeepEqual(converted, expectedConversions) {
		t.Fatalf("Build error = %v; conversions = %q, want %q", err, converted, expectedConversions)
	}
	profile.Types["custom"] = func(Text) (any, error) { return nil, errors.New("caller replacement") }
	for i := range entries {
		entries[i].Path[0] = "caller replacement"
		entries[i].Raw, entries[i].Type = "changed", "changed"
	}
	for round := 0; round < 2; round++ {
		selected, err := cfg.SelectPrefix(nil)
		if err != nil || !reflect.DeepEqual(selected, want) {
			t.Fatalf("SelectPrefix round %d = %#v, %v; want %#v", round, selected, err, want)
		}
		selected[0].Path[0], selected[0].RelativePath[0] = "changed full", "changed relative"
		for key, path := range paths {
			states, err := cfg.IsUnsetAll(path)
			if err != nil || !reflect.DeepEqual(states, wantUnset[key]) {
				t.Fatalf("IsUnsetAll(%q) = %#v, %v", path, states, err)
			}
			states[0] = !states[0]
			for index, expected := range wantByKey[key] {
				// Each entry's declaration determines its requested type.
				inputIndex := key + index*len(paths)
				wantType := "text"
				if inputIndex%7 == 0 {
					wantType = "custom"
				}
				got, err := cfg.GetAt(path, index, wantType)
				if err != nil || got != expected {
					t.Fatalf("GetAt(%q, %d) = %#v, %v; want %#v", path, index, got, err, expected)
				}
			}
			_, err = cfg.Get(path, "missing-type")
			assertProfileErrorLine(t, err, ErrMultipleValues, secondLines[key])
			_, err = cfg.IsUnset(path)
			assertProfileErrorLine(t, err, ErrMultipleValues, secondLines[key])
			_, err = cfg.GetAt(path, len(wantByKey[key]), "text")
			assertProfileErrorLine(t, err, ErrMissing, 0)
		}
	}
	if !reflect.DeepEqual(converted, expectedConversions) {
		t.Fatal("stored declared values were converted again during retrieval")
	}
}

func TestStoredOccurrencesStopAtFirstConversionFailure(t *testing.T) {
	var calls []string
	profile := Profile{StringType: "text", Types: map[string]Converter{
		"checked": func(text Text) (any, error) {
			calls = append(calls, text.Value)
			if text.Value == "fail" {
				return nil, errors.New("private conversion error")
			}
			return text.Value, nil
		},
	}}
	entries := []Entry{
		{Line: 3, Kind: Value, Path: []string{"A"}, Raw: "first", Type: "checked", HasType: true},
		{Line: 4, Kind: Value, Path: []string{"B"}, Raw: "other", Type: "checked", HasType: true},
		{Line: 8, Kind: Value, Path: []string{"A"}, Raw: "second", Type: "checked", HasType: true},
		{Line: 12, Kind: Value, Path: []string{"A"}, Raw: "fail", Type: "checked", HasType: true},
		{Line: 20, Kind: Value, Path: []string{"B"}, Raw: `"\q"`, Type: "checked", HasType: true},
	}
	cfg, err := Build(entries, profile)
	assertProfileErrorLine(t, err, ErrTypeConversion, 12)
	if cfg != nil || !reflect.DeepEqual(calls, []string{"first", "other", "second", "fail"}) {
		t.Fatalf("failed Build = %#v; conversions = %q", cfg, calls)
	}
}
