package dotreg

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

var benchmarkSelectedValues []SelectedValue

type prefixBenchmarkFixture struct {
	source string
	want   []SelectedValue
}

// Keep a fixed key set as the occurrence count grows. This measures the
// consumer's repeated-value storage and global order, rather than only growth
// in distinct keys. Values deliberately interleave unrelated branches, the
// prefix's own values, several declared types, and quoted element boundaries.
func makePrefixBenchmarkFixture(count int) prefixBenchmarkFixture {
	patterns := []struct {
		source   string
		path     []string
		value    any
		typeName string
		unset    bool
	}{
		{"Root.Target:base", []string{"Root", "Target"}, "base", "", false},
		{"Other.Value:outside", []string{"Other", "Value"}, "outside", "", false},
		{"Root.Target.Item(int):001", []string{"Root", "Target", "Item"}, int64(1), "int", false},
		{`Root."Target.Branch".Item:quoted boundary`, []string{"Root", "Target.Branch", "Item"}, "quoted boundary", "", false},
		{`Root.Target.Item(string):"001"`, []string{"Root", "Target", "Item"}, "001", "string", false},
		{`Root.Target.Empty:""`, []string{"Root", "Target", "Empty"}, "", "", false},
		{`Root."":empty-name base`, []string{"Root", ""}, "empty-name base", "", false},
		{"Root.Target.Missing:", []string{"Root", "Target", "Missing"}, "", "", true},
		{"Root.Target.Deep.Leaf(bool):true", []string{"Root", "Target", "Deep", "Leaf"}, true, "bool", false},
		{"Root.Target.Item:plain", []string{"Root", "Target", "Item"}, "plain", "", false},
		{"Root.Targeting.Item:longer element", []string{"Root", "Targeting", "Item"}, "longer element", "", false},
		{`Root.Target."":empty-name child`, []string{"Root", "Target", ""}, "empty-name child", "", false},
		{"Root.Target.Branch.Leaf:ordinary boundary", []string{"Root", "Target", "Branch", "Leaf"}, "ordinary boundary", "", false},
		{`Root.Target(string):"typed base"`, []string{"Root", "Target"}, "typed base", "string", false},
		{"Other.Value:outside", []string{"Other", "Value"}, "outside", "", false},
		{`Root."".Child:empty-name descendant`, []string{"Root", "", "Child"}, "empty-name descendant", "", false},
	}
	var source strings.Builder
	source.WriteString("@dotReg 1\n\nRoot.Target.\n")
	want := make([]SelectedValue, 0, count)
	for i := 0; i < count; i++ {
		pattern := patterns[i%len(patterns)]
		source.WriteString(pattern.source)
		source.WriteByte('\n')
		path := append([]string(nil), pattern.path...)
		want = append(want, SelectedValue{
			Path: path, RelativePath: append([]string(nil), path...),
			Value: pattern.value, Type: pattern.typeName, HasType: pattern.typeName != "",
			Unset: pattern.unset, Line: i + 4,
		})
	}
	return prefixBenchmarkFixture{source: source.String(), want: want}
}

// The oracle works directly on decoded element slices and fixture literals;
// it does not inspect Config internals, pathID, or a parser-generated result.
func (fixture prefixBenchmarkFixture) expected(prefix []string) []SelectedValue {
	want := make([]SelectedValue, 0)
	for _, item := range fixture.want {
		if len(item.Path) < len(prefix) {
			continue
		}
		match := true
		for i, element := range prefix {
			if item.Path[i] != element {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		item.RelativePath = make([]string, len(item.Path)-len(prefix))
		copy(item.RelativePath, item.Path[len(prefix):])
		want = append(want, item)
	}
	return want
}

func buildPrefixBenchmarkConfig(b testing.TB, fixture prefixBenchmarkFixture) *Config {
	b.Helper()
	entries, err := Parse(strings.NewReader(fixture.source))
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := Build(entries, DemoProfile())
	if err != nil {
		b.Fatal(err)
	}
	got, err := cfg.SelectPrefix(nil)
	if err != nil || !reflect.DeepEqual(got, fixture.want) {
		b.Fatalf("ordered consumer fixture differs from its independent expected values: %v", err)
	}
	return cfg
}

// Unlike BenchmarkParseBuild512's distinct keys, these loads retain repeated
// values and the occurrence-order index used by the current retrieval contract.
func BenchmarkParseBuildOrdered(b *testing.B) {
	for _, count := range []int{512, 8192} {
		fixture := makePrefixBenchmarkFixture(count)
		b.Run(fmt.Sprintf("Occurrences%d", count), func(b *testing.B) {
			buildPrefixBenchmarkConfig(b, fixture)
			profile := DemoProfile()
			b.ReportAllocs()
			b.SetBytes(int64(len(fixture.source)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries, err := Parse(strings.NewReader(fixture.source))
				if err != nil {
					b.Fatal(err)
				}
				benchmarkConfig, err = Build(entries, profile)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(count), "values/op")
		})
	}
}

// Selection is measured on an already built Config. No input-byte throughput
// is reported because this operation does not parse or read the source text.
func BenchmarkSelectPrefix(b *testing.B) {
	conditions := []struct {
		name   string
		prefix []string
	}{
		{"All", nil},
		{"EmptyPrefix", []string{}},
		{"BaseAndDescendants", []string{"Root", "Target"}},
		{"SelectiveDescendant", []string{"Root", "Target", "Deep"}},
		{"NoMatch", []string{"Root", "Missing"}},
		{"QuotedElementBoundary", []string{"Root", "Target.Branch"}},
		{"EmptyName", []string{"Root", ""}},
	}
	for _, count := range []int{512, 8192} {
		fixture := makePrefixBenchmarkFixture(count)
		cfg := buildPrefixBenchmarkConfig(b, fixture)
		for _, condition := range conditions {
			want := fixture.expected(condition.prefix)
			b.Run(fmt.Sprintf("Occurrences%d/%s", count, condition.name), func(b *testing.B) {
				got, err := cfg.SelectPrefix(condition.prefix)
				if err != nil || !reflect.DeepEqual(got, want) {
					b.Fatalf("selection differs from its independent expected values: %v", err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					benchmarkSelectedValues, err = cfg.SelectPrefix(condition.prefix)
					if err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				b.ReportMetric(float64(count), "stored-values/op")
				b.ReportMetric(float64(len(want)), "selected-values/op")
			})
		}
	}
	b.Run("EmptyConfig", func(b *testing.B) {
		cfg, err := Build(nil, DemoProfile())
		if err != nil {
			b.Fatal(err)
		}
		got, err := cfg.SelectPrefix(nil)
		if err != nil || got == nil || len(got) != 0 {
			b.Fatalf("empty Config selection = %#v, %v; want non-nil empty result", got, err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkSelectedValues, err = cfg.SelectPrefix(nil)
			if err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		b.ReportMetric(0, "stored-values/op")
		b.ReportMetric(0, "selected-values/op")
	})
}
