package dotreg

import "strings"

// SelectedValue is one stored value occurrence selected by a key prefix. Path
// is the complete decoded key, and RelativePath excludes the requested prefix.
// RelativePath is a non-nil empty slice for the prefix's own value; an empty
// string element instead denotes a child with an empty name.
// Each path slice is independent of the Config, other results, and other calls.
// Value is the already converted declared value, or the decoded string when no
// type was declared. Custom converted values retain the ownership rules of
// Build and are not cloned.
type SelectedValue struct {
	Path         []string
	RelativePath []string
	Value        any
	Type         string
	HasType      bool
	Unset        bool
	Line         int
}

// settingRef owns one setting in source order. All equal IDs share the first
// occurrence's immutable storage, without retaining decoded path slices.
type settingRef struct {
	pathID string
	item   setting
}

// SelectPrefix returns values at prefix and all its descendants in source
// occurrence order, preserving repeated keys and identical values. Matching
// compares exact decoded key elements; neither wildcards nor case or Unicode
// folding apply.
// A nil or empty prefix selects all value occurrences. An empty string element
// is an actual key name and is distinct from an empty prefix.
//
// Selection does not request a type or invoke converters. It can therefore
// contain different declared types. Headings do not produce selected values.
// No matches return a non-nil empty slice. A nil Config returns ErrMissing.
func (cfg *Config) SelectPrefix(prefix []string) ([]SelectedValue, error) {
	if cfg == nil {
		return nil, problem(0, ErrMissing)
	}
	selected := make([]SelectedValue, 0)
	prefixID := pathID(prefix)
	order := cfg.order
	for i := range order {
		ref := &order[i]
		// The length-prefixed encoding ensures that a matching prefix ends at
		// an element boundary, including when element names contain dots.
		if !strings.HasPrefix(ref.pathID, prefixID) {
			continue
		}
		item := ref.item
		path := pathFromID(ref.pathID)
		relative := make([]string, len(path)-len(prefix))
		copy(relative, path[len(prefix):])
		value := item.value
		if !item.hasType {
			value = item.text.Value
		}
		selected = append(selected, SelectedValue{
			Path: path, RelativePath: relative, Value: value,
			Type: item.typeName, HasType: item.hasType,
			Unset: item.text.Unset, Line: item.line,
		})
	}
	return selected, nil
}

// pathFromID reverses an internal pathID. IDs are produced only by pathID, so
// their lengths and delimiters are already valid. Element strings share the
// immutable ID storage; the returned slice is newly allocated on every call.
func pathFromID(id string) []string {
	path := make([]string, 0)
	for offset := 0; offset < len(id); {
		length := 0
		for id[offset] != ':' {
			length = length*10 + int(id[offset]-'0')
			offset++
		}
		offset++
		path = append(path, id[offset:offset+length])
		offset += length
	}
	return path
}
