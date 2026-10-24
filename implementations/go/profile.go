package dotreg

import (
	"errors"
	"strconv"
)

// Converter interprets the already decoded string and its unset state. Its
// errors are replaced with a diagnostic that does not include the source value.
type Converter func(Text) (any, error)

// Profile supplies application-defined type names and conversions. StringType
// names the string type, whose conversion is always the decoded string itself.
// Types contains only other types. Type identity is exact string equality in
// this sample; aliases and compound expressions are not inferred.
type Profile struct {
	StringType string
	Types      map[string]Converter
}

// DemoProfile is an illustrative profile, not the notation's standard type set.
// int accepts signed decimal digits and returns int64. bool accepts exactly
// true or false. Both reject unset values. No environment expansion occurs.
func DemoProfile() Profile {
	return Profile{
		StringType: "string",
		Types: map[string]Converter{
			"int": func(text Text) (any, error) {
				if text.Unset || !decimal(text.Value) {
					return nil, errConversion
				}
				return strconv.ParseInt(text.Value, 10, 64)
			},
			"bool": func(text Text) (any, error) {
				if !text.Unset {
					switch text.Value {
					case "true":
						return true, nil
					case "false":
						return false, nil
					}
				}
				return nil, errConversion
			},
		},
	}
}

// Config retains every occurrence in source order, including equal values,
// with its unset state and optional declared type. It is immutable after Build;
// custom converted values are owned by the application and should themselves be
// treated as immutable.
type Config struct {
	profile Profile
	values  map[string]settingGroup
	order   []settingRef
}

// settingGroup indexes one key's occurrences within the source-order array.
// Repeated values grow a compact index slice, without copying stored settings.
type settingGroup struct {
	first int
	more  []int
}

type setting struct {
	line     int
	text     Text
	typeName string
	hasType  bool
	value    any
}

// Build decodes all value entries, applying only their explicitly declared
// types. Untyped values stay strings until retrieval requests a conversion.
// Repeated keys append independent occurrences without deduplication or
// overwriting. Heading entries do not create settings. Failure returns no
// partial Config.
func Build(entries []Entry, profile Profile) (*Config, error) {
	if _, exists := profile.Types[profile.StringType]; profile.StringType == "" || exists {
		return nil, problem(0, ErrInput)
	}
	copyProfile := Profile{StringType: profile.StringType, Types: make(map[string]Converter, len(profile.Types))}
	for name, converter := range profile.Types {
		if name == "" || converter == nil {
			return nil, problem(0, ErrInput)
		}
		copyProfile.Types[name] = converter
	}
	// Reserve source-order storage only for values. This pass does not
	// decode or validate entries, so conversion and failure order stay intact.
	valueCount := 0
	for i := range entries {
		if entries[i].Kind == Value {
			valueCount++
		}
	}
	cfg := &Config{
		profile: copyProfile,
		values:  make(map[string]settingGroup),
		order:   make([]settingRef, 0, valueCount),
	}
	for _, entry := range entries {
		if entry.Kind == Heading {
			continue
		}
		if entry.Kind != Value || len(entry.Path) == 0 {
			return nil, problem(entry.Line, ErrInput)
		}
		key := pathID(entry.Path)
		text, err := DecodeString(entry)
		if err != nil {
			return nil, err
		}
		item := setting{line: entry.Line, text: text, typeName: entry.Type, hasType: entry.HasType}
		if entry.HasType {
			item.value, err = cfg.convert(text, entry.Type)
			if err != nil {
				return nil, problem(entry.Line, ErrTypeConversion)
			}
		}
		index := len(cfg.order)
		if group, exists := cfg.values[key]; exists {
			// Retain the first occurrence's immutable ID rather than another
			// allocation for every repetition of the same decoded key.
			key = cfg.order[group.first].pathID
			group.more = append(group.more, index)
			cfg.values[key] = group
		} else {
			cfg.values[key] = settingGroup{first: index}
		}
		cfg.order = append(cfg.order, settingRef{pathID: key, item: item})
	}
	return cfg, nil
}

// Get requests a particular profile type for a key with exactly one occurrence.
// Multiple occurrences return ErrMultipleValues at the second header, before
// any retrieval conversion. Use GetAll to retrieve repeated values.
func (cfg *Config) Get(path []string, want string) (any, error) {
	group, ok := cfg.lookup(path)
	if !ok {
		return nil, problem(0, ErrMissing)
	}
	if len(group.more) > 0 {
		return nil, problem(cfg.order[group.more[0]].item.line, ErrMultipleValues)
	}
	return cfg.get(cfg.order[group.first].item, want)
}

// GetAll retrieves every occurrence in source order. Each declaration must
// match want; untyped occurrences convert on demand without caching. A failure
// reports that occurrence's header and returns no partial values.
func (cfg *Config) GetAll(path []string, want string) ([]any, error) {
	group, ok := cfg.lookup(path)
	if !ok {
		return nil, problem(0, ErrMissing)
	}
	values := make([]any, 1+len(group.more))
	for i := range values {
		index := group.first
		if i > 0 {
			index = group.more[i-1]
		}
		value, err := cfg.get(cfg.order[index].item, want)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

// GetAt retrieves one zero-based occurrence using that occurrence's declaration
// and conversion rules. Missing keys and out-of-range indices return ErrMissing.
func (cfg *Config) GetAt(path []string, index int, want string) (any, error) {
	group, ok := cfg.lookup(path)
	if !ok || index < 0 || index > len(group.more) {
		return nil, problem(0, ErrMissing)
	}
	if index == 0 {
		return cfg.get(cfg.order[group.first].item, want)
	}
	return cfg.get(cfg.order[group.more[index-1]].item, want)
}

// A declaration is checked before conversion or unset-to-empty interpretation.
// Retrieval conversion never changes the stored string or declaration.
func (cfg *Config) get(item setting, want string) (any, error) {
	if item.hasType {
		if item.typeName != want {
			return nil, problem(item.line, ErrTypeMismatch)
		}
		return item.value, nil
	}
	value, err := cfg.convert(item.text, want)
	if err != nil {
		return nil, problem(item.line, ErrTypeMismatch)
	}
	return value, nil
}

// IsUnset reports the state for a key with exactly one occurrence. It does not
// request or convert a type. Repeated keys return ErrMultipleValues.
func (cfg *Config) IsUnset(path []string) (bool, error) {
	group, ok := cfg.lookup(path)
	if !ok {
		return false, problem(0, ErrMissing)
	}
	if len(group.more) > 0 {
		return false, problem(cfg.order[group.more[0]].item.line, ErrMultipleValues)
	}
	return cfg.order[group.first].item.text.Unset, nil
}

// IsUnsetAll returns each occurrence's original unset state in source order.
// The returned slice is independent of the Config and of other calls.
func (cfg *Config) IsUnsetAll(path []string) ([]bool, error) {
	group, ok := cfg.lookup(path)
	if !ok {
		return nil, problem(0, ErrMissing)
	}
	states := make([]bool, 1+len(group.more))
	states[0] = cfg.order[group.first].item.text.Unset
	for i, index := range group.more {
		states[i+1] = cfg.order[index].item.text.Unset
	}
	return states, nil
}

func (cfg *Config) lookup(path []string) (settingGroup, bool) {
	if cfg == nil || len(path) == 0 {
		return settingGroup{}, false
	}
	key := pathID(path)
	group, ok := cfg.values[key]
	return group, ok
}

func (cfg *Config) convert(text Text, want string) (any, error) {
	if want == cfg.profile.StringType {
		return text.Value, nil
	}
	converter, exists := cfg.profile.Types[want]
	if !exists {
		return nil, errConversion
	}
	return converter(text)
}

// decimal deliberately excludes hexadecimal, underscores, and other Go syntax.
func decimal(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '+' || value[0] == '-' {
		value = value[1:]
	}
	if value == "" {
		return false
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

// The underlying converter error is never included in public diagnostics.
var errConversion = errors.New("value cannot be converted")
