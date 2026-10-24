package dotreg

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSelectPrefixPreservesBaseAndDescendantsInSourceOrder(t *testing.T) {
	const source = "@dotReg 1\n\n" +
		"A.B: base\n" +
		"A.B.\n" +
		"  .One(int): 001\n" +
		"Else: outside\n" +
		"A.B.Two: \"x\\n\"\n" +
		"A.B.One(int): 2\n" +
		"A.B.Deep.\n" +
		"  .Leaf(bool): true\n" +
		"A.B.Missing:\n" +
		"A.B.Empty: \"\"\n" +
		"A.B(string): another base\n" +
		"A.B.HeadingOnly.\n" +
		"A.B.One: plain\n"
	entries, err := Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	want := []SelectedValue{
		{Path: []string{"A", "B"}, RelativePath: []string{}, Value: "base", Line: 3},
		{Path: []string{"A", "B", "One"}, RelativePath: []string{"One"}, Value: int64(1), Type: "int", HasType: true, Line: 5},
		{Path: []string{"A", "B", "Two"}, RelativePath: []string{"Two"}, Value: "x\n", Line: 7},
		{Path: []string{"A", "B", "One"}, RelativePath: []string{"One"}, Value: int64(2), Type: "int", HasType: true, Line: 8},
		{Path: []string{"A", "B", "Deep", "Leaf"}, RelativePath: []string{"Deep", "Leaf"}, Value: true, Type: "bool", HasType: true, Line: 10},
		{Path: []string{"A", "B", "Missing"}, RelativePath: []string{"Missing"}, Value: "", Unset: true, Line: 11},
		{Path: []string{"A", "B", "Empty"}, RelativePath: []string{"Empty"}, Value: "", Line: 12},
		{Path: []string{"A", "B"}, RelativePath: []string{}, Value: "another base", Type: "string", HasType: true, Line: 13},
		{Path: []string{"A", "B", "One"}, RelativePath: []string{"One"}, Value: "plain", Line: 15},
	}
	for range 2 {
		got, err := cfg.SelectPrefix([]string{"A", "B"})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("SelectPrefix() = %#v, %v; want %#v", got, err, want)
		}
	}
}

func TestSelectPrefixIncludesBaseOccurrencesAndEmptyNamedChildren(t *testing.T) {
	const source = "@dotReg 1\n\n" +
		"ip.filter:\n" +
		"ip.filter.\n" +
		"  .100099: pass * * * * *\n" +
		"Outside: outside\n" +
		"ip.filter(string): typed base\n" +
		"ip.filter.\"\": empty-name child\n" +
		"ip.filter: raw base\n" +
		"ip.filter(int): 001\n" +
		"ip.filter.HeadingOnly.\n" +
		"ip.filter.100099: pass * * * * *\n"
	entries, err := Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	want := []SelectedValue{
		{Path: []string{"ip", "filter"}, RelativePath: []string{}, Value: "", Unset: true, Line: 3},
		{Path: []string{"ip", "filter", "100099"}, RelativePath: []string{"100099"}, Value: "pass * * * * *", Line: 5},
		{Path: []string{"ip", "filter"}, RelativePath: []string{}, Value: "typed base", Type: "string", HasType: true, Line: 7},
		{Path: []string{"ip", "filter", ""}, RelativePath: []string{""}, Value: "empty-name child", Line: 8},
		{Path: []string{"ip", "filter"}, RelativePath: []string{}, Value: "raw base", Line: 9},
		{Path: []string{"ip", "filter"}, RelativePath: []string{}, Value: int64(1), Type: "int", HasType: true, Line: 10},
		{Path: []string{"ip", "filter", "100099"}, RelativePath: []string{"100099"}, Value: "pass * * * * *", Line: 12},
	}
	got, err := cfg.SelectPrefix([]string{"ip", "filter"})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("selection = %#v, %v; want %#v", got, err, want)
	}
	for _, index := range []int{0, 2, 4, 5} {
		encoded, err := json.Marshal(got[index].RelativePath)
		if err != nil || string(encoded) != "[]" {
			t.Errorf("base relative path JSON = %s, %v; want []", encoded, err)
		}
	}
	encoded, err := json.Marshal(got[3].RelativePath)
	if err != nil || string(encoded) != `[""]` {
		t.Errorf("empty-named child relative path JSON = %s, %v; want [\"\"]", encoded, err)
	}
}

func TestSelectPrefixUsesEntryOrderRatherThanLineNumbers(t *testing.T) {
	entries := []Entry{
		{Line: 20, Kind: Value, Path: []string{"A", "B", "First"}, Raw: "one"},
		{Line: 1, Kind: Value, Path: []string{"Outside"}, Raw: "outside"},
		{Line: 8, Kind: Value, Path: []string{"A", "B", "Second"}, Raw: "two"},
		{Line: 0, Kind: Heading, Path: []string{"A", "B", "Heading"}},
		{Line: 0, Kind: Value, Path: []string{"A", "B", "First"}, Raw: "three"},
		{Line: 20, Kind: Value, Path: []string{"A", "B", "Second"}, Raw: "four"},
		{Line: 1, Kind: Value, Path: []string{"A", "B", "Third"}, Raw: "five"},
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	got, err := cfg.SelectPrefix([]string{"A", "B"})
	if err != nil {
		t.Fatal(err)
	}
	var values []any
	var lines []int
	for _, selected := range got {
		values = append(values, selected.Value)
		lines = append(lines, selected.Line)
	}
	if !reflect.DeepEqual(values, []any{"one", "two", "three", "four", "five"}) ||
		!reflect.DeepEqual(lines, []int{20, 8, 0, 20, 1}) {
		t.Fatalf("selection changed Build entry order: values = %#v, lines = %#v", values, lines)
	}
}

func TestSelectPrefixMatchesWholePathElements(t *testing.T) {
	items := []struct {
		path  []string
		value string
	}{
		{[]string{"A", "B"}, "base"},
		{[]string{"A", "B", "Child"}, "exact"},
		{[]string{"A", "B", "Child", "Leaf"}, "deep"},
		{[]string{"A", "Beta", "Child"}, "longer-name"},
		{[]string{"A", "B.C", "Child"}, "literal-dot"},
		{[]string{"A", "*"}, "literal-star-base"},
		{[]string{"A", "*", "Child"}, "literal-star"},
		{[]string{""}, "empty-base"},
		{[]string{"", "Child"}, "empty-element"},
		{[]string{"A", "", "Child"}, "empty-middle"},
		{[]string{"A", "b", "Child"}, "lowercase"},
		{[]string{"A", "é", "Child"}, "composed"},
		{[]string{"A", "e\u0301", "Child"}, "decomposed"},
		{[]string{"A.B", "Child"}, "single-root-element"},
	}
	var entries []Entry
	var all []any
	for _, item := range items {
		entries = append(entries, Entry{Kind: Value, Path: item.path, Raw: item.value})
		all = append(all, item.value)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		prefix []string
		want   []any
	}{
		{"nil selects all", nil, all},
		{"empty slice selects all", []string{}, all},
		{"base and descendants at all depths", []string{"A", "B"}, []any{"base", "exact", "deep"}},
		{"literal dot", []string{"A", "B.C"}, []any{"literal-dot"}},
		{"literal star", []string{"A", "*"}, []any{"literal-star-base", "literal-star"}},
		{"empty element is a name", []string{""}, []any{"empty-base", "empty-element"}},
		{"empty middle element", []string{"A", ""}, []any{"empty-middle"}},
		{"case is exact", []string{"A", "b"}, []any{"lowercase"}},
		{"composed Unicode", []string{"A", "é"}, []any{"composed"}},
		{"decomposed Unicode", []string{"A", "e\u0301"}, []any{"decomposed"}},
		{"dot is not split", []string{"A.B"}, []any{"single-root-element"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			selected, err := cfg.SelectPrefix(test.prefix)
			if err != nil {
				t.Fatal(err)
			}
			var values []any
			for _, item := range selected {
				values = append(values, item.Value)
				if len(test.prefix) == 0 && !reflect.DeepEqual(item.RelativePath, item.Path) {
					t.Fatalf("root selection lost full relative path: %#v", item)
				}
			}
			if !reflect.DeepEqual(values, test.want) {
				t.Fatalf("selected values = %#v; want %#v", values, test.want)
			}
		})
	}
}

func TestSelectPrefixPreservesColonUnicodeAndNumericNames(t *testing.T) {
	const root = "root:00:prefix"
	const branch = "設定😀日本語abc"
	cfg, err := Build([]Entry{
		{Line: 3, Kind: Value, Path: []string{root, branch, "0123456789:tail", "🦊:emoji"}, Raw: "long names"},
		{Line: 4, Kind: Value, Path: []string{root, branch, ""}, Raw: "empty descendant"},
		{Line: 5, Kind: Value, Path: []string{root, branch, "001"}, Raw: "leading-zero base"},
		{Line: 6, Kind: Value, Path: []string{root, branch, "1"}, Raw: "decimal base"},
		{Line: 7, Kind: Value, Path: []string{root, branch, "001", "value:🦊"}, Raw: "leading-zero child"},
		{Line: 8, Kind: Value, Path: []string{root, branch, "1", "value:🦊"}, Raw: "decimal child"},
	}, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	want := []SelectedValue{
		{Path: []string{root, branch, "0123456789:tail", "🦊:emoji"}, RelativePath: []string{"0123456789:tail", "🦊:emoji"}, Value: "long names", Line: 3},
		{Path: []string{root, branch, ""}, RelativePath: []string{""}, Value: "empty descendant", Line: 4},
		{Path: []string{root, branch, "001"}, RelativePath: []string{"001"}, Value: "leading-zero base", Line: 5},
		{Path: []string{root, branch, "1"}, RelativePath: []string{"1"}, Value: "decimal base", Line: 6},
		{Path: []string{root, branch, "001", "value:🦊"}, RelativePath: []string{"001", "value:🦊"}, Value: "leading-zero child", Line: 7},
		{Path: []string{root, branch, "1", "value:🦊"}, RelativePath: []string{"1", "value:🦊"}, Value: "decimal child", Line: 8},
	}
	got, err := cfg.SelectPrefix([]string{root, branch})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("selection changed literal key elements: got %#v, %v; want %#v", got, err, want)
	}
	for _, test := range []struct {
		number   string
		base     string
		baseLine int
		value    string
		line     int
	}{
		{"001", "leading-zero base", 5, "leading-zero child", 7},
		{"1", "decimal base", 6, "decimal child", 8},
	} {
		got, err := cfg.SelectPrefix([]string{root, branch, test.number})
		want := []SelectedValue{
			{Path: []string{root, branch, test.number}, RelativePath: []string{}, Value: test.base, Line: test.baseLine},
			{Path: []string{root, branch, test.number, "value:🦊"}, RelativePath: []string{"value:🦊"}, Value: test.value, Line: test.line},
		}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("SelectPrefix number %q = %#v, %v; want %#v", test.number, got, err, want)
		}
	}
}

func TestSelectPrefixEmptySelectionAndNilConfig(t *testing.T) {
	cfg, err := Build([]Entry{
		{Kind: Heading, Path: []string{"HeadingOnly"}},
		{Kind: Value, Path: []string{"BaseOnly"}, Raw: "value"},
	}, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range [][]string{{"Missing"}, {"HeadingOnly"}} {
		got, err := cfg.SelectPrefix(prefix)
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("SelectPrefix(%q) = %#v, %v; want non-nil empty selection", prefix, got, err)
		}
	}
	base, err := cfg.SelectPrefix([]string{"BaseOnly"})
	wantBase := []SelectedValue{{Path: []string{"BaseOnly"}, RelativePath: []string{}, Value: "value"}}
	if err != nil || !reflect.DeepEqual(base, wantBase) {
		t.Fatalf("base-only selection = %#v, %v; want %#v", base, err, wantBase)
	}
	empty, err := Build(nil, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	got, err := empty.SelectPrefix(nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty Config selection = %#v, %v; want non-nil empty selection", got, err)
	}
	var missing *Config
	for _, prefix := range [][]string{nil, {"A"}} {
		got, err := missing.SelectPrefix(prefix)
		var diagnostic *Error
		if got != nil || !errors.As(err, &diagnostic) || diagnostic.Code != ErrMissing || diagnostic.Line != 0 {
			t.Errorf("nil Config selection = %#v, %v; want missing at line 0", got, err)
		}
	}
}

func TestSelectPrefixDoesNotRequestOrRepeatConversions(t *testing.T) {
	type customValue struct{ label string }
	converted := &customValue{label: "converted"}
	calls := 0
	profile := Profile{StringType: "text", Types: map[string]Converter{
		"custom": func(text Text) (any, error) {
			calls++
			if text != (Text{Value: "typed\n"}) {
				t.Fatalf("converter input = %#v", text)
			}
			return converted, nil
		},
		"number": func(text Text) (any, error) {
			calls++
			return "number:" + text.Value, nil
		},
	}}
	cfg, err := Build([]Entry{
		{Line: 3, Kind: Value, Path: []string{"A", "Typed"}, Raw: `"typed\n"`, Type: "custom", HasType: true},
		{Line: 4, Kind: Value, Path: []string{"A", "Untyped"}, Raw: "001"},
		{Line: 5, Kind: Value, Path: []string{"A", "Text"}, Raw: `"text\n"`, Type: "text", HasType: true},
		{Line: 6, Kind: Value, Path: []string{"A", "Unset"}, Unset: true, Type: "text", HasType: true},
	}, profile)
	if err != nil || calls != 1 {
		t.Fatalf("Build() = %v; conversions = %d", err, calls)
	}
	want := []SelectedValue{
		{Path: []string{"A", "Typed"}, RelativePath: []string{"Typed"}, Value: converted, Type: "custom", HasType: true, Line: 3},
		{Path: []string{"A", "Untyped"}, RelativePath: []string{"Untyped"}, Value: "001", Line: 4},
		{Path: []string{"A", "Text"}, RelativePath: []string{"Text"}, Value: "text\n", Type: "text", HasType: true, Line: 5},
		{Path: []string{"A", "Unset"}, RelativePath: []string{"Unset"}, Value: "", Type: "text", HasType: true, Unset: true, Line: 6},
	}
	for range 2 {
		got, err := cfg.SelectPrefix([]string{"A"})
		if err != nil || !reflect.DeepEqual(got, want) || calls != 1 {
			t.Fatalf("selection = %#v, %v; conversions = %d", got, err, calls)
		}
		stored, err := cfg.Get(got[0].Path, "custom")
		if err != nil || got[0].Value != stored || stored != converted {
			t.Fatalf("custom value ownership differs from Get: selected = %#v, Get = %#v, error = %v", got[0].Value, stored, err)
		}
	}
	if _, err := cfg.Get([]string{"A", "Untyped"}, "number"); err != nil || calls != 2 {
		t.Fatalf("on-demand conversion failed: %v; conversions = %d", err, calls)
	}
	got, err := cfg.SelectPrefix([]string{"A"})
	if err != nil || !reflect.DeepEqual(got, want) || calls != 2 {
		t.Fatalf("selection after Get = %#v, %v; conversions = %d", got, err, calls)
	}
}

func TestSelectPrefixReturnsIndependentPaths(t *testing.T) {
	inputPath := []string{"A", "B", "Child", "Leaf"}
	entries := []Entry{
		{Kind: Value, Path: inputPath, Raw: "first"},
		{Kind: Value, Path: inputPath, Raw: "second"},
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		t.Fatal(err)
	}
	inputPath[0] = "changed input"
	inputPath[2] = "changed child"
	prefix := []string{"A", "B"}
	first, err := cfg.SelectPrefix(prefix)
	if err != nil || len(first) != 2 {
		t.Fatalf("selection after mutating Build input = %#v, %v", first, err)
	}
	prefix[0] = "changed prefix"
	full := []string{"A", "B", "Child", "Leaf"}
	relative := []string{"Child", "Leaf"}
	for _, item := range first {
		if !reflect.DeepEqual(item.Path, full) || !reflect.DeepEqual(item.RelativePath, relative) {
			t.Fatalf("caller input altered selected paths: %#v", first)
		}
	}
	first[0].Path[2] = "changed full path"
	if !reflect.DeepEqual(first[0].RelativePath, relative) || !reflect.DeepEqual(first[1].Path, full) {
		t.Fatalf("full path aliases another selected path: %#v", first)
	}
	first[0].RelativePath[1] = "changed relative path"
	if first[0].Path[3] != "Leaf" || !reflect.DeepEqual(first[1].RelativePath, relative) {
		t.Fatalf("relative path aliases another selected path: %#v", first)
	}
	first[1].Path[0] = "changed other occurrence"
	first[1].RelativePath[0] = "changed other relative path"
	second, err := cfg.SelectPrefix([]string{"A", "B"})
	if err != nil || len(second) != 2 {
		t.Fatalf("selection after mutating results = %#v, %v", second, err)
	}
	for _, item := range second {
		if !reflect.DeepEqual(item.Path, full) || !reflect.DeepEqual(item.RelativePath, relative) {
			t.Fatalf("result mutation altered Config or another call: %#v", second)
		}
	}
	second[0].Path[3] = "changed second call"
	second[0].RelativePath[0] = "changed second call relative"
	if first[0].Path[3] != "Leaf" || first[0].RelativePath[0] != "Child" {
		t.Fatalf("separate selection calls share path storage: first = %#v, second = %#v", first, second)
	}
	value, err := cfg.Get(full, "string")
	assertValueCode(t, err, ErrMultipleValues)
	if value != nil {
		t.Fatalf("mutated selection changed exact-key multiplicity: %#v", value)
	}
	values, err := cfg.GetAll(full, "string")
	if err != nil || !reflect.DeepEqual(values, []any{"first", "second"}) {
		t.Fatalf("mutated selection changed exact-key values: %#v, %v", values, err)
	}
	all, err := cfg.SelectPrefix(nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("root selection = %#v, %v", all, err)
	}
	all[0].Path[0] = "changed root path"
	if !reflect.DeepEqual(all[0].RelativePath, full) || !reflect.DeepEqual(all[1].Path, full) {
		t.Fatalf("root selection paths share storage: %#v", all)
	}
	all[0].RelativePath[1] = "changed root relative path"
	if all[0].Path[1] != "B" || !reflect.DeepEqual(all[1].RelativePath, full) {
		t.Fatalf("root selection relative paths share storage: %#v", all)
	}
}
