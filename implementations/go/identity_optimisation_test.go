package dotreg

import (
	"reflect"
	"strings"
	"testing"
)

func TestKeyIdentityLengthBoundariesAndOwnership(t *testing.T) {
	paths := [][]string{{"A.B", ""}, {"A", "B", ""}, {"", "A.B"}}
	for _, size := range []int{9, 10, 99, 100, 999, 1000} {
		paths = append(paths, []string{strings.Repeat("x", size), "leaf"})
		// Lengths in the internal identity count UTF-8 bytes, not runes.
		paths = append(paths, []string{strings.Repeat("界", size/3) + strings.Repeat("x", size%3), ""})
	}
	entries := make([]Entry, 0, 2*len(paths))
	for pass := 0; pass < 2; pass++ {
		for i, path := range paths {
			entries = append(entries, Entry{Line: len(entries) + 3, Kind: Value,
				Path: append([]string(nil), path...), Raw: strings.Repeat("v", i+pass+1)})
		}
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		for j := range entries[i].Path {
			entries[i].Path[j] = "caller changed input"
		}
	}
	for i, path := range paths {
		want := []any{strings.Repeat("v", i+1), strings.Repeat("v", i+2)}
		got, err := cfg.GetAll(path, "string")
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("path %d values = %#v, %v; want %#v", i, got, err, want)
		}
		selected, err := cfg.SelectPrefix(path)
		if err != nil || len(selected) != 2 {
			t.Fatalf("path %d selection = %d values, %v", i, len(selected), err)
		}
		for j, item := range selected {
			if !reflect.DeepEqual(item.Path, path) || item.RelativePath == nil || len(item.RelativePath) != 0 || item.Value != want[j] {
				t.Fatalf("path %d occurrence %d = %#v", i, j, item)
			}
		}
	}
}
